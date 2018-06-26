// Package retrystore provides a datastore wrapper which
// allows to retry operations.
package retrystore

import (
	"fmt"
	"time"

	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
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

// Get retrieves a value given a key.
func (d *Datastore) Get(k ds.Key) (interface{}, error) {
	var val interface{}
	err := d.runOp(func() error {
		var err error
		val, err = d.Batching.Get(k)
		return err
	})

	return val, err
}

// Put stores a key/value.
func (d *Datastore) Put(k ds.Key, val interface{}) error {
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
