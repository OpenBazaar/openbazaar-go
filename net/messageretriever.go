package net

import (
	"time"
	"golang.org/x/net/context"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/golang/protobuf/proto"
	"github.com/ipfs/go-ipfs/core"
	"github.com/OpenBazaar/openbazaar-go/pb"
	multihash "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	routing "github.com/ipfs/go-ipfs/routing/dht"
	"github.com/OpenBazaar/openbazaar-go/net/service"
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

func (m *MessageRetriever) Run(){
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	m.fetchPointers()
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
	for {
		select {
		case  p:= <- peerOut:
			if len(p.Addrs) > 0 && !m.db.OfflineMessages().Exists(p.Addrs[0].String()) {
				if p.Addrs[0].Protocols()[0].Code == 421 {
					m.fetchIPFS(m.ctx, p.Addrs[0])
				}
				m.db.OfflineMessages().Put(p.Addrs[0].String())
			}
		case <-ctx.Done():
			return
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
