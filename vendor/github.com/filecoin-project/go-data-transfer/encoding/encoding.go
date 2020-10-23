package encoding

import (
	"bytes"
	"reflect"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	cborgen "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"
)

// Encodable is an object that can be written to CBOR and decoded back
type Encodable interface{}

// Encode encodes an encodable to CBOR, using the best available path for
// writing to CBOR
func Encode(value Encodable) ([]byte, error) {
	if cbgEncodable, ok := value.(cborgen.CBORMarshaler); ok {
		buf := new(bytes.Buffer)
		err := cbgEncodable.MarshalCBOR(buf)
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	if ipldEncodable, ok := value.(ipld.Node); ok {
		buf := new(bytes.Buffer)
		err := dagcbor.Encoder(ipldEncodable, buf)
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	return cbor.DumpObject(value)
}

// Decoder is CBOR decoder for a given encodable type
type Decoder interface {
	DecodeFromCbor([]byte) (Encodable, error)
}

// NewDecoder creates a new Decoder that will decode into new instances of the given
// object type. It will use the decoding that is optimal for that type
// It returns error if it's not possible to setup a decoder for this type
func NewDecoder(decodeType Encodable) (Decoder, error) {
	// check if type is ipld.Node, if so, just use style
	if ipldDecodable, ok := decodeType.(ipld.Node); ok {
		return &ipldDecoder{ipldDecodable.Prototype()}, nil
	}
	// check if type is a pointer, as we need that to make new copies
	// for cborgen types & regular IPLD types
	decodeReflectType := reflect.TypeOf(decodeType)
	if decodeReflectType.Kind() != reflect.Ptr {
		return nil, xerrors.New("type must be a pointer")
	}
	// check if type is a cbor-gen type
	if _, ok := decodeType.(cborgen.CBORUnmarshaler); ok {
		return &cbgDecoder{decodeReflectType}, nil
	}
	// type does is neither ipld-prime nor cbor-gen, so we need to see if it
	// can rountrip with oldschool ipld-format
	encoded, err := cbor.DumpObject(decodeType)
	if err != nil {
		return nil, xerrors.New("Object type did not encode")
	}
	newDecodable := reflect.New(decodeReflectType.Elem()).Interface()
	if err := cbor.DecodeInto(encoded, newDecodable); err != nil {
		return nil, xerrors.New("Object type did not decode")
	}
	return &defaultDecoder{decodeReflectType}, nil
}

type ipldDecoder struct {
	style ipld.NodePrototype
}

func (decoder *ipldDecoder) DecodeFromCbor(encoded []byte) (Encodable, error) {
	builder := decoder.style.NewBuilder()
	buf := bytes.NewReader(encoded)
	err := dagcbor.Decoder(builder, buf)
	if err != nil {
		return nil, err
	}
	return builder.Build(), nil
}

type cbgDecoder struct {
	cbgType reflect.Type
}

func (decoder *cbgDecoder) DecodeFromCbor(encoded []byte) (Encodable, error) {
	decodedValue := reflect.New(decoder.cbgType.Elem())
	decoded, ok := decodedValue.Interface().(cborgen.CBORUnmarshaler)
	if !ok || reflect.ValueOf(decoded).IsNil() {
		return nil, xerrors.New("problem instantiating decoded value")
	}
	buf := bytes.NewReader(encoded)
	err := decoded.UnmarshalCBOR(buf)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

type defaultDecoder struct {
	ptrType reflect.Type
}

func (decoder *defaultDecoder) DecodeFromCbor(encoded []byte) (Encodable, error) {
	decodedValue := reflect.New(decoder.ptrType.Elem())
	decoded, ok := decodedValue.Interface().(Encodable)
	if !ok || reflect.ValueOf(decoded).IsNil() {
		return nil, xerrors.New("problem instantiating decoded value")
	}
	err := cbor.DecodeInto(encoded, decoded)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}
