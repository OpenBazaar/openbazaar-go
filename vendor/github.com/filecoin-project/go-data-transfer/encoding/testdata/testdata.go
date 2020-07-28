package testdata

import (
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipld/go-ipld-prime/fluent"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
)

// Prime = an instance of an ipld prime piece of data
var Prime = fluent.MustBuildMap(basicnode.Style.Map, 2, func(na fluent.MapAssembler) {
	nva := na.AssembleEntry("X")
	nva.AssignInt(100)
	nva = na.AssembleEntry("Y")
	nva.AssignString("appleSauce")
})

type standardType struct {
	X int
	Y string
}

func init() {
	cbor.RegisterCborType(standardType{})
}

// Standard = an instance that is neither ipld prime nor cbor
var Standard *standardType = &standardType{X: 100, Y: "appleSauce"}

//go:generate cbor-gen-for cbgType

type cbgType struct {
	X uint64
	Y string
}

// Cbg = an instance of a cbor-gen type
var Cbg *cbgType = &cbgType{X: 100, Y: "appleSauce"}
