package bench

import (
	"bytes"
	"testing"

	"github.com/polydawn/refmt"
	"github.com/polydawn/refmt/cbor"
	"github.com/polydawn/refmt/json"
)

var fixture_mapAlpha = map[string]interface{}{
	"B": map[string]interface{}{
		"R": map[string]interface{}{
			"R": map[string]interface{}{
				"R": map[string]interface{}{
					"R": nil,
					"M": "",
				},
				"M": "asdf",
			},
			"M": "quir",
		},
	},
	"C": map[string]interface{}{
		"N": "n",
		"M": 13,
	},
	"C2": map[string]interface{}{
		"N": "n2",
		"M": 14,
	},
	"X": 1,
	"Y": 2,
	"Z": "3",
	"W": "4",
}
var fixture_mapAlpha_json = fixture_structAlpha_json
var fixture_mapAlpha_cbor = fixture_structAlpha_cbor

func Benchmark_MapAlpha_MarshalToCborRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseMarshaller(b,
		refmt.NewMarshaller(cbor.EncodeOptions{}, &buf), &buf,
		fixture_mapAlpha, fixture_mapAlpha_cbor,
	)
}
func Benchmark_MapAlpha_MarshalToJsonRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseMarshaller(b,
		refmt.NewMarshaller(json.EncodeOptions{}, &buf), &buf,
		fixture_mapAlpha, fixture_mapAlpha_json,
	)
}
func Benchmark_MapAlpha_MarshalToJsonStdlib(b *testing.B) {
	exerciseStdlibJsonMarshaller(b,
		fixture_mapAlpha, fixture_mapAlpha_json,
	)
}

func Benchmark_MapAlpha_UnmarshalFromCborRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseUnmarshaller(b,
		refmt.NewUnmarshaller(cbor.DecodeOptions{}, &buf), &buf,
		fixture_mapAlpha_cbor, func() interface{} { return &map[string]interface{}{} }, &fixture_mapAlpha,
	)
}
func Benchmark_MapAlpha_UnmarshalFromJsonRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseUnmarshaller(b,
		refmt.NewUnmarshaller(json.DecodeOptions{}, &buf), &buf,
		fixture_mapAlpha_json, func() interface{} { return &map[string]interface{}{} }, &fixture_mapAlpha,
	)
}
func Benchmark_MapAlpha_UnmarshalFromJsonStdlib(b *testing.B) {
	exerciseStdlibJsonUnmarshaller(b,
		fixture_mapAlpha_json, func() interface{} { return &map[string]interface{}{} }, &fixture_mapAlpha,
	)
}
