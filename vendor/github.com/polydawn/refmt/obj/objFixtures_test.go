package obj

import (
	"fmt"
	"reflect"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/polydawn/refmt/obj/atlas"
	. "github.com/polydawn/refmt/tok"
	"github.com/polydawn/refmt/tok/fixtures"
)

var skipMe = fmt.Errorf("skipme")

type marshalResults struct {
	title string
	// Yields a value to hand to the marshaller.
	// A func returning a wildcard is used rather than just an `interface{}`, because `&target` conveys very different type information.
	valueFn func() interface{}

	expectErr error
	errString string
}
type unmarshalResults struct {
	title string
	// Yields the handle we should give to the unmarshaller to fill.
	// Like `valueFn`, the indirection here is to help
	slotFn func() interface{}

	// Yields the value we will compare the unmarshal result against.
	// A func returning a wildcard is used rather than just an `interface{}`, because `&target` conveys very different type information.
	valueFn   func() interface{}
	expectErr error
	errString string
}

type tObjStr struct {
	X string
}

type tObjStr2 struct {
	X string
	Y string
}

type tObjK struct {
	K []tObjK2
}
type tObjK2 struct {
	K2 int
}

type tDefStr string
type tDefInt int
type tDefBytes []byte

type tObjStrp struct {
	X *string
}

type tObjPtrObjStrp struct {
	P *tObjStrp
}

type tObjPtrObjStrp2 struct {
	P1 *tObjStrp
	P2 *tObjStrp
}

type tObjMap struct {
	X map[string]interface{}
}

type tObjPtrObjMap struct {
	P *tObjMap
}

type t5 struct {
	K1 string
	K2 string
	K3 string
	K4 []string
	K5 string
}

type t5b struct {
	K1 tObjStr
	K2 string
	K3 tObjStr2
	K4 []tObjStr
	K5 tObjStr
}

type tFieldSort1 struct {
	Aaaaa string
	Bbbb  string
	Ddd   string
	Ccc   string
	Eee   string
	Ff    string
	G     string
}

