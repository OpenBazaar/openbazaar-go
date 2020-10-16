package adt

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/cbor"
	cid "github.com/ipfs/go-cid"
	errors "github.com/pkg/errors"
	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"
)

// Multimap stores multiple values per key in a HAMT of AMTs.
// The order of insertion of values for each key is retained.
type Multimap struct {
	mp *Map
}

// Interprets a store as a HAMT-based map of AMTs with root `r`.
func AsMultimap(s Store, r cid.Cid) (*Multimap, error) {
	m, err := AsMap(s, r)
	if err != nil {
		return nil, err
	}

	return &Multimap{m}, nil
}

// Creates a new map backed by an empty HAMT and flushes it to the store.
func MakeEmptyMultimap(s Store) *Multimap {
	m := MakeEmptyMap(s)
	return &Multimap{m}
}

// Returns the root cid of the underlying HAMT.
func (mm *Multimap) Root() (cid.Cid, error) {
	return mm.mp.Root()
}

// Adds a value for a key.
func (mm *Multimap) Add(key abi.Keyer, value cbor.Marshaler) error {
	// Load the array under key, or initialize a new empty one if not found.
	array, found, err := mm.Get(key)
	if err != nil {
		return err
	}
	if !found {
		array = MakeEmptyArray(mm.mp.store)
	}

	// Append to the array.
	if err = array.AppendContinuous(value); err != nil {
		return errors.Wrapf(err, "failed to add multimap key %v value %v", key, value)
	}

	c, err := array.Root()
	if err != nil {
		return xerrors.Errorf("failed to flush child array: %w", err)
	}

	// Store the new array root under key.
	newArrayRoot := cbg.CborCid(c)
	err = mm.mp.Put(key, &newArrayRoot)
	if err != nil {
		return errors.Wrapf(err, "failed to store multimap values")
	}
	return nil
}

// Removes all values for a key.
func (mm *Multimap) RemoveAll(key abi.Keyer) error {
	err := mm.mp.Delete(key)
	if err != nil {
		return errors.Wrapf(err, "failed to delete multimap key %v root %v", key, mm.mp.root)
	}
	return nil
}

// Iterates all entries for a key in the order they were inserted, deserializing each value in turn into `out` and then
// calling a function.
// Iteration halts if the function returns an error.
// If the output parameter is nil, deserialization is skipped.
func (mm *Multimap) ForEach(key abi.Keyer, out cbor.Unmarshaler, fn func(i int64) error) error {
	array, found, err := mm.Get(key)
	if err != nil {
		return err
	}
	if found {
		return array.ForEach(out, fn)
	}
	return nil
}

func (mm *Multimap) ForAll(fn func(k string, arr *Array) error) error {
	var arrRoot cbg.CborCid
	if err := mm.mp.ForEach(&arrRoot, func(k string) error {
		arr, err := AsArray(mm.mp.store, cid.Cid(arrRoot))
		if err != nil {
			return err
		}

		return fn(k, arr)
	}); err != nil {
		return err
	}

	return nil
}

func (mm *Multimap) Get(key abi.Keyer) (*Array, bool, error) {
	var arrayRoot cbg.CborCid
	found, err := mm.mp.Get(key, &arrayRoot)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to load multimap key %v", key)
	}
	var array *Array
	if found {
		array, err = AsArray(mm.mp.store, cid.Cid(arrayRoot))
		if err != nil {
			return nil, false, xerrors.Errorf("failed to load value %v as an array: %w", key, err)
		}
	}
	return array, found, nil
}
