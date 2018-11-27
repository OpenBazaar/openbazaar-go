package cmdkit

import (
	"encoding/json"
	"testing"
)

func TestMarshal(t *testing.T) {
	type testcase struct {
		msg  string
		code ErrorType
	}

	tcs := []testcase{
		{msg: "error msg", code: 0},
		{msg: "error msg", code: 1},
		{msg: "some other error msg", code: 1},
	}

	for _, tc := range tcs {
		e := Error{
			Message: tc.msg,
			Code:    tc.code,
		}

		buf, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}

		m := make(map[string]interface{})

		err = json.Unmarshal(buf, &m)
		if err != nil {
			t.Fatal(err)
		}

		if len(m) != 3 {
			t.Errorf("expected three map elements, got %d", len(m))
		}

		if m["Message"].(string) != tc.msg {
			t.Errorf(`expected m["Message"] to be %q, got %q`, tc.msg, m["Message"])
		}

		icode := ErrorType(m["Code"].(float64))
		if icode != tc.code {
			t.Errorf(`expected m["Code"] to be %v, got %v`, tc.code, icode)
		}

		if m["Type"].(string) != "error" {
			t.Errorf(`expected m["Type"] to be %q, got %q`, "error", m["Type"])
		}
	}
}

func TestUnmarshal(t *testing.T) {
	type testcase struct {
		json string
		msg  string
		code ErrorType

		err string
	}

	tcs := []testcase{
		{json: `{"Message":"error msg","Code":0}`, msg: "error msg", err: "not of type error"},
		{json: `{"Message":"error msg","Code":0,"Type":"error"}`, msg: "error msg"},
		{json: `{"Message":"error msg","Code":1,"Type":"error"}`, msg: "error msg", code: 1},
		{json: `{"Message":"some other error msg","Code":1,"Type":"error"}`, msg: "some other error msg", code: 1},
	}

	for i, tc := range tcs {
		t.Log("at test case", i)
		var e Error
		err := json.Unmarshal([]byte(tc.json), &e)
		if err != nil && err.Error() != tc.err {
			t.Errorf("expected parse error %q but got %q", tc.err, err)
		} else if err == nil && tc.err != "" {
			t.Errorf("expected parse error %q but got %q", tc.err, err)
		}

		if err != nil {
			continue
		}

		if e.Message != tc.msg {
			t.Errorf("expected e.Message to be %q, got %q", tc.msg, e.Message)
		}

		if e.Code != tc.code {
			t.Errorf("expected e.Code to be %q, got %q", tc.code, e.Code)
		}
	}
}
