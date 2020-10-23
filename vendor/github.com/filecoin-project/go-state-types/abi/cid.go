package abi

import (
	cid "github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

var (
	// HashFunction is the default hash function for computing CIDs.
	//
	// This is currently Blake2b-256.
	HashFunction = uint64(mh.BLAKE2B_MIN + 31)

	// When producing a CID for an IPLD block less than or equal to CIDInlineLimit
	// bytes in length, the identity hash function will be used instead of
	// HashFunction. This will effectively "inline" the block into the CID, allowing
	// it to be extracted directly from the CID with no disk/network operations.
	//
	// This is currently -1 for "disabled".
	//
	// This is exposed for testing. Do not modify unless you know what you're doing.
	CIDInlineLimit = -1
)

type cidBuilder struct {
	codec uint64
}

func (cidBuilder) WithCodec(c uint64) cid.Builder {
	return cidBuilder{codec: c}
}

func (b cidBuilder) GetCodec() uint64 {
	return b.codec
}

func (b cidBuilder) Sum(data []byte) (cid.Cid, error) {
	hf := HashFunction
	if len(data) <= CIDInlineLimit {
		hf = mh.IDENTITY
	}
	return cid.V1Builder{Codec: b.codec, MhType: hf}.Sum(data)
}

// CidBuilder is the default CID builder for Filecoin.
//
// - The default codec is CBOR. This can be changed with CidBuilder.WithCodec.
// - The default hash function is 256bit blake2b.
var CidBuilder cid.Builder = cidBuilder{codec: cid.DagCBOR}
