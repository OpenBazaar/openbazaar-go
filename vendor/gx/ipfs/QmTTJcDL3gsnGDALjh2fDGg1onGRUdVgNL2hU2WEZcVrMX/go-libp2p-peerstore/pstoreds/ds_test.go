package pstoreds

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	badger "gx/ipfs/QmaixNkKwtinV3umL5VD1VDD5CQjnZhXY31awM2YHTzbui/go-ds-badger"
	leveldb "gx/ipfs/QmccqjKZUTqp4ikWNyAbjBuP5HEdqSqRuAr9mcEhYab54a/go-ds-leveldb"

	pstore "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
	pt "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore/test"
)

type datastoreFactory func(tb testing.TB) (ds.TxnDatastore, func())

var dstores = map[string]datastoreFactory{
	"Badger": badgerStore,
	// TODO: Enable once go-ds-leveldb supports TTL via a shim.
	// "Leveldb": leveldbStore,
}

func TestDsPeerstore(t *testing.T) {
	for name, dsFactory := range dstores {
		t.Run(name, func(t *testing.T) {
			pt.TestPeerstore(t, peerstoreFactory(t, dsFactory, DefaultOpts()))
		})
	}
}

func TestDsAddrBook(t *testing.T) {
	for name, dsFactory := range dstores {
		t.Run(name, func(t *testing.T) {
			t.Run("Cacheful", func(t *testing.T) {
				t.Parallel()

				opts := DefaultOpts()
				opts.TTLInterval = 100 * time.Microsecond
				opts.CacheSize = 1024

				pt.TestAddrBook(t, addressBookFactory(t, dsFactory, opts))
			})

			t.Run("Cacheless", func(t *testing.T) {
				t.Parallel()

				opts := DefaultOpts()
				opts.TTLInterval = 100 * time.Microsecond
				opts.CacheSize = 0

				pt.TestAddrBook(t, addressBookFactory(t, dsFactory, opts))
			})
		})
	}
}

func TestDsKeyBook(t *testing.T) {
	for name, dsFactory := range dstores {
		t.Run(name, func(t *testing.T) {
			pt.TestKeyBook(t, keyBookFactory(t, dsFactory, DefaultOpts()))
		})
	}
}

func BenchmarkDsPeerstore(b *testing.B) {
	caching := DefaultOpts()
	caching.CacheSize = 1024

	cacheless := DefaultOpts()
	cacheless.CacheSize = 0

	for name, dsFactory := range dstores {
		b.Run(name, func(b *testing.B) {
			pt.BenchmarkPeerstore(b, peerstoreFactory(b, dsFactory, caching), "Caching")
			pt.BenchmarkPeerstore(b, peerstoreFactory(b, dsFactory, cacheless), "Cacheless")
		})
	}
}

func badgerStore(tb testing.TB) (ds.TxnDatastore, func()) {
	dataPath, err := ioutil.TempDir(os.TempDir(), "badger")
	if err != nil {
		tb.Fatal(err)
	}
	store, err := badger.NewDatastore(dataPath, nil)
	if err != nil {
		tb.Fatal(err)
	}
	closer := func() {
		store.Close()
		os.RemoveAll(dataPath)
	}
	return store, closer
}

func leveldbStore(tb testing.TB) (ds.TxnDatastore, func()) {
	dataPath, err := ioutil.TempDir(os.TempDir(), "leveldb")
	if err != nil {
		tb.Fatal(err)
	}
	store, err := leveldb.NewDatastore(dataPath, nil)
	if err != nil {
		tb.Fatal(err)
	}
	closer := func() {
		store.Close()
		os.RemoveAll(dataPath)
	}
	return store, closer
}

func peerstoreFactory(tb testing.TB, storeFactory datastoreFactory, opts Options) pt.PeerstoreFactory {
	return func() (pstore.Peerstore, func()) {
		store, closeFunc := storeFactory(tb)

		ps, err := NewPeerstore(context.Background(), store, opts)
		if err != nil {
			tb.Fatal(err)
		}

		return ps, closeFunc
	}
}

func addressBookFactory(tb testing.TB, storeFactory datastoreFactory, opts Options) pt.AddrBookFactory {
	return func() (pstore.AddrBook, func()) {
		store, closeFunc := storeFactory(tb)

		ab, err := NewAddrBook(context.Background(), store, opts)
		if err != nil {
			tb.Fatal(err)
		}

		return ab, closeFunc
	}
}

func keyBookFactory(tb testing.TB, storeFactory datastoreFactory, opts Options) pt.KeyBookFactory {
	return func() (pstore.KeyBook, func()) {
		store, closeFunc := storeFactory(tb)

		kb, err := NewKeyBook(context.Background(), store, opts)
		if err != nil {
			tb.Fatal(err)
		}

		return kb, closeFunc
	}
}
