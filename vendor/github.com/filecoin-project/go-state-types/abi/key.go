package abi

import (
	"encoding/binary"
	"errors"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
)

// Keyer defines an interface required to put values in mapping.
type Keyer interface {
	Key() string
}

// Adapts an address as a mapping key.
type AddrKey address.Address

func (k AddrKey) Key() string {
	return string(address.Address(k).Bytes())
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
