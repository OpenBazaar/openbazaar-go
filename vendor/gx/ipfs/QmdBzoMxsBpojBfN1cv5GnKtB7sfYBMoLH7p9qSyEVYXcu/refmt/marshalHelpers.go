package refmt

import (
	"io"

	"gx/ipfs/QmdBzoMxsBpojBfN1cv5GnKtB7sfYBMoLH7p9qSyEVYXcu/refmt/cbor"
	"gx/ipfs/QmdBzoMxsBpojBfN1cv5GnKtB7sfYBMoLH7p9qSyEVYXcu/refmt/json"
	"gx/ipfs/QmdBzoMxsBpojBfN1cv5GnKtB7sfYBMoLH7p9qSyEVYXcu/refmt/obj/atlas"
)

type EncodeOptions interface {
	IsEncodeOptions() // marker method.
}

func Marshal(opts EncodeOptions, v interface{}) ([]byte, error) {
	switch o2 := opts.(type) {
	case json.EncodeOptions:
		return json.MarshalAtlased(o2, v, atlas.MustBuild())
	case cbor.EncodeOptions:
		return cbor.MarshalAtlased(v, atlas.MustBuild())
	default:
		panic("incorrect usage: unknown EncodeOptions type")
	}
}

func MarshalAtlased(opts EncodeOptions, v interface{}, atl atlas.Atlas) ([]byte, error) {
	switch o2 := opts.(type) {
	case json.EncodeOptions:
		return json.MarshalAtlased(o2, v, atl)
	case cbor.EncodeOptions:
		return cbor.MarshalAtlased(v, atl)
	default:
		panic("incorrect usage: unknown EncodeOptions type")
	}
}

type Marshaller interface {
	Marshal(v interface{}) error
}

func NewMarshaller(opts EncodeOptions, wr io.Writer) Marshaller {
	switch o2 := opts.(type) {
	case json.EncodeOptions:
		return json.NewMarshallerAtlased(wr, o2, atlas.MustBuild())
	case cbor.EncodeOptions:
		return cbor.NewMarshaller(wr)
	default:
		panic("incorrect usage: unknown EncodeOptions type")
	}
}

func NewMarshallerAtlased(opts EncodeOptions, wr io.Writer, atl atlas.Atlas) Marshaller {
	switch o2 := opts.(type) {
	case json.EncodeOptions:
		return json.NewMarshallerAtlased(wr, o2, atl)
	case cbor.EncodeOptions:
		return cbor.NewMarshallerAtlased(wr, atl)
	default:
		panic("incorrect usage: unknown EncodeOptions type")
	}
}
