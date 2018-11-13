package format

import (
	"errors"
	"testing"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	blocks "gx/ipfs/QmRcHuYzAyswytBuMF78rj3LTChYszomRFXNg4685ZN1WM/go-block-format"
)

func init() {
	Register(cid.Raw, func(b blocks.Block) (Node, error) {
		node := &EmptyNode{}
		if b.RawData() != nil || !b.Cid().Equals(node.Cid()) {
			return nil, errors.New("can only decode empty blocks")
		}
		return node, nil
	})
}

func TestDecode(t *testing.T) {
	id, err := cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   mh.ID,
		MhLength: 0,
	}.Sum(nil)

	if err != nil {
		t.Fatalf("failed to create cid: %s", err)
	}

	block, err := blocks.NewBlockWithCid(nil, id)
	if err != nil {
		t.Fatalf("failed to create empty block: %s", err)
	}
	node, err := Decode(block)
	if err != nil {
		t.Fatalf("failed to decode empty node: %s", err)
	}
	if !node.Cid().Equals(id) {
		t.Fatalf("empty node doesn't have the right cid")
	}

	if _, ok := node.(*EmptyNode); !ok {
		t.Fatalf("empty node doesn't have the right type")
	}

}
