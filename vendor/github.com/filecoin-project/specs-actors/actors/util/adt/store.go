package adt

import (
	"context"
	"encoding/binary"

	addr "github.com/filecoin-project/go-address"
	cid "github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	vmr "github.com/filecoin-project/specs-actors/actors/runtime"
	exitcode "github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
)

// Store defines an interface required to back the ADTs in this package.
type Store interface {
	Context() context.Context
	cbor.IpldStore
}

// Adapts a vanilla IPLD store as an ADT store.
func WrapStore(ctx context.Context, store cbor.IpldStore) Store {
	return &wstore{
		ctx:       ctx,
		IpldStore: store,
	}
}

type wstore struct {
	ctx context.Context
	cbor.IpldStore
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
	if !r.Store().Get(c, out.(vmr.CBORUnmarshaler)) {
		r.Abortf(exitcode.ErrNotFound, "not found")
	}
	return nil
}

func (r rtStore) Put(_ context.Context, v interface{}) (cid.Cid, error) {
	// The Go context is (un/fortunately?) dropped here.
	// See https://github.com/filecoin-project/specs-actors/issues/140
	return r.Store().Put(v.(vmr.CBORMarshaler)), nil
}

// Keyer defines an interface required to put values in mapping.
type Keyer interface {
	Key() string
}

// Adapts an address as a mapping key.
type AddrKey addr.Address

func (k AddrKey) Key() string {
	return string(addr.Address(k).Bytes())
}

type CidKey cid.Cid

func (k CidKey) Key() string {
	return cid.Cid(k).KeyString()
}

// Adapts an int64 as a mapping key.
type intKey struct {
	int64
}

//noinspection GoExportedFuncWithUnexportedType
func IntKey(k int64) intKey {
	return intKey{k}
}

func (k intKey) Key() string {
	buf := make([]byte, 10)
	n := binary.PutVarint(buf, k.int64)
	return string(buf[:n])
}

//noinspection GoUnusedExportedFunction
func ParseIntKey(k string) (int64, error) {
	i, n := binary.Varint([]byte(k))
	if n != len(k) {
		return 0, errors.New("failed to decode varint key")
	}
	return i, nil
}

// Adapts a uint64 as a mapping key.
type uintKey struct {
	uint64
}

//noinspection GoExportedFuncWithUnexportedType
func UIntKey(k uint64) uintKey {
	return uintKey{k}
}

func (k uintKey) Key() string {
	buf := make([]byte, 10)
	n := binary.PutUvarint(buf, k.uint64)
	return string(buf[:n])
}

func ParseUIntKey(k string) (uint64, error) {
	i, n := binary.Uvarint([]byte(k))
	if n != len(k) {
		return 0, errors.New("failed to decode varint key")
	}
	return i, nil
}
