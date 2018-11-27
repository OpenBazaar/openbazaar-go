// Package failstore implements a datastore which can produce
// custom failures on operations by calling a user-provided
// error function.
package failstore

import (
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	dsq "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/query"
)

// Failstore is a datastore which fails according to a user-provided
// function.
type Failstore struct {
	child   ds.Datastore
	errfunc func(string) error
}

// NewFailstore creates a new datastore with the given error function.
// The efunc will be called with different strings depending on the
// datastore function: put, get, has, delete, query, batch, batch-put,
// batch-delete and batch-commit are the possible values.
func NewFailstore(c ds.Datastore, efunc func(string) error) *Failstore {
	return &Failstore{
		child:   c,
		errfunc: efunc,
	}
}

// Put puts a key/value into the datastore.
func (d *Failstore) Put(k ds.Key, val []byte) error {
	err := d.errfunc("put")
	if err != nil {
		return err
	}

	return d.child.Put(k, val)
}

// Get retrieves a value from the datastore.
func (d *Failstore) Get(k ds.Key) ([]byte, error) {
	err := d.errfunc("get")
	if err != nil {
		return nil, err
	}

	return d.child.Get(k)
}

// Has returns if the datastore contains a key/value.
func (d *Failstore) Has(k ds.Key) (bool, error) {
	err := d.errfunc("has")
	if err != nil {
		return false, err
	}

	return d.child.Has(k)
}

// GetSize returns the size of the value in the datastore, if present.
func (d *Failstore) GetSize(k ds.Key) (int, error) {
	err := d.errfunc("getsize")
	if err != nil {
		return -1, err
	}

	return d.child.GetSize(k)
}

// Delete removes a key/value from the datastore.
func (d *Failstore) Delete(k ds.Key) error {
	err := d.errfunc("delete")
	if err != nil {
		return err
	}

	return d.child.Delete(k)
}

// Query performs a query on the datastore.
func (d *Failstore) Query(q dsq.Query) (dsq.Results, error) {
	err := d.errfunc("query")
	if err != nil {
		return nil, err
	}

	return d.child.Query(q)
}

// DiskUsage implements the PersistentDatastore interface.
func (d *Failstore) DiskUsage() (uint64, error) {
	if err := d.errfunc("disk-usage"); err != nil {
		return 0, err
	}
	return ds.DiskUsage(d.child)
}

// FailBatch implements batching operations on the Failstore.
type FailBatch struct {
	cb     ds.Batch
	dstore *Failstore
}

// Batch returns a new Batch Failstore.
func (d *Failstore) Batch() (ds.Batch, error) {
	if err := d.errfunc("batch"); err != nil {
		return nil, err
	}

	b, err := d.child.(ds.Batching).Batch()
	if err != nil {
		return nil, err
	}

	return &FailBatch{
		cb:     b,
		dstore: d,
	}, nil
}

// Put does a batch put.
func (b *FailBatch) Put(k ds.Key, val []byte) error {
	if err := b.dstore.errfunc("batch-put"); err != nil {
		return err
	}

	return b.cb.Put(k, val)
}

// Delete does a batch delete.
func (b *FailBatch) Delete(k ds.Key) error {
	if err := b.dstore.errfunc("batch-delete"); err != nil {
		return err
	}

	return b.cb.Delete(k)
}

// Commit commits all operations in the batch.
func (b *FailBatch) Commit() error {
	if err := b.dstore.errfunc("batch-commit"); err != nil {
		return err
	}

	return b.cb.Commit()
}
