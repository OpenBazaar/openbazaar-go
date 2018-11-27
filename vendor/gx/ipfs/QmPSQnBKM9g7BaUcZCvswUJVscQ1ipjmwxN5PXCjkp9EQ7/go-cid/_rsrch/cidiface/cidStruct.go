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

var _ Cid = CidStruct{}

//var _ map[CidStruct]struct{} = nil // Will not compile!  See struct def docs.
//var _ map[Cid]struct{} = map[Cid]struct{}{CidStruct{}: struct{}{}} // Legal to compile...
// but you'll get panics: "runtime error: hash of unhashable type cid.CidStruct"

// CidStruct represents a CID in a struct format.
//
// This format complies with the exact same Cid interface as the CidStr
// implementation, but completely pre-parses the Cid metadata.
// CidStruct is a tad quicker in case of repeatedly accessed fields,
// but requires more reshuffling to parse and to serialize.
// CidStruct is not usable as a map key, because it contains a Multihash
// reference, which is a slice, and thus not "comparable" as a primitive.
//
// Beware of zero-valued CidStruct: it is difficult to distinguish an
// incorrectly-initialized "invalid" CidStruct from one representing a v0 cid.
type CidStruct struct {
	version uint64
	codec   uint64
	hash    mh.Multihash
}

// EmptyCidStruct is a constant for a zero/uninitialized/sentinelvalue cid;
// it is declared mainly for readability in checks for sentinel values.
//
// Note: it's not actually a const; the compiler does not allow const structs.
var EmptyCidStruct = CidStruct{}

func (c CidStruct) Version() uint64 {
	return c.version
}

func (c CidStruct) Multicodec() uint64 {
	return c.codec
}

func (c CidStruct) Multihash() mh.Multihash {
	return c.hash
}

// String returns the default string representation of a Cid.
// Currently, Base58 is used as the encoding for the multibase string.
func (c CidStruct) String() string {
	switch c.Version() {
	case 0:
		return c.Multihash().B58String()
	case 1:
		mbstr, err := mbase.Encode(mbase.Base58BTC, c.Bytes())
		if err != nil {
			panic("should not error with hardcoded mbase: " + err.Error())
		}
		return mbstr
	default:
		panic("not possible to reach this point")
	}
}

// Bytes produces a raw binary format of the CID.
func (c CidStruct) Bytes() []byte {
	switch c.version {
	case 0:
		return []byte(c.hash)
	case 1:
		// two 8 bytes (max) numbers plus hash
		buf := make([]byte, 2*binary.MaxVarintLen64+len(c.hash))
		n := binary.PutUvarint(buf, c.version)
		n += binary.PutUvarint(buf[n:], c.codec)
		cn := copy(buf[n:], c.hash)
		if cn != len(c.hash) {
			panic("copy hash length is inconsistent")
		}
		return buf[:n+len(c.hash)]
	default:
		panic("not possible to reach this point")
	}
}

// Prefix builds and returns a Prefix out of a Cid.
func (c CidStruct) Prefix() Prefix {
	dec, _ := mh.Decode(c.hash) // assuming we got a valid multiaddr, this will not error
	return Prefix{
		MhType:   dec.Code,
		MhLength: dec.Length,
		Version:  c.version,
		Codec:    c.codec,
	}
}

//==================================
// parsers & validators & factories
//==================================

// CidStructParse takes a binary byte slice, parses it, and returns either
// a valid CidStruct, or the zero CidStruct and an error.
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
// if you have a multibase-enveloped string, use CidStructDecode instead.
//
// CidStructParse is the inverse of Cid.Bytes().
func CidStructParse(data []byte) (CidStruct, error) {
	if len(data) == 34 && data[0] == 18 && data[1] == 32 {
		h, err := mh.Cast(data)
		if err != nil {
			return EmptyCidStruct, err
		}
		return CidStruct{
			codec:   DagProtobuf,
			version: 0,
			hash:    h,
		}, nil
	}

	vers, n := binary.Uvarint(data)
	if err := uvError(n); err != nil {
		return EmptyCidStruct, err
	}

	if vers != 0 && vers != 1 {
		return EmptyCidStruct, fmt.Errorf("invalid cid version number: %d", vers)
	}

	codec, cn := binary.Uvarint(data[n:])
	if err := uvError(cn); err != nil {
		return EmptyCidStruct, err
	}

	rest := data[n+cn:]
	h, err := mh.Cast(rest)
	if err != nil {
		return EmptyCidStruct, err
	}

	return CidStruct{
		version: vers,
		codec:   codec,
		hash:    h,
	}, nil
}
