package cbornode

import (
	"math/big"

	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"

	encoding "gx/ipfs/QmVwsD9oGSAKcCoD4BGi3Hx8XRo4jNy6aMzsjDHVTgYv2D/go-ipld-cbor/encoding"

	"gx/ipfs/QmdBzoMxsBpojBfN1cv5GnKtB7sfYBMoLH7p9qSyEVYXcu/refmt/obj/atlas"
)

// This atlas describes the CBOR Tag (42) for IPLD links, such that refmt can marshal and unmarshal them
var cidAtlasEntry = atlas.BuildEntry(cid.Cid{}).
	UseTag(CBORTagLink).
	Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		castCidToBytes,
	)).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		castBytesToCid,
	)).
	Complete()

// BigIntAtlasEntry gives a reasonable default encoding for big.Int. It is not
// included in the entries by default.
var BigIntAtlasEntry = atlas.BuildEntry(big.Int{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(i big.Int) ([]byte, error) {
			return i.Bytes(), nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x []byte) (big.Int, error) {
			return *big.NewInt(0).SetBytes(x), nil
		})).
	Complete()

// CborAtlas is the refmt.Atlas used by the CBOR IPLD decoder/encoder.
var CborAtlas atlas.Atlas
var cborSortingMode = atlas.KeySortMode_RFC7049
var atlasEntries = []*atlas.AtlasEntry{cidAtlasEntry}

var (
	cloner       encoding.PooledCloner
	unmarshaller encoding.PooledUnmarshaller
	marshaller   encoding.PooledMarshaller
)

func init() {
	rebuildAtlas()
}

func rebuildAtlas() {
	CborAtlas = atlas.MustBuild(atlasEntries...).
		WithMapMorphism(atlas.MapMorphism{KeySortMode: atlas.KeySortMode_RFC7049})

	marshaller = encoding.NewPooledMarshaller(CborAtlas)
	unmarshaller = encoding.NewPooledUnmarshaller(CborAtlas)
	cloner = encoding.NewPooledCloner(CborAtlas)
}

// RegisterCborType allows to register a custom cbor type
func RegisterCborType(i interface{}) {
	var entry *atlas.AtlasEntry
	if ae, ok := i.(*atlas.AtlasEntry); ok {
		entry = ae
	} else {
		entry = atlas.BuildEntry(i).StructMap().AutogenerateWithSortingScheme(atlas.KeySortMode_RFC7049).Complete()
	}
	atlasEntries = append(atlasEntries, entry)
	rebuildAtlas()
}
