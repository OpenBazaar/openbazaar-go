package dagcbor

import (
	"io"

	"github.com/polydawn/refmt/cbor"

	ipld "github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
)

var (
	_ cidlink.MulticodecDecoder = Decoder
	_ cidlink.MulticodecEncoder = Encoder
)

func init() {
	cidlink.RegisterMulticodecDecoder(0x71, Decoder)
	cidlink.RegisterMulticodecEncoder(0x71, Encoder)
}

func Decoder(na ipld.NodeAssembler, r io.Reader) error {
	// Probe for a builtin fast path.  Shortcut to that if possible.
	//  (ipldcbor.NodeBuilder supports this, for example.)
	type detectFastPath interface {
		DecodeDagCbor(io.Reader) error
	}
	if na2, ok := na.(detectFastPath); ok {
		return na2.DecodeDagCbor(r)
	}
	// Okay, generic builder path.
	return Unmarshal(na, cbor.NewDecoder(cbor.DecodeOptions{}, r))
}

func Encoder(n ipld.Node, w io.Writer) error {
	// Probe for a builtin fast path.  Shortcut to that if possible.
	//  (ipldcbor.Node supports this, for example.)
	type detectFastPath interface {
		EncodeDagCbor(io.Writer) error
	}
	if n2, ok := n.(detectFastPath); ok {
		return n2.EncodeDagCbor(w)
	}
	// Okay, generic inspection path.
	return Marshal(n, cbor.NewEncoder(w))
}