var objFixtures = []struct {
	title string

	// The serial sequence of tokens the value is isomorphic to.
	sequence fixtures.Sequence

	// The suite of mappings to use.
	atlas atlas.Atlas

	// The results to expect from various marshalling starting points.
	// This is a slice because we occasionally have several different kinds of objects
	// which we expect will converge on the same token fixture given the same atlas.
	marshalResults []marshalResults

	// The results to expect from various unmarshal situations.
	// This is a slice because unmarshal may have different outcomes (usually,
	// erroring vs not) depending on the type of value it was given to populate.
	unmarshalResults []unmarshalResults
}{
	{title: "string literal",
		sequence: fixtures.SequenceMap["flat string"],
		marshalResults: []marshalResults{
			{title: "from string literal",
				valueFn: func() interface{} { str := "value"; return str }},
			{title: "from *string",
				valueFn: func() interface{} { str := "value"; return &str }},
			{title: "from **string",
				valueFn: func() interface{} { str := "value"; strp := &str; return &strp }},
			{title: "from string in iface slot",
				valueFn: func() interface{} { var iface interface{}; iface = "value"; return iface }},
			{title: "from string in *iface slot",
				valueFn: func() interface{} { var iface interface{}; iface = "value"; return &iface }},
			{title: "from *string in iface slot",
				valueFn: func() interface{} { str := "value"; var iface interface{}; iface = &str; return iface }},
			{title: "from *string in *iface slot",
				valueFn: func() interface{} { str := "value"; var iface interface{}; iface = &str; return &iface }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into string",
				slotFn:    func() interface{} { var str string; return str },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf("")}},
			{title: "into *string",
				slotFn:  func() interface{} { var str string; return &str },
				valueFn: func() interface{} { str := "value"; return str }},
			{title: "into **string",
				slotFn:  func() interface{} { var strp *string; return &strp },
				valueFn: func() interface{} { str := "value"; return &str }},
			{title: "into ***string",
				slotFn:  func() interface{} { var strpp **string; return &strpp },
				valueFn: func() interface{} { str := "value"; strp := &str; return &strp }},
			{title: "into wildcard",
				slotFn:    func() interface{} { var v interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(interface{}(nil))}},
			{title: "into *wildcard",
				slotFn:  func() interface{} { var v interface{}; return &v },
				valueFn: func() interface{} { str := "value"; return str }},
			{title: "into map[str]iface",
				slotFn:    func() interface{} { var v map[string]interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}(nil))}},
			{title: "into *map[str]iface",
				slotFn:    func() interface{} { var v map[string]interface{}; return &v },
				expectErr: ErrUnmarshalTypeCantFit{Token{Type: TString, Str: "value"}, reflect.ValueOf(map[string]interface{}(nil)), 0}},
			{title: "into []iface",
				slotFn:    func() interface{} { var v []interface{}; return v },
				expectErr: skipMe},
			{title: "into *[]iface",
				slotFn:    func() interface{} { var v []interface{}; return &v },
				expectErr: skipMe},
		},
	},
	{title: "empty maps",
		sequence: fixtures.SequenceMap["empty map"],
		marshalResults: []marshalResults{
			{title: "from map[str]iface",
				valueFn: func() interface{} { return map[string]interface{}{} }},
			{title: "from map[str]str",
				valueFn: func() interface{} { return map[string]string{} }},
			{title: "from *map[str]str",
				valueFn: func() interface{} { m := map[string]string{}; return &m }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into string",
				slotFn:    func() interface{} { var str string; return str },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf("")}},
			{title: "into *string",
				slotFn:    func() interface{} { var str string; return &str },
				expectErr: ErrUnmarshalTypeCantFit{Token{Type: TMapOpen, Length: 0}, reflect.ValueOf(""), 0}},
			{title: "into wildcard",
				slotFn:    func() interface{} { var v interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(interface{}(nil))}},
			{title: "into *wildcard",
				slotFn:  func() interface{} { var v interface{}; return &v },
				valueFn: func() interface{} { return map[string]interface{}{} }},
			{title: "into map[str]iface",
				slotFn:    func() interface{} { var v map[string]interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}(nil))}},
			{title: "into made map[str]iface",
				slotFn:    func() interface{} { v := make(map[string]interface{}); return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}{})}},
			{title: "into *map[str]iface",
				slotFn:  func() interface{} { var v map[string]interface{}; return &v },
				valueFn: func() interface{} { return map[string]interface{}{} }},
			{title: "into *map[str]str",
				slotFn:  func() interface{} { var v map[string]string; return &v },
				valueFn: func() interface{} { return map[string]string{} }},
			{title: "into []iface",
				slotFn:    func() interface{} { var v []interface{}; return v },
				expectErr: skipMe},
			{title: "into *[]iface",
				slotFn:    func() interface{} { var v []interface{}; return &v },
				expectErr: skipMe},
		},
	},
	{title: "object with one string field, with atlas entry",
		sequence: fixtures.SequenceMap["single row map"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjStr{}).StructMap().
				AddField("X", atlas.StructMapEntry{SerialName: "key"}).
				Complete(),
		),
		marshalResults: []marshalResults{
			{title: "from object with one field",
				valueFn: func() interface{} { return tObjStr{"value"} }},
			{title: "from map[str]iface with one entry",
				valueFn: func() interface{} { return map[string]interface{}{"key": "value"} }},
			{title: "from map[str]str with one entry",
				valueFn: func() interface{} { return map[string]string{"key": "value"} }},
			{title: "from *map[str]str",
				valueFn: func() interface{} { m := map[string]string{"key": "value"}; return &m }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into string",
				slotFn:    func() interface{} { var str string; return str },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf("")}},
			{title: "into *string",
				slotFn:    func() interface{} { var str string; return &str },
				expectErr: ErrUnmarshalTypeCantFit{Token{Type: TMapOpen, Length: 1}, reflect.ValueOf(""), 0}},
			{title: "into wildcard",
				slotFn:    func() interface{} { var v interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(interface{}(nil))}},
			{title: "into *wildcard",
				slotFn:  func() interface{} { var v interface{}; return &v },
				valueFn: func() interface{} { return map[string]interface{}{"key": "value"} }},
			{title: "into map[str]iface",
				slotFn:    func() interface{} { var v map[string]interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}(nil))}},
			{title: "into made map[str]iface",
				slotFn:    func() interface{} { v := make(map[string]interface{}); return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}{})}},
			{title: "into *map[str]iface",
				slotFn:  func() interface{} { var v map[string]interface{}; return &v },
				valueFn: func() interface{} { return map[string]interface{}{"key": "value"} }},
			{title: "into *map[str]str",
				slotFn:  func() interface{} { var v map[string]string; return &v },
				valueFn: func() interface{} { return map[string]string{"key": "value"} }},
			{title: "into []iface",
				slotFn:    func() interface{} { var v []interface{}; return v },
				expectErr: skipMe},
			{title: "into *[]iface",
				slotFn:    func() interface{} { var v []interface{}; return &v },
				expectErr: skipMe},
		},
	},
	{title: "object with two string fields, with atlas entry",
		sequence: fixtures.SequenceMap["duo row map"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjStr2{}).StructMap().
				AddField("X", atlas.StructMapEntry{SerialName: "key"}).
				AddField("Y", atlas.StructMapEntry{SerialName: "k2"}).
				Complete(),
		),
		marshalResults: []marshalResults{
			{title: "from object with two fields",
				valueFn: func() interface{} { return tObjStr2{"value", "v2"} }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into string",
				slotFn:    func() interface{} { var str string; return str },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf("")}},
			{title: "into *string",
				slotFn:    func() interface{} { var str string; return &str },
				expectErr: ErrUnmarshalTypeCantFit{Token{Type: TMapOpen, Length: 2}, reflect.ValueOf(""), 0}},
			{title: "into wildcard",
				slotFn:    func() interface{} { var v interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(interface{}(nil))}},
			{title: "into *wildcard",
				slotFn:  func() interface{} { var v interface{}; return &v },
				valueFn: func() interface{} { return map[string]interface{}{"key": "value", "k2": "v2"} }},
			{title: "into map[str]iface",
				slotFn:    func() interface{} { var v map[string]interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}(nil))}},
			{title: "into made map[str]iface",
				slotFn:    func() interface{} { v := make(map[string]interface{}); return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}{})}},
			{title: "into *map[str]iface",
				slotFn:  func() interface{} { var v map[string]interface{}; return &v },
				valueFn: func() interface{} { return map[string]interface{}{"key": "value", "k2": "v2"} }},
			{title: "into *map[str]str",
				slotFn:  func() interface{} { var v map[string]string; return &v },
				valueFn: func() interface{} { return map[string]string{"key": "value", "k2": "v2"} }},
			{title: "into []iface",
				slotFn:    func() interface{} { var v []interface{}; return v },
				expectErr: skipMe},
			{title: "into *[]iface",
				slotFn:    func() interface{} { var v []interface{}; return &v },
				expectErr: skipMe},
			{title: "into tObjStr2",
				slotFn:    func() interface{} { return tObjStr2{} },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(tObjStr2{})}},
			{title: "into *tObjStr2",
				slotFn:  func() interface{} { return &tObjStr2{} },
				valueFn: func() interface{} { return tObjStr2{"value", "v2"} }},
		},
	},
	{title: "object with two string fields, with atlas entry, unmarshalling accepts alt2 serial ordering",
		sequence: fixtures.SequenceMap["duo row map alt2"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjStr2{}).StructMap().
				AddField("X", atlas.StructMapEntry{SerialName: "key"}).
				AddField("Y", atlas.StructMapEntry{SerialName: "k2"}).
				Complete(),
		),
		unmarshalResults: []unmarshalResults{
			{title: "into *wildcard",
				slotFn:  func() interface{} { var v interface{}; return &v },
				valueFn: func() interface{} { return map[string]interface{}{"key": "value", "k2": "v2"} }},
			{title: "into *map[str]iface",
				slotFn:  func() interface{} { var v map[string]interface{}; return &v },
				valueFn: func() interface{} { return map[string]interface{}{"key": "value", "k2": "v2"} }},
			{title: "into *map[str]str",
				slotFn:  func() interface{} { var v map[string]string; return &v },
				valueFn: func() interface{} { return map[string]string{"key": "value", "k2": "v2"} }},
			{title: "into *tObjStr2",
				slotFn:  func() interface{} { return &tObjStr2{} },
				valueFn: func() interface{} { return tObjStr2{"value", "v2"} }},
		},
	},
	{title: "object with two string fields, with atlas entry, unmarshalling rejects when fields mismatch",
		sequence: fixtures.SequenceMap["duo row map"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjStr2{}).StructMap().
				AddField("X", atlas.StructMapEntry{SerialName: "key"}).
				AddField("Y", atlas.StructMapEntry{SerialName: "mismatch"}). // we alter this to not line up with the token stream
				Complete(),
		),
		unmarshalResults: []unmarshalResults{
			{title: "into *tObjStr2",
				slotFn:    func() interface{} { return &tObjStr2{} },
				expectErr: ErrNoSuchField{"k2", reflect.TypeOf(tObjStr2{}).String()}},
		},
	},
	{title: "object with four string fields, with atlas entry (default key ordering), marshals ordered correctly",
		// Note this test is capable of passing *by luck* since map walks are semi-random.
		// Map walks *are* biased towards their declaration order though, as far as I can tell,
		// so it's less than a 1/4 chance here.
		sequence: fixtures.SequenceMap["quad map default order"],
		atlas:    atlas.MustBuild().WithMapMorphism(atlas.MapMorphism{atlas.KeySortMode_Default}),
		marshalResults: []marshalResults{
			{title: "from map",
				valueFn: func() interface{} {
					return map[string]string{
						"d":  "4", // n.b. intentionally not in same order as tokens should be
						"b":  "2",
						"bc": "3",
						"1":  "1",
					}
				}},
		},
	},
	{title: "object with four string fields, with atlas entry (rfc7049 key ordering), marshals ordered correctly",
		// Note this test is capable of passing *by luck* since map walks are semi-random.
		// Map walks *are* biased towards their declaration order though, as far as I can tell,
		// so it's less than a 1/4 chance here.
		sequence: fixtures.SequenceMap["quad map rfc7049 order"],
		atlas:    atlas.MustBuild().WithMapMorphism(atlas.MapMorphism{atlas.KeySortMode_RFC7049}),
		marshalResults: []marshalResults{
			{title: "from map",
				valueFn: func() interface{} {
					return map[string]string{
						"d":  "3", // n.b. intentionally not in same order as tokens should be
						"b":  "2",
						"bc": "4",
						"1":  "1",
					}
				}},
		},
	},
	{title: "object with 10 string fields, with atlas entry (rfc7049 key ordering), marshals ordered correctly",
		sequence: fixtures.SequenceMap["10 map rfc7049 order"],
		atlas:    atlas.MustBuild().WithMapMorphism(atlas.MapMorphism{atlas.KeySortMode_RFC7049}),
		marshalResults: []marshalResults{
			{title: "from map",
				valueFn: func() interface{} {
					return map[string]string{
						"hello":  "9",
						"d":      "4",
						"b":      "3",
						"bc":     "6",
						"1":      "1",
						"bccccc": "10",
						"2":      "2",
						"11":     "5",
						"hell":   "8",
						"he":     "7",
					}
				}},
		},
	},
	{title: "struct with fields in different than expected order (rfc7049 expected)",
		sequence: fixtures.SequenceMap["7 struct rfc7049 order"],
		atlas:    atlas.MustBuild(atlas.BuildEntry(tFieldSort1{}).StructMap().AutogenerateWithSortingScheme(atlas.KeySortMode_RFC7049).Complete()),
		marshalResults: []marshalResults{
			{title: "from struct",
				valueFn: func() interface{} {
					return tFieldSort1{
						G:     "1",
						Ff:    "2",
						Ccc:   "3",
						Ddd:   "4",
						Eee:   "5",
						Bbbb:  "6",
						Aaaaa: "7",
					}
				}},
		},
	},
	{title: "empty primitive arrays",
		sequence: fixtures.SequenceMap["empty array"],
		marshalResults: []marshalResults{
			{title: "from int array",
				valueFn: func() interface{} { return [0]int{} }},
			{title: "from int slice",
				valueFn: func() interface{} { return []int{} }},
			{title: "from iface array",
				valueFn: func() interface{} { return [0]interface{}{} }},
			{title: "from iface slice",
				valueFn: func() interface{} { return []interface{}{} }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into string",
				slotFn:    func() interface{} { var str string; return str },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf("")}},
			{title: "into *string",
				slotFn:    func() interface{} { var str string; return &str },
				expectErr: ErrUnmarshalTypeCantFit{Token{Type: TArrOpen, Length: 0}, reflect.ValueOf(""), 0}},
			{title: "into wildcard",
				slotFn:    func() interface{} { var v interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(interface{}(nil))}},
			{title: "into *wildcard",
				slotFn:  func() interface{} { var v interface{}; return &v },
				valueFn: func() interface{} { return []interface{}{} }},
			{title: "into map[str]iface",
				slotFn:    func() interface{} { var v map[string]interface{}; return v },
				expectErr: skipMe},
			{title: "into made map[str]iface",
				slotFn:    func() interface{} { v := make(map[string]interface{}); return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}{})}},
			{title: "into *map[str]iface",
				slotFn:    func() interface{} { var v map[string]interface{}; return &v },
				expectErr: skipMe},
			{title: "into *map[str]str",
				slotFn:    func() interface{} { var v map[string]string; return &v },
				expectErr: skipMe},
			{title: "into []iface",
				slotFn: func() interface{} { var v []interface{}; return v },
				// array/slice direct: theoretically possible, as long as it's short enough.  but not supported right now.
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]interface{}{})}},
			{title: "into *[]iface",
				slotFn:  func() interface{} { var v []interface{}; return &v },
				valueFn: func() interface{} { return []interface{}{} }},
			{title: "into []str",
				slotFn:    func() interface{} { var v []string; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]string{})}},
			{title: "into *[]str",
				slotFn:  func() interface{} { var v []string; return &v },
				valueFn: func() interface{} { return []string{} }},
			{title: "into [0]str",
				slotFn:    func() interface{} { var v []string; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]string{})}},
			{title: "into *[0]str",
				slotFn:  func() interface{} { var v [0]string; return &v },
				valueFn: func() interface{} { return [0]string{} }},
			{title: "into [2]str",
				slotFn:    func() interface{} { var v []string; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]string{})}},
			{title: "into *[2]str",
				slotFn:  func() interface{} { var v [2]string; return &v },
				valueFn: func() interface{} { return [2]string{} }},
			{title: "into []int",
				slotFn:    func() interface{} { var v []int; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]int{})}},
			{title: "into *[]int",
				slotFn:  func() interface{} { var v []int; return &v },
				valueFn: func() interface{} { return []int{} }}, // fine *despite the type mismatch* because with no tokens, well, nobody's the wiser.
		},
	},
	{title: "short primitive arrays",
		sequence: fixtures.SequenceMap["duo entry array"],
		marshalResults: []marshalResults{
			{title: "from str array",
				valueFn: func() interface{} { return [2]string{"value", "v2"} }},
			{title: "from str slice",
				valueFn: func() interface{} { return []string{"value", "v2"} }},
			{title: "from iface array",
				valueFn: func() interface{} { return [2]interface{}{"value", "v2"} }},
			{title: "from iface slice",
				valueFn: func() interface{} { return []interface{}{"value", "v2"} }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into string",
				slotFn:    func() interface{} { var str string; return str },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf("")}},
			{title: "into *string",
				slotFn:    func() interface{} { var str string; return &str },
				expectErr: ErrUnmarshalTypeCantFit{Token{Type: TArrOpen, Length: 2}, reflect.ValueOf(""), 0}},
			{title: "into wildcard",
				slotFn:    func() interface{} { var v interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(interface{}(nil))}},
			{title: "into *wildcard",
				slotFn:  func() interface{} { var v interface{}; return &v },
				valueFn: func() interface{} { return []interface{}{"value", "v2"} }},
			{title: "into map[str]iface",
				slotFn:    func() interface{} { var v map[string]interface{}; return v },
				expectErr: skipMe},
			{title: "into made map[str]iface",
				slotFn:    func() interface{} { v := make(map[string]interface{}); return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}{})}},
			{title: "into *map[str]iface",
				slotFn:    func() interface{} { var v map[string]interface{}; return &v },
				expectErr: skipMe},
			{title: "into *map[str]str",
				slotFn:    func() interface{} { var v map[string]string; return &v },
				expectErr: skipMe},
			{title: "into []iface",
				slotFn:    func() interface{} { var v []interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]interface{}{})}},
			{title: "into *[]iface",
				slotFn:  func() interface{} { var v []interface{}; return &v },
				valueFn: func() interface{} { return []interface{}{"value", "v2"} }},
			{title: "into []str",
				slotFn:    func() interface{} { var v []string; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]string{})}},
			{title: "into *[]str",
				slotFn:  func() interface{} { var v []string; return &v },
				valueFn: func() interface{} { return []string{"value", "v2"} }},
			{title: "into [0]str",
				slotFn:    func() interface{} { var v []string; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]string{})}},
			{title: "into *[0]str",
				slotFn:    func() interface{} { var v [0]string; return &v },
				expectErr: ErrMalformedTokenStream{TString, "end of array (out of space)"}},
			{title: "into [2]str",
				slotFn:    func() interface{} { var v []string; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]string{})}},
			{title: "into *[2]str",
				slotFn:  func() interface{} { var v [2]string; return &v },
				valueFn: func() interface{} { return [2]string{"value", "v2"} }},
			{title: "into []int",
				slotFn:    func() interface{} { var v []int; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]int{})}},
			{title: "into *[]int",
				slotFn:    func() interface{} { var v []int; return &v },
				expectErr: ErrUnmarshalTypeCantFit{Token{Type: TString, Str: "value"}, reflect.ValueOf(0), 0}},
		},
	},
	{title: "maps in maps",
		sequence: fixtures.SequenceMap["maps nested in maps"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjPtrObjStrp{}).StructMap().
				AddField("P", atlas.StructMapEntry{SerialName: "k"}).
				Complete(),
			atlas.BuildEntry(tObjPtrObjStrp2{}).StructMap().
				AddField("P1", atlas.StructMapEntry{SerialName: "k"}).
				Complete(),
			atlas.BuildEntry(tObjStrp{}).StructMap().
				AddField("X", atlas.StructMapEntry{SerialName: "k2"}).
				Complete(),
		),
		marshalResults: []marshalResults{
			{title: "from map[str]iface with map[str]str",
				valueFn: func() interface{} {
					return map[string]interface{}{"k": map[string]string{"k2": "v2"}}
				}},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into string",
				slotFn:    func() interface{} { var str string; return str },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf("")}},
			{title: "into *map[str]map[str]str",
				slotFn: func() interface{} { var m map[string]map[string]string; return &m },
				valueFn: func() interface{} {
					return map[string]map[string]string{"k": {"k2": "v2"}}
				}},
			{title: "into *tObjPtrObjStrp{}",
				slotFn: func() interface{} { return &tObjPtrObjStrp{} },
				valueFn: func() interface{} {
					str := "v2"
					return tObjPtrObjStrp{&tObjStrp{&str}}
				}},
			{title: "into *tObjPtrObjStrp2{}, using the first field",
				slotFn: func() interface{} { return &tObjPtrObjStrp2{} },
				valueFn: func() interface{} {
					str := "v2"
					return tObjPtrObjStrp2{P1: &tObjStrp{&str}}
				}},
		},
	},
	{title: "maps in maps, using an atlas hitting different field orders",
		sequence: fixtures.SequenceMap["maps nested in maps"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjPtrObjStrp2{}).StructMap().
				AddField("P2", atlas.StructMapEntry{SerialName: "k"}).
				Complete(),
			atlas.BuildEntry(tObjStrp{}).StructMap().
				AddField("X", atlas.StructMapEntry{SerialName: "k2"}).
				Complete(),
		),
		unmarshalResults: []unmarshalResults{
			{title: "into *tObjPtrObjStrp2{}, using the second field",
				slotFn: func() interface{} { return &tObjPtrObjStrp2{} },
				valueFn: func() interface{} {
					str := "v2"
					return tObjPtrObjStrp2{P2: &tObjStrp{&str}}
				}},
		},
	},
	{title: "maps in maps with mixed nulls",
		sequence: fixtures.SequenceMap["maps nested in maps with mixed nulls"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjPtrObjStrp2{}).StructMap().
				AddField("P1", atlas.StructMapEntry{SerialName: "k"}).
				AddField("P2", atlas.StructMapEntry{SerialName: "k2"}).
				Complete(),
			atlas.BuildEntry(tObjStrp{}).StructMap().
				AddField("X", atlas.StructMapEntry{SerialName: "k2"}).
				Complete(),
		),
		unmarshalResults: []unmarshalResults{
			{title: "into *tObjPtrObjStrp2{}, null going into the second field",
				slotFn: func() interface{} { return &tObjPtrObjStrp2{} },
				valueFn: func() interface{} {
					str := "v2"
					return tObjPtrObjStrp2{P1: &tObjStrp{&str}}
				}},
			{title: "into *tObjPtrObjStrp2{}, null going into the second field and overwriting an earlier value there",
				slotFn: func() interface{} { return &tObjPtrObjStrp2{P2: &tObjStrp{}} },
				valueFn: func() interface{} {
					str := "v2"
					return tObjPtrObjStrp2{P1: &tObjStrp{&str}}
				}},
			{title: "into **tObjPtrObjStrp2{}, which hits a ptrdefer machine multiple times",
				slotFn: func() interface{} { v := &tObjPtrObjStrp2{}; return &v },
				valueFn: func() interface{} {
					str := "v2"
					return &tObjPtrObjStrp2{P1: &tObjStrp{&str}}
				}},
		},
	},
	{title: "array nest in map pt1",
		sequence: fixtures.SequenceMap["array nested in map as non-first and final entry"],
		marshalResults: []marshalResults{
			{title: "from map[str]iface with nested []iface",
				valueFn: func() interface{} {
					return map[string]interface{}{"k1": "v1", "ke": []interface{}{"oh", "whee", "wow"}}
				}},
			{title: "from map[str]iface with nested []str",
				valueFn: func() interface{} {
					return map[string]interface{}{"k1": "v1", "ke": []string{"oh", "whee", "wow"}}
				}},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into string",
				slotFn:    func() interface{} { var str string; return str },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf("")}},
			{title: "into *string",
				slotFn:    func() interface{} { var str string; return &str },
				expectErr: ErrUnmarshalTypeCantFit{Token{Type: TMapOpen, Length: 2}, reflect.ValueOf(""), 0}},
			{title: "into wildcard",
				slotFn:    func() interface{} { var v interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(interface{}(nil))}},
			{title: "into *wildcard",
				slotFn: func() interface{} { var v interface{}; return &v },
				valueFn: func() interface{} {
					return map[string]interface{}{"k1": "v1", "ke": []interface{}{"oh", "whee", "wow"}}
				}},
			{title: "into map[str]iface",
				slotFn:    func() interface{} { var v map[string]interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}(nil))}},
			{title: "into made map[str]iface",
				slotFn:    func() interface{} { v := make(map[string]interface{}); return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}{})}},
			{title: "into *map[str]iface",
				slotFn: func() interface{} { var v map[string]interface{}; return &v },
				valueFn: func() interface{} {
					return map[string]interface{}{"k1": "v1", "ke": []interface{}{"oh", "whee", "wow"}}
				}},
			{title: "into *map[str]str",
				slotFn: func() interface{} { var v map[string]string; return &v },
				//expectErr: ErrUnmarshalTypeCantFit{Token{Type: TArrOpen, Length: 3}, reflect.ValueOf("")}},
				expectErr: skipMe}, // big tricky todo: currently falls in the cracks where reflect core panics.
			{title: "into []iface",
				slotFn:    func() interface{} { var v []interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]interface{}{})}},
			{title: "into *[]iface",
				slotFn:    func() interface{} { var v []interface{}; return &v },
				expectErr: skipMe}, // should certainly error, but not well spec'd yet
			{title: "into []str",
				slotFn:    func() interface{} { var v []string; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]string{})}},
			{title: "into *[]str",
				slotFn:    func() interface{} { var v []string; return &v },
				expectErr: skipMe}, // should certainly error, but not well spec'd yet
			{title: "into []int",
				slotFn:    func() interface{} { var v []int; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]int{})}},
			{title: "into *[]int",
				slotFn:    func() interface{} { var v []int; return &v },
				expectErr: skipMe}, // should certainly error, but not well spec'd yet
		},
	},
	{title: "nested maps and arrays with no wildcards",
		sequence: fixtures.SequenceMap["map[str][]map[str]int"],
		marshalResults: []marshalResults{
			{title: "from oh-so-much type info",
				valueFn: func() interface{} {
					return map[string][]map[string]int{"k": []map[string]int{
						map[string]int{"k2": 1},
						map[string]int{"k2": 2},
					}}
				}},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into oh-so-much type info",
				slotFn: func() interface{} { var v map[string][]map[string]int; return &v },
				valueFn: func() interface{} {
					return map[string][]map[string]int{"k": []map[string]int{
						map[string]int{"k2": 1},
						map[string]int{"k2": 2},
					}}
				}},
		},
	},
	{title: "nested maps and arrays as atlased structs",
		sequence: fixtures.SequenceMap["map[str][]map[str]int"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjK{}).StructMap().
				AddField("K", atlas.StructMapEntry{SerialName: "k"}).
				Complete(),
			atlas.BuildEntry(tObjK2{}).StructMap().
				AddField("K2", atlas.StructMapEntry{SerialName: "k2"}).
				Complete(),
		),
		marshalResults: []marshalResults{
			{title: "from tObjK{[]tObjK2{}}",
				valueFn: func() interface{} {
					return tObjK{[]tObjK2{{1}, {2}}}
				}},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into tObjK{[]tObjK2{}}",
				slotFn: func() interface{} { return &tObjK{} },
				valueFn: func() interface{} {
					return tObjK{[]tObjK2{{1}, {2}}}
				}},
		},
	},
	{title: "nested map-struct-map",
		// the tmp slot required in map unmarshalling creates the potential for
		//  some really wild edge cases...
		sequence: fixtures.SequenceMap["map[str]map[str]map[str]str"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjMap{}).StructMap().
				AddField("X", atlas.StructMapEntry{SerialName: "f"}).
				Complete(),
		),
		marshalResults: []marshalResults{
			{title: "from map[str]tObjMap{map[str]iface}",
				valueFn: func() interface{} {
					return map[string]tObjMap{
						"k1": {X: map[string]interface{}{"d": "aa"}},
						"k2": {X: map[string]interface{}{"d": "bb"}},
					}
				}},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into map[str]tObjMap",
				slotFn: func() interface{} { return &map[string]tObjMap{} },
				valueFn: func() interface{} {
					return map[string]tObjMap{
						"k1": {X: map[string]interface{}{"d": "aa"}},
						"k2": {X: map[string]interface{}{"d": "bb"}},
					}
				}},
		},
	},
	{title: "transform funks (struct<->string)",
		sequence: fixtures.SequenceMap["flat string"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjStr{}).Transform().
				TransformMarshal(atlas.MakeMarshalTransformFunc(
					func(x tObjStr) (string, error) {
						return x.X, nil
					})).
				TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
					func(x string) (tObjStr, error) {
						return tObjStr{x}, nil
					})).
				Complete(),
		),
		marshalResults: []marshalResults{
			{title: "from tObjStr",
				valueFn: func() interface{} { return tObjStr{"value"} }},
			{title: "from *tObjStr",
				valueFn: func() interface{} { return &tObjStr{"value"} }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into *tObjStr",
				slotFn:  func() interface{} { return &tObjStr{} },
				valueFn: func() interface{} { return tObjStr{"value"} }},
			// There are no tests here for "into interface{}" because by definition
			//  those situations wouldn't provide type info that would trigger these paths.
		},
	},
	{title: "transform funks (struct<->string) in a slice",
		sequence: fixtures.SequenceMap["duo entry array"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjStr{}).Transform().
				TransformMarshal(atlas.MakeMarshalTransformFunc(
					func(x tObjStr) (string, error) {
						return x.X, nil
					})).
				TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
					func(x string) (tObjStr, error) {
						return tObjStr{x}, nil
					})).
				Complete(),
		),
		marshalResults: []marshalResults{
			{title: "from []tObjStr",
				valueFn: func() interface{} { return []tObjStr{{"value"}, {"v2"}} }},
			{title: "from *[]tObjStr",
				valueFn: func() interface{} { return &[]tObjStr{{"value"}, {"v2"}} }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into *tObjStr",
				slotFn:    func() interface{} { return &tObjStr{} },
				expectErr: ErrUnmarshalTypeCantFit{Token{Type: TArrOpen, Length: 2}, reflect.ValueOf(""), 0}},
			{title: "into []tObjStr",
				slotFn:    func() interface{} { return []tObjStr{} },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]tObjStr{})}},
			{title: "into *[]tObjStr",
				slotFn:  func() interface{} { return &[]tObjStr{} },
				valueFn: func() interface{} { return []tObjStr{{"value"}, {"v2"}} }},
		},
	},
	{title: "typedef strings",
		sequence: fixtures.SequenceMap["flat string"],
		// no atlas necessary: the default behavior for a kind, even if typedef'd,
		//  is to simply to the natural thing for that kind.
		marshalResults: []marshalResults{
			{title: "from tDefStr literal",
				valueFn: func() interface{} { str := tDefStr("value"); return str }},
			{title: "from *tDefStr",
				valueFn: func() interface{} { str := tDefStr("value"); return &str }},
			{title: "from tDefStr in iface slot",
				valueFn: func() interface{} { var iface interface{}; iface = tDefStr("value"); return iface }},
			{title: "from tDefStr in *iface slot",
				valueFn: func() interface{} { var iface interface{}; iface = tDefStr("value"); return &iface }},
			{title: "from *tDefStr in iface slot",
				valueFn: func() interface{} { str := tDefStr("value"); var iface interface{}; iface = &str; return iface }},
			{title: "from *tDefStr in *iface slot",
				valueFn: func() interface{} { str := tDefStr("value"); var iface interface{}; iface = &str; return &iface }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into tDefStr",
				slotFn:    func() interface{} { var str tDefStr; return str },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(tDefStr(""))}},
			{title: "into *tDefStr",
				slotFn:  func() interface{} { var str tDefStr; return &str },
				valueFn: func() interface{} { str := tDefStr("value"); return str }},
		},
	},
	{title: "typedef ints",
		sequence: fixtures.SequenceMap["integer one"],
		// no atlas necessary: the default behavior for a kind, even if typedef'd,
		//  is to simply to the natural thing for that kind.
		marshalResults: []marshalResults{
			{title: "from tDefInt literal",
				valueFn: func() interface{} { v := tDefInt(1); return v }},
			{title: "from *tDefInt",
				valueFn: func() interface{} { v := tDefInt(1); return &v }},
			{title: "from tDefInt in iface slot",
				valueFn: func() interface{} { var iface interface{}; iface = tDefInt(1); return iface }},
			{title: "from tDefInt in *iface slot",
				valueFn: func() interface{} { var iface interface{}; iface = tDefInt(1); return &iface }},
			{title: "from *tDefInt in iface slot",
				valueFn: func() interface{} { v := tDefInt(1); var iface interface{}; iface = &v; return iface }},
			{title: "from *tDefInt in *iface slot",
				valueFn: func() interface{} { v := tDefInt(1); var iface interface{}; iface = &v; return &iface }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into tDefInt",
				slotFn:    func() interface{} { var v tDefInt; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(tDefInt(0))}},
			{title: "into *tDefInt",
				slotFn:  func() interface{} { var v tDefInt; return &v },
				valueFn: func() interface{} { v := tDefInt(1); return v }},
		},
	},
	{title: "typedef bytes",
		sequence: fixtures.SequenceMap["short byte array"],
		// no atlas necessary: the default behavior for a kind, even if typedef'd,
		//  is to simply to the natural thing for that kind.
		marshalResults: []marshalResults{
			{title: "from tDefBytes literal",
				valueFn: func() interface{} { v := tDefBytes(`value`); return v }},
			{title: "from *tDefBytes",
				valueFn: func() interface{} { v := tDefBytes(`value`); return &v }},
			{title: "from tDefBytes in iface slot",
				valueFn: func() interface{} { var iface interface{}; iface = tDefBytes(`value`); return iface }},
			{title: "from tDefBytes in *iface slot",
				valueFn: func() interface{} { var iface interface{}; iface = tDefBytes(`value`); return &iface }},
			{title: "from *tDefBytes in iface slot",
				valueFn: func() interface{} { v := tDefBytes(`value`); var iface interface{}; iface = &v; return iface }},
			{title: "from *tDefBytes in *iface slot",
				valueFn: func() interface{} { v := tDefBytes(`value`); var iface interface{}; iface = &v; return &iface }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into tDefBytes",
				slotFn:    func() interface{} { var v tDefBytes; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(tDefBytes{})}},
			{title: "into *tDefBytes",
				slotFn:  func() interface{} { var v tDefBytes; return &v },
				valueFn: func() interface{} { v := tDefBytes(`value`); return v }},
		},
	},
	{title: "maps with typedef keys (still of string kind)",
		sequence: fixtures.SequenceMap["single row map"],
		marshalResults: []marshalResults{
			{title: "from map[tDefStr]iface with one entry",
				valueFn: func() interface{} { return map[tDefStr]interface{}{"key": "value"} }},
			{title: "from map[tDefStr]str with one entry",
				valueFn: func() interface{} { return map[tDefStr]string{"key": "value"} }},
			{title: "from *map[tDefStr]str",
				valueFn: func() interface{} { m := map[tDefStr]string{"key": "value"}; return &m }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into map[tDefStr]iface",
				slotFn:    func() interface{} { var v map[tDefStr]interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[tDefStr]interface{}(nil))}},
			{title: "into *map[tDefStr]iface",
				slotFn:  func() interface{} { var v map[tDefStr]interface{}; return &v },
				valueFn: func() interface{} { return map[tDefStr]interface{}{"key": "value"} }},
		},
	},
	{title: "empty",
		sequence:       fixtures.SequenceMap["empty"],
		marshalResults: []marshalResults{
			// not much marshals to empty!
		},
		unmarshalResults: []unmarshalResults{
			{title: "into string",
				slotFn:    func() interface{} { var str string; return str },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf("")}},
			{title: "into *string",
				slotFn:  func() interface{} { var str string; return &str },
				valueFn: func() interface{} { return "" }},
			{title: "into wildcard",
				slotFn:    func() interface{} { var v interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(interface{}(nil))}},
			{title: "into *wildcard",
				slotFn:  func() interface{} { var v interface{}; return &v },
				valueFn: func() interface{} { return nil }},
			{title: "into map[str]iface",
				slotFn:  func() interface{} { var v map[string]interface{}; return v },
				valueFn: func() interface{} { return map[string]interface{}(nil) }},
			{title: "into made map[str]iface",
				slotFn:    func() interface{} { v := make(map[string]interface{}); return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}{})}},
			{title: "into *map[str]iface",
				slotFn:  func() interface{} { var v map[string]interface{}; return &v },
				valueFn: func() interface{} { return map[string]interface{}(nil) }},
			{title: "into *map[str]str",
				slotFn:  func() interface{} { var v map[string]string; return &v },
				valueFn: func() interface{} { return map[string]string(nil) }},
			{title: "into []iface",
				slotFn:    func() interface{} { var v []interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]interface{}{})}},
			{title: "into *[]iface",
				slotFn:  func() interface{} { var v []interface{}; return &v },
				valueFn: func() interface{} { return []interface{}(nil) }},
			{title: "into []str",
				slotFn:    func() interface{} { var v []string; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]string{})}},
			{title: "into *[]str",
				slotFn:  func() interface{} { var v []string; return &v },
				valueFn: func() interface{} { return []string(nil) }},
			{title: "into []int",
				slotFn:    func() interface{} { var v []int; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]int{})}},
			{title: "into *[]int",
				slotFn:  func() interface{} { var v []int; return &v },
				valueFn: func() interface{} { return []int(nil) }},
		},
	},
	{title: "null",
		sequence: fixtures.SequenceMap["null"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjStr{}).StructMap().
				AddField("X", atlas.StructMapEntry{SerialName: "key"}).
				Complete(),
		),
		marshalResults: []marshalResults{
			{title: "from *string",
				valueFn: func() interface{} { var strp *string; return strp }},
			{title: "from *string in iface slot",
				valueFn: func() interface{} { var strp *string; var iface interface{}; iface = strp; return iface }},
			{title: "from *string in *iface slot",
				valueFn: func() interface{} { var strp *string; var iface interface{}; iface = strp; return &iface }},
			{title: "from **string",
				valueFn: func() interface{} { var strp *string; return &strp }},
			{title: "from **string in iface slot",
				valueFn: func() interface{} { var strp *string; var iface interface{}; iface = &strp; return iface }},
			{title: "from nil return",
				valueFn: func() interface{} { return nil }}, // this is the illusive "invalid" kind!  even `reflect.ValueOf(nil).Type()` will panic!
			{title: "from nil in iface slot",
				valueFn: func() interface{} { var iface interface{}; iface = nil; return iface }}, // same as previous test row.
			{title: "from nil in *iface slot",
				valueFn: func() interface{} { var iface interface{}; iface = nil; return &iface }},
			{title: "from map[str]iface",
				valueFn: func() interface{} { return map[string]interface{}(nil) }},
			{title: "from map[str]str",
				valueFn: func() interface{} { return map[string]string(nil) }},
			{title: "from *map[str]str (nil map, valid ptr)",
				valueFn: func() interface{} { m := map[string]string(nil); return &m }},
			{title: "from *map[str]str (nil ptr)",
				valueFn: func() interface{} { var mp *map[string]string; return mp }},
			//{title: "from int array", // Not Possible!  Compiler says: "cannot convert nil to type [0]int"
			//	valueFn: func() interface{} { return [0]int(nil) }},
			{title: "from int slice",
				valueFn: func() interface{} { return []int(nil) }},
			{title: "from byte slice",
				valueFn: func() interface{} { return []byte(nil) }},
			//{title: "from iface array", // Not Possible!  Compiler says: "cannot convert nil to type [0]interface {}"
			//	valueFn: func() interface{} { return [0]interface{}(nil) }},
			{title: "from iface slice",
				valueFn: func() interface{} { return []interface{}(nil) }},
			{title: "from *int array",
				valueFn: func() interface{} { var v *[0]int; return v }},
			{title: "from *int slice",
				valueFn: func() interface{} { var v *[]int; return v }},
			{title: "from *iface array",
				valueFn: func() interface{} { var v *[0]interface{}; return v }},
			{title: "from *iface slice",
				valueFn: func() interface{} { var v *[]interface{}; return v }},
			{title: "from *tObjStr",
				valueFn: func() interface{} { var v *tObjStr; return v }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into string",
				slotFn:    func() interface{} { var str string; return str },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf("")}},
			{title: "into *string",
				slotFn:    func() interface{} { var str string; return &str },
				expectErr: ErrUnmarshalTypeCantFit{Token{Type: TNull}, reflect.ValueOf(""), 0}},
			{title: "into **string",
				slotFn:  func() interface{} { var strp *string; return &strp },
				valueFn: func() interface{} { return (*string)(nil) }},
			{title: "into wildcard",
				slotFn:    func() interface{} { var v interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(interface{}(nil))}},
			{title: "into *wildcard",
				slotFn:  func() interface{} { var v interface{}; return &v },
				valueFn: func() interface{} { return nil }},
			{title: "into map[str]iface",
				slotFn:    func() interface{} { var v map[string]interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}(nil))}},
			{title: "into made map[str]iface",
				slotFn:    func() interface{} { v := make(map[string]interface{}); return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}{})}},
			{title: "into *map[str]iface",
				slotFn:  func() interface{} { var v map[string]interface{}; return &v },
				valueFn: func() interface{} { return map[string]interface{}(nil) }},
			{title: "into *map[str]str",
				slotFn:  func() interface{} { var v map[string]string; return &v },
				valueFn: func() interface{} { return map[string]string(nil) }},
			{title: "into []iface",
				slotFn:    func() interface{} { var v []interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]interface{}{})}},
			{title: "into *[]iface",
				slotFn:  func() interface{} { var v []interface{}; return &v },
				valueFn: func() interface{} { return []interface{}(nil) }},
			{title: "into []str",
				slotFn:    func() interface{} { var v []string; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]string{})}},
			{title: "into *[]str",
				slotFn:  func() interface{} { var v []string; return &v },
				valueFn: func() interface{} { return []string(nil) }},
			{title: "into *[]byte",
				slotFn:  func() interface{} { var v []byte; return &v },
				valueFn: func() interface{} { return []byte(nil) }},
			{title: "into [0]str",
				slotFn:    func() interface{} { var v []string; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]string{})}},
			{title: "into *[0]str",
				slotFn:  func() interface{} { var v [0]string; return &v },
				valueFn: func() interface{} { return [0]string{} }},
			{title: "into [2]str",
				slotFn:    func() interface{} { var v []string; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]string{})}},
			{title: "into *[2]str",
				slotFn:  func() interface{} { var v [2]string; return &v },
				valueFn: func() interface{} { return [2]string{} }},
			{title: "into []int",
				slotFn:    func() interface{} { var v []int; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf([]int{})}},
			{title: "into *[]int",
				slotFn:  func() interface{} { var v []int; return &v },
				valueFn: func() interface{} { return []int(nil) }},
			{title: "into *tObjStr",
				slotFn: func() interface{} { var v tObjStr; return &v },
				// no, the answer is *not* a nil a la `{ var v *tObjStr; return v }` -- because the way unmarshalling uses the first pointer, it can't really set it to nil.
				// stdlib json behavior is the same for the same reasons: https://play.golang.org/p/kd8iqNPdlM
				valueFn: func() interface{} { return tObjStr{""} }},
		},
	},
	{title: "omitEmpty resulting in empty map",
		sequence: fixtures.SequenceMap["empty map"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjStr{}).StructMap().
				AddField("X", atlas.StructMapEntry{SerialName: "key", OmitEmpty: true}).
				Complete(),
		),
		marshalResults: []marshalResults{
			{title: "from tObjStr",
				valueFn: func() interface{} { var v tObjStr; return v }},
		},
	},
	{title: "nulls in map values and struct fields",
		sequence: fixtures.SequenceMap["null in map"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjStrp{}).StructMap().
				AddField("X", atlas.StructMapEntry{SerialName: "k"}).
				Complete(),
			atlas.BuildEntry(tObjMap{}).StructMap().
				AddField("X", atlas.StructMapEntry{SerialName: "k"}).
				Complete(),
		),
		marshalResults: []marshalResults{
			{title: "from tObjStrp",
				valueFn: func() interface{} { return tObjStrp{} }},
			{title: "from *tObjStrp",
				valueFn: func() interface{} { return &tObjStrp{} }},
			{title: "from map[str]iface",
				valueFn: func() interface{} { return map[string]interface{}{"k": nil} }},
			{title: "from map[str]*str",
				valueFn: func() interface{} { return map[string]*string{"k": nil} }},
			{title: "from map[str]map[str]str",
				valueFn: func() interface{} { return map[string]map[string]string{"k": nil} }},
			{title: "from map[str]*map[str]str",
				valueFn: func() interface{} { return map[string]*map[string]string{"k": nil} }},
			{title: "from tObjMap",
				valueFn: func() interface{} { return tObjMap{} }},
		},
	},
	{title: "nulls in deeper structs",
		sequence: fixtures.SequenceMap["null in map"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjPtrObjMap{}).StructMap().
				AddField("P", atlas.StructMapEntry{SerialName: "k"}).
				Complete(),
			atlas.BuildEntry(tObjMap{}).StructMap().
				AddField("X", atlas.StructMapEntry{SerialName: "k"}).
				Complete(),
		),
		marshalResults: []marshalResults{
			{title: "from tObjPtrObjMap",
				valueFn: func() interface{} { return tObjPtrObjMap{} }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "to tObjPtrObjMap",
				slotFn:    func() interface{} { return tObjPtrObjMap{} },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(tObjPtrObjMap{})}},
			{title: "to *tObjPtrObjMap",
				slotFn:  func() interface{} { return &tObjPtrObjMap{} },
				valueFn: func() interface{} { return tObjPtrObjMap{} }},
			{title: "to *tObjPtrObjMap with already intialized field",
				slotFn:  func() interface{} { return &tObjPtrObjMap{&tObjMap{}} },
				valueFn: func() interface{} { return tObjPtrObjMap{} }},
		},
	},
	{title: "nulls in deeper arrays",
		sequence: fixtures.SequenceMap["null in array in array"],
		marshalResults: []marshalResults{
			{title: "from []iface[]iface",
				valueFn: func() interface{} { return []interface{}{[]interface{}{nil}} }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into *iface",
				slotFn:  func() interface{} { var v interface{}; return &v },
				valueFn: func() interface{} { return []interface{}{[]interface{}{nil}} }},
			{title: "into []iface",
				slotFn:  func() interface{} { var v []interface{}; return &v },
				valueFn: func() interface{} { return []interface{}{[]interface{}{nil}} }},
			{title: "into *map[str]iface",
				slotFn:    func() interface{} { var v map[string]interface{}; return &v },
				expectErr: ErrUnmarshalTypeCantFit{Token{Type: TArrOpen, Length: 1}, reflect.ValueOf(map[string]interface{}{}), 0}},
		},
	},
	{title: "nulls in midst of arrays",
		sequence: fixtures.SequenceMap["null in middle of array"],
		marshalResults: []marshalResults{
			{title: "from []iface",
				valueFn: func() interface{} { return []interface{}{"one", nil, "three", nil, "five"} }},
			{title: "from []*str",
				valueFn: func() interface{} {
					one, three, five := "one", "three", "five"
					return []*string{&one, nil, &three, nil, &five}
				}},
		},
		unmarshalResults: []unmarshalResults{
			{title: "into *iface",
				slotFn:  func() interface{} { var v interface{}; return &v },
				valueFn: func() interface{} { return []interface{}{"one", nil, "three", nil, "five"} }},
			{title: "into []iface",
				slotFn:  func() interface{} { var v []interface{}; return &v },
				valueFn: func() interface{} { return []interface{}{"one", nil, "three", nil, "five"} }},
			{title: "into *map[str]iface",
				slotFn:    func() interface{} { var v map[string]interface{}; return &v },
				expectErr: ErrUnmarshalTypeCantFit{Token{Type: TArrOpen, Length: 5}, reflect.ValueOf(map[string]interface{}{}), 0}},
		},
	},
	{title: "tagged object",
		sequence: fixtures.SequenceMap["tagged object"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjStr{}).UseTag(50).StructMap().
				AddField("X", atlas.StructMapEntry{SerialName: "k"}).
				Complete(),
		),
		marshalResults: []marshalResults{
			{title: "from tObjStr",
				valueFn: func() interface{} { return tObjStr{"v"} }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "to tObjStr",
				slotFn:    func() interface{} { return tObjStr{} },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(tObjStr{})}},
			{title: "to *tObjStr",
				slotFn:  func() interface{} { return &tObjStr{} },
				valueFn: func() interface{} { return tObjStr{"v"} }},
			{title: "into wildcard",
				slotFn:    func() interface{} { var v interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(interface{}(nil))}},
			{title: "into *wildcard",
				slotFn:  func() interface{} { var v interface{}; return &v },
				valueFn: func() interface{} { return tObjStr{"v"} }},
			{title: "into map[str]iface",
				slotFn:    func() interface{} { var v map[string]interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}(nil))}},
			{title: "into *map[str]iface", // DUBIOUS: at the moment, we don't *reject* if forced into something other than what the tag would hint.
				slotFn:  func() interface{} { var v map[string]interface{}; return &v },
				valueFn: func() interface{} { return map[string]interface{}{"k": "v"} }},
			{title: "into *map[str]str", // DUBIOUS: at the moment, we don't *reject* if forced into something other than what the tag would hint.
				slotFn:  func() interface{} { var v map[string]string; return &v },
				valueFn: func() interface{} { return map[string]string{"k": "v"} }},
		},
	},
	{title: "unmarshalling tag with no matching configuration",
		sequence: fixtures.SequenceMap["tagged object"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjStr{}).UseTag(59).StructMap().
				AddField("X", atlas.StructMapEntry{SerialName: "k"}).
				Complete(),
		),
		unmarshalResults: []unmarshalResults{
			{title: "to tObjStr",
				slotFn:    func() interface{} { return tObjStr{} },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(tObjStr{})}},
			{title: "to *tObjStr", // DUBIOUS: at the moment, we don't *reject* if forced into something other than what the tag would hint.
				slotFn:  func() interface{} { return &tObjStr{} },
				valueFn: func() interface{} { return tObjStr{"v"} }},
			{title: "into wildcard",
				slotFn:    func() interface{} { var v interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(interface{}(nil))}},
			{title: "into *wildcard",
				slotFn:    func() interface{} { var v interface{}; return &v },
				expectErr: fmt.Errorf("missing an unmarshaller for tag 50")},
			{title: "into map[str]iface",
				slotFn:    func() interface{} { var v map[string]interface{}; return v },
				expectErr: ErrInvalidUnmarshalTarget{reflect.TypeOf(map[string]interface{}(nil))}},
			{title: "into *map[str]iface", // DUBIOUS: at the moment, we don't *reject* if forced into something other than what the tag would hint.
				slotFn:  func() interface{} { var v map[string]interface{}; return &v },
				valueFn: func() interface{} { return map[string]interface{}{"k": "v"} }},
			{title: "into *map[str]str", // DUBIOUS: at the moment, we don't *reject* if forced into something other than what the tag would hint.
				slotFn:  func() interface{} { var v map[string]string; return &v },
				valueFn: func() interface{} { return map[string]string{"k": "v"} }},
		},
	},
	{title: "tagged and transformed",
		sequence: fixtures.SequenceMap["tagged string"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(tObjStr{}).UseTag(50).Transform().
				TransformMarshal(atlas.MakeMarshalTransformFunc(
					func(x tObjStr) (string, error) {
						return x.X, nil
					})).
				TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
					func(x string) (tObjStr, error) {
						return tObjStr{x}, nil
					})).
				Complete(),
		),
		marshalResults: []marshalResults{
			{title: "from tObjStr",
				valueFn: func() interface{} { return tObjStr{"wahoo"} }},
		},
		unmarshalResults: []unmarshalResults{
			{title: "to *tObjStr",
				slotFn:  func() interface{} { return &tObjStr{} },
				valueFn: func() interface{} { return tObjStr{"wahoo"} }},
			{title: "into *wildcard",
				slotFn:  func() interface{} { var v interface{}; return &v },
				valueFn: func() interface{} { return tObjStr{"wahoo"} }},
		},
	},
	{title: "tagged complex objects without tagged atlas",
		sequence: fixtures.SequenceMap["object with deeper tagged values"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(t5{}).StructMap().Autogenerate().Complete(),
		),
		// No marshal entries: Nothing marshals to tags without custom atlasing.
		unmarshalResults: []unmarshalResults{
			{title: "to *t5",
				slotFn:  func() interface{} { return &t5{} },
				valueFn: func() interface{} { return t5{"500", "untagged", "600", []string{"asdf", "qwer"}, "505"} }},
		},
	},
	{title: "tagged complex objects",
		sequence: fixtures.SequenceMap["object with deeper tagged values"],
		atlas: atlas.MustBuild(
			atlas.BuildEntry(t5{}).StructMap().Autogenerate().Complete(),
			atlas.BuildEntry(t5b{}).StructMap().Autogenerate().Complete(),
			atlas.BuildEntry(tObjStr{}).UseTag(50).Transform().
				TransformMarshal(atlas.MakeMarshalTransformFunc(
					func(x tObjStr) (string, error) {
						return x.X, nil
					})).
				TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
					func(x string) (tObjStr, error) {
						return tObjStr{x}, nil
					})).
				Complete(),
			atlas.BuildEntry(tObjStr2{}).UseTag(60).Transform().
				TransformMarshal(atlas.MakeMarshalTransformFunc(
					func(x tObjStr2) (string, error) {
						return x.X, nil
					})).
				TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
					func(x string) (tObjStr2, error) {
						return tObjStr2{x, ""}, nil
					})).
				Complete(),
		),
		marshalResults: []marshalResults{
			{title: "from t5b",
				valueFn: func() interface{} {
					return t5b{
						tObjStr{"500"},
						"untagged",
						tObjStr2{"600", ""},
						[]tObjStr{{"asdf"}, {"qwer"}},
						tObjStr{"505"},
					}
				}},
		},
		unmarshalResults: []unmarshalResults{
			{title: "to *t5",
				slotFn:  func() interface{} { return &t5{} },
				valueFn: func() interface{} { return t5{"500", "untagged", "600", []string{"asdf", "qwer"}, "505"} }},
			{title: "to *iface{}",
				slotFn: func() interface{} { var v interface{}; return &v },
				valueFn: func() interface{} {
					return map[string]interface{}{
						"k1": tObjStr{"500"},
						"k2": "untagged",
						"k3": tObjStr2{"600", ""},
						"k4": []interface{}{tObjStr{"asdf"}, tObjStr{"qwer"}},
						"k5": tObjStr{"505"},
					}
				}},
		},
	},
}

