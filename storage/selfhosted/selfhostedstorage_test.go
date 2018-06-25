package selfhosted

import (
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"os"
	"testing"
)

func TestSelfHostedStorage_Store(t *testing.T) {
	ctx, err := ipfs.MockCmdsCtx()
	if err != nil {
		t.Error(err)
	}
	err = os.Mkdir("./outbox", os.ModePerm)
	if err != nil {
		t.Error(err)
	}
	storage := NewSelfHostedStorage("./", ctx, []peer.ID{}, func(peerID string, cids []cid.Cid) error { return nil })
	pid, err := peer.IDB58Decode("QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ")
	if err != nil {
		t.Error(err)
	}
	ma, err := storage.Store(pid, []byte("hello world"))
	if err != nil {
		t.Error(err)
	}
	if ma.String() != "/ipfs/zb2rhj7crUKTQYRGCRATFaQ6YFLTde2YzdqbbhAASkL9uRDXn" {
		t.Error("Self-hosted storage returned incorrect multiaddr")
	}
	os.RemoveAll("./outbox")
}
