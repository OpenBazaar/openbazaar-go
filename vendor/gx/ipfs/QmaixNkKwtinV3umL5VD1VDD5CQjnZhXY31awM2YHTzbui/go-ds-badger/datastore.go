package badger

import (
	"fmt"
	"strings"
	"time"

	osh "gx/ipfs/QmXuBJ7DR6k3rmUEKtvVMhwjmXDuJgXXPUt4LQXKBMsU93/go-os-helper"
	badger "gx/ipfs/QmZ7bFqkoHU2ARF68y9fSQVKcmhjYrTQgtCQ4i3chwZCgQ/badger"

	goprocess "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	dsq "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/query"
)

type Datastore struct {
	DB *badger.DB

	gcDiscardRatio float64
}

// Implements the datastore.Txn interface, enabling transaction support for
// the badger Datastore.
type txn struct {
	txn *badger.Txn

	// Whether this transaction has been implicitly created as a result of a direct Datastore
	// method invocation.
	implicit bool
}

// Options are the badger datastore options, reexported here for convenience.
type Options struct {
	gcDiscardRatio float64

	badger.Options
}

var DefaultOptions = Options{
	gcDiscardRatio: 0.1,

	Options: badger.DefaultOptions,
}

var _ ds.Datastore = (*Datastore)(nil)
var _ ds.TxnDatastore = (*Datastore)(nil)
var _ ds.TTLDatastore = (*Datastore)(nil)

// NewDatastore creates a new badger datastore.
//
// DO NOT set the Dir and/or ValuePath fields of opt, they will be set for you.
func NewDatastore(path string, options *Options) (*Datastore, error) {
	// Copy the options because we modify them.
	var opt badger.Options
	var gcDiscardRatio float64
	if options == nil {
		opt = badger.DefaultOptions
		gcDiscardRatio = DefaultOptions.gcDiscardRatio
	} else {
		opt = options.Options
		gcDiscardRatio = options.gcDiscardRatio
	}

	if osh.IsWindows() && opt.SyncWrites {
		opt.Truncate = true
	}

	opt.Dir = path
	opt.ValueDir = path

	kv, err := badger.Open(opt)
	if err != nil {
		if strings.HasPrefix(err.Error(), "manifest has unsupported version:") {
			err = fmt.Errorf("unsupported badger version, use github.com/ipfs/badgerds-upgrade to upgrade: %s", err.Error())
		}
		return nil, err
	}

	return &Datastore{
		DB:             kv,
		gcDiscardRatio: gcDiscardRatio,
	}, nil
}

// NewTransaction starts a new transaction. The resulting transaction object
// can be mutated without incurring changes to the underlying Datastore until
// the transaction is Committed.
func (d *Datastore) NewTransaction(readOnly bool) (ds.Txn, error) {
	return &txn{d.DB.NewTransaction(!readOnly), false}, nil
}

// newImplicitTransaction creates a transaction marked as 'implicit'.
// Implicit transactions are created by Datastore methods performing single operations.
func (d *Datastore) newImplicitTransaction(readOnly bool) ds.Txn {
	return &txn{d.DB.NewTransaction(!readOnly), true}
}

func (d *Datastore) Put(key ds.Key, value []byte) error {
	txn := d.newImplicitTransaction(false)
	defer txn.Discard()

	if err := txn.Put(key, value); err != nil {
		return err
	}

	return txn.Commit()
}

func (d *Datastore) PutWithTTL(key ds.Key, value []byte, ttl time.Duration) error {
	txn := d.newImplicitTransaction(false).(*txn)
	defer txn.Discard()

	if err := txn.PutWithTTL(key, value, ttl); err != nil {
		return err
	}

	return txn.Commit()
}

func (d *Datastore) SetTTL(key ds.Key, ttl time.Duration) error {
	txn := d.newImplicitTransaction(false).(*txn)
	defer txn.Discard()

	if err := txn.SetTTL(key, ttl); err != nil {
		return err
	}

	return txn.Commit()
}

func (d *Datastore) GetExpiration(key ds.Key) (time.Time, error) {
	txn := d.newImplicitTransaction(false).(*txn)
	defer txn.Discard()

	return txn.GetExpiration(key)
}

func (d *Datastore) Get(key ds.Key) (value []byte, err error) {
	txn := d.newImplicitTransaction(true)
	defer txn.Discard()

	return txn.Get(key)
}

func (d *Datastore) Has(key ds.Key) (bool, error) {
	txn := d.newImplicitTransaction(true)
	defer txn.Discard()

	return txn.Has(key)
}

func (d *Datastore) GetSize(key ds.Key) (size int, err error) {
	txn := d.newImplicitTransaction(true)
	defer txn.Discard()

	return txn.GetSize(key)
}

func (d *Datastore) Delete(key ds.Key) error {
	txn := d.newImplicitTransaction(false)
	defer txn.Discard()

	err := txn.Delete(key)
	if err != nil {
		return err
	}

	return txn.Commit()
}

