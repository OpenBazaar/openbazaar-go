package multistore

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"go.uber.org/multierr"
	"golang.org/x/xerrors"

	"github.com/ipfs/go-datastore"
	ktds "github.com/ipfs/go-datastore/keytransform"
	"github.com/ipfs/go-datastore/query"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
)

// StoreID identifies a unique instance of a store
type StoreID uint64

// MultiStore is a wrapper around a datastore that provides multiple isolated
// instances of IPFS storage components -> BlockStore, FileStore, DAGService, etc
type MultiStore struct {
	ds datastore.Batching

	open map[StoreID]*Store
	next StoreID

	lk sync.RWMutex
}

var dsListKey = datastore.NewKey("/list")
var dsMultiKey = datastore.NewKey("/multi")

// NewMultiDstore returns a new instance of a MultiStore for the given datastore
// instance
func NewMultiDstore(ds datastore.Batching) (*MultiStore, error) {
	listBytes, err := ds.Get(dsListKey)
	if xerrors.Is(err, datastore.ErrNotFound) {
		listBytes, _ = json.Marshal(StoreIDList{})
	} else if err != nil {
		return nil, xerrors.Errorf("could not read multistore list: %w", err)
	}

	var ids StoreIDList
	if err := json.Unmarshal(listBytes, &ids); err != nil {
		return nil, xerrors.Errorf("could not unmarshal multistore list: %w", err)
	}

	mds := &MultiStore{
		ds:   ds,
		open: map[StoreID]*Store{},
	}

	for _, i := range ids {
		if i > mds.next {
			mds.next = i
		}

		_, err := mds.Get(i)
		if err != nil {
			return nil, xerrors.Errorf("open store %d: %w", i, err)
		}
	}

	return mds, nil
}

// Next returns the next available StoreID
func (mds *MultiStore) Next() StoreID {
	mds.lk.Lock()
	defer mds.lk.Unlock()

	mds.next++
	return mds.next
}

func (mds *MultiStore) updateStores() error {
	stores := make(StoreIDList, 0, len(mds.open))
	for k := range mds.open {
		stores = append(stores, k)
	}
	sort.Sort(stores)

	listBytes, err := json.Marshal(stores)
	if err != nil {
		return xerrors.Errorf("could not marshal list: %w", err)
	}
	err = mds.ds.Put(dsListKey, listBytes)
	if err != nil {
		return xerrors.Errorf("could not save stores list: %w", err)
	}
	return nil
}

// Get returns the store for the given ID
func (mds *MultiStore) Get(i StoreID) (*Store, error) {
	mds.lk.Lock()
	defer mds.lk.Unlock()

	store, ok := mds.open[i]
	if ok {
		return store, nil
	}

	wds := ktds.Wrap(mds.ds, ktds.PrefixTransform{
		Prefix: dsMultiKey.ChildString(fmt.Sprintf("%d", i)),
	})

	var err error
	mds.open[i], err = openStore(wds)
	if err != nil {
		return nil, xerrors.Errorf("could not open new store: %w", err)
	}

	err = mds.updateStores()
	if err != nil {
		return nil, xerrors.Errorf("updating stores: %w", err)
	}

	return mds.open[i], nil
}

// List returns a list of all known store IDs
func (mds *MultiStore) List() StoreIDList {
	mds.lk.RLock()
	defer mds.lk.RUnlock()

	out := make(StoreIDList, 0, len(mds.open))
	for i := range mds.open {
		out = append(out, i)
	}
	sort.Sort(out)

	return out
}

// Delete deletes the store with the given id, including all of its data
func (mds *MultiStore) Delete(i StoreID) error {
	mds.lk.Lock()
	defer mds.lk.Unlock()

	store, ok := mds.open[i]
	if !ok {
		return nil
	}
	delete(mds.open, i)
	err := store.Close()
	if err != nil {
		return xerrors.Errorf("closing store: %w", err)
	}

	err = mds.updateStores()
	if err != nil {
		return xerrors.Errorf("updating stores: %w", err)
	}

	qres, err := store.ds.Query(query.Query{KeysOnly: true})
	if err != nil {
		return xerrors.Errorf("query error: %w", err)
	}
	defer qres.Close() //nolint:errcheck

	b, err := store.ds.Batch()
	if err != nil {
		return xerrors.Errorf("batch error: %w", err)
	}

	for r := range qres.Next() {
		if r.Error != nil {
			_ = b.Commit()
			return xerrors.Errorf("iterator error: %w", err)
		}
		err := b.Delete(datastore.NewKey(r.Key))
		if err != nil {
			_ = b.Commit()
			return xerrors.Errorf("adding to batch: %w", err)
		}
	}

	err = b.Commit()
	if err != nil {
		return xerrors.Errorf("committing: %w", err)
	}

	return nil
}

// Close closes all open datastores
func (mds *MultiStore) Close() error {
	mds.lk.Lock()
	defer mds.lk.Unlock()

	var err error
	for _, s := range mds.open {
		err = multierr.Append(err, s.Close())
	}
	mds.open = make(map[StoreID]*Store)

	return err
}

// MultiReadBlockstore returns a single Blockstore that will try to read from
// all of the blockstores tracked by this multistore
func (mds *MultiStore) MultiReadBlockstore() blockstore.Blockstore {
	return &multiReadBs{mds}
}

// StoreIDList is just a list of StoreID that implements sort.Interface
type StoreIDList []StoreID

func (s StoreIDList) Len() int {
	return len(s)
}

func (s StoreIDList) Less(i, j int) bool {
	return s[i] < s[j]
}

func (s StoreIDList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
