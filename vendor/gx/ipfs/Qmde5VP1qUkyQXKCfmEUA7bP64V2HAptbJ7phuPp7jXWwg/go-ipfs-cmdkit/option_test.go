package cmdkit

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestOptionValueExtractBoolNotFound(t *testing.T) {
	t.Log("ensure that no error is returned when value is not found")
	optval := &OptionValue{ValueFound: false}
	_, _, err := optval.Bool()
	if err != nil {
		t.Fatal("Found was false. Err should have been nil")
	}
}

func TestOptionValueExtractWrongType(t *testing.T) {

	t.Log("ensure that error is returned when value if of wrong type")

	optval := &OptionValue{Value: "wrong type: a string", ValueFound: true}
	_, _, err := optval.Bool()
	if err == nil {
		t.Fatal("No error returned. Failure.")
	}

	optval = &OptionValue{Value: "wrong type: a string", ValueFound: true}
	_, _, err = optval.Int()
	if err == nil {
		t.Fatal("No error returned. Failure.")
	}
}

func TestLackOfDescriptionOfOptionDoesNotPanic(t *testing.T) {
	opt := BoolOption("a", "")
	opt.Description()
}

func TestDotIsAddedInDescripton(t *testing.T) {
	opt := BoolOption("a", "desc without dot")
	dest := opt.Description()
	if !strings.HasSuffix(dest, ".") {
		t.Fatal("dot should have been added at the end of description")
	}
}

func TestOptionName(t *testing.T) {
	exp := map[string]interface{}{
		"Name()":  "main",
		"Names()": []string{"main", "m", "alias"},
	}

	assert := func(name string, value interface{}) {
		if !reflect.DeepEqual(value, exp[name]) {
			t.Errorf(`expected %s to return %q, got %q`, name, exp[name], value)
		}
	}

	opt := StringOption("main", "m", "alias", `an option with main name "main" and "m" and "alias" as aliases`)
	assert("Name()", opt.Name())
	assert("Names()", opt.Names())
}

func TestParse(t *testing.T) {
	type testcase struct {
		opt Option
		str string
		v   interface{}
		err string
	}

	tcs := []testcase{
		{opt: StringOption("str"), str: "i'm a string!", v: "i'm a string!"},
		{opt: IntOption("int1"), str: "42", v: 42},
		{opt: IntOption("int1"), str: "fourtytwo", err: `strconv.ParseInt: parsing "fourtytwo": invalid syntax`},
		{opt: IntOption("int2"), str: "-42", v: -42},
		{opt: UintOption("uint1"), str: "23", v: uint(23)},
		{opt: UintOption("uint2"), str: "-23", err: `strconv.ParseUint: parsing "-23": invalid syntax`},
		{opt: Int64Option("int3"), str: "100001", v: int64(100001)},
		{opt: Int64Option("int3"), str: "2147483648", v: int64(math.MaxInt32 + 1)},
		{opt: Int64Option("int3"), str: "fly", err: `strconv.ParseInt: parsing "fly": invalid syntax`},
		{opt: Uint64Option("uint3"), str: "23", v: uint64(23)},
		{opt: Uint64Option("uint3"), str: "-23", err: `strconv.ParseUint: parsing "-23": invalid syntax`},
		{opt: BoolOption("true"), str: "true", v: true},
		{opt: BoolOption("true"), str: "", v: true},
		{opt: BoolOption("false"), str: "false", v: false},
		{opt: FloatOption("float"), str: "2.718281828459045", v: 2.718281828459045},
	}

	for _, tc := range tcs {
		v, err := tc.opt.Parse(tc.str)
		if err != nil && err.Error() != tc.err {
			t.Errorf("unexpected error: %s", err)
		} else if err == nil && tc.err != "" {
			t.Errorf("expected error %q but got nil", tc.err)
		}

		if v != tc.v {
			t.Errorf("expected %v but got %v", tc.v, v)
		}
	}
}

func TestDescription(t *testing.T) {
	type testcase struct {
		opt  Option
		desc string
	}

	tcs := []testcase{
		{opt: StringOption("str", "some random option"), desc: "some random option."},
		{opt: StringOption("str", "some random option (<<default>>)"), desc: "some random option (<<default>>)."},
		{opt: StringOption("str", "some random option (<<default>>)").WithDefault("random=4"), desc: "some random option (Default: random=4.)."},
		{opt: StringOption("str", "some random option").WithDefault("random=4"), desc: "some random option. Default: random=4."},
	}

	for _, tc := range tcs {
		if desc := tc.opt.Description(); desc != tc.desc {
			t.Errorf("expected\n%q\nbut got\n%q", tc.desc, desc)
		}
	}
}
