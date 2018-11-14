package tcpreuse

import (
	"net"
	"testing"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	manet "gx/ipfs/Qmaabb1tJZ2CX5cp6MuuiGgns71NYoxdgQP6Xdid1dVceC/go-multiaddr-net"
)

var loopbackV4, _ = ma.NewMultiaddr("/ip4/127.0.0.1/tcp/0")
var loopbackV6, _ = ma.NewMultiaddr("/ip6/::1/tcp/0")
var unspecV6, _ = ma.NewMultiaddr("/ip6/::/tcp/0")
var unspecV4, _ = ma.NewMultiaddr("/ip4/0.0.0.0/tcp/0")

var globalV4 ma.Multiaddr
var globalV6 ma.Multiaddr

func init() {
	addrs, err := manet.InterfaceMultiaddrs()
	if err != nil {
		return
	}
	for _, addr := range addrs {
		if !manet.IsIP6LinkLocal(addr) && !manet.IsIPLoopback(addr) {
			tcp, _ := ma.NewMultiaddr("/tcp/0")
			switch addr.Protocols()[0].Code {
			case ma.P_IP4:
				if globalV4 == nil {
					globalV4 = addr.Encapsulate(tcp)
				}
			case ma.P_IP6:
				if globalV6 == nil {
					globalV6 = addr.Encapsulate(tcp)
				}
			}
		}
	}
}

func acceptOne(t *testing.T, listener manet.Listener) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := listener.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		c.Close()
	}()
	return done
}

func dialOne(t *testing.T, tr *Transport, listener manet.Listener, expected ...int) int {
	t.Helper()

	done := acceptOne(t, listener)
	c, err := tr.Dial(listener.Multiaddr())
	if err != nil {
		t.Fatal(err)
	}
	port := c.LocalAddr().(*net.TCPAddr).Port
	<-done
	c.Close()
	if len(expected) == 0 {
		return port
	}
	for _, p := range expected {
		if p == port {
			return port
		}
	}
	t.Errorf("dialed from %d, expected to dial from one of %v", port, expected)
	return 0
}

func TestNoneAndSingle(t *testing.T) {
	var trA Transport
	var trB Transport
	listenerA, err := trA.Listen(loopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	dialOne(t, &trB, listenerA)

	listenerB, err := trB.Listen(loopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB.Close()

	dialOne(t, &trB, listenerA, listenerB.Addr().(*net.TCPAddr).Port)
}

func TestTwoLocal(t *testing.T) {
	var trA Transport
	var trB Transport
	listenerA, err := trA.Listen(loopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	listenerB1, err := trB.Listen(loopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB1.Close()

	listenerB2, err := trB.Listen(loopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB2.Close()

	dialOne(t, &trB, listenerA,
		listenerB1.Addr().(*net.TCPAddr).Port,
		listenerB2.Addr().(*net.TCPAddr).Port)
}

func TestGlobalPreferenceV4(t *testing.T) {
	if globalV4 == nil {
		t.Skip("no global IPv4 addresses configured")
		return
	}
	testPrefer(t, loopbackV4, loopbackV4, globalV4)
	testPrefer(t, loopbackV4, unspecV4, globalV4)

	testPrefer(t, globalV4, unspecV4, globalV4)
	testPrefer(t, globalV4, unspecV4, loopbackV4)
}

func TestGlobalPreferenceV6(t *testing.T) {
	if globalV6 == nil {
		t.Skip("no global IPv6 addresses configured")
		return
	}
	testPrefer(t, loopbackV6, loopbackV6, globalV6)
	testPrefer(t, loopbackV6, unspecV6, globalV6)

	testPrefer(t, globalV6, unspecV6, globalV6)
	testPrefer(t, globalV6, unspecV6, loopbackV6)
}

func TestLoopbackPreference(t *testing.T) {
	testPrefer(t, loopbackV4, loopbackV4, unspecV4)
	testPrefer(t, loopbackV6, loopbackV6, unspecV6)
}

func testPrefer(t *testing.T, listen, prefer, avoid ma.Multiaddr) {
	var trA Transport
	var trB Transport
	listenerA, err := trA.Listen(listen)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	listenerB1, err := trB.Listen(avoid)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB1.Close()

	dialOne(t, &trB, listenerA, listenerB1.Addr().(*net.TCPAddr).Port)

	listenerB2, err := trB.Listen(prefer)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB2.Close()

	dialOne(t, &trB, listenerA, listenerB2.Addr().(*net.TCPAddr).Port)

	// Closing the listener should reset the dialer.
	listenerB2.Close()

	dialOne(t, &trB, listenerA, listenerB1.Addr().(*net.TCPAddr).Port)
}

func TestV6V4(t *testing.T) {
	testUseFirst(t, loopbackV4, loopbackV4, loopbackV6)
	testUseFirst(t, loopbackV6, loopbackV6, loopbackV4)
}

func TestGlobalToGlobal(t *testing.T) {
	if globalV4 == nil {
		t.Skip("no globalV4 addresses configured")
		return
	}
	testUseFirst(t, globalV4, globalV4, loopbackV4)
	testUseFirst(t, globalV6, globalV6, loopbackV6)
}

func testUseFirst(t *testing.T, listen, use, never ma.Multiaddr) {
	var trA Transport
	var trB Transport
	listenerA, err := trA.Listen(globalV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	listenerB1, err := trB.Listen(loopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB1.Close()

	// It works (random port)
	dialOne(t, &trB, listenerA)

	listenerB2, err := trB.Listen(globalV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB2.Close()

	// Uses globalV4 port.
	dialOne(t, &trB, listenerA, listenerB2.Addr().(*net.TCPAddr).Port)

	// Closing the listener should reset the dialer.
	listenerB2.Close()

	// It still works.
	dialOne(t, &trB, listenerA)
}

func TestDuplicateGlobal(t *testing.T) {
	if globalV4 == nil {
		t.Skip("no globalV4 addresses configured")
		return
	}

	var trA Transport
	var trB Transport
	listenerA, err := trA.Listen(globalV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	listenerB1, err := trB.Listen(globalV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB1.Close()

	listenerB2, err := trB.Listen(globalV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB2.Close()

	// Check which port we're using
	port := dialOne(t, &trB, listenerA)

	// Check consistency
	for i := 0; i < 10; i++ {
		dialOne(t, &trB, listenerA, port)
	}
}
