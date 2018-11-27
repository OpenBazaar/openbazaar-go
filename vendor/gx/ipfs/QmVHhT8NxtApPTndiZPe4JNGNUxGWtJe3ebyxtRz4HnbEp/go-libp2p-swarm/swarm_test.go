package swarm_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	pstore "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
	inet "gx/ipfs/QmXuRkCR7BNQa9uqfpTiFWsTQLzmTWYg91Ja1w95gnqb6u/go-libp2p-net"
	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"

	. "gx/ipfs/QmVHhT8NxtApPTndiZPe4JNGNUxGWtJe3ebyxtRz4HnbEp/go-libp2p-swarm"
	. "gx/ipfs/QmVHhT8NxtApPTndiZPe4JNGNUxGWtJe3ebyxtRz4HnbEp/go-libp2p-swarm/testing"
)

var log = logging.Logger("swarm_test")

func EchoStreamHandler(stream inet.Stream) {
	go func() {
		defer stream.Close()

		// pull out the ipfs conn
		c := stream.Conn()
		log.Infof("%s ponging to %s", c.LocalPeer(), c.RemotePeer())

		buf := make([]byte, 4)

		for {
			if _, err := stream.Read(buf); err != nil {
				if err != io.EOF {
					log.Error("ping receive error:", err)
				}
				return
			}

			if !bytes.Equal(buf, []byte("ping")) {
				log.Errorf("ping receive error: ping != %s %v", buf, buf)
				return
			}

			if _, err := stream.Write([]byte("pong")); err != nil {
				log.Error("pond send error:", err)
				return
			}
		}
	}()
}

func makeDialOnlySwarm(ctx context.Context, t *testing.T) *Swarm {
	swarm := GenSwarm(t, ctx, OptDialOnly)
	swarm.SetStreamHandler(EchoStreamHandler)

	return swarm
}

func makeSwarms(ctx context.Context, t *testing.T, num int, opts ...Option) []*Swarm {
	swarms := make([]*Swarm, 0, num)

	for i := 0; i < num; i++ {
		swarm := GenSwarm(t, ctx, opts...)
		swarm.SetStreamHandler(EchoStreamHandler)
		swarms = append(swarms, swarm)
	}

	return swarms
}

func connectSwarms(t *testing.T, ctx context.Context, swarms []*Swarm) {

	var wg sync.WaitGroup
	connect := func(s *Swarm, dst peer.ID, addr ma.Multiaddr) {
		// TODO: make a DialAddr func.
		s.Peerstore().AddAddr(dst, addr, pstore.PermanentAddrTTL)
		if _, err := s.DialPeer(ctx, dst); err != nil {
			t.Fatal("error swarm dialing to peer", err)
		}
		wg.Done()
	}

	log.Info("Connecting swarms simultaneously.")
	for i, s1 := range swarms {
		for _, s2 := range swarms[i+1:] {
			wg.Add(1)
			connect(s1, s2.LocalPeer(), s2.ListenAddresses()[0]) // try the first.
		}
	}
	wg.Wait()

	for _, s := range swarms {
		log.Infof("%s swarm routing table: %s", s.LocalPeer(), s.Peers())
	}
}

func SubtestSwarm(t *testing.T, SwarmNum int, MsgNum int) {
	// t.Skip("skipping for another test")

	ctx := context.Background()
	swarms := makeSwarms(ctx, t, SwarmNum, OptDisableReuseport)

	// connect everyone
	connectSwarms(t, ctx, swarms)

	// ping/pong
	for _, s1 := range swarms {
		log.Debugf("-------------------------------------------------------")
		log.Debugf("%s ping pong round", s1.LocalPeer())
		log.Debugf("-------------------------------------------------------")

		_, cancel := context.WithCancel(ctx)
		got := map[peer.ID]int{}
		errChan := make(chan error, MsgNum*len(swarms))
		streamChan := make(chan inet.Stream, MsgNum)

		// send out "ping" x MsgNum to every peer
		go func() {
			defer close(streamChan)

			var wg sync.WaitGroup
			send := func(p peer.ID) {
				defer wg.Done()

				// first, one stream per peer (nice)
				stream, err := s1.NewStream(ctx, p)
				if err != nil {
					errChan <- err
					return
				}

				// send out ping!
				for k := 0; k < MsgNum; k++ { // with k messages
					msg := "ping"
					log.Debugf("%s %s %s (%d)", s1.LocalPeer(), msg, p, k)
					if _, err := stream.Write([]byte(msg)); err != nil {
						errChan <- err
						continue
					}
				}

				// read it later
				streamChan <- stream
			}

			for _, s2 := range swarms {
				if s2.LocalPeer() == s1.LocalPeer() {
					continue // dont send to self...
				}

				wg.Add(1)
				go send(s2.LocalPeer())
			}
			wg.Wait()
		}()

		// receive "pong" x MsgNum from every peer
		go func() {
			defer close(errChan)
			count := 0
			countShouldBe := MsgNum * (len(swarms) - 1)
			for stream := range streamChan { // one per peer
				defer stream.Close()

				// get peer on the other side
				p := stream.Conn().RemotePeer()

				// receive pings
				msgCount := 0
				msg := make([]byte, 4)
				for k := 0; k < MsgNum; k++ { // with k messages

					// read from the stream
					if _, err := stream.Read(msg); err != nil {
						errChan <- err
						continue
					}

					if string(msg) != "pong" {
						errChan <- fmt.Errorf("unexpected message: %s", msg)
						continue
					}

					log.Debugf("%s %s %s (%d)", s1.LocalPeer(), msg, p, k)
					msgCount++
				}

				got[p] = msgCount
				count += msgCount
			}

			if count != countShouldBe {
				errChan <- fmt.Errorf("count mismatch: %d != %d", count, countShouldBe)
			}
		}()

		// check any errors (blocks till consumer is done)
		for err := range errChan {
			if err != nil {
				t.Error(err.Error())
			}
		}

		log.Debugf("%s got pongs", s1.LocalPeer())
		if (len(swarms) - 1) != len(got) {
			t.Errorf("got (%d) less messages than sent (%d).", len(got), len(swarms))
		}

		for p, n := range got {
			if n != MsgNum {
				t.Error("peer did not get all msgs", p, n, "/", MsgNum)
			}
		}

		cancel()
		<-time.After(10 * time.Millisecond)
	}

	for _, s := range swarms {
		s.Close()
	}
}

