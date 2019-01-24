// Package retrystore provides a datastore wrapper which
// allows to retry operations.
package retrystore

import (
	"fmt"
	"time"

	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
)

// Datastore wraps a Batching datastore with a
// user-provided TempErrorFunc -which determines if an error
// is a temporal error and thus, worth retrying-, an amount of Retries
// -which specify how many times to retry an operation after
// a temporal error- and a base Delay, which is multiplied by the
// current retry and performs a pause before attempting the operation again.
type Datastore struct {
	TempErrFunc func(error) bool
	Retries     int
	Delay       time.Duration

	ds.Batching
}

var errFmtString = "ran out of retries trying to get past temporary error: %s"

func (d *Datastore) runOp(op func() error) error {
	err := op()
	if err == nil || !d.TempErrFunc(err) {
		return err
	}

	for i := 0; i < d.Retries; i++ {
		time.Sleep(time.Duration(i+1) * d.Delay)

		err = op()
		if err == nil || !d.TempErrFunc(err) {
			return err
		}
	}

	return fmt.Errorf(errFmtString, err)
}

// DiskUsage implements the PersistentDatastore interface.
func (d *Datastore) DiskUsage() (uint64, error) {
	var size uint64
	err := d.runOp(func() error {
		var err error
		size, err = ds.DiskUsage(d.Batching)
		return err
	})
	return size, err
}

// Get retrieves a value given a key.
func (d *Datastore) Get(k ds.Key) ([]byte, error) {
	var val []byte
	err := d.runOp(func() error {
		var err error
		val, err = d.Batching.Get(k)
		return err
	})

	return val, err
}

// Put stores a key/value.
func (d *Datastore) Put(k ds.Key, val []byte) error {
	return d.runOp(func() error {
		return d.Batching.Put(k, val)
	})
}

// Has checks if a key is stored.
func (d *Datastore) Has(k ds.Key) (bool, error) {
	var has bool
	err := d.runOp(func() error {
		var err error
		has, err = d.Batching.Has(k)
		return err
	})
	return has, err
}

// GetSize returns the size of the value in the datastore, if present.
func (d *Datastore) GetSize(k ds.Key) (int, error) {
	var size int
	err := d.runOp(func() error {
		var err error
		size, err = d.Batching.GetSize(k)
		return err
	})
	return size, err
}
