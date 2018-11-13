package manet

import (
	"testing"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
)

func TestIsPublicAddr(t *testing.T) {
	a, err := ma.NewMultiaddr("/ip4/192.168.1.1/tcp/80")
	if err != nil {
		t.Fatal(err)
	}

	if IsPublicAddr(a) {
		t.Fatal("192.168.1.1 is not a public address!")
	}

	if !IsPrivateAddr(a) {
		t.Fatal("192.168.1.1 is a private address!")
	}

	a, err = ma.NewMultiaddr("/ip4/1.1.1.1/tcp/80")
	if err != nil {
		t.Fatal(err)
	}

	if !IsPublicAddr(a) {
		t.Fatal("1.1.1.1 is a public address!")
	}

	if IsPrivateAddr(a) {
		t.Fatal("1.1.1.1 is not a private address!")
	}

	a, err = ma.NewMultiaddr("/tcp/80/ip4/1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}

	if IsPublicAddr(a) {
		t.Fatal("shouldn't consider an address that starts with /tcp/ as *public*")
	}

	if IsPrivateAddr(a) {
		t.Fatal("shouldn't consider an address that starts with /tcp/ as *private*")
	}
}
