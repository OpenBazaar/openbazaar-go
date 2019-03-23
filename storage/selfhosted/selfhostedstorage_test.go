package selfhosted

import (
	"gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	"gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	"os"
	"testing"

	"github.com/ipfs/go-ipfs/core/mock"
)

func TestSelfHostedStorage_Store(t *testing.T) {
	ctx, err := coremock.NewMockNode()
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
	if ma.String() != "/ipfs/Qmf412jQZiuVUtdgnB36FXFX7xg5V6KEbSJ4dpQuhkLyfD" {
		t.Error("Self-hosted storage returned incorrect multiaddr")
	}
	os.RemoveAll("./outbox")
}
