package refmt

import (
	"io"

	"gx/ipfs/QmPAdjGx1huCjnrR26qy9QUUNSqA6EStyZ68RrwbtCTDML/refmt/cbor"
	"gx/ipfs/QmPAdjGx1huCjnrR26qy9QUUNSqA6EStyZ68RrwbtCTDML/refmt/json"
	"gx/ipfs/QmPAdjGx1huCjnrR26qy9QUUNSqA6EStyZ68RrwbtCTDML/refmt/obj/atlas"
)

type DecodeOptions interface {
	IsDecodeOptions() // marker method.
}

func Unmarshal(opts DecodeOptions, data []byte, v interface{}) error {
	switch opts.(type) {
	case json.DecodeOptions:
		return json.Unmarshal(data, v)
	case cbor.DecodeOptions:
		return cbor.Unmarshal(data, v)
	default:
		panic("incorrect usage: unknown DecodeOptions type")
	}
}

func UnmarshalAtlased(opts DecodeOptions, data []byte, v interface{}, atl atlas.Atlas) error {
	switch opts.(type) {
	case json.DecodeOptions:
		return json.UnmarshalAtlased(data, v, atl)
	case cbor.DecodeOptions:
		return cbor.UnmarshalAtlased(data, v, atl)
	default:
		panic("incorrect usage: unknown DecodeOptions type")
	}
}

type Unmarshaller interface {
	Unmarshal(v interface{}) error
}

func NewUnmarshaller(opts DecodeOptions, r io.Reader) Unmarshaller {
	switch opts.(type) {
	case json.DecodeOptions:
		return json.NewUnmarshaller(r)
	case cbor.DecodeOptions:
		return cbor.NewUnmarshaller(r)
	default:
		panic("incorrect usage: unknown DecodeOptions type")
	}
}

func NewUnmarshallerAtlased(opts DecodeOptions, r io.Reader, atl atlas.Atlas) Unmarshaller {
	switch opts.(type) {
	case json.DecodeOptions:
		return json.NewUnmarshallerAtlased(r, atl)
	case cbor.DecodeOptions:
		return cbor.NewUnmarshallerAtlased(r, atl)
	default:
		panic("incorrect usage: unknown DecodeOptions type")
	}
}