func (d *Datastore) Query(q dsq.Query) (dsq.Results, error) {
	txn := d.newImplicitTransaction(true)
	// We cannot defer txn.Discard() here, as the txn must remain active while the iterator is open.
	// https://github.com/dgraph-io/badger/commit/b1ad1e93e483bbfef123793ceedc9a7e34b09f79
	// The closing logic in the query goprocess takes care of discarding the implicit transaction.
	return txn.Query(q)
}

// DiskUsage implements the PersistentDatastore interface.
// It returns the sum of lsm and value log files sizes in bytes.
func (d *Datastore) DiskUsage() (uint64, error) {
	lsm, vlog := d.DB.Size()
	return uint64(lsm + vlog), nil
}

func (d *Datastore) Close() error {
	return d.DB.Close()
}

func (d *Datastore) IsThreadSafe() {}

func (d *Datastore) Batch() (ds.Batch, error) {
	tx, _ := d.NewTransaction(false)
	return tx, nil
}

func (d *Datastore) CollectGarbage() error {
	err := d.DB.RunValueLogGC(d.gcDiscardRatio)
	if err == badger.ErrNoRewrite {
		err = nil
	}
	return err
}

func (t *txn) Put(key ds.Key, value []byte) error {
	return t.txn.Set(key.Bytes(), value)
}

func (t *txn) PutWithTTL(key ds.Key, value []byte, ttl time.Duration) error {
	return t.txn.SetWithTTL(key.Bytes(), value, ttl)
}

func (t *txn) GetExpiration(key ds.Key) (time.Time, error) {
	item, err := t.txn.Get(key.Bytes())
	if err == badger.ErrKeyNotFound {
		return time.Time{}, ds.ErrNotFound
	} else if err != nil {
		return time.Time{}, err
	}
	return time.Unix(int64(item.ExpiresAt()), 0), nil
}

func (t *txn) SetTTL(key ds.Key, ttl time.Duration) error {
	data, err := t.Get(key)
	if err != nil {
		return err
	}

	return t.PutWithTTL(key, data, ttl)
}

func (t *txn) Get(key ds.Key) ([]byte, error) {
	item, err := t.txn.Get(key.Bytes())
	if err == badger.ErrKeyNotFound {
		err = ds.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return item.ValueCopy(nil)
}

func (t *txn) Has(key ds.Key) (bool, error) {
	_, err := t.txn.Get(key.Bytes())
	switch err {
	case badger.ErrKeyNotFound:
		return false, nil
	case nil:
		return true, nil
	default:
		return false, err
	}
}

func (t *txn) GetSize(key ds.Key) (int, error) {
	item, err := t.txn.Get(key.Bytes())
	switch err {
	case nil:
		return int(item.ValueSize()), nil
	case badger.ErrKeyNotFound:
		return -1, ds.ErrNotFound
	default:
		return -1, err
	}
}

func (t *txn) Delete(key ds.Key) error {
	return t.txn.Delete(key.Bytes())
}

func (t *txn) Query(q dsq.Query) (dsq.Results, error) {
	prefix := []byte(q.Prefix)
	opt := badger.DefaultIteratorOptions
	opt.PrefetchValues = !q.KeysOnly

	txn := t.txn

	it := txn.NewIterator(opt)
	it.Seek([]byte(q.Prefix))
	if q.Offset > 0 {
		for j := 0; j < q.Offset; j++ {
			it.Next()
		}
	}

	qrb := dsq.NewResultBuilder(q)

	qrb.Process.Go(func(worker goprocess.Process) {
		if t.implicit {
			// this iterator is part of an implicit transaction, so when we're done we must discard
			// the transaction. It's safe to discard the txn it because it contains the iterator only.
			defer t.Discard()
		}
		defer it.Close()

		for sent := 0; it.ValidForPrefix(prefix); sent++ {
			if qrb.Query.Limit > 0 && sent >= qrb.Query.Limit {
				break
			}

			item := it.Item()

			k := string(item.Key())
			e := dsq.Entry{Key: k}

			var result dsq.Result
			if !q.KeysOnly {
				b, err := item.ValueCopy(nil)
				if err != nil {
					result = dsq.Result{Error: err}
				} else {
					e.Value = b
					result = dsq.Result{Entry: e}
				}
			} else {
				result = dsq.Result{Entry: e}
			}

			if q.ReturnExpirations {
				result.Expiration = time.Unix(int64(item.ExpiresAt()), 0)
			}

			select {
			case qrb.Output <- result:
			case <-worker.Closing(): // client told us to close early
				return
			}
			it.Next()
		}

		return
	})

	go qrb.Process.CloseAfterChildren()

	// Now, apply remaining things (filters, order)
	qr := qrb.Results()
	for _, f := range q.Filters {
		qr = dsq.NaiveFilter(qr, f)
	}
	for _, o := range q.Orders {
		qr = dsq.NaiveOrder(qr, o)
	}

	return qr, nil
}

func (t *txn) Commit() error {
	return t.txn.Commit(nil)
}

func (t *txn) Discard() {
	t.txn.Discard()
}
