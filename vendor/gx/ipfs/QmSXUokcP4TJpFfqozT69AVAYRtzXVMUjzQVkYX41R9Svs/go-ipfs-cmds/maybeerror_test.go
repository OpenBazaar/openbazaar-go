package cmds

import (
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"

	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

func errcmp(t *testing.T, exp, got error, msg string) {
	if exp == nil && got == nil {
		return
	}

	if exp != nil && got != nil {
		if exp.Error() == got.Error() {
			return
		}

		t.Errorf("expected %s to be %q but got %q", msg, exp, got)
		return
	}

	if exp == nil {
		t.Errorf("expected %s to be nil but got %q", msg, got)
	}

	if got == nil {
		t.Errorf("expected %s to be %q but got nil", msg, exp)
	}
}

type Foo struct {
	Bar int
}

type Bar struct {
	Foo string
}

type ValueError struct {
	DecodeError error
	Error       *cmdkit.Error
	Value       interface{}
}

type anyTestCase struct {
	Name    string
	Value   interface{}
	JSON    string
	Decoded []ValueError
}

func TestMaybeError(t *testing.T) {
	testcases := []anyTestCase{
		{
			Name:  "typed-pointer",
			Value: &Foo{},
			JSON:  `{"Bar":23}{"Bar":42}{"Message":"some error", "Type": "error"}`,
			Decoded: []ValueError{
				ValueError{Value: &Foo{23}},
				ValueError{Value: &Foo{42}},
				ValueError{Error: &cmdkit.Error{Message: "some error", Code: 0}},
			},
		},
		{
			Name:  "typed-value",
			Value: Foo{},
			JSON:  `{"Bar":23}{"Bar":42}{"Message":"some error", "Type": "error"}`,
			Decoded: []ValueError{
				ValueError{Value: &Foo{23}},
				ValueError{Value: &Foo{42}},
				ValueError{Error: &cmdkit.Error{Message: "some error", Code: 0}},
			},
		},
		{
			Name:  "typed2-pointer",
			Value: &Bar{},
			JSON:  `{"Foo":""}{"Foo":"Qmabc"}{"Message":"some error", "Type": "error"}`,
			Decoded: []ValueError{
				ValueError{Value: &Bar{""}},
				ValueError{Value: &Bar{"Qmabc"}},
				ValueError{Error: &cmdkit.Error{Message: "some error", Code: 0}},
			},
		},
		{
			Name:  "typed2-value",
			Value: Bar{},
			JSON:  `{"Foo":""}{"Foo":"Qmabc"}{"Message":"some error", "Type": "error"}`,
			Decoded: []ValueError{
				ValueError{Value: &Bar{""}},
				ValueError{Value: &Bar{"Qmabc"}},
				ValueError{Error: &cmdkit.Error{Message: "some error", Code: 0}},
			},
		},
		{
			Name: "untyped",
			JSON: `{"Foo":"bar", "i": 4}"some string"5{"Message":"some error", "Type": "error"}`,
			Decoded: []ValueError{
				ValueError{Value: map[string]interface{}{"Foo": "bar", "i": 4.0}},
				ValueError{Value: "some string"},
				ValueError{Value: 5.0},
				ValueError{Error: &cmdkit.Error{Message: "some error", Code: 0}},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			r := strings.NewReader(tc.JSON)
			d := json.NewDecoder(r)

			var err error

			for _, dec := range tc.Decoded {
				m := &MaybeError{Value: tc.Value}

				err = d.Decode(m)
				errcmp(t, dec.DecodeError, err, "decode error")
				val, err := m.Get()

				if dec.Value != nil {
					ex := dec.Value
					if !reflect.DeepEqual(ex, val) {
						t.Errorf("value is %#v(%T), expected %#v(%T)", val, val, ex, ex)
					}
				} else {
					errcmp(t, dec.Error, err, "response error")
				}
			}

			m := &MaybeError{Value: tc.Value}
			err = d.Decode(m)
			val, e := m.Get()
			if err != io.EOF {
				t.Log("superflouus data:", val, e)
				errcmp(t, io.EOF, err, "final decode error")
			}
		})
	}
}
