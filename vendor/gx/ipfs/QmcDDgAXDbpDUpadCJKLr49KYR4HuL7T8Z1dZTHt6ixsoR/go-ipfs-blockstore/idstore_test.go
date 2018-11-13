package blockstore

import (
	"context"
	"testing"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	blk "gx/ipfs/QmRcHuYzAyswytBuMF78rj3LTChYszomRFXNg4685ZN1WM/go-block-format"
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
)

func createTestStores() (Blockstore, *callbackDatastore) {
	cd := &callbackDatastore{f: func() {}, ds: ds.NewMapDatastore()}
	ids := NewIdStore(NewBlockstore(cd))
	return ids, cd
}

func TestIdStore(t *testing.T) {
	idhash1, _ := cid.NewPrefixV1(cid.Raw, mh.ID).Sum([]byte("idhash1"))
	idblock1, _ := blk.NewBlockWithCid([]byte("idhash1"), idhash1)
	hash1, _ := cid.NewPrefixV1(cid.Raw, mh.SHA2_256).Sum([]byte("hash1"))
	block1, _ := blk.NewBlockWithCid([]byte("hash1"), hash1)
	emptyHash, _ := cid.NewPrefixV1(cid.Raw, mh.SHA2_256).Sum([]byte("emptyHash"))
	emptyBlock, _ := blk.NewBlockWithCid([]byte{}, emptyHash)

	ids, cb := createTestStores()

	have, _ := ids.Has(idhash1)
	if !have {
		t.Fatal("Has() failed on idhash")
	}

	_, err := ids.Get(idhash1)
	if err != nil {
		t.Fatalf("Get() failed on idhash: %v", err)
	}

	noop := func() {}
	failIfPassThough := func() {
		t.Fatal("operation on identity hash passed though to datastore")
	}

	cb.f = failIfPassThough
	err = ids.Put(idblock1)
	if err != nil {
		t.Fatal(err)
	}

	cb.f = noop
	err = ids.Put(block1)
	if err != nil {
		t.Fatalf("Put() failed on normal block: %v", err)
	}

	have, _ = ids.Has(hash1)
	if !have {
		t.Fatal("normal block not added to datastore")
	}

	blockSize, _ := ids.GetSize(hash1)
	if blockSize == -1 {
		t.Fatal("normal block not added to datastore")
	}

	_, err = ids.Get(hash1)
	if err != nil {
		t.Fatal(err)
	}

	err = ids.Put(emptyBlock)
	if err != nil {
		t.Fatalf("Put() failed on normal block: %v", err)
	}

	have, _ = ids.Has(emptyHash)
	if !have {
		t.Fatal("normal block not added to datastore")
	}

	blockSize, _ = ids.GetSize(emptyHash)
	if blockSize != 0 {
		t.Fatal("normal block not added to datastore")
	}

	cb.f = failIfPassThough
	err = ids.DeleteBlock(idhash1)
	if err != nil {
		t.Fatal(err)
	}

	cb.f = noop
	err = ids.DeleteBlock(hash1)
	if err != nil {
		t.Fatal(err)
	}

	have, _ = ids.Has(hash1)
	if have {
		t.Fatal("normal block not deleted from datastore")
	}

	blockSize, _ = ids.GetSize(hash1)
	if blockSize > -1 {
		t.Fatal("normal block not deleted from datastore")
	}

	err = ids.DeleteBlock(emptyHash)
	if err != nil {
		t.Fatal(err)
	}

	idhash2, _ := cid.NewPrefixV1(cid.Raw, mh.ID).Sum([]byte("idhash2"))
	idblock2, _ := blk.NewBlockWithCid([]byte("idhash2"), idhash2)
	hash2, _ := cid.NewPrefixV1(cid.Raw, mh.SHA2_256).Sum([]byte("hash2"))
	block2, _ := blk.NewBlockWithCid([]byte("hash2"), hash2)

	cb.f = failIfPassThough
	err = ids.PutMany([]blk.Block{idblock1, idblock2})
	if err != nil {
		t.Fatal(err)
	}

	opCount := 0
	cb.f = func() {
		opCount++
	}

	err = ids.PutMany([]blk.Block{block1, block2})
	if err != nil {
		t.Fatal(err)
	}
	if opCount != 4 {
		// one call to Has and Put for each Cid
		t.Fatalf("expected exactly 4 operations got %d", opCount)
	}

	opCount = 0
	err = ids.PutMany([]blk.Block{idblock1, block1})
	if err != nil {
		t.Fatal(err)
	}
	if opCount != 1 {
		// just one call to Put from the normal (non-id) block
		t.Fatalf("expected exactly 1 operations got %d", opCount)
	}

	ch, err := ids.AllKeysChan(context.TODO())
	cnt := 0
	for c := range ch {
		cnt++
		if c.Prefix().MhType == mh.ID {
			t.Fatalf("block with identity hash found in blockstore")
		}
	}
	if cnt != 2 {
		t.Fatalf("expected exactly two keys returned by AllKeysChan got %d", cnt)
	}
}
