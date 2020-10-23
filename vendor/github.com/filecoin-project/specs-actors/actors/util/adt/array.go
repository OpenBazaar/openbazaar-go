package adt

import (
	"bytes"

	amt "github.com/filecoin-project/go-amt-ipld/v2"
	"github.com/filecoin-project/go-state-types/cbor"
	cid "github.com/ipfs/go-cid"
	errors "github.com/pkg/errors"
	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"
)

// Array stores a sparse sequence of values in an AMT.
type Array struct {
	root  *amt.Root
	store Store
}

// AsArray interprets a store as an AMT-based array with root `r`.
func AsArray(s Store, r cid.Cid) (*Array, error) {
	root, err := amt.LoadAMT(s.Context(), s, r)
	if err != nil {
		return nil, xerrors.Errorf("failed to root: %w", err)
	}

	return &Array{
		root:  root,
		store: s,
	}, nil
}

// Creates a new map backed by an empty HAMT and flushes it to the store.
func MakeEmptyArray(s Store) *Array {
	root := amt.NewAMT(s)
	return &Array{
		root:  root,
		store: s,
	}
}

// Returns the root CID of the underlying AMT.
func (a *Array) Root() (cid.Cid, error) {
	return a.root.Flush(a.store.Context())
}

// Appends a value to the end of the array. Assumes continuous array.
// If the array isn't continuous use Set and a separate counter
func (a *Array) AppendContinuous(value cbor.Marshaler) error {
	if err := a.root.Set(a.store.Context(), a.root.Count, value); err != nil {
		return errors.Wrapf(err, "array append failed to set index %v value %v in root %v, ", a.root.Count, value, a.root)
	}
	return nil
}

func (a *Array) Set(i uint64, value cbor.Marshaler) error {
	if err := a.root.Set(a.store.Context(), i, value); err != nil {
		return xerrors.Errorf("array set failed to set index %v in root %v: %w", i, a.root, err)
	}
	return nil
}

func (a *Array) Delete(i uint64) error {
	if err := a.root.Delete(a.store.Context(), i); err != nil {
		return xerrors.Errorf("array delete failed to delete index %v in root %v: %w", i, a.root, err)
	}
	return nil
}

func (a *Array) BatchDelete(ix []uint64) error {
	if err := a.root.BatchDelete(a.store.Context(), ix); err != nil {
		return xerrors.Errorf("array delete failed to batchdelete: %w", err)
	}
	return nil
}

// Iterates all entries in the array, deserializing each value in turn into `out` and then calling a function.
// Iteration halts if the function returns an error.
// If the output parameter is nil, deserialization is skipped.
func (a *Array) ForEach(out cbor.Unmarshaler, fn func(i int64) error) error {
	return a.root.ForEach(a.store.Context(), func(k uint64, val *cbg.Deferred) error {
		if out != nil {
			if deferred, ok := out.(*cbg.Deferred); ok {
				// fast-path deferred -> deferred to avoid re-decoding.
				*deferred = *val
			} else if err := out.UnmarshalCBOR(bytes.NewReader(val.Raw)); err != nil {
				return err
			}
		}
		return fn(int64(k))
	})
}

func (a *Array) Length() uint64 {
	return a.root.Count
}

// Get retrieves array element into the 'out' unmarshaler, returning a boolean
//  indicating whether the element was found in the array
func (a *Array) Get(k uint64, out cbor.Unmarshaler) (bool, error) {

	if err := a.root.Get(a.store.Context(), k, out); err == nil {
		return true, nil
	} else if _, nf := err.(*amt.ErrNotFound); nf {
		return false, nil
	} else {
		return false, err
	}
}