func TestSwarm(t *testing.T) {
	// t.Skip("skipping for another test")
	t.Parallel()

	// msgs := 1000
	msgs := 100
	swarms := 5
	SubtestSwarm(t, swarms, msgs)
}

func TestBasicSwarm(t *testing.T) {
	// t.Skip("skipping for another test")
	t.Parallel()

	msgs := 1
	swarms := 2
	SubtestSwarm(t, swarms, msgs)
}

func TestConnHandler(t *testing.T) {
	// t.Skip("skipping for another test")
	t.Parallel()

	ctx := context.Background()
	swarms := makeSwarms(ctx, t, 5)

	gotconn := make(chan struct{}, 10)
	swarms[0].SetConnHandler(func(conn inet.Conn) {
		gotconn <- struct{}{}
	})

	connectSwarms(t, ctx, swarms)

	<-time.After(time.Millisecond)
	// should've gotten 5 by now.

	swarms[0].SetConnHandler(nil)

	expect := 4
	for i := 0; i < expect; i++ {
		select {
		case <-time.After(time.Second):
			t.Fatal("failed to get connections")
		case <-gotconn:
		}
	}

	select {
	case <-gotconn:
		t.Fatalf("should have connected to %d swarms, got an extra.", expect)
	default:
	}
}

func TestAddrBlocking(t *testing.T) {
	ctx := context.Background()
	swarms := makeSwarms(ctx, t, 2)

	swarms[0].SetConnHandler(func(conn inet.Conn) {
		t.Errorf("no connections should happen! -- %s", conn)
	})

	_, block, err := net.ParseCIDR("127.0.0.1/8")
	if err != nil {
		t.Fatal(err)
	}

	swarms[1].Filters.AddDialFilter(block)

	swarms[1].Peerstore().AddAddr(swarms[0].LocalPeer(), swarms[0].ListenAddresses()[0], pstore.PermanentAddrTTL)
	_, err = swarms[1].DialPeer(ctx, swarms[0].LocalPeer())
	if err == nil {
		t.Fatal("dial should have failed")
	}

	swarms[0].Peerstore().AddAddr(swarms[1].LocalPeer(), swarms[1].ListenAddresses()[0], pstore.PermanentAddrTTL)
	_, err = swarms[0].DialPeer(ctx, swarms[1].LocalPeer())
	if err == nil {
		t.Fatal("dial should have failed")
	}
}

func TestFilterBounds(t *testing.T) {
	ctx := context.Background()
	swarms := makeSwarms(ctx, t, 2)

	conns := make(chan struct{}, 8)
	swarms[0].SetConnHandler(func(conn inet.Conn) {
		conns <- struct{}{}
	})

	// Address that we wont be dialing from
	_, block, err := net.ParseCIDR("192.0.0.1/8")
	if err != nil {
		t.Fatal(err)
	}

	// set filter on both sides, shouldnt matter
	swarms[1].Filters.AddDialFilter(block)
	swarms[0].Filters.AddDialFilter(block)

	connectSwarms(t, ctx, swarms)

	select {
	case <-time.After(time.Second):
		t.Fatal("should have gotten connection")
	case <-conns:
		t.Log("got connect")
	}
}
