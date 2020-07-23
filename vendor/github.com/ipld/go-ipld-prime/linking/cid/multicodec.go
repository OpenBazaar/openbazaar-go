package cidlink

import (
	"io"

	ipld "github.com/ipld/go-ipld-prime"
)

type MulticodecDecodeTable map[uint64]MulticodecDecoder

type MulticodecEncodeTable map[uint64]MulticodecEncoder

// MulticodecDecoder builds an ipld.Node by unmarshalling bytes and funnelling
// the data tree into an ipld.NodeAssembler.  The resulting Node is not
// returned; typically you call this function with an ipld.NodeBuilder,
// and you can extract the result from there.
//
// MulticodecDecoder are used by registering them in a MulticodecDecoderTable,
// which makes them available to be used internally by cidlink.Link.Load.
//
// Consider implementing decoders to probe their NodeBuilder to see if it
// has special features that may be able to do the job more efficiently.
// For example, ipldcbor.NodeBuilder has special unmarshaller functions
// that know how to fastpath their work *if* we're doing a cbor decode;
// if possible, detect and use that; if not, fall back to general generic
// NodeBuilder usage.
type MulticodecDecoder func(ipld.NodeAssembler, io.Reader) error

// MulticodecEncoder marshals and ipld.Node into bytes and sends them to
// an io.Writer.
//
// MulticodecEncoder are used by registering them in a MulticodecEncoderTable,
// which makes them available to be used internally by cidlink.LinkBuilder.
//
// Tends to be implemented by probing the node to see if it matches a special
// interface that we know can do this particular kind of encoding
// (e.g. if you're using ipldgit.Node and making a MulticodecEncoder to register
// as the rawgit multicodec, you'll probe for that specific thing, since it's
// implemented on the node itself),
// but may also be able to work based on the ipld.Node interface alone
// (e.g. you can do dag-cbor to any kind of Node).
type MulticodecEncoder func(ipld.Node, io.Writer) error
