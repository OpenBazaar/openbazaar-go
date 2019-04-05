package leveldb

import (
	"os"
	"path/filepath"

	"gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	ds "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	dsq "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore/query"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/errors"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/iterator"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/opt"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/storage"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/util"
)

type Datastore struct {
	*accessor
	DB   *leveldb.DB
	path string
}

var _ ds.Datastore = (*Datastore)(nil)
var _ ds.TxnDatastore = (*Datastore)(nil)

// Options is an alias of syndtr/goleveldb/opt.Options which might be extended
// in the future.
type Options opt.Options

// NewDatastore returns a new datastore backed by leveldb
//
// for path == "", an in memory bachend will be chosen
func NewDatastore(path string, opts *Options) (*Datastore, error) {
	var nopts opt.Options
	if opts != nil {
		nopts = opt.Options(*opts)
	}

	var err error
	var db *leveldb.DB

	if path == "" {
		db, err = leveldb.Open(storage.NewMemStorage(), &nopts)
	} else {
		db, err = leveldb.OpenFile(path, &nopts)
		if errors.IsCorrupted(err) && !nopts.GetReadOnly() {
			db, err = leveldb.RecoverFile(path, &nopts)
		}
	}

	if err != nil {
		return nil, err
	}

	return &Datastore{
		accessor: &accessor{ldb: db},
		DB:       db,
		path:     path,
	}, nil
}

// An extraction of the common interface between LevelDB Transactions and the DB itself.
//
// It allows to plug in either inside the `accessor`.
type levelDbOps interface {
	Put(key, value []byte, wo *opt.WriteOptions) error
	Get(key []byte, ro *opt.ReadOptions) (value []byte, err error)
	Has(key []byte, ro *opt.ReadOptions) (ret bool, err error)
	Delete(key []byte, wo *opt.WriteOptions) error
	NewIterator(slice *util.Range, ro *opt.ReadOptions) iterator.Iterator
}

// Datastore operations using either the DB or a transaction as the backend.
type accessor struct {
	ldb levelDbOps
}

func (a *accessor) Put(key ds.Key, value []byte) (err error) {
	return a.ldb.Put(key.Bytes(), value, nil)
}

func (a *accessor) Get(key ds.Key) (value []byte, err error) {
	val, err := a.ldb.Get(key.Bytes(), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, ds.ErrNotFound
		}
		return nil, err
	}
	return val, nil
}

func (a *accessor) Has(key ds.Key) (exists bool, err error) {
	return a.ldb.Has(key.Bytes(), nil)
}

func (d *accessor) GetSize(key ds.Key) (size int, err error) {
	return ds.GetBackedSize(d, key)
}

func (a *accessor) Delete(key ds.Key) (err error) {
	// leveldb Delete will not return an error if the key doesn't
	// exist (see https://github.com/syndtr/goleveldb/issues/109),
	// so check that the key exists first and if not return an
	// error
	exists, err := a.ldb.Has(key.Bytes(), nil)
	if !exists {
		return ds.ErrNotFound
	} else if err != nil {
		return err
	}
	return a.ldb.Delete(key.Bytes(), nil)
}

func (a *accessor) Query(q dsq.Query) (dsq.Results, error) {
	return a.queryNew(q)
}

func (a *accessor) queryNew(q dsq.Query) (dsq.Results, error) {
	if len(q.Filters) > 0 ||
		len(q.Orders) > 0 ||
		q.Limit > 0 ||
		q.Offset > 0 {
		return a.queryOrig(q)
	}
	var rnge *util.Range
	if q.Prefix != "" {
		rnge = util.BytesPrefix([]byte(q.Prefix))
	}
	i := a.ldb.NewIterator(rnge, nil)
	return dsq.ResultsFromIterator(q, dsq.Iterator{
		Next: func() (dsq.Result, bool) {
			ok := i.Next()
			if !ok {
				return dsq.Result{}, false
			}
			k := string(i.Key())
			e := dsq.Entry{Key: k}

			if !q.KeysOnly {
				buf := make([]byte, len(i.Value()))
				copy(buf, i.Value())
				e.Value = buf
			}
			return dsq.Result{Entry: e}, true
		},
		Close: func() error {
			i.Release()
			return nil
		},
	}), nil
}