func TestMarshaller(t *testing.T) {
	// Package all the values from one step into a struct, just so that
	// we can assert on them all at once and make one green checkmark render per step.
	// Stringify the token first so extraneous fields in the union are hidden.
	type step struct {
		tok string
		err error
	}

	Convey("Marshaller suite:", t, func() {
		for _, tr := range objFixtures {
			Convey(fmt.Sprintf("%q fixture sequence:", tr.title), func() {
				for _, trr := range tr.marshalResults {
					maybe := Convey
					if trr.expectErr == skipMe {
						maybe = SkipConvey
					}
					// Conjure value.  Also format title for test, using its type info.
					value := trr.valueFn()
					valueKind := reflect.ValueOf(value).Kind()
					maybe(fmt.Sprintf("working %s (%s|%T):", trr.title, valueKind, value), func() {
						// Set up marshaller.
						marshaller := NewMarshaller(tr.atlas)
						marshaller.Bind(value)

						Convey("Steps...", func() {
							// Run steps until the marshaller says done or error.
							// For each step, assert the token matches fixtures;
							// when error and expected one, skip token check on that step
							// and finalize with the assertion.
							// If marshaller doesn't stop when we expected it to based
							// on fixture length, let it keep running three more steps
							// so we get that much more debug info.
							var done bool
							var err error
							var tok Token
							expectSteps := len(tr.sequence.Tokens) - 1
							for nStep := 0; nStep < expectSteps+3; nStep++ {
								done, err = marshaller.Step(&tok)
								if err != nil && trr.expectErr != nil {
									Convey("Result (error expected)", func() {
										So(err.Error(), ShouldResemble, trr.expectErr.Error())
									})
									return
								}
								if nStep <= expectSteps {
									So(
										step{tok.String(), err},
										ShouldResemble,
										step{tr.sequence.Tokens[nStep].String(), nil},
									)
								} else {
									So(
										step{tok.String(), err},
										ShouldResemble,
										step{Token{}.String(), fmt.Errorf("overshoot")},
									)
								}
								if done {
									Convey("Result (halted correctly)", func() {
										So(nStep, ShouldEqual, expectSteps)
									})
									return
								}
							}
						})
					})
				}
			})
		}
	})
}

