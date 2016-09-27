package net

import (
	peer "gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
	libp2p "gx/ipfs/QmUWER4r4qMvaCnX5zREcfyiWN7cXN9g3a7fkRqNz8qWPP/go-libp2p-crypto"
	multihash "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/golang/protobuf/proto"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	routing "github.com/ipfs/go-ipfs/routing/dht"
	"github.com/op/go-logging"
	"golang.org/x/net/context"
)

var log = logging.MustGetLogger("retriever")

type MessageRetriever struct {
	db           repo.Datastore
	node         *core.IpfsNode
	ctx          commands.Context
	service      net.NetworkService
	prefixLen    int
	sendAck      func(peerId string, pointerID peer.ID) error
	messageQueue chan pb.Envelope
	sync.WaitGroup
}

func NewMessageRetriever(db repo.Datastore, ctx commands.Context, node *core.IpfsNode, service net.NetworkService, prefixLen int, sendAck func(peerId string, pointerID peer.ID) error) *MessageRetriever {
	mr := MessageRetriever{db, node, ctx, service, prefixLen, sendAck, make(chan pb.Envelope, 128)}
	mr.Add(1) // Add one for initial wait at start up
	return &mr
}

func (m *MessageRetriever) Run() {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	go m.fetchPointers()
	for {
		select {
		case <-tick.C:
			go m.fetchPointers()
		}
	}
}

func (m *MessageRetriever) fetchPointers() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := new(sync.WaitGroup)
	wg.Add(1)
	mh, _ := multihash.FromB58String(m.node.Identity.Pretty())
	peerOut := ipfs.FindPointersAsync(m.node.Routing.(*routing.IpfsDHT), ctx, mh, m.prefixLen)

	// Iterate over the pointers. Add 1 to the waitgroup for each found pointer
	for p := range peerOut {
		if len(p.Addrs) > 0 && !m.db.OfflineMessages().Has(p.Addrs[0].String()) {
			// ipfs
			if len(p.Addrs[0].Protocols()) == 1 && p.Addrs[0].Protocols()[0].Code == 421 {
				wg.Add(1)
				go m.fetchIPFS(p.ID, m.ctx, p.Addrs[0], wg)
			}
			// https
			if len(p.Addrs[0].Protocols()) == 2 && p.Addrs[0].Protocols()[0].Code == 421 && p.Addrs[0].Protocols()[1].Code == 443 {
				enc, err := p.Addrs[0].ValueForProtocol(421)
				if err != nil {
					continue
				}
				mh, err := multihash.FromB58String(enc)
				if err != nil {
					continue
				}
				d, err := multihash.Decode(mh)
				if err != nil {
					continue
				}
				wg.Add(1)
				go m.fetchHTTPS(p.ID, string(d.Digest), p.Addrs[0], wg)
			}
		}
	}
	wg.Done() // We've finished fetching pointers from the dht

	// Wait for each goroutine to finish then process any remaining messages that needed
	// to be processed last
	wg.Wait()

DRAIN_LOOP:
	for {
		select {
		case env := <-m.messageQueue:
			m.handleMessage(env, nil)
		default:
			break DRAIN_LOOP
		}
	}

	// For initial start up. We can ignore afterwards
	if m.WaitGroup != nil {
		m.Done()
		m.WaitGroup = nil
	}
}

func (m *MessageRetriever) fetchIPFS(pid peer.ID, ctx commands.Context, addr ma.Multiaddr, wg *sync.WaitGroup) {
	defer wg.Done()
	ciphertext, err := ipfs.Cat(ctx, addr.String())
	if err != nil {
		log.Errorf("Error retrieving offline message: %s", err.Error())
		return
	}
	m.attemptDecrypt(ciphertext, pid)
	m.db.OfflineMessages().Put(addr.String())
}

func (m *MessageRetriever) fetchHTTPS(pid peer.ID, url string, addr ma.Multiaddr, wg *sync.WaitGroup) {
	defer wg.Done()
	resp, err := http.Get(url)
	if err != nil {
		log.Errorf("Error retrieving offline message: %s", err.Error())
		return
	}
	ciphertext, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Error retrieving offline message: %s", err.Error())
		return
	}
	m.attemptDecrypt(ciphertext, pid)
	m.db.OfflineMessages().Put(addr.String())
}

func (m *MessageRetriever) attemptDecrypt(ciphertext []byte, pid peer.ID) {
	plaintext, err := net.Decrypt(m.node.PrivateKey, ciphertext)

	if err == nil {

		// Unmarshal plaintext
		env := pb.Envelope{}
		err := proto.Unmarshal(plaintext, &env)
		if err != nil {
			return
		}

		// Validate the signature
		ser, err := proto.Marshal(env.Message)
		if err != nil {
			return
		}
		pubkey, err := libp2p.UnmarshalPublicKey(env.Pubkey)
		if err != nil {
			return
		}
		valid, err := pubkey.Verify(ser, env.Signature)
		if err != nil || !valid {
			return
		}

		id, err := peer.IDFromPublicKey(pubkey)
		if err != nil {
			return
		}

		// Respond with an ack
		if env.Message.MessageType != pb.Message_OFFLINE_ACK {
			m.sendAck(id.Pretty(), pid)
		}

		// Order messages need to be processed in the correct order, so cancel messages
		// need to be processed last.
		if env.Message.MessageType == pb.Message_ORDER_CANCEL {
			m.messageQueue <- env
			return
		}

		m.handleMessage(env, &id)
	}
}

func (m *MessageRetriever) handleMessage(env pb.Envelope, id *peer.ID) {
	if id == nil {
		pubkey, err := libp2p.UnmarshalPublicKey(env.Pubkey)
		if err != nil {
			return
		}
		i, err := peer.IDFromPublicKey(pubkey)
		if err != nil {
			return
		}
		id = &i
	}
	// get handler for this msg type.
	handler := m.service.HandlerForMsgType(env.Message.MessageType)
	if handler == nil {
		log.Debug("Got back nil handler from handlerForMsgType")
		return
	}

	// dispatch handler.
	_, err := handler(*id, env.Message)
	if err != nil {
		log.Debugf("handle message error: %s", err)
		return
	}
}
