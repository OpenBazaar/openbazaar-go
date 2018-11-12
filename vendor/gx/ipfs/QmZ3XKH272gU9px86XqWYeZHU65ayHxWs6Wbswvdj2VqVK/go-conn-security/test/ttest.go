package ss_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"math/rand"

	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	connsec "gx/ipfs/QmZ3XKH272gU9px86XqWYeZHU65ayHxWs6Wbswvdj2VqVK/go-conn-security"
)

var Subtests = map[string]func(t *testing.T, at, bt connsec.Transport, ap, bp peer.ID){
	"RW":                      SubtestRW,
	"Keys":                    SubtestKeys,
	"WrongPeer":               SubtestWrongPeer,
	"Stream":                  SubtestStream,
	"CancelHandshakeInbound":  SubtestCancelHandshakeInbound,
	"CancelHandshakeOutbound": SubtestCancelHandshakeOutbound,
}

var TestMessage = []byte("hello world!")
var TestStreamLen int64 = 1024 * 1024
var TestSeed int64 = 1812

func SubtestAll(t *testing.T, at, bt connsec.Transport, ap, bp peer.ID) {
	for n, f := range Subtests {
		t.Run(n, func(t *testing.T) {
			f(t, at, bt, ap, bp)
			f(t, bt, at, bp, ap)
		})
	}
}

func randStream() io.Reader {
	return &io.LimitedReader{
		R: rand.New(rand.NewSource(TestSeed)),
		N: TestStreamLen,
	}
}

func testWriteSustain(t *testing.T, c connsec.Conn) {
	source := randStream()
	n := int64(0)
	for {
		coppied, err := io.CopyN(c, source, int64(rand.Intn(8000)))
		n += coppied

		switch err {
		case io.EOF:
			if n != TestStreamLen {
				t.Fatal("incorrect random stream length")
			}
			return
		case nil:
		default:
			t.Fatal(err)
		}
	}
}

func testReadSustain(t *testing.T, c connsec.Conn) {
	expected := randStream()
	total := 0
	ebuf := make([]byte, 1024)
	abuf := make([]byte, 1024)
	for {
		n, err := c.Read(abuf)
		if err != nil {
			t.Fatal(err)
		}
		total += n
		_, err = io.ReadFull(expected, ebuf[:n])
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(abuf[:n], ebuf[:n]) {
			t.Fatal("bytes not equal")
		}
		if total == int(TestStreamLen) {
			return
		}
	}
}
func testWrite(t *testing.T, c connsec.Conn) {
	n, err := c.Write(TestMessage)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(TestMessage) {
		t.Errorf("wrote %d bytes, expected to write %d bytes", n, len(TestMessage))
	}
}

func testRead(t *testing.T, c connsec.Conn) {
	buf := make([]byte, 100)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(TestMessage) {
		t.Errorf("wrote %d bytes, expected to write %d bytes", n, len(TestMessage))
	}
	if !bytes.Equal(buf[:n], TestMessage) {
		t.Errorf("received bad test message: %s", string(buf[:n]))
	}
}

func testWriteFail(t *testing.T, c connsec.Conn) {
	n, err := c.Write(TestMessage)
	if n != 0 || err == nil {
		t.Error("shouldn't have been able to write to a closed conn")
	}
}

func testReadFail(t *testing.T, c connsec.Conn) {
	buf := make([]byte, len(TestMessage))
	n, err := c.Read(buf)
	if n != 0 || err == nil {
		t.Error("shouldn't have been able to write to a closed conn")
	}
}

func testEOF(t *testing.T, c connsec.Conn) {
	buf := make([]byte, 100)
	n, err := c.Read(buf)
	if n != 0 {
		t.Errorf("didn't expect to read any bytes, read: %d", n)
	}
	if err != io.EOF {
		t.Errorf("expected read to fail with EOF, got: %s", err)
	}
}

func SubtestRW(t *testing.T, at, bt connsec.Transport, ap, bp peer.ID) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, b := net.Pipe()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		c, err := at.SecureInbound(ctx, a)
		if err != nil {
			a.Close()
			t.Fatal(err)
		}

		if c.LocalPeer() != ap {
			t.Errorf("expected local peer %s, got %s", ap, c.LocalPeer())
		}
		testWrite(t, c)
		testRead(t, c)
		c.Close()
		testWriteFail(t, c)
		testReadFail(t, c)
	}()

	go func() {
		defer wg.Done()
		c, err := bt.SecureOutbound(ctx, b, ap)
		if err != nil {
			b.Close()
			t.Fatal(err)
		}

		if c.RemotePeer() != ap {
			t.Errorf("expected remote peer %s, got %s", ap, c.RemotePeer())
		}
		if c.LocalPeer() != bp {
			t.Errorf("expected local peer %s, got %s", bp, c.LocalPeer())
		}
		testRead(t, c)
		testWrite(t, c)
		testEOF(t, c)
		testWriteFail(t, c)
		c.Close()
	}()
	wg.Wait()
}