func (a *accessor) queryOrig(q dsq.Query) (dsq.Results, error) {
	// we can use multiple iterators concurrently. see:
	// https://godoc.org/github.com/syndtr/goleveldb/leveldb#DB.NewIterator
	// advance the iterator only if the reader reads
	//
	// run query in own sub-process tied to Results.Process(), so that
	// it waits for us to finish AND so that clients can signal to us
	// that resources should be reclaimed.
	qrb := dsq.NewResultBuilder(q)
	qrb.Process.Go(func(worker goprocess.Process) {
		a.runQuery(worker, qrb)
	})

	// go wait on the worker (without signaling close)
	go qrb.Process.CloseAfterChildren()

	// Now, apply remaining things (filters, order)
	qr := qrb.Results()
	for _, f := range q.Filters {
		qr = dsq.NaiveFilter(qr, f)
	}
	if len(q.Orders) > 0 {
		switch q.Orders[0].(type) {
		case dsq.OrderByKey, *dsq.OrderByKey:
			// Default ordering
		default:
			qr = dsq.NaiveOrder(qr, q.Orders...)
		}
	}
	return qr, nil
}

func (a *accessor) runQuery(worker goprocess.Process, qrb *dsq.ResultBuilder) {
	var rnge *util.Range
	if qrb.Query.Prefix != "" {
		rnge = util.BytesPrefix([]byte(qrb.Query.Prefix))
	}
	i := a.ldb.NewIterator(rnge, nil)
	defer i.Release()

	// advance iterator for offset
	if qrb.Query.Offset > 0 {
		for j := 0; j < qrb.Query.Offset; j++ {
			i.Next()
		}
	}

	// iterate, and handle limit, too
	for sent := 0; i.Next(); sent++ {
		// end early if we hit the limit
		if qrb.Query.Limit > 0 && sent >= qrb.Query.Limit {
			break
		}

		k := string(i.Key())
		e := dsq.Entry{Key: k}

		if !qrb.Query.KeysOnly {
			buf := make([]byte, len(i.Value()))
			copy(buf, i.Value())
			e.Value = buf
		}

		select {
		case qrb.Output <- dsq.Result{Entry: e}: // we sent it out
		case <-worker.Closing(): // client told us to end early.
			break
		}
	}

	if err := i.Error(); err != nil {
		select {
		case qrb.Output <- dsq.Result{Error: err}: // client read our error
		case <-worker.Closing(): // client told us to end.
			return
		}
	}
}

// DiskUsage returns the current disk size used by this levelDB.
// For in-mem datastores, it will return 0.
func (d *Datastore) DiskUsage() (uint64, error) {
	if d.path == "" { // in-mem
		return 0, nil
	}

	var du uint64

	err := filepath.Walk(d.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		du += uint64(info.Size())
		return nil
	})

	if err != nil {
		return 0, err
	}

	return du, nil
}

// LevelDB needs to be closed.
func (d *Datastore) Close() (err error) {
	return d.DB.Close()
}

func (d *Datastore) IsThreadSafe() {}

type leveldbBatch struct {
	b  *leveldb.Batch
	db *leveldb.DB
}

func (d *Datastore) Batch() (ds.Batch, error) {
	return &leveldbBatch{
		b:  new(leveldb.Batch),
		db: d.DB,
	}, nil
}

func (b *leveldbBatch) Put(key ds.Key, value []byte) error {
	b.b.Put(key.Bytes(), value)
	return nil
}

func (b *leveldbBatch) Commit() error {
	return b.db.Write(b.b, nil)
}

func (b *leveldbBatch) Delete(key ds.Key) error {
	b.b.Delete(key.Bytes())
	return nil
}

// A leveldb transaction embedding the accessor backed by the transaction.
type transaction struct {
	*accessor
	tx *leveldb.Transaction
}

func (t *transaction) Commit() error {
	return t.tx.Commit()
}

func (t *transaction) Discard() {
	t.tx.Discard()
}

func (d *Datastore) NewTransaction(readOnly bool) (ds.Txn, error) {
	tx, err := d.DB.OpenTransaction()
	if err != nil {
		return nil, err
	}
	accessor := &accessor{tx}
	return &transaction{accessor, tx}, nil
}
