package adt

import (
	"github.com/filecoin-project/go-state-types/abi"
	cid "github.com/ipfs/go-cid"
)

// Set interprets a Map as a set, storing keys (with empty values) in a HAMT.
type Set struct {
	m *Map
}

// AsSet interprets a store as a HAMT-based set with root `r`.
func AsSet(s Store, r cid.Cid) (*Set, error) {
	m, err := AsMap(s, r)
	if err != nil {
		return nil, err
	}

	return &Set{
		m: m,
	}, nil
}

// NewSet creates a new HAMT with root `r` and store `s`.
func MakeEmptySet(s Store) *Set {
	m := MakeEmptyMap(s)
	return &Set{m}
}

// Root return the root cid of HAMT.
func (h *Set) Root() (cid.Cid, error) {
	return h.m.Root()
}

// Put adds `k` to the set.
func (h *Set) Put(k abi.Keyer) error {
	return h.m.Put(k, nil)
}

// Has returns true iff `k` is in the set.
func (h *Set) Has(k abi.Keyer) (bool, error) {
	return h.m.Get(k, nil)
}

// Delete removes `k` from the set.
func (h *Set) Delete(k abi.Keyer) error {
	return h.m.Delete(k)
}

// ForEach iterates over all values in the set, calling the callback for each value.
// Returning error from the callback stops the iteration.
func (h *Set) ForEach(cb func(k string) error) error {
	return h.m.ForEach(nil, cb)
}

// Collects all the keys from the set into a slice of strings.
func (h *Set) CollectKeys() (out []string, err error) {
	return h.m.CollectKeys()
}