func SubtestKeys(t *testing.T, at, bt connsec.Transport, ap, bp peer.ID) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, b := net.Pipe()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		c, err := at.SecureInbound(ctx, a)
		if err != nil {
			a.Close()
			t.Fatal(err)
		}
		defer c.Close()

		if c.RemotePeer() != bp {
			t.Errorf("expected remote peer %s, got remote peer %s", bp, c.RemotePeer())
		}
		if c.LocalPeer() != ap {
			t.Errorf("expected local peer %s, got local peer %s", ap, c.LocalPeer())
		}
		if !c.LocalPeer().MatchesPrivateKey(c.LocalPrivateKey()) {
			t.Error("local private key mismatch")
		}
		if !c.RemotePeer().MatchesPublicKey(c.RemotePublicKey()) {
			t.Error("local private key mismatch")
		}
	}()

	go func() {
		defer wg.Done()
		c, err := bt.SecureOutbound(ctx, b, ap)
		if err != nil {
			b.Close()
			t.Fatal(err)
		}
		defer c.Close()

		if c.RemotePeer() != ap {
			t.Errorf("expected remote peer %s, got remote peer %s", ap, c.RemotePeer())
		}
		if c.LocalPeer() != bp {
			t.Errorf("expected local peer %s, got local peer %s", bp, c.LocalPeer())
		}
		if !c.LocalPeer().MatchesPrivateKey(c.LocalPrivateKey()) {
			t.Error("local private key mismatch")
		}
		if !c.RemotePeer().MatchesPublicKey(c.RemotePublicKey()) {
			t.Error("local private key mismatch")
		}
		c.Close()
	}()
	wg.Wait()
}

func SubtestWrongPeer(t *testing.T, at, bt connsec.Transport, ap, bp peer.ID) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, b := net.Pipe()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer a.Close()
		_, err := at.SecureInbound(ctx, a)
		if err == nil {
			t.Fatal("conection should have failed")
		}
	}()

	go func() {
		defer wg.Done()
		defer b.Close()
		_, err := bt.SecureOutbound(ctx, b, bp)
		if err == nil {
			t.Fatal("connection should have failed")
		}
	}()
	wg.Wait()
}

func SubtestCancelHandshakeOutbound(t *testing.T, at, bt connsec.Transport, ap, bp peer.ID) {
	ctx, cancel := context.WithCancel(context.Background())
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := at.SecureOutbound(ctx, a, ap)
		if err == nil {
			t.Fatal("connection should have failed")
		}
	}()
	time.Sleep(time.Millisecond)
	cancel()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := bt.SecureInbound(ctx, b)
		if err == nil {
			t.Fatal("connection should have failed")
		}
	}()

	wg.Wait()

}

func SubtestCancelHandshakeInbound(t *testing.T, at, bt connsec.Transport, ap, bp peer.ID) {
	ctx, cancel := context.WithCancel(context.Background())
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := at.SecureInbound(ctx, a)
		if err == nil {
			t.Fatal("connection should have failed")
		}
	}()
	time.Sleep(time.Millisecond)
	cancel()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := bt.SecureOutbound(ctx, b, bp)
		if err == nil {
			t.Fatal("connection should have failed")
		}
	}()

	wg.Wait()

}

func SubtestStream(t *testing.T, at, bt connsec.Transport, ap, bp peer.ID) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, b := net.Pipe()

	defer a.Close()
	defer b.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()

		c, err := at.SecureInbound(ctx, a)
		if err != nil {
			t.Fatal(err)
		}

		var swg sync.WaitGroup
		swg.Add(2)
		go func() {
			defer swg.Done()
			testWriteSustain(t, c)
		}()
		go func() {
			defer swg.Done()
			testReadSustain(t, c)
		}()
		swg.Wait()
		c.Close()
	}()

	go func() {
		defer wg.Done()
		c, err := bt.SecureOutbound(ctx, b, ap)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(c, c)
		c.Close()
	}()
	wg.Wait()
}
