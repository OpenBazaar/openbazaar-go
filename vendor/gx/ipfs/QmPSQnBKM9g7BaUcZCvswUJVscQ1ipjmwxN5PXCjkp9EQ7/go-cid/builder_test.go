package cid

import (
	"testing"

	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
)

func TestV0Builder(t *testing.T) {
	data := []byte("this is some test content")

	// Construct c1
	format := V0Builder{}
	c1, err := format.Sum(data)
	if err != nil {
		t.Fatal(err)
	}

	// Construct c2
	hash, err := mh.Sum(data, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	c2 := NewCidV0(hash)

	if !c1.Equals(c2) {
		t.Fatal("cids mismatch")
	}
	if c1.Prefix() != c2.Prefix() {
		t.Fatal("prefixes mismatch")
	}
}

func TestV1Builder(t *testing.T) {
	data := []byte("this is some test content")

	// Construct c1
	format := V1Builder{Codec: DagCBOR, MhType: mh.SHA2_256}
	c1, err := format.Sum(data)
	if err != nil {
		t.Fatal(err)
	}

	// Construct c2
	hash, err := mh.Sum(data, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	c2 := NewCidV1(DagCBOR, hash)

	if !c1.Equals(c2) {
		t.Fatal("cids mismatch")
	}
	if c1.Prefix() != c2.Prefix() {
		t.Fatal("prefixes mismatch")
	}
}

func TestCodecChange(t *testing.T) {
	t.Run("Prefix-CidV0", func(t *testing.T) {
		p := Prefix{Version: 0, Codec: DagProtobuf, MhType: mh.SHA2_256, MhLength: mh.DefaultLengths[mh.SHA2_256]}
		testCodecChange(t, p)
	})
	t.Run("Prefix-CidV1", func(t *testing.T) {
		p := Prefix{Version: 1, Codec: DagProtobuf, MhType: mh.SHA2_256, MhLength: mh.DefaultLengths[mh.SHA2_256]}
		testCodecChange(t, p)
	})
	t.Run("V0Builder", func(t *testing.T) {
		testCodecChange(t, V0Builder{})
	})
	t.Run("V1Builder", func(t *testing.T) {
		testCodecChange(t, V1Builder{Codec: DagProtobuf, MhType: mh.SHA2_256})
	})
}

func testCodecChange(t *testing.T, b Builder) {
	data := []byte("this is some test content")

	if b.GetCodec() != DagProtobuf {
		t.Fatal("original builder not using Protobuf codec")
	}

	b = b.WithCodec(Raw)
	c, err := b.Sum(data)
	if err != nil {
		t.Fatal(err)
	}

	if c.Type() != Raw {
		t.Fatal("new cid codec did not change to Raw")
	}
}
