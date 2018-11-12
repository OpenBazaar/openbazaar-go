package cid

import (
	"encoding/binary"
	"fmt"

	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	mbase "gx/ipfs/QmekxXDhCxCJRNuzmHreuaT3BsuJcsjcXWNrtV9C8DRHtd/go-multibase"
)

//=================
// def & accessors
//=================

var _ Cid = CidStr("")
var _ map[CidStr]struct{} = nil

// CidStr is a representation of a Cid as a string type containing binary.
//
// Using golang's string type is preferable over byte slices even for binary
// data because golang strings are immutable, usable as map keys,
// trivially comparable with built-in equals operators, etc.
//
// Please do not cast strings or bytes into the CidStr type directly;
// use a parse method which validates the data and yields a CidStr.
type CidStr string

// EmptyCidStr is a constant for a zero/uninitialized/sentinelvalue cid;
// it is declared mainly for readability in checks for sentinel values.
const EmptyCidStr = CidStr("")

func (c CidStr) Version() uint64 {
	bytes := []byte(c)
	v, _ := binary.Uvarint(bytes)
	return v
}

func (c CidStr) Multicodec() uint64 {
	bytes := []byte(c)
	_, n := binary.Uvarint(bytes) // skip version length
	codec, _ := binary.Uvarint(bytes[n:])
	return codec
}

func (c CidStr) Multihash() mh.Multihash {
	bytes := []byte(c)
	_, n1 := binary.Uvarint(bytes)      // skip version length
	_, n2 := binary.Uvarint(bytes[n1:]) // skip codec length
	return mh.Multihash(bytes[n1+n2:])  // return slice of remainder
}

// String returns the default string representation of a Cid.
// Currently, Base58 is used as the encoding for the multibase string.
func (c CidStr) String() string {
	switch c.Version() {
	case 0:
		return c.Multihash().B58String()
	case 1:
		mbstr, err := mbase.Encode(mbase.Base58BTC, []byte(c))
		if err != nil {
			panic("should not error with hardcoded mbase: " + err.Error())
		}
		return mbstr
	default:
		panic("not possible to reach this point")
	}
}

// Bytes produces a raw binary format of the CID.
//
// (For CidStr, this method is only distinct from casting because of
// compatibility with v0 CIDs.)
func (c CidStr) Bytes() []byte {
	switch c.Version() {
	case 0:
		return c.Multihash()
	case 1:
		return []byte(c)
	default:
		panic("not possible to reach this point")
	}
}

// Prefix builds and returns a Prefix out of a Cid.
func (c CidStr) Prefix() Prefix {
	dec, _ := mh.Decode(c.Multihash()) // assuming we got a valid multiaddr, this will not error
	return Prefix{
		MhType:   dec.Code,
		MhLength: dec.Length,
		Version:  c.Version(),
		Codec:    c.Multicodec(),
	}
}

//==================================
// parsers & validators & factories
//==================================

func NewCidStr(version uint64, codecType uint64, mhash mh.Multihash) CidStr {
	hashlen := len(mhash)
	// two 8 bytes (max) numbers plus hash
	buf := make([]byte, 2*binary.MaxVarintLen64+hashlen)
	n := binary.PutUvarint(buf, version)
	n += binary.PutUvarint(buf[n:], codecType)
	cn := copy(buf[n:], mhash)
	if cn != hashlen {
		panic("copy hash length is inconsistent")
	}
	return CidStr(buf[:n+hashlen])
}

// CidStrParse takes a binary byte slice, parses it, and returns either
// a valid CidStr, or the zero CidStr and an error.
//
// For CidV1, the data buffer is in the form:
//
//     <version><codec-type><multihash>
//
// CidV0 are also supported. In particular, data buffers starting
// with length 34 bytes, which starts with bytes [18,32...] are considered
// binary multihashes.
//
// The multicodec bytes are not parsed to verify they're a valid varint;
// no further reification is performed.
//
// Multibase encoding should already have been unwrapped before parsing;
// if you have a multibase-enveloped string, use CidStrDecode instead.
//
// CidStrParse is the inverse of Cid.Bytes().
func CidStrParse(data []byte) (CidStr, error) {
	if len(data) == 34 && data[0] == 18 && data[1] == 32 {
		h, err := mh.Cast(data)
		if err != nil {
			return EmptyCidStr, err
		}
		return NewCidStr(0, DagProtobuf, h), nil
	}

	vers, n := binary.Uvarint(data)
	if err := uvError(n); err != nil {
		return EmptyCidStr, err
	}

	if vers != 0 && vers != 1 {
		return EmptyCidStr, fmt.Errorf("invalid cid version number: %d", vers)
	}

	_, cn := binary.Uvarint(data[n:])
	if err := uvError(cn); err != nil {
		return EmptyCidStr, err
	}

	rest := data[n+cn:]
	h, err := mh.Cast(rest)
	if err != nil {
		return EmptyCidStr, err
	}

	// REVIEW: if the data is longer than the mh.len expects, we silently ignore it?  should we?
	return CidStr(data[0 : n+cn+len(h)]), nil
}
