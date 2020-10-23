package adt

import (
	"context"

	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	cid "github.com/ipfs/go-cid"
	ipldcbor "github.com/ipfs/go-ipld-cbor"

	vmr "github.com/filecoin-project/specs-actors/v2/actors/runtime"
)

// Store defines an interface required to back the ADTs in this package.
type Store interface {
	Context() context.Context
	ipldcbor.IpldStore
}

// Adapts a vanilla IPLD store as an ADT store.
func WrapStore(ctx context.Context, store ipldcbor.IpldStore) Store {
	return &wstore{
		ctx:       ctx,
		IpldStore: store,
	}
}

type wstore struct {
	ctx context.Context
	ipldcbor.IpldStore
}

var _ Store = &wstore{}

func (s *wstore) Context() context.Context {
	return s.ctx
}

// Adapter for a Runtime as an ADT Store.

// Adapts a Runtime as an ADT store.
func AsStore(rt vmr.Runtime) Store {
	return rtStore{rt}
}

type rtStore struct {
	vmr.Runtime
}

var _ Store = &rtStore{}

func (r rtStore) Context() context.Context {
	return r.Runtime.Context()
}

func (r rtStore) Get(_ context.Context, c cid.Cid, out interface{}) error {
	// The Go context is (un/fortunately?) dropped here.
	// See https://github.com/filecoin-project/specs-actors/issues/140
	if !r.StoreGet(c, out.(cbor.Unmarshaler)) {
		r.Abortf(exitcode.ErrNotFound, "not found")
	}
	return nil
}

func (r rtStore) Put(_ context.Context, v interface{}) (cid.Cid, error) {
	// The Go context is (un/fortunately?) dropped here.
	// See https://github.com/filecoin-project/specs-actors/issues/140
	return r.StorePut(v.(cbor.Marshaler)), nil
}
