package offline

import (
	"context"
	"testing"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	blocks "gx/ipfs/QmRcHuYzAyswytBuMF78rj3LTChYszomRFXNg4685ZN1WM/go-block-format"
	blocksutil "gx/ipfs/QmUf9xAmbHndjh31YNRugNeyNX6mDTUDuWN1bVTmUriwdZ/go-ipfs-blocksutil"
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	ds_sync "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/sync"
	blockstore "gx/ipfs/QmcDDgAXDbpDUpadCJKLr49KYR4HuL7T8Z1dZTHt6ixsoR/go-ipfs-blockstore"
)

func TestBlockReturnsErr(t *testing.T) {
	off := Exchange(bstore())
	c := cid.NewCidV0(u.Hash([]byte("foo")))
	_, err := off.GetBlock(context.Background(), c)
	if err != nil {
		return // as desired
	}
	t.Fail()
}

func TestHasBlockReturnsNil(t *testing.T) {
	store := bstore()
	ex := Exchange(store)
	block := blocks.NewBlock([]byte("data"))

	err := ex.HasBlock(block)
	if err != nil {
		t.Fail()
	}

	if _, err := store.Get(block.Cid()); err != nil {
		t.Fatal(err)
	}
}

func TestGetBlocks(t *testing.T) {
	store := bstore()
	ex := Exchange(store)
	g := blocksutil.NewBlockGenerator()

	expected := g.Blocks(2)

	for _, b := range expected {
		if err := ex.HasBlock(b); err != nil {
			t.Fail()
		}
	}

	request := func() []cid.Cid {
		var ks []cid.Cid

		for _, b := range expected {
			ks = append(ks, b.Cid())
		}
		return ks
	}()

	received, err := ex.GetBlocks(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for range received {
		count++
	}
	if len(expected) != count {
		t.Fail()
	}
}

func bstore() blockstore.Blockstore {
	return blockstore.NewBlockstore(ds_sync.MutexWrap(ds.NewMapDatastore()))
}
