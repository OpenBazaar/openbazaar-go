package identify

import (
	"testing"
	"time"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	net "gx/ipfs/QmXuRkCR7BNQa9uqfpTiFWsTQLzmTWYg91Ja1w95gnqb6u/go-libp2p-net"
)

// TestObsAddrSet
func TestObsAddrSet(t *testing.T) {
	m := func(s string) ma.Multiaddr {
		m, err := ma.NewMultiaddr(s)
		if err != nil {
			t.Error(err)
		}
		return m
	}

	addrsMarch := func(a, b []ma.Multiaddr) bool {
		if len(a) != len(b) {
			return false
		}

		for _, aa := range a {
			found := false
			for _, bb := range b {
				if aa.Equal(bb) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}

	a1 := m("/ip4/1.2.3.4/tcp/1231")
	a2 := m("/ip4/1.2.3.4/tcp/1232")
	a3 := m("/ip4/1.2.3.4/tcp/1233")
	a4 := m("/ip4/1.2.3.4/tcp/1234")
	a5 := m("/ip4/1.2.3.4/tcp/1235")

	b1 := m("/ip4/1.2.3.6/tcp/1236")
	b2 := m("/ip4/1.2.3.7/tcp/1237")
	b3 := m("/ip4/1.2.3.8/tcp/1237")
	b4 := m("/ip4/1.2.3.9/tcp/1237")
	b5 := m("/ip4/1.2.3.10/tcp/1237")

	oas := &ObservedAddrSet{}

	if !addrsMarch(oas.Addrs(), nil) {
		t.Error("addrs should be empty")
	}

	add := func(oas *ObservedAddrSet, observed, observer ma.Multiaddr) {
		dummyLocal := m("/ip4/127.0.0.1/tcp/10086")
		dummyDirection := net.DirOutbound

		oas.Add(observed, dummyLocal, observer, dummyDirection)
	}

	add(oas, a1, a4)
	add(oas, a2, a4)
	add(oas, a3, a4)

	// these are all different so we should not yet get them.
	if !addrsMarch(oas.Addrs(), nil) {
		t.Error("addrs should _still_ be empty (once)")
	}

	// same observer, so should not yet get them.
	add(oas, a1, a4)
	add(oas, a2, a4)
	add(oas, a3, a4)
	if !addrsMarch(oas.Addrs(), nil) {
		t.Error("addrs should _still_ be empty (same obs)")
	}

	// different observer, but same observer group.
	add(oas, a1, a5)
	add(oas, a2, a5)
	add(oas, a3, a5)
	if !addrsMarch(oas.Addrs(), nil) {
		t.Error("addrs should _still_ be empty (same obs group)")
	}

	add(oas, a1, b1)
	add(oas, a1, b2)
	add(oas, a1, b3)
	if !addrsMarch(oas.Addrs(), []ma.Multiaddr{a1}) {
		t.Error("addrs should only have a1")
	}

	add(oas, a2, a5)
	add(oas, a1, a5)
	add(oas, a1, a5)
	add(oas, a2, b1)
	add(oas, a1, b1)
	add(oas, a1, b1)
	add(oas, a2, b2)
	add(oas, a1, b2)
	add(oas, a1, b2)
	add(oas, a2, b4)
	add(oas, a2, b5)
	if !addrsMarch(oas.Addrs(), []ma.Multiaddr{a1, a2}) {
		t.Error("addrs should only have a1, a2")
	}

	// change the timeout constant so we can time it out.
	oas.SetTTL(time.Millisecond * 200)
	<-time.After(time.Millisecond * 210)
	if !addrsMarch(oas.Addrs(), nil) {
		t.Error("addrs should have timed out")
	}
}