func TestUnmarshaller(t *testing.T) {
	// Package all the values from one step into a struct, just so that
	// we can assert on them all at once and make one green checkmark render per step.
	// Stringify the token first so extraneous fields in the union are hidden.
	type step struct {
		tok  string
		err  error
		done bool
	}

	Convey("Unmarshaller suite:", t, func() {
		for _, tr := range objFixtures {
			Convey(fmt.Sprintf("%q fixture sequence:", tr.title), func() {
				for _, trr := range tr.unmarshalResults {
					maybe := Convey
					if trr.expectErr == skipMe {
						maybe = SkipConvey
					}
					// Conjure slot.  Also format title for test, using its type info.
					slot := trr.slotFn()
					slotKind := reflect.ValueOf(slot).Kind()
					maybe(fmt.Sprintf("targetting %s (%s|%T):", trr.title, slotKind, slot), func() {

						// Set up unmarshaller.
						unmarshaller := NewUnmarshaller(tr.atlas)
						err := unmarshaller.Bind(slot)
						if err != nil && trr.expectErr != nil {
							Convey("Result (error expected)", func() {
								So(err.Error(), ShouldResemble, trr.expectErr.Error())
							})
							return
						}

						Convey("Steps...", func() {
							// Run steps.
							// This is less complicated than the marshaller test
							// because we know exactly when we'll run out of them.
							var done bool
							var err error
							expectSteps := len(tr.sequence.Tokens) - 1
							for nStep, tok := range tr.sequence.Tokens {
								done, err = unmarshaller.Step(&tok)
								if err != nil && trr.expectErr != nil {
									Convey("Result (error expected)", func() {
										So(err.Error(), ShouldResemble, trr.expectErr.Error())
									})
									return
								}
								if nStep == expectSteps {
									So(
										step{tok.String(), err, done},
										ShouldResemble,
										step{tr.sequence.Tokens[nStep].String(), nil, true},
									)
								} else {
									So(
										step{tok.String(), err, done},
										ShouldResemble,
										step{tr.sequence.Tokens[nStep].String(), nil, false},
									)
								}
							}

							Convey("Result", func() {
								// Get value back out.  Some reflection required to get around pointers.
								rv := reflect.ValueOf(slot)
								if rv.Kind() == reflect.Ptr {
									rv = rv.Elem()
								}
								So(rv.Interface(), ShouldResemble, trr.valueFn())
							})
						})
					})
				}
			})
		}
	})
}
