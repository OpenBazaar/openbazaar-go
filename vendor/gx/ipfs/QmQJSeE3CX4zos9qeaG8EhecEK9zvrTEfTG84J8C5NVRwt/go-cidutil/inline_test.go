package cidutil

import (
	"math/rand"
	"testing"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mhash "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
)

func TestInlineBuilderSmallValue(t *testing.T) {
	builder := InlineBuilder{cid.V0Builder{}, 64}
	c, err := builder.Sum([]byte("Hello World"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Prefix().MhType != mhash.ID {
		t.Fatal("Inliner builder failed to use ID Multihash on small values")
	}
}

func TestInlinerBuilderLargeValue(t *testing.T) {
	builder := InlineBuilder{cid.V0Builder{}, 64}
	data := make([]byte, 512)
	rand.Read(data)
	c, err := builder.Sum(data)
	if err != nil {
		t.Fatal(err)
	}
	if c.Prefix().MhType == mhash.ID {
		t.Fatal("Inliner builder used ID Multihash on large values")
	}
}
