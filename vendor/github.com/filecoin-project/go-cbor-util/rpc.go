package cborutil

import (
	"bytes"
	"encoding/hex"
	"io"
	"math"

	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	cbg "github.com/whyrusleeping/cbor-gen"
)

var log = logging.Logger("cborrrpc")

// Debug will produce more debugging messages
const Debug = false

func init() {
	if Debug {
		log.Warn("CBOR-RPC Debugging enabled")
	}
}

// WriteCborRPC with encode an object to cbor, opting for fast path if possible
// and then write it into the given io.Writer
func WriteCborRPC(w io.Writer, obj interface{}) error {
	if m, ok := obj.(cbg.CBORMarshaler); ok {
		// TODO: impl debug
		return m.MarshalCBOR(w)
	}
	data, err := cbor.DumpObject(obj)
	if err != nil {
		return err
	}

	if Debug {
		log.Infof("> %s", hex.EncodeToString(data))
	}

	_, err = w.Write(data)
	return err
}

// ReadCborRPC will read an object from the given io.Reader
// opting for fast path if possible
func ReadCborRPC(r io.Reader, out interface{}) error {
	if um, ok := out.(cbg.CBORUnmarshaler); ok {
		return um.UnmarshalCBOR(r)
	}
	return cbor.DecodeReader(r, out)
}

// Dump returns the cbor bytes representation of an object
func Dump(obj interface{}) ([]byte, error) {
	var out bytes.Buffer
	if err := WriteCborRPC(&out, obj); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// AsIpld converts an object to an ipld.Node interface
// TODO: this is a bit ugly, and this package is not exactly the best place
func AsIpld(obj interface{}) (ipld.Node, error) {
	if m, ok := obj.(cbg.CBORMarshaler); ok {
		b, err := Dump(m)
		if err != nil {
			return nil, err
		}
		return cbor.Decode(b, math.MaxUint64, -1)
	}
	return cbor.WrapObject(obj, math.MaxUint64, -1)
}

// Equals is true if two objects have the same cbor representation
func Equals(a cbg.CBORMarshaler, b cbg.CBORMarshaler) (bool, error) {
	ab, err := Dump(a)
	if err != nil {
		return false, err
	}
	bb, err := Dump(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(ab, bb), nil
}
