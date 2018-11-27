package swarm_test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	pstore "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
	swarmt "gx/ipfs/QmVHhT8NxtApPTndiZPe4JNGNUxGWtJe3ebyxtRz4HnbEp/go-libp2p-swarm/testing"
	addrutil "gx/ipfs/QmXeCbQtGnurbWEWuxSyHAFRUzCvdarF9aNCGKSqRNzYv6/go-addr-util"
	testutil "gx/ipfs/Qma6ESRQTf1ZLPgzpCwDTqQJefPnU6uLvMjP18vK8EWp8L/go-testutil"
	ci "gx/ipfs/Qma6ESRQTf1ZLPgzpCwDTqQJefPnU6uLvMjP18vK8EWp8L/go-testutil/ci"
	manet "gx/ipfs/Qmaabb1tJZ2CX5cp6MuuiGgns71NYoxdgQP6Xdid1dVceC/go-multiaddr-net"
	transport "gx/ipfs/QmbCkisBsdejwSzusQcdbYjpSX3yvUw1ek2YSsJ89QbZYX/go-libp2p-transport"

	. "gx/ipfs/QmVHhT8NxtApPTndiZPe4JNGNUxGWtJe3ebyxtRz4HnbEp/go-libp2p-swarm"
)

func init() {
	transport.DialTimeout = time.Second
}

func closeSwarms(swarms []*Swarm) {
	for _, s := range swarms {
		s.Close()
	}
}

func TestBasicDialPeer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	swarms := makeSwarms(ctx, t, 2)
	defer closeSwarms(swarms)
	s1 := swarms[0]
	s2 := swarms[1]

	s1.Peerstore().AddAddrs(s2.LocalPeer(), s2.ListenAddresses(), pstore.PermanentAddrTTL)

	c, err := s1.DialPeer(ctx, s2.LocalPeer())
	if err != nil {
		t.Fatal(err)
	}

	s, err := c.NewStream()
	if err != nil {
		t.Fatal(err)
	}

	s.Close()
}

func TestDialWithNoListeners(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s1 := makeDialOnlySwarm(ctx, t)

	swarms := makeSwarms(ctx, t, 1)
	defer closeSwarms(swarms)
	s2 := swarms[0]

	s1.Peerstore().AddAddrs(s2.LocalPeer(), s2.ListenAddresses(), pstore.PermanentAddrTTL)

	c, err := s1.DialPeer(ctx, s2.LocalPeer())
	if err != nil {
		t.Fatal(err)
	}

	s, err := c.NewStream()
	if err != nil {
		t.Fatal(err)
	}

	s.Close()
}

func acceptAndHang(l net.Listener) {
	conns := make([]net.Conn, 0, 10)
	for {
		c, err := l.Accept()
		if err != nil {
			break
		}
		if c != nil {
			conns = append(conns, c)
		}
	}
	for _, c := range conns {
		c.Close()
	}
}

func TestSimultDials(t *testing.T) {
	// t.Skip("skipping for another test")
	t.Parallel()

	ctx := context.Background()
	swarms := makeSwarms(ctx, t, 2, swarmt.OptDisableReuseport)

	// connect everyone
	{
		var wg sync.WaitGroup
		connect := func(s *Swarm, dst peer.ID, addr ma.Multiaddr) {
			// copy for other peer
			log.Debugf("TestSimultOpen: connecting: %s --> %s (%s)", s.LocalPeer(), dst, addr)
			s.Peerstore().AddAddr(dst, addr, pstore.TempAddrTTL)
			if _, err := s.DialPeer(ctx, dst); err != nil {
				t.Fatal("error swarm dialing to peer", err)
			}
			wg.Done()
		}

		ifaceAddrs0, err := swarms[0].InterfaceListenAddresses()
		if err != nil {
			t.Fatal(err)
		}
		ifaceAddrs1, err := swarms[1].InterfaceListenAddresses()
		if err != nil {
			t.Fatal(err)
		}

		log.Info("Connecting swarms simultaneously.")
		for i := 0; i < 10; i++ { // connect 10x for each.
			wg.Add(2)
			go connect(swarms[0], swarms[1].LocalPeer(), ifaceAddrs1[0])
			go connect(swarms[1], swarms[0].LocalPeer(), ifaceAddrs0[0])
		}
		wg.Wait()
	}

	// should still just have 1, at most 2 connections :)
	c01l := len(swarms[0].ConnsToPeer(swarms[1].LocalPeer()))
	if c01l > 2 {
		t.Error("0->1 has", c01l)
	}
	c10l := len(swarms[1].ConnsToPeer(swarms[0].LocalPeer()))
	if c10l > 2 {
		t.Error("1->0 has", c10l)
	}

	for _, s := range swarms {
		s.Close()
	}
}

