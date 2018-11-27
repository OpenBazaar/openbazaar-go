package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"sync"
	"testing"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	smux "gx/ipfs/QmY9JXR3FupnYAYJWK9aMr9bCpqWKcToQ1tz8DVGTrHpHw/go-stream-muxer"
	tpt "gx/ipfs/QmbCkisBsdejwSzusQcdbYjpSX3yvUw1ek2YSsJ89QbZYX/go-libp2p-transport"
)

var testData = []byte("this is some test data")

type streamAndConn struct {
	stream smux.Stream
	conn   tpt.Conn
}

func SubtestProtocols(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID) {
	rawIPAddr, _ := ma.NewMultiaddr("/ip4/1.2.3.4")
	if ta.CanDial(rawIPAddr) || tb.CanDial(rawIPAddr) {
		t.Error("nothing should be able to dial raw IP")
	}

	tprotos := make(map[int]bool)
	for _, p := range ta.Protocols() {
		tprotos[p] = true
	}

	if !ta.Proxy() {
		protos := maddr.Protocols()
		proto := protos[len(protos)-1]
		if !tprotos[proto.Code] {
			t.Errorf("transport should have reported that it supports protocol '%s' (%d)", proto.Name, proto.Code)
		}
	} else {
		found := false
		for _, proto := range maddr.Protocols() {
			if tprotos[proto.Code] {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("didn't find any matching proxy protocols in maddr: %s", maddr)
		}
	}
}

func SubtestBasic(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	list, err := ta.Listen(maddr)
	if err != nil {
		t.Fatal(err)
	}
	defer list.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := list.Accept()
		if err != nil {
			t.Fatal(err)
			return
		}
		s, err := c.AcceptStream()
		if err != nil {
			c.Close()
			t.Fatal(err)
			return
		}

		buf := make([]byte, len(testData))
		_, err = io.ReadFull(s, buf)
		if err != nil {
			t.Fatal(err)
			return
		}

		n, err := s.Write(testData)
		if err != nil {
			t.Fatal(err)
			return
		}
		s.Close()

		if n != len(testData) {
			t.Fatal(err)
			return
		}
	}()

	if !tb.CanDial(list.Multiaddr()) {
		t.Error("CanDial should have returned true")
	}

	c, err := tb.Dial(ctx, list.Multiaddr(), peerA)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	s, err := c.OpenStream()
	if err != nil {
		t.Fatal(err)
	}

	n, err := s.Write(testData)
	if err != nil {
		t.Fatal(err)
		return
	}

	if n != len(testData) {
		t.Fatalf("failed to write enough data (a->b)")
		return
	}

	buf := make([]byte, len(testData))
	_, err = io.ReadFull(s, buf)
	if err != nil {
		t.Fatal(err)
		return
	}
}

func SubtestPingPong(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID) {
	streams := 100

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	list, err := ta.Listen(maddr)
	if err != nil {
		t.Fatal(err)
	}
	defer list.Close()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		c, err := list.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close()

		var sWg sync.WaitGroup
		for i := 0; i < streams; i++ {
			s, err := c.AcceptStream()
			if err != nil {
				t.Error(err)
				return
			}

			sWg.Add(1)
			go func() {
				defer sWg.Done()

				data, err := ioutil.ReadAll(s)
				if err != nil {
					s.Reset()
					t.Error(err)
					return
				}
				if !bytes.HasPrefix(data, testData) {
					t.Errorf("expected %q to have prefix %q", string(data), string(testData))
				}

				n, err := s.Write(data)
				if err != nil {
					s.Reset()
					t.Error(err)
					return
				}

				if n != len(data) {
					s.Reset()
					t.Error(err)
					return
				}
				s.Close()
			}()
		}
		sWg.Wait()
	}()

	if !tb.CanDial(list.Multiaddr()) {
		t.Error("CanDial should have returned true")
	}

	c, err := tb.Dial(ctx, list.Multiaddr(), peerA)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	for i := 0; i < streams; i++ {
		s, err := c.OpenStream()
		if err != nil {
			t.Error(err)
			continue
		}

		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			data := []byte(fmt.Sprintf("%s - %d", testData, i))
			n, err := s.Write(data)
			if err != nil {
				s.Reset()
				t.Error(err)
				return
			}

			if n != len(data) {
				s.Reset()
				t.Error("failed to write enough data (a->b)")
				return
			}
			s.Close()

			ret, err := ioutil.ReadAll(s)
			if err != nil {
				s.Reset()
				t.Error(err)
				return
			}
			if !bytes.Equal(data, ret) {
				t.Errorf("expected %q, got %q", string(data), string(ret))
			}
		}(i)
	}
	wg.Wait()
}

func SubtestCancel(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID) {
	list, err := ta.Listen(maddr)
	if err != nil {
		t.Fatal(err)
	}
	defer list.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c, err := tb.Dial(ctx, list.Multiaddr(), peerA)
	if err == nil {
		c.Close()
		t.Fatal("dial should have failed")
	}
}
