package cid

import (
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
)

// Cid represents a self-describing content adressed identifier.
//
// A CID is composed of:
//
//   - a Version of the CID itself,
//   - a Multicodec (indicates the encoding of the referenced content),
//   - and a Multihash (which identifies the referenced content).
//
// (Note that the Multihash further contains its own version and hash type
// indicators.)
type Cid interface {
	// n.b. 'yields' means "without copy", 'produces' means a malloc.

	Version() uint64         // Yields the version prefix as a uint.
	Multicodec() uint64      // Yields the multicodec as a uint.
	Multihash() mh.Multihash // Yields the multihash segment.

	String() string // Produces the CID formatted as b58 string.
	Bytes() []byte  // Produces the CID formatted as raw binary.

	Prefix() Prefix // Produces a tuple of non-content metadata.

	// some change notes:
	// - `KeyString() CidString` is gone because we're natively a map key now, you're welcome.
	// - `StringOfBase(mbase.Encoding) (string, error)` is skipped, maybe it can come back but maybe it should be a formatter's job.
	// - `Equals(o Cid) bool` is gone because it's now `==`, you're welcome.

	// TODO: make a multi-return method for {v,mc,mh} decomposition.  CidStr will be able to implement this more efficiently than if one makes a series of the individual getter calls.
}

// Prefix represents all the metadata of a Cid,
// that is, the Version, the Codec, the Multihash type
// and the Multihash length. It does not contains
// any actual content information.
// NOTE: The use -1 in MhLength to mean default length is deprecated,
//   use the V0Builder or V1Builder structures instead
type Prefix struct {
	Version  uint64
	Codec    uint64
	MhType   uint64
	MhLength int
}
