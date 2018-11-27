package manet

import (
	"bytes"
	"fmt"
	"net"
	"sync"
	"testing"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
)

func newMultiaddr(t *testing.T, m string) ma.Multiaddr {
	maddr, err := ma.NewMultiaddr(m)
	if err != nil {
		t.Fatal("failed to construct multiaddr:", m, err)
	}
	return maddr
}

func TestDial(t *testing.T) {

	listener, err := net.Listen("tcp", "127.0.0.1:4321")
	if err != nil {
		t.Fatal("failed to listen")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {

		cB, err := listener.Accept()
		if err != nil {
			t.Fatal("failed to accept")
		}

		// echo out
		buf := make([]byte, 1024)
		for {
			_, err := cB.Read(buf)
			if err != nil {
				break
			}
			cB.Write(buf)
		}

		wg.Done()
	}()

	maddr := newMultiaddr(t, "/ip4/127.0.0.1/tcp/4321")
	cA, err := Dial(maddr)
	if err != nil {
		t.Fatal("failed to dial")
	}

	buf := make([]byte, 1024)
	if _, err := cA.Write([]byte("beep boop")); err != nil {
		t.Fatal("failed to write:", err)
	}

	if _, err := cA.Read(buf); err != nil {
		t.Fatal("failed to read:", buf, err)
	}

	if !bytes.Equal(buf[:9], []byte("beep boop")) {
		t.Fatal("failed to echo:", buf)
	}

	maddr2 := cA.RemoteMultiaddr()
	if !maddr2.Equal(maddr) {
		t.Fatal("remote multiaddr not equal:", maddr, maddr2)
	}

	cA.Close()
	wg.Wait()
}

func TestListen(t *testing.T) {

	maddr := newMultiaddr(t, "/ip4/127.0.0.1/tcp/4322")
	listener, err := Listen(maddr)
	if err != nil {
		t.Fatal("failed to listen")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {

		cB, err := listener.Accept()
		if err != nil {
			t.Fatal("failed to accept")
		}

		if !cB.LocalMultiaddr().Equal(maddr) {
			t.Fatal("local multiaddr not equal:", maddr, cB.LocalMultiaddr())
		}

		// echo out
		buf := make([]byte, 1024)
		for {
			_, err := cB.Read(buf)
			if err != nil {
				break
			}
			cB.Write(buf)
		}

		wg.Done()
	}()

	cA, err := net.Dial("tcp", "127.0.0.1:4322")
	if err != nil {
		t.Fatal("failed to dial")
	}

	buf := make([]byte, 1024)
	if _, err := cA.Write([]byte("beep boop")); err != nil {
		t.Fatal("failed to write:", err)
	}

	if _, err := cA.Read(buf); err != nil {
		t.Fatal("failed to read:", buf, err)
	}

	if !bytes.Equal(buf[:9], []byte("beep boop")) {
		t.Fatal("failed to echo:", buf)
	}

	maddr2, err := FromNetAddr(cA.RemoteAddr())
	if err != nil {
		t.Fatal("failed to convert", err)
	}
	if !maddr2.Equal(maddr) {
		t.Fatal("remote multiaddr not equal:", maddr, maddr2)
	}

	cA.Close()
	wg.Wait()
}

func TestListenAddrs(t *testing.T) {

	test := func(addr, resaddr string, succeed bool) {
		if resaddr == "" {
			resaddr = addr
		}

		maddr := newMultiaddr(t, addr)
		l, err := Listen(maddr)
		if !succeed {
			if err == nil {
				t.Fatal("succeeded in listening", addr)
			}
			return
		}
		if succeed && err != nil {
			t.Error("failed to listen", addr, err)
		}
		if l == nil {
			t.Error("failed to listen", addr, succeed, err)
		}
		if l.Multiaddr().String() != resaddr {
			t.Error("listen addr did not resolve properly", l.Multiaddr().String(), resaddr, succeed, err)
		}

		if err = l.Close(); err != nil {
			t.Fatal("failed to close listener", addr, err)
		}
	}

	test("/ip4/127.0.0.1/tcp/4324", "", true)
	test("/ip4/127.0.0.1/udp/4325", "", false)
	test("/ip4/127.0.0.1/udp/4326/udt", "", false)
	test("/ip4/0.0.0.0/tcp/4324", "", true)
	test("/ip4/0.0.0.0/udp/4325", "", false)
	test("/ip4/0.0.0.0/udp/4326/udt", "", false)

	test("/ip6/::1/tcp/4324", "", true)
	test("/ip6/::1/udp/4325", "", false)
	test("/ip6/::1/udp/4326/udt", "", false)
	test("/ip6/::/tcp/4324", "", true)
	test("/ip6/::/udp/4325", "", false)
	test("/ip6/::/udp/4326/udt", "", false)

	/* "An implementation should also support the concept of a "default"
	 * zone for each scope.  And, when supported, the index value zero
	 * at each scope SHOULD be reserved to mean "use the default zone"."
	 * -- rfc4007. So, this _should_ work everywhere(?). */
	test("/ip6zone/0/ip6/::1/tcp/4324", "/ip6/::1/tcp/4324", true)
	test("/ip6zone/0/ip6/::1/udp/4324", "", false)
}

func TestListenAndDial(t *testing.T) {

	maddr := newMultiaddr(t, "/ip4/127.0.0.1/tcp/4323")
	listener, err := Listen(maddr)
	if err != nil {
		t.Fatal("failed to listen")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {

		cB, err := listener.Accept()
		if err != nil {
			t.Fatal("failed to accept")
		}

		if !cB.LocalMultiaddr().Equal(maddr) {
			t.Fatal("local multiaddr not equal:", maddr, cB.LocalMultiaddr())
		}

		// echo out
		buf := make([]byte, 1024)
		for {
			_, err := cB.Read(buf)
			if err != nil {
				break
			}
			cB.Write(buf)
		}

		wg.Done()
	}()

	cA, err := Dial(newMultiaddr(t, "/ip4/127.0.0.1/tcp/4323"))
	if err != nil {
		t.Fatal("failed to dial")
	}

	buf := make([]byte, 1024)
	if _, err := cA.Write([]byte("beep boop")); err != nil {
		t.Fatal("failed to write:", err)
	}

	if _, err := cA.Read(buf); err != nil {
		t.Fatal("failed to read:", buf, err)
	}

	if !bytes.Equal(buf[:9], []byte("beep boop")) {
		t.Fatal("failed to echo:", buf)
	}

	maddr2 := cA.RemoteMultiaddr()
	if !maddr2.Equal(maddr) {
		t.Fatal("remote multiaddr not equal:", maddr, maddr2)
	}

	cA.Close()
	wg.Wait()
}

func TestListenPacketAndDial(t *testing.T) {
	maddr := newMultiaddr(t, "/ip4/127.0.0.1/udp/4324")
	pc, err := ListenPacket(maddr)
	if err != nil {
		t.Fatal("failed to listen", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		if !pc.Multiaddr().Equal(maddr) {
			t.Fatal("connection multiaddr not equal:", maddr, pc.Multiaddr())
		}

		buffer := make([]byte, 1024)
		_, addr, err := pc.ReadFrom(buffer)
		if err != nil {
			t.Fatal("failed to read into buffer", err)
		}
		pc.WriteTo(buffer, addr)

		wg.Done()
	}()

	cn, err := Dial(maddr)
	if err != nil {
		t.Fatal("failed to dial", err)
	}

	buf := make([]byte, 1024)
	if _, err := cn.Write([]byte("beep boop")); err != nil {
		t.Fatal("failed to write", err)
	}

	if _, err := cn.Read(buf); err != nil {
		t.Fatal("failed to read:", buf, err)
	}

	if !bytes.Equal(buf[:9], []byte("beep boop")) {
		t.Fatal("failed to echk:", buf)
	}

	maddr2 := cn.RemoteMultiaddr()
	if !maddr2.Equal(maddr) {
		t.Fatal("remote multiaddr not equal:", maddr, maddr2)
	}

	cn.Close()
	pc.Close()
	wg.Wait()
}

func TestIPLoopback(t *testing.T) {
	if IP4Loopback.String() != "/ip4/127.0.0.1" {
		t.Error("IP4Loopback incorrect:", IP4Loopback)
	}

	if IP6Loopback.String() != "/ip6/::1" {
		t.Error("IP6Loopback incorrect:", IP6Loopback)
	}

	if IP4MappedIP6Loopback.String() != "/ip6/::ffff:127.0.0.1" {
		t.Error("IP4MappedIP6Loopback incorrect:", IP4MappedIP6Loopback)
	}

	if !IsIPLoopback(IP4Loopback) {
		t.Error("IsIPLoopback failed (IP4Loopback)")
	}

	if !IsIPLoopback(newMultiaddr(t, "/ip4/127.1.80.9")) {
		t.Error("IsIPLoopback failed (/ip4/127.1.80.9)")
	}

	if IsIPLoopback(newMultiaddr(t, "/ip4/112.123.11.1")) {
		t.Error("IsIPLoopback false positive (/ip4/112.123.11.1)")
	}

	if IsIPLoopback(newMultiaddr(t, "/ip4/192.168.0.1/ip6/::1")) {
		t.Error("IsIPLoopback false positive (/ip4/192.168.0.1/ip6/::1)")
	}

	if !IsIPLoopback(IP6Loopback) {
		t.Error("IsIPLoopback failed (IP6Loopback)")
	}

	if !IsIPLoopback(newMultiaddr(t, "/ip6/127.0.0.1")) {
		t.Error("IsIPLoopback failed (/ip6/127.0.0.1)")
	}

	if !IsIPLoopback(newMultiaddr(t, "/ip6/127.99.3.2")) {
		t.Error("IsIPLoopback failed (/ip6/127.99.3.2)")
	}

	if IsIPLoopback(newMultiaddr(t, "/ip6/::fffa:127.99.3.2")) {
		t.Error("IsIPLoopback false positive (/ip6/::fffa:127.99.3.2)")
	}

	if !IsIPLoopback(newMultiaddr(t, "/ip6zone/0/ip6/::1")) {
		t.Error("IsIPLoopback failed (/ip6zone/0/ip6/::1)")
	}

	if !IsIPLoopback(newMultiaddr(t, "/ip6zone/xxx/ip6/::1")) {
		t.Error("IsIPLoopback failed (/ip6zone/xxx/ip6/::1)")
	}

	if IsIPLoopback(newMultiaddr(t, "/ip6zone/0/ip6/1::1")) {
		t.Errorf("IsIPLoopback false positive (/ip6zone/0/ip6/1::1)")
	}
}

func TestIPUnspecified(t *testing.T) {
	if IP4Unspecified.String() != "/ip4/0.0.0.0" {
		t.Error("IP4Unspecified incorrect:", IP4Unspecified)
	}

	if IP6Unspecified.String() != "/ip6/::" {
		t.Error("IP6Unspecified incorrect:", IP6Unspecified)
	}

	if !IsIPUnspecified(IP4Unspecified) {
		t.Error("IsIPUnspecified failed (IP4Unspecified)")
	}

	if !IsIPUnspecified(IP6Unspecified) {
		t.Error("IsIPUnspecified failed (IP6Unspecified)")
	}

	if !IsIPUnspecified(newMultiaddr(t, "/ip6zone/xxx/ip6/::")) {
		t.Error("IsIPUnspecified failed (/ip6zone/xxx/ip6/::)")
	}
}

func TestIP6LinkLocal(t *testing.T) {
	for a := 0; a < 65536; a++ {
		isLinkLocal := (a&0xffc0 == 0xfe80 || a&0xff0f == 0xff02)
		m := newMultiaddr(t, fmt.Sprintf("/ip6/%x::1", a))
		if IsIP6LinkLocal(m) != isLinkLocal {
			t.Errorf("IsIP6LinkLocal failed (%s != %v)", m, isLinkLocal)
		}
	}

	if !IsIP6LinkLocal(newMultiaddr(t, "/ip6zone/hello/ip6/fe80::9999")) {
		t.Error("IsIP6LinkLocal failed (/ip6/fe80::9999)")
	}
}

func TestConvertNetAddr(t *testing.T) {
	m1 := newMultiaddr(t, "/ip4/1.2.3.4/tcp/4001")

	n1, err := ToNetAddr(m1)
	if err != nil {
		t.Fatal(err)
	}

	m2, err := FromNetAddr(n1)
	if err != nil {
		t.Fatal(err)
	}

	if m1.String() != m2.String() {
		t.Fatal("ToNetAddr + FromNetAddr did not work")
	}
}

func TestWrapNetConn(t *testing.T) {
	// test WrapNetConn nil
	if _, err := WrapNetConn(nil); err == nil {
		t.Error("WrapNetConn(nil) should return an error")
	}

	checkErr := func(err error, s string) {
		if err != nil {
			t.Fatal(s, err)
		}
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	checkErr(err, "failed to listen")

	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Add(1)
	go func() {
		defer wg.Done()
		cB, err := listener.Accept()
		checkErr(err, "failed to accept")
		_ = cB.(halfOpen)
		cB.Close()
	}()

	cA, err := net.Dial("tcp", listener.Addr().String())
	checkErr(err, "failed to dial")
	defer cA.Close()
	_ = cA.(halfOpen)

	lmaddr, err := FromNetAddr(cA.LocalAddr())
	checkErr(err, "failed to get local addr")
	rmaddr, err := FromNetAddr(cA.RemoteAddr())
	checkErr(err, "failed to get remote addr")

	mcA, err := WrapNetConn(cA)
	checkErr(err, "failed to wrap conn")

	_ = mcA.(halfOpen)

	if mcA.LocalAddr().String() != cA.LocalAddr().String() {
		t.Error("wrapped conn local addr differs")
	}
	if mcA.RemoteAddr().String() != cA.RemoteAddr().String() {
		t.Error("wrapped conn remote addr differs")
	}
	if mcA.LocalMultiaddr().String() != lmaddr.String() {
		t.Error("wrapped conn local maddr differs")
	}
	if mcA.RemoteMultiaddr().String() != rmaddr.String() {
		t.Error("wrapped conn remote maddr differs")
	}
}

func TestAddrMatch(t *testing.T) {

	test := func(m ma.Multiaddr, input, expect []ma.Multiaddr) {
		actual := AddrMatch(m, input)
		testSliceEqual(t, expect, actual)
	}

	a := []ma.Multiaddr{
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/1234"),
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/2345"),
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/1234/tcp/2345"),
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/1234/tcp/2345"),
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/1234/udp/1234"),
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/1234/udp/1234"),
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/1234/ip6/::1"),
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/1234/ip6/::1"),
		newMultiaddr(t, "/ip6/::1/tcp/1234"),
		newMultiaddr(t, "/ip6/::1/tcp/2345"),
		newMultiaddr(t, "/ip6/::1/tcp/1234/tcp/2345"),
		newMultiaddr(t, "/ip6/::1/tcp/1234/tcp/2345"),
		newMultiaddr(t, "/ip6/::1/tcp/1234/udp/1234"),
		newMultiaddr(t, "/ip6/::1/tcp/1234/udp/1234"),
		newMultiaddr(t, "/ip6/::1/tcp/1234/ip6/::1"),
		newMultiaddr(t, "/ip6/::1/tcp/1234/ip6/::1"),
	}

	test(a[0], a, []ma.Multiaddr{
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/1234"),
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/2345"),
	})
	test(a[2], a, []ma.Multiaddr{
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/1234/tcp/2345"),
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/1234/tcp/2345"),
	})
	test(a[4], a, []ma.Multiaddr{
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/1234/udp/1234"),
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/1234/udp/1234"),
	})
	test(a[6], a, []ma.Multiaddr{
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/1234/ip6/::1"),
		newMultiaddr(t, "/ip4/1.2.3.4/tcp/1234/ip6/::1"),
	})
	test(a[8], a, []ma.Multiaddr{
		newMultiaddr(t, "/ip6/::1/tcp/1234"),
		newMultiaddr(t, "/ip6/::1/tcp/2345"),
	})
	test(a[10], a, []ma.Multiaddr{
		newMultiaddr(t, "/ip6/::1/tcp/1234/tcp/2345"),
		newMultiaddr(t, "/ip6/::1/tcp/1234/tcp/2345"),
	})
	test(a[12], a, []ma.Multiaddr{
		newMultiaddr(t, "/ip6/::1/tcp/1234/udp/1234"),
		newMultiaddr(t, "/ip6/::1/tcp/1234/udp/1234"),
	})
	test(a[14], a, []ma.Multiaddr{
		newMultiaddr(t, "/ip6/::1/tcp/1234/ip6/::1"),
		newMultiaddr(t, "/ip6/::1/tcp/1234/ip6/::1"),
	})

}

func testSliceEqual(t *testing.T, a, b []ma.Multiaddr) {
	if len(a) != len(b) {
		t.Error("differ", a, b)
	}
	for i, addrA := range a {
		if !addrA.Equal(b[i]) {
			t.Error("differ", a, b)
		}
	}
}

func TestInterfaceAddressesWorks(t *testing.T) {
	_, err := InterfaceMultiaddrs()
	if err != nil {
		t.Fatal(err)
	}
}

func TestNetListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:1234")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	malist, err := WrapNetListener(listener)
	if err != nil {
		t.Fatal(err)
	}
	if !malist.Multiaddr().Equal(newMultiaddr(t, "/ip4/127.0.0.1/tcp/1234")) {
		t.Fatal("unexpected multiaddr")
	}

	go func() {
		c, err := Dial(malist.Multiaddr())
		if err != nil {
			t.Fatal("failed to dial")
		}
		if !c.RemoteMultiaddr().Equal(malist.Multiaddr()) {
			t.Fatal("dialed wrong target")
		}
		c.Close()

		c, err = Dial(malist.Multiaddr())
		if err != nil {
			t.Fatal("failed to dial")
		}
		c.Close()
	}()

	c, err := malist.Accept()
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	netList := NetListener(malist)
	malist2, err := WrapNetListener(netList)
	if err != nil {
		t.Fatal(err)
	}
	if malist2 != malist {
		t.Fatal("expected WrapNetListener(NetListener(malist)) == malist")
	}
	nc, err := netList.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if !nc.(Conn).LocalMultiaddr().Equal(malist.Multiaddr()) {
		t.Fatal("wrong multiaddr on conn")
	}
	nc.Close()
}
