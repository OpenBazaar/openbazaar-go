package merkledag_test

import (
	"context"
	"testing"

	. "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag"
	dstest "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag/test"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
)

func TestReadonlyProperties(t *testing.T) {
	ds := dstest.Mock()
	ro := NewReadOnlyDagService(ds)

	ctx := context.Background()
	nds := []ipld.Node{
		NewRawNode([]byte("foo1")),
		NewRawNode([]byte("foo2")),
		NewRawNode([]byte("foo3")),
		NewRawNode([]byte("foo4")),
	}
	cids := []cid.Cid{
		nds[0].Cid(),
		nds[1].Cid(),
		nds[2].Cid(),
		nds[3].Cid(),
	}

	// add to the actual underlying datastore
	if err := ds.Add(ctx, nds[2]); err != nil {
		t.Fatal(err)
	}
	if err := ds.Add(ctx, nds[3]); err != nil {
		t.Fatal(err)
	}

	if err := ro.Add(ctx, nds[0]); err != ErrReadOnly {
		t.Fatal("expected ErrReadOnly")
	}
	if err := ro.Add(ctx, nds[2]); err != ErrReadOnly {
		t.Fatal("expected ErrReadOnly")
	}

	if err := ro.AddMany(ctx, nds[0:1]); err != ErrReadOnly {
		t.Fatal("expected ErrReadOnly")
	}

	if err := ro.Remove(ctx, cids[3]); err != ErrReadOnly {
		t.Fatal("expected ErrReadOnly")
	}
	if err := ro.RemoveMany(ctx, cids[1:2]); err != ErrReadOnly {
		t.Fatal("expected ErrReadOnly")
	}

	if _, err := ro.Get(ctx, cids[0]); err != ipld.ErrNotFound {
		t.Fatal("expected ErrNotFound")
	}
	if _, err := ro.Get(ctx, cids[3]); err != nil {
		t.Fatal(err)
	}
}
