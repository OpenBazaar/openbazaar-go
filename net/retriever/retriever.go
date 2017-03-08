package net

import (
	"context"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/golang/protobuf/proto"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	routing "github.com/ipfs/go-ipfs/routing/dht"
	"github.com/op/go-logging"
	"golang.org/x/net/proxy"
	ma "gx/ipfs/QmUAQaWbKxGCUTuoQVvvicbQNZ9APF5pDGWyAZSe93AtKH/go-multiaddr"
	multihash "gx/ipfs/QmYDds3421prZgqKbLpEK7T9Aa2eVdQ7o3YarX1LVLdP2J/go-multihash"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
	libp2p "gx/ipfs/QmfWDLQjGjVe4fr5CoztYW2DYYjRysMJrFe1RCsXLPTf46/go-libp2p-crypto"
	"io/ioutil"
	gonet "net"
	"net/http"
	"sync"
	"time"
)

var log = logging.MustGetLogger("retriever")

type MessageRetriever struct {
	db           repo.Datastore
	node         *core.IpfsNode
	bm           *net.BanManager
	ctx          commands.Context
	service      net.NetworkService
	prefixLen    int
	sendAck      func(peerId string, pointerID peer.ID) error
	messageQueue []pb.Envelope
	httpClient   *http.Client
	queueLock    *sync.Mutex
	*sync.WaitGroup
}

func NewMessageRetriever(db repo.Datastore, ctx commands.Context, node *core.IpfsNode, bm *net.BanManager, service net.NetworkService, prefixLen int, dialer proxy.Dialer, sendAck func(peerId string, pointerID peer.ID) error) *MessageRetriever {
	dial := gonet.Dial
	if dialer != nil {
		dial = dialer.Dial
	}
	tbTransport := &http.Transport{Dial: dial}
	client := &http.Client{Transport: tbTransport, Timeout: time.Second * 10}
	mr := MessageRetriever{db, node, bm, ctx, service, prefixLen, sendAck, nil, client, new(sync.Mutex), new(sync.WaitGroup)}
	// Add one for initial wait at start up
	mr.Add(1)
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
	downloaded := 0
	mh, _ := multihash.FromB58String(m.node.Identity.Pretty())
	peerOut := ipfs.FindPointersAsync(m.node.Routing.(*routing.IpfsDHT), ctx, mh, m.prefixLen)

	// Iterate over the pointers, adding 1 to the waitgroup for each pointer found
	for p := range peerOut {
		if len(p.Addrs) > 0 && !m.db.OfflineMessages().Has(p.Addrs[0].String()) {
			// IPFS
			if len(p.Addrs[0].Protocols()) == 1 && p.Addrs[0].Protocols()[0].Code == ma.P_IPFS {
				wg.Add(1)
				downloaded++
				go m.fetchIPFS(p.ID, m.ctx, p.Addrs[0], wg)
			}

			// HTTPS
			if len(p.Addrs[0].Protocols()) == 2 && p.Addrs[0].Protocols()[0].Code == ma.P_IPFS && p.Addrs[0].Protocols()[1].Code == ma.P_HTTPS {
				enc, err := p.Addrs[0].ValueForProtocol(ma.P_IPFS)
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
				downloaded++
				go m.fetchHTTPS(p.ID, string(d.Digest), p.Addrs[0], wg)
			}
		}
	}
	// We have finished fetching pointers from the DHT
	wg.Done()

	// Wait for each goroutine to finish then process any remaining messages that needed to be processed last
	wg.Wait()
	for _, env := range m.messageQueue {
		m.handleMessage(env, nil)
	}
	m.messageQueue = []pb.Envelope{}

	// For initial start up only
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
	resp, err := m.httpClient.Get(url)
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
	// Decrypt and unmarshal plaintext
	plaintext, err := net.Decrypt(m.node.PrivateKey, ciphertext)
	if err != nil {
		return
	}

	// Unmarshal plaintext
	env := pb.Envelope{}
	err = proto.Unmarshal(plaintext, &env)
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

	if m.bm.IsBanned(id) {
		return
	}

	// Respond with an ACK
	if env.Message.MessageType != pb.Message_OFFLINE_ACK {
		m.sendAck(id.Pretty(), pid)
	}

	/* Order messages need to be processed in the correct order, so cancel messages
	   need to be processed last. */
	if env.Message.MessageType == pb.Message_ORDER_CANCEL {
		m.queueLock.Lock()
		m.messageQueue = append(m.messageQueue, env)
		m.queueLock.Unlock()
		return
	}

	m.handleMessage(env, &id)
}

func (m *MessageRetriever) handleMessage(env pb.Envelope, id *peer.ID) {
	if id == nil {
		// Get the peer ID from the public key
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

	// Get handler for this message type
	handler := m.service.HandlerForMsgType(env.Message.MessageType)
	if handler == nil {
		log.Debug("Got back nil handler from HandlerForMsgType")
		return
	}

	// Dispatch handler
	_, err := handler(*id, env.Message, true)
	if err != nil {
		log.Errorf("Handle message error: %s", err)
		return
	}
}
