// Package delayed wraps a datastore allowing to artificially
// delay all operations.
package delayed

import (
	"io"

	ds "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	dsq "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore/query"
	delay "gx/ipfs/QmUe1WCHkQaz4UeNKiHDUBV2T6i9prc3DniqyHPXyfGaUq/go-ipfs-delay"
)

// New returns a new delayed datastore.
func New(ds ds.Datastore, delay delay.D) *Delayed {
	return &Delayed{ds: ds, delay: delay}
}

// Delayed is an adapter that delays operations on the inner datastore.
type Delayed struct {
	ds    ds.Datastore
	delay delay.D
}

var _ ds.Batching = (*Delayed)(nil)
var _ io.Closer = (*Delayed)(nil)

// Put implements the ds.Datastore interface.
func (dds *Delayed) Put(key ds.Key, value []byte) (err error) {
	dds.delay.Wait()
	return dds.ds.Put(key, value)
}

// Get implements the ds.Datastore interface.
func (dds *Delayed) Get(key ds.Key) (value []byte, err error) {
	dds.delay.Wait()
	return dds.ds.Get(key)
}

// Has implements the ds.Datastore interface.
func (dds *Delayed) Has(key ds.Key) (exists bool, err error) {
	dds.delay.Wait()
	return dds.ds.Has(key)
}

// GetSize implements the ds.Datastore interface.
func (dds *Delayed) GetSize(key ds.Key) (size int, err error) {
	dds.delay.Wait()
	return dds.ds.GetSize(key)
}

// Delete implements the ds.Datastore interface.
func (dds *Delayed) Delete(key ds.Key) (err error) {
	dds.delay.Wait()
	return dds.ds.Delete(key)
}

// Query implements the ds.Datastore interface.
func (dds *Delayed) Query(q dsq.Query) (dsq.Results, error) {
	dds.delay.Wait()
	return dds.ds.Query(q)
}

// Batch implements the ds.Batching interface.
func (dds *Delayed) Batch() (ds.Batch, error) {
	return ds.NewBasicBatch(dds), nil
}

// DiskUsage implements the ds.PersistentDatastore interface.
func (dds *Delayed) DiskUsage() (uint64, error) {
	dds.delay.Wait()
	return ds.DiskUsage(dds.ds)
}

// Close closes the inner datastore (if it implements the io.Closer interface).
func (dds *Delayed) Close() error {
	if closer, ok := dds.ds.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
