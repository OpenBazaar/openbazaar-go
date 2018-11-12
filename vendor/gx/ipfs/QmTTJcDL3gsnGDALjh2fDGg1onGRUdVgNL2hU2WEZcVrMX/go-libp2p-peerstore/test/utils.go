package test

import (
	"context"
	"fmt"
	"testing"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	pt "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer/test"
)

func multiaddr(m string) ma.Multiaddr {
	maddr, err := ma.NewMultiaddr(m)
	if err != nil {
		panic(err)
	}
	return maddr
}

type peerpair struct {
	ID   peer.ID
	Addr []ma.Multiaddr
}

func randomPeer(b *testing.B, addrCount int) *peerpair {
	var (
		pid   peer.ID
		err   error
		addrs = make([]ma.Multiaddr, addrCount)
		aFmt  = "/ip4/127.0.0.1/tcp/%d/ipfs/%s"
	)

	b.Helper()
	if pid, err = pt.RandPeerID(); err != nil {
		b.Fatal(err)
	}

	for i := 0; i < addrCount; i++ {
		if addrs[i], err = ma.NewMultiaddr(fmt.Sprintf(aFmt, i, pid.Pretty())); err != nil {
			b.Fatal(err)
		}
	}
	return &peerpair{pid, addrs}
}

func addressProducer(ctx context.Context, b *testing.B, addrs chan *peerpair, addrsPerPeer int) {
	b.Helper()
	defer close(addrs)
	for {
		p := randomPeer(b, addrsPerPeer)
		select {
		case addrs <- p:
		case <-ctx.Done():
			return
		}
	}
}
