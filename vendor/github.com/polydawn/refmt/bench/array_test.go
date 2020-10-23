package bench

import (
	"bytes"
	"testing"

	"github.com/polydawn/refmt"
	"github.com/polydawn/refmt/cbor"
	"github.com/polydawn/refmt/json"
)

var fixture_arrayFlatInt = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}
var fixture_arrayFlatInt_json = []byte(`[1,2,3,4,5,6,7,8,9,0]`)
var fixture_arrayFlatInt_cbor = []byte{0x80 + 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0}

func Benchmark_ArrayFlatInt_MarshalToCborRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseMarshaller(b,
		refmt.NewMarshaller(cbor.EncodeOptions{}, &buf), &buf,
		fixture_arrayFlatInt, fixture_arrayFlatInt_cbor,
	)
}
func Benchmark_ArrayFlatInt_MarshalToJsonRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseMarshaller(b,
		refmt.NewMarshaller(json.EncodeOptions{}, &buf), &buf,
		fixture_arrayFlatInt, fixture_arrayFlatInt_json,
	)
}
func Benchmark_ArrayFlatInt_MarshalToJsonStdlib(b *testing.B) {
	exerciseStdlibJsonMarshaller(b,
		fixture_arrayFlatInt, fixture_arrayFlatInt_json,
	)
}

func Benchmark_ArrayFlatInt_UnmarshalFromCborRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseUnmarshaller(b,
		refmt.NewUnmarshaller(cbor.DecodeOptions{}, &buf), &buf,
		fixture_arrayFlatInt_cbor, func() interface{} { return &[]int{} }, &fixture_arrayFlatInt,
	)
}
func Benchmark_ArrayFlatInt_UnmarshalFromJsonRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseUnmarshaller(b,
		refmt.NewUnmarshaller(json.DecodeOptions{}, &buf), &buf,
		fixture_arrayFlatInt_json, func() interface{} { return &[]int{} }, &fixture_arrayFlatInt,
	)
}
func Benchmark_ArrayFlatInt_UnmarshalFromJsonStdlib(b *testing.B) {
	exerciseStdlibJsonUnmarshaller(b,
		fixture_arrayFlatInt_json, func() interface{} { return &[]int{} }, &fixture_arrayFlatInt,
	)
}

var fixture_arrayFlatInt20 = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0}
var fixture_arrayFlatInt20_json = []byte(`[1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0]`)
var fixture_arrayFlatInt20_cbor = []byte{0x80 + 20, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0}

func Benchmark_ArrayFlatInt20_MarshalToCborRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseMarshaller(b,
		refmt.NewMarshaller(cbor.EncodeOptions{}, &buf), &buf,
		fixture_arrayFlatInt20, fixture_arrayFlatInt20_cbor,
	)
}
func Benchmark_ArrayFlatInt20_MarshalToJsonRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseMarshaller(b,
		refmt.NewMarshaller(json.EncodeOptions{}, &buf), &buf,
		fixture_arrayFlatInt20, fixture_arrayFlatInt20_json,
	)
}
func Benchmark_ArrayFlatInt20_MarshalToJsonStdlib(b *testing.B) {
	exerciseStdlibJsonMarshaller(b,
		fixture_arrayFlatInt20, fixture_arrayFlatInt20_json,
	)
}

func Benchmark_ArrayFlatInt20_UnmarshalFromCborRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseUnmarshaller(b,
		refmt.NewUnmarshaller(cbor.DecodeOptions{}, &buf), &buf,
		fixture_arrayFlatInt20_cbor, func() interface{} { return &[]int{} }, &fixture_arrayFlatInt20,
	)
}
func Benchmark_ArrayFlatInt20_UnmarshalFromJsonRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseUnmarshaller(b,
		refmt.NewUnmarshaller(json.DecodeOptions{}, &buf), &buf,
		fixture_arrayFlatInt20_json, func() interface{} { return &[]int{} }, &fixture_arrayFlatInt20,
	)
}
func Benchmark_ArrayFlatInt20_UnmarshalFromJsonStdlib(b *testing.B) {
	exerciseStdlibJsonUnmarshaller(b,
		fixture_arrayFlatInt20_json, func() interface{} { return &[]int{} }, &fixture_arrayFlatInt20,
	)
}

var fixture_arrayFlatStr = []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"}
var fixture_arrayFlatStr_json = []byte(`["1","2","3","4","5","6","7","8","9","0"]`)
var fixture_arrayFlatStr_cbor = []byte{0x80 + 10, 0x60 + 1, 0x30 + 1, 0x60 + 1, 0x30 + 2, 0x60 + 1, 0x30 + 3, 0x60 + 1, 0x30 + 4, 0x60 + 1, 0x30 + 5, 0x60 + 1, 0x30 + 6, 0x60 + 1, 0x30 + 7, 0x60 + 1, 0x30 + 8, 0x60 + 1, 0x30 + 9, 0x60 + 1, 0x30 + 0}

func Benchmark_ArrayFlatStr_MarshalToCborRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseMarshaller(b,
		refmt.NewMarshaller(cbor.EncodeOptions{}, &buf), &buf,
		fixture_arrayFlatStr, fixture_arrayFlatStr_cbor,
	)
}
func Benchmark_ArrayFlatStr_MarshalToJsonRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseMarshaller(b,
		refmt.NewMarshaller(json.EncodeOptions{}, &buf), &buf,
		fixture_arrayFlatStr, fixture_arrayFlatStr_json,
	)
}
func Benchmark_ArrayFlatStr_MarshalToJsonStdlib(b *testing.B) {
	exerciseStdlibJsonMarshaller(b,
		fixture_arrayFlatStr, fixture_arrayFlatStr_json,
	)
}

func Benchmark_ArrayFlatStr_UnmarshalFromCborRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseUnmarshaller(b,
		refmt.NewUnmarshaller(cbor.DecodeOptions{}, &buf), &buf,
		fixture_arrayFlatStr_cbor, func() interface{} { return &[]string{} }, &fixture_arrayFlatStr,
	)
}
func Benchmark_ArrayFlatStr_UnmarshalFromJsonRefmt(b *testing.B) {
	var buf bytes.Buffer
	exerciseUnmarshaller(b,
		refmt.NewUnmarshaller(json.DecodeOptions{}, &buf), &buf,
		fixture_arrayFlatStr_json, func() interface{} { return &[]string{} }, &fixture_arrayFlatStr,
	)
}
func Benchmark_ArrayFlatStr_UnmarshalFromJsonStdlib(b *testing.B) {
	exerciseStdlibJsonUnmarshaller(b,
		fixture_arrayFlatStr_json, func() interface{} { return &[]string{} }, &fixture_arrayFlatStr,
	)
}
