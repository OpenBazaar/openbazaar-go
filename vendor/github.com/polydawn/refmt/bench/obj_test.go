package bench

import (
	"bytes"
	"testing"

	"github.com/polydawn/refmt"
	"github.com/polydawn/refmt/cbor"
	"github.com/polydawn/refmt/json"
	"github.com/polydawn/refmt/obj/atlas"
)

type structAlpha struct {
	B  *structBeta
	C  structGamma
	C2 structGamma
	W  string
	X  int
	Y  int
	Z  string
}
type structBeta struct {
	R *structRecursive
}
type structGamma struct {
	M int
	N string
}
type structRecursive struct {
	M string
	R *structRecursive
}

var fixture_structAlpha = structAlpha{
	&structBeta{
		&structRecursive{
			"quir",
			&structRecursive{
				"asdf",
				&structRecursive{},
			},
		},
	},
	structGamma{13, "n"},
	structGamma{14, "n2"},
	"4", 1, 2, "3",
}

// note: 18 string keys, 7 string values; total 25 strings.
var fixture_structAlpha_json = []byte(`{"B":{"R":{"M":"quir","R":{"M":"asdf","R":{"M":"","R":null}}}},"C":{"M":13,"N":"n"},"C2":{"M":14,"N":"n2"},"W":"4","X":1,"Y":2,"Z":"3"}`)
var fixture_structAlpha_cbor = []byte{0xa7, 0x61, 0x42, 0xa1, 0x61, 0x52, 0xa2, 0x61, 0x4d, 0x64, 0x71, 0x75, 0x69, 0x72, 0x61, 0x52, 0xa2, 0x61, 0x4d, 0x64, 0x61, 0x73, 0x64, 0x66, 0x61, 0x52, 0xa2, 0x61, 0x4d, 0x60, 0x61, 0x52, 0xf6, 0x61, 0x43, 0xa2, 0x61, 0x4d, 0x0d, 0x61, 0x4e, 0x61, 0x6e, 0x62, 0x43, 0x32, 0xa2, 0x61, 0x4d, 0x0e, 0x61, 0x4e, 0x62, 0x6e, 0x32, 0x61, 0x57, 0x61, 0x34, 0x61, 0x58, 0x01, 0x61, 0x59, 0x02, 0x61, 0x5a, 0x61, 0x33}
var fixture_structAlpha_atlas = atlas.MustBuild(
	atlas.BuildEntry(structAlpha{}).StructMap().
		AddField("B", atlas.StructMapEntry{SerialName: "B"}).
		AddField("C", atlas.StructMapEntry{SerialName: "C"}).
		AddField("C2", atlas.StructMapEntry{SerialName: "C2"}).
		AddField("W", atlas.StructMapEntry{SerialName: "W"}).
		AddField("X", atlas.StructMapEntry{SerialName: "X"}).
		AddField("Y", atlas.StructMapEntry{SerialName: "Y"}).
		AddField("Z", atlas.StructMapEntry{SerialName: "Z"}).
		Complete(),
	atlas.BuildEntry(structBeta{}).StructMap().
		AddField("R", atlas.StructMapEntry{SerialName: "R"}).
		Complete(),
	atlas.BuildEntry(structGamma{}).StructMap().
		AddField("M", atlas.StructMapEntry{SerialName: "M"}).
		AddField("N", atlas.StructMapEntry{SerialName: "N"}).
		Complete(),
	atlas.BuildEntry(structRecursive{}).StructMap().
		AddField("M", atlas.StructMapEntry{SerialName: "M"}).
		AddField("R", atlas.StructMapEntry{SerialName: "R"}).
		Complete(),
)

func Benchmark_StructAlpha_MarshalToCborRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseMarshaller(b,
		refmt.NewMarshallerAtlased(cbor.EncodeOptions{}, &buf, fixture_structAlpha_atlas), &buf,
		fixture_structAlpha, fixture_structAlpha_cbor,
	)
}
func Benchmark_StructAlpha_MarshalToJsonRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseMarshaller(b,
		refmt.NewMarshallerAtlased(json.EncodeOptions{}, &buf, fixture_structAlpha_atlas), &buf,
		fixture_structAlpha, fixture_structAlpha_json,
	)
}
func Benchmark_StructAlpha_MarshalToJsonStdlib(b *testing.B) {
	exerciseStdlibJsonMarshaller(b,
		fixture_structAlpha, fixture_structAlpha_json,
	)
}

func Benchmark_StructAlpha_UnmarshalFromCborRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseUnmarshaller(b,
		refmt.NewUnmarshallerAtlased(cbor.DecodeOptions{}, &buf, fixture_structAlpha_atlas), &buf,
		fixture_structAlpha_cbor, func() interface{} { return &structAlpha{} }, &fixture_structAlpha,
	)
}
func Benchmark_StructAlpha_UnmarshalFromJsonRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseUnmarshaller(b,
		refmt.NewUnmarshallerAtlased(json.DecodeOptions{}, &buf, fixture_structAlpha_atlas), &buf,
		fixture_structAlpha_json, func() interface{} { return &structAlpha{} }, &fixture_structAlpha,
	)
}
func Benchmark_StructAlpha_UnmarshalFromJsonStdlib(b *testing.B) {
	exerciseStdlibJsonUnmarshaller(b,
		fixture_structAlpha_json, func() interface{} { return &structAlpha{} }, &fixture_structAlpha,
	)
}
