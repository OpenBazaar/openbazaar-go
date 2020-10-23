package bench

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"
)

type typeA struct {
	Alpha string
	Beta  string
	Gamma typeB
	Delta int
}

type typeB struct {
	Msg string
}

type Atlas interface {
	Fields() []string
	Addr(fieldName string) interface{}
}

type AtlasField struct {
	Name string

	// *One* of the following:

	FieldName  FieldName                     // look up the fields by string name.
	fieldRoute FieldRoute                    // autoatlas fills these.
	AddrFunc   func(interface{}) interface{} // custom user function.

	// Behavioral options:

	OmitEmpty bool
}

type FieldName []string
type FieldRoute []int

func (fr FieldRoute) TraverseToValue(v reflect.Value) reflect.Value {
	for _, i := range fr {
		if v.Kind() == reflect.Ptr {
			if v.IsNil() {
				return reflect.Value{}
			}
			v = v.Elem()
		}
		v = v.Field(i)
	}
	return v
}

type RackRow struct {
	Name string
	Addr interface{}
}

var atlForTypeA_funcy = []AtlasField{
	{Name: "alpha", AddrFunc: func(v interface{}) interface{} { return &(v.(*typeA).Alpha) }},
	{Name: "beta", AddrFunc: func(v interface{}) interface{} { return &(v.(*typeA).Beta) }},
	{Name: "gamma", AddrFunc: func(v interface{}) interface{} { return &(v.(*typeA).Gamma.Msg) }},
	{Name: "delta", AddrFunc: func(v interface{}) interface{} { return &(v.(*typeA).Delta) }},
}
var atlForTypeA_fieldRouted = []AtlasField{
	{Name: "alpha", fieldRoute: []int{0}},
	{Name: "beta", fieldRoute: []int{1}},
	{Name: "gamma", fieldRoute: []int{2, 0}},
	{Name: "delta", fieldRoute: []int{3}},
}
var atlForTypeA_glommed = func(v interface{}) map[string]interface{} {
	x := v.(*typeA)
	return map[string]interface{}{
		"alpha": &(x.Alpha),
		"beta":  &(x.Beta),
		"gamma": &(x.Gamma.Msg),
		"delta": &(x.Delta),
	}
}
var atlForTypeA_racked = func(v interface{}) []RackRow {
	x := v.(*typeA)
	return []RackRow{
		{"alpha", &(v.(*typeA).Alpha)},
		{"beta", &(v.(*typeA).Beta)},
		{"gamma", &(v.(*typeA).Gamma.Msg)},
		{"delta", &(x.Delta)},
	}
}

// GC on:  261 ns/op
// GC off: 194 ns/op
// Membench:  48 B/op  3 allocs/op
func Benchmark_WalkStructReadViaFuncy(b *testing.B) {
	var dump interface{}
	var dumpRef = &dump
	var val = typeA{
		"str1",
		"str2",
		typeB{"str3"},
		4,
	}
	for i := 0; i < b.N; i++ {
		for _, row := range atlForTypeA_funcy {
			switch v2 := row.AddrFunc(&val).(type) {
			case *interface{}:
				*dumpRef = *v2
			case *string:
				*dumpRef = *v2
			}
		}
	}
	_ = dump
}

// GC on:  404 ns/op
// GC off: 430 ns/op
// Membench:  56 B/op  4 allocs/op
func Benchmark_WalkStructReadViaFieldroutes(b *testing.B) {
	var dump interface{}
	var dumpRef = &dump
	var val = typeA{
		"str1",
		"str2",
		typeB{"str3"},
		4,
	}
	for i := 0; i < b.N; i++ {
		rv := reflect.ValueOf(&val)
		for _, row := range atlForTypeA_fieldRouted {
			switch v2 := row.fieldRoute.TraverseToValue(rv).Interface().(type) {
			case *interface{}:
				*dumpRef = *v2
			case *string:
				*dumpRef = *v2
			}
		}
	}
	_ = dump
}

// GC on:  980 ns/op -- or as high as 1055
// GC off: 907 ns/op
// Membench:  384 B/op  5 allocs/op -- really high.  maps make this costly, evidentally.
func Benchmark_WalkStructReadViaGlommed(b *testing.B) {
	var dump interface{}
	var dumpRef = &dump
	var val = typeA{
		"str1",
		"str2",
		typeB{"str3"},
		4,
	}
	for i := 0; i < b.N; i++ {
		glom := atlForTypeA_glommed(&val)
		for _, addr := range glom {
			switch v2 := addr.(type) {
			case *interface{}:
				*dumpRef = *v2
			case *string:
				*dumpRef = *v2
			}
		}
	}
	_ = dump
}

// GC on:  367 ns/op
// GC off: 217 ns/op
// Mem:    176 B/op   4 allocs/op
// Commentary:
//  - Somewhat surprisingly, still slower than funcy.
//  - One more alloc than funcy; one less than glommed.  Also, same as fieldroutes.  Shows in time.
//  - Does score well for clarity.
//  - Those four allocs -- know where they come from?  The addr grabbing.
//    (I don't entirely understand how the AddrFunc approach is immune to this, but
//    evidentally that escape analysis there is key to that technique's efficiency.)
func Benchmark_WalkStructReadViaRacked(b *testing.B) {
	var dump interface{}
	var dumpRef = &dump
	var val = typeA{
		"str1",
		"str2",
		typeB{"str3"},
		4,
	}
	for i := 0; i < b.N; i++ {
		rack := atlForTypeA_racked(&val)
		for _, row := range rack {
			switch v2 := row.Addr.(type) {
			case *interface{}:
				*dumpRef = *v2
			case *string:
				*dumpRef = *v2
			}
		}
	}
	_ = dump
}

// GC on:  1124 ns/op
// GC off: 1046 ns/op
// Membench: 72 B/op  2 allocs/op -- Passing by value ends up with *more* allocs than using ref
func Benchmark_WalkStructJson(b *testing.B) {
	var dump = ioutil.Discard
	var val = typeA{
		"str1",
		"str2",
		typeB{"str3"},
		4,
	}
	encoder := json.NewEncoder(dump)
	for i := 0; i < b.N; i++ {
		encoder.Encode(val)
	}
}

// GC on:  1095 ns/op -- note these times are getting *very* erratic down here because gc may spill between bench funcs
// GC off: 1084 ns/op
// Membench: 8 B/op  1 allocs/op -- Does result in substantially different number of alloc
func Benchmark_WalkStructRefJson(b *testing.B) {
	var dump = ioutil.Discard
	var val = typeA{
		"str1",
		"str2",
		typeB{"str3"},
		4,
	}
	encoder := json.NewEncoder(dump)
	for i := 0; i < b.N; i++ {
		encoder.Encode(&val)
	}
}
