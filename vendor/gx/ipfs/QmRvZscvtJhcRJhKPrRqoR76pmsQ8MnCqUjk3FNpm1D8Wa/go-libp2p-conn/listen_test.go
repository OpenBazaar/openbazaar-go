package conn

import (
	"context"
	"testing"
	"time"

	grc "gx/ipfs/QmTd4Jgb4nbJq5uR55KJgGLyHWmM3dovS21D1HcwRneSLu/gorocheck"
	tu "gx/ipfs/QmVvkK7s5imCiq3JVbL3pGfnhcCnf3LrFJPF4GE2sAoGZf/go-testutil"
)

func TestAcceptLeak(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p1 := tu.RandPeerNetParamsOrFatal(t)

	l1, err := Listen(ctx, p1.Addr, p1.ID, p1.PrivKey)
	if err != nil {
		t.Fatal(err)
	}
	p1.Addr = l1.Multiaddr() // Addr has been determined by kernel.

	for i := 0; i < connAcceptBuffer+1; i++ {
		// Dial a full valid connection, but never Accept it, and cancel instead
		p := tu.RandPeerNetParamsOrFatal(t)
		d := NewDialer(p.ID, p.PrivKey, nil)
		if _, err := d.Dial(ctx, l1.Multiaddr(), p1.ID); err != nil {
			t.Fatal("dial failed: ", err)
		}
	}

	cancel()
	time.Sleep(time.Millisecond * 100)

	err = grc.CheckForLeaks(goroFilter)
	if err != nil {
		t.Fatal(err)
	}
}
