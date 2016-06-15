package net

import (
	multihash "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	"gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net/service"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/golang/protobuf/proto"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	routing "github.com/ipfs/go-ipfs/routing/dht"
	"golang.org/x/net/context"
)

type MessageRetriever struct {
	db        repo.Datastore
	node      *core.IpfsNode
	ctx       commands.Context
	service   *service.OpenBazaarService
	prefixLen int
}

func NewMessageRetriever(db repo.Datastore, ctx commands.Context, node *core.IpfsNode, service *service.OpenBazaarService, prefixLen int) *MessageRetriever {
	return &MessageRetriever{
		db:        db,
		node:      node,
		ctx:       ctx,
		service:   service,
		prefixLen: prefixLen,
	}
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
	mh, _ := multihash.FromB58String(m.node.Identity.Pretty())

	peerOut := ipfs.FindPointersAsync(m.node.Routing.(*routing.IpfsDHT), ctx, mh, m.prefixLen)
	for p := range peerOut {
		if len(p.Addrs) > 0 && !m.db.OfflineMessages().Exists(p.Addrs[0].String()) {
			// ipfs
			if len(p.Addrs[0].Protocols()) == 1 && p.Addrs[0].Protocols()[0].Code == 421 {
				go m.fetchIPFS(m.ctx, p.Addrs[0])
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
				go m.fetchHTTPS(string(d.Digest))
			}
			m.db.OfflineMessages().Put(p.Addrs[0].String())
		}
	}
}

func (m *MessageRetriever) fetchIPFS(ctx commands.Context, addr ma.Multiaddr) {
	ciphertext, err := ipfs.Cat(ctx, addr.String())
	if err != nil {
		return
	}
	m.attemptDecrypt(ciphertext)
}

func (m *MessageRetriever) fetchHTTPS(url string) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	ciphertext, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	m.attemptDecrypt(ciphertext)
}

func (m *MessageRetriever) attemptDecrypt(ciphertext []byte) {
	plaintext, err := m.node.PrivateKey.Decrypt(ciphertext)
	if err == nil {
		env := pb.Envelope{}
		proto.Unmarshal(plaintext, &env)
		id, err := peer.IDB58Decode(env.PeerID)
		if err != nil {
			return
		}
		// get handler for this msg type.
		handler := m.service.HandlerForMsgType(env.Message.MessageType)
		if handler == nil {
			log.Debug("Got back nil handler from handlerForMsgType")
			return
		}

		// dispatch handler.
		_, err = handler(id, env.Message)
		if err != nil {
			log.Debugf("handle message error: %s", err)
			return
		}
	}
}
