package offline

import (
	"bytes"
	"context"
	"testing"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	testutil "gx/ipfs/Qma6ESRQTf1ZLPgzpCwDTqQJefPnU6uLvMjP18vK8EWp8L/go-testutil"
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	ropt "gx/ipfs/QmcQ81jSyWCp1jpkQ8CMbtpXT3jK7Wg6ZtYmoyWFgBoF9c/go-libp2p-routing/options"
)

type blankValidator struct{}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

func TestOfflineRouterStorage(t *testing.T) {
	ctx := context.Background()

	nds := ds.NewMapDatastore()
	offline := NewOfflineRouter(nds, blankValidator{})

	if err := offline.PutValue(ctx, "key", []byte("testing 1 2 3")); err != nil {
		t.Fatal(err)
	}

	val, err := offline.GetValue(ctx, "key")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte("testing 1 2 3"), val) {
		t.Fatal("OfflineRouter does not properly store")
	}

	_, err = offline.GetValue(ctx, "notHere")
	if err == nil {
		t.Fatal("Router should throw errors for unfound records")
	}

	local, err := offline.GetValue(ctx, "key", ropt.Offline)
	if err != nil {
		t.Fatal(err)
	}

	_, err = offline.GetValue(ctx, "notHere", ropt.Offline)
	if err == nil {
		t.Fatal("Router should throw errors for unfound records")
	}

	if !bytes.Equal([]byte("testing 1 2 3"), local) {
		t.Fatal("OfflineRouter does not properly store")
	}
}

func TestOfflineRouterLocal(t *testing.T) {
	ctx := context.Background()

	nds := ds.NewMapDatastore()
	offline := NewOfflineRouter(nds, blankValidator{})

	id, _ := testutil.RandPeerID()
	_, err := offline.FindPeer(ctx, id)
	if err != ErrOffline {
		t.Fatal("OfflineRouting should alert that its offline")
	}

	h, _ := mh.Sum([]byte("test data1"), mh.SHA2_256, -1)
	c1 := cid.NewCidV0(h)
	pChan := offline.FindProvidersAsync(ctx, c1, 1)
	p, ok := <-pChan
	if ok {
		t.Fatalf("FindProvidersAsync did not return a closed channel. Instead we got %+v !", p)
	}

	h2, _ := mh.Sum([]byte("test data1"), mh.SHA2_256, -1)
	c2 := cid.NewCidV0(h2)
	err = offline.Provide(ctx, c2, true)
	if err != ErrOffline {
		t.Fatal("OfflineRouting should alert that its offline")
	}

	err = offline.Bootstrap(ctx)
	if err != nil {
		t.Fatal("You shouldn't be able to bootstrap offline routing.")
	}
}