func newSilentPeer(t *testing.T) (peer.ID, ma.Multiaddr, net.Listener) {
	dst := testutil.RandPeerIDFatal(t)
	lst, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	addr, err := manet.FromNetAddr(lst.Addr())
	if err != nil {
		t.Fatal(err)
	}
	addrs := []ma.Multiaddr{addr}
	addrs, err = addrutil.ResolveUnspecifiedAddresses(addrs, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("new silent peer:", dst, addrs[0])
	return dst, addrs[0], lst
}

func TestDialWait(t *testing.T) {
	// t.Skip("skipping for another test")
	t.Parallel()

	ctx := context.Background()
	swarms := makeSwarms(ctx, t, 1)
	s1 := swarms[0]
	defer s1.Close()

	// dial to a non-existent peer.
	s2p, s2addr, s2l := newSilentPeer(t)
	go acceptAndHang(s2l)
	defer s2l.Close()
	s1.Peerstore().AddAddr(s2p, s2addr, pstore.PermanentAddrTTL)

	before := time.Now()
	if c, err := s1.DialPeer(ctx, s2p); err == nil {
		defer c.Close()
		t.Fatal("error swarm dialing to unknown peer worked...", err)
	} else {
		t.Log("correctly got error:", err)
	}
	duration := time.Since(before)

	if duration < transport.DialTimeout*DialAttempts {
		t.Error("< transport.DialTimeout * DialAttempts not being respected", duration, transport.DialTimeout*DialAttempts)
	}
	if duration > 2*transport.DialTimeout*DialAttempts {
		t.Error("> 2*transport.DialTimeout * DialAttempts not being respected", duration, 2*transport.DialTimeout*DialAttempts)
	}

	if !s1.Backoff().Backoff(s2p) {
		t.Error("s2 should now be on backoff")
	}
}

func TestDialBackoff(t *testing.T) {
	// t.Skip("skipping for another test")
	if ci.IsRunning() {
		t.Skip("travis will never have fun with this test")
	}

	t.Parallel()

	ctx := context.Background()
	swarms := makeSwarms(ctx, t, 2)
	s1 := swarms[0]
	s2 := swarms[1]
	defer s1.Close()
	defer s2.Close()

	s2addrs, err := s2.InterfaceListenAddresses()
	if err != nil {
		t.Fatal(err)
	}
	s1.Peerstore().AddAddrs(s2.LocalPeer(), s2addrs, pstore.PermanentAddrTTL)

	// dial to a non-existent peer.
	s3p, s3addr, s3l := newSilentPeer(t)
	go acceptAndHang(s3l)
	defer s3l.Close()
	s1.Peerstore().AddAddr(s3p, s3addr, pstore.PermanentAddrTTL)

	// in this test we will:
	//   1) dial 10x to each node.
	//   2) all dials should hang
	//   3) s1->s2 should succeed.
	//   4) s1->s3 should not (and should place s3 on backoff)
	//   5) disconnect entirely
	//   6) dial 10x to each node again
	//   7) s3 dials should all return immediately (except 1)
	//   8) s2 dials should all hang, and succeed
	//   9) last s3 dial ends, unsuccessful

	dialOnlineNode := func(dst peer.ID, times int) <-chan bool {
		ch := make(chan bool)
		for i := 0; i < times; i++ {
			go func() {
				if _, err := s1.DialPeer(ctx, dst); err != nil {
					t.Error("error dialing", dst, err)
					ch <- false
				} else {
					ch <- true
				}
			}()
		}
		return ch
	}

	dialOfflineNode := func(dst peer.ID, times int) <-chan bool {
		ch := make(chan bool)
		for i := 0; i < times; i++ {
			go func() {
				if c, err := s1.DialPeer(ctx, dst); err != nil {
					ch <- false
				} else {
					t.Error("succeeded in dialing", dst)
					ch <- true
					c.Close()
				}
			}()
		}
		return ch
	}

	{
		// 1) dial 10x to each node.
		N := 10
		s2done := dialOnlineNode(s2.LocalPeer(), N)
		s3done := dialOfflineNode(s3p, N)

		// when all dials should be done by:
		dialTimeout1x := time.After(transport.DialTimeout)
		dialTimeout10Ax := time.After(transport.DialTimeout * 2 * 10) // DialAttempts * 10)

		// 2) all dials should hang
		select {
		case <-s2done:
			t.Error("s2 should not happen immediately")
		case <-s3done:
			t.Error("s3 should not happen yet")
		case <-time.After(time.Millisecond):
			// s2 may finish very quickly, so let's get out.
		}

		// 3) s1->s2 should succeed.
		for i := 0; i < N; i++ {
			select {
			case r := <-s2done:
				if !r {
					t.Error("s2 should not fail")
				}
			case <-s3done:
				t.Error("s3 should not happen yet")
			case <-dialTimeout1x:
				t.Error("s2 took too long")
			}
		}

		select {
		case <-s2done:
			t.Error("s2 should have no more")
		case <-s3done:
			t.Error("s3 should not happen yet")
		case <-dialTimeout1x: // let it pass
		}

		// 4) s1->s3 should not (and should place s3 on backoff)
		// N-1 should finish before dialTimeout1x * 2
		for i := 0; i < N; i++ {
			select {
			case <-s2done:
				t.Error("s2 should have no more")
			case r := <-s3done:
				if r {
					t.Error("s3 should not succeed")
				}
			case <-(dialTimeout1x):
				if i < (N - 1) {
					t.Fatal("s3 took too long")
				}
				t.Log("dialTimeout1x * 1.3 hit for last peer")
			case <-dialTimeout10Ax:
				t.Fatal("s3 took too long")
			}
		}

		// check backoff state
		if s1.Backoff().Backoff(s2.LocalPeer()) {
			t.Error("s2 should not be on backoff")
		}
		if !s1.Backoff().Backoff(s3p) {
			t.Error("s3 should be on backoff")
		}

		// 5) disconnect entirely

		for _, c := range s1.Conns() {
			c.Close()
		}
		for i := 0; i < 100 && len(s1.Conns()) > 0; i++ {
			<-time.After(time.Millisecond)
		}
		if len(s1.Conns()) > 0 {
			t.Fatal("s1 conns must exit")
		}
	}

	{
		// 6) dial 10x to each node again
		N := 10
		s2done := dialOnlineNode(s2.LocalPeer(), N)
		s3done := dialOfflineNode(s3p, N)

		// when all dials should be done by:
		dialTimeout1x := time.After(transport.DialTimeout)
		dialTimeout10Ax := time.After(transport.DialTimeout * 2 * 10) // DialAttempts * 10)

		// 7) s3 dials should all return immediately (except 1)
		for i := 0; i < N-1; i++ {
			select {
			case <-s2done:
				t.Error("s2 should not succeed yet")
			case r := <-s3done:
				if r {
					t.Error("s3 should not succeed")
				}
			case <-dialTimeout1x:
				t.Fatal("s3 took too long")
			}
		}

		// 8) s2 dials should all hang, and succeed
		for i := 0; i < N; i++ {
			select {
			case r := <-s2done:
				if !r {
					t.Error("s2 should succeed")
				}
			// case <-s3done:
			case <-(dialTimeout1x):
				t.Fatal("s3 took too long")
			}
		}

		// 9) the last s3 should return, failed.
		select {
		case <-s2done:
			t.Error("s2 should have no more")
		case r := <-s3done:
			if r {
				t.Error("s3 should not succeed")
			}
		case <-dialTimeout10Ax:
			t.Fatal("s3 took too long")
		}

		// check backoff state (the same)
		if s1.Backoff().Backoff(s2.LocalPeer()) {
			t.Error("s2 should not be on backoff")
		}
		if !s1.Backoff().Backoff(s3p) {
			t.Error("s3 should be on backoff")
		}
	}
}

func TestDialBackoffClears(t *testing.T) {
	// t.Skip("skipping for another test")
	t.Parallel()

	ctx := context.Background()
	swarms := makeSwarms(ctx, t, 2)
	s1 := swarms[0]
	s2 := swarms[1]
	defer s1.Close()
	defer s2.Close()

	// use another address first, that accept and hang on conns
	_, s2bad, s2l := newSilentPeer(t)
	go acceptAndHang(s2l)
	defer s2l.Close()

	// phase 1 -- dial to non-operational addresses
	s1.Peerstore().AddAddr(s2.LocalPeer(), s2bad, pstore.PermanentAddrTTL)

	before := time.Now()
	if c, err := s1.DialPeer(ctx, s2.LocalPeer()); err == nil {
		t.Fatal("dialing to broken addr worked...", err)
		defer c.Close()
	} else {
		t.Log("correctly got error:", err)
	}
	duration := time.Since(before)

	if duration < transport.DialTimeout*DialAttempts {
		t.Error("< transport.DialTimeout * DialAttempts not being respected", duration, transport.DialTimeout*DialAttempts)
	}
	if duration > 2*transport.DialTimeout*DialAttempts {
		t.Error("> 2*transport.DialTimeout * DialAttempts not being respected", duration, 2*transport.DialTimeout*DialAttempts)
	}

	if !s1.Backoff().Backoff(s2.LocalPeer()) {
		t.Error("s2 should now be on backoff")
	} else {
		t.Log("correctly added to backoff")
	}

	// phase 2 -- add the working address. dial should succeed.
	ifaceAddrs1, err := swarms[1].InterfaceListenAddresses()
	if err != nil {
		t.Fatal(err)
	}
	s1.Peerstore().AddAddrs(s2.LocalPeer(), ifaceAddrs1, pstore.PermanentAddrTTL)

	if _, err := s1.DialPeer(ctx, s2.LocalPeer()); err == nil {
		t.Fatal("should have failed to dial backed off peer")
	}

	time.Sleep(BackoffBase)

	if c, err := s1.DialPeer(ctx, s2.LocalPeer()); err != nil {
		t.Fatal(err)
	} else {
		c.Close()
		t.Log("correctly connected")
	}

	if s1.Backoff().Backoff(s2.LocalPeer()) {
		t.Error("s2 should no longer be on backoff")
	} else {
		t.Log("correctly cleared backoff")
	}
}
