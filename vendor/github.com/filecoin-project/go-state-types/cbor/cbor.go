package cbor

import "io"

// These interfaces are intended to match those from whyrusleeping/cbor-gen, such that code generated from that
// system is automatically usable here (but not mandatory).
type Marshaler interface {
	MarshalCBOR(w io.Writer) error
}

type Unmarshaler interface {
	UnmarshalCBOR(r io.Reader) error
}

type Er interface {
	Marshaler
	Unmarshaler
}
