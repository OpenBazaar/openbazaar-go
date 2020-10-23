package log

import (
	"fmt"
	"testing"
)

func TestFieldString(t *testing.T) {
	testCases := []struct {
		field    Field
		expected string
	}{
		{
			field:    String("key", "value"),
			expected: "key:value",
		},
		{
			field:    Bool("key", true),
			expected: "key:true",
		},
		{
			field:    Int("key", 5),
			expected: "key:5",
		},
		{
			field:    Error(fmt.Errorf("err msg")),
			expected: "error.object:err msg",
		},
		{
			field:    Error(nil),
			expected: "error.object:<nil>",
		},
		{
			field:    Noop(),
			expected: ":<nil>",
		},
		{
			field:    Event("test"),
			expected: "event:test",
		},
		{
			field:    Message("test2"),
			expected: "message:test2",
		},
	}
	for i, tc := range testCases {
		if str := tc.field.String(); str != tc.expected {
			t.Errorf("%d: expected '%s', got '%s'", i, tc.expected, str)
		}
	}
}

func TestNoopDoesNotMarshal(t *testing.T) {
	mockEncoder := struct {
		Encoder
	}{}
	f := Noop()
	f.Marshal(mockEncoder) // panics if any Encoder method is invoked
}
