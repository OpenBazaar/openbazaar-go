package net

import (
	"time"
	"golang.org/x/net/context"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/ipfs/go-ipfs/commands"
	multihash "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	routing "github.com/ipfs/go-ipfs/routing/dht"
	"gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer/addr"
)

type MessageRetriever struct {
	db        repo.Datastore
	dht       *routing.IpfsDHT
	ctx       commands.Context
	peerID    peer.ID
	prefixLen int
}

func NewMessageRetriever(db repo.Datastore, ctx commands.Context, dht *routing.IpfsDHT, peerId peer.ID, prefixLen int) *MessageRetriever {
	return &MessageRetriever{
		db: db,
		dht: dht,
		ctx: ctx,
		peerID: peerId,
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

	mh, _ := multihash.FromB58String(m.peerID.Pretty())

	peerOut := ipfs.FindPointersAsync(m.dht, ctx, mh, m.prefixLen)
	for {
		var p peer.PeerInfo
		select {
		case p <- peerOut:
			if !m.db.OfflineMessages().Exists(p.Addrs[0].String()) {
				if p.Addrs[0].Protocols()[0].Code == 421 {
					fetchIPFS(m.ctx, p.Addrs[0])
				}
				m.db.OfflineMessages().Put(p.Addrs[0].String())
			}
		case <-ctx.Done():
			return
		}
	}
}

func fetchIPFS(ctx commands.Context, addr ma.Multiaddr) {
	ciphertext, err := ipfs.Cat(ctx, addr.String())
	if err != nil {
		return
	}
	attemptDecrypt(ciphertext)
}

func attemptDecrypt(ciphertext []byte) {

}
