package cmds

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestArguments(t *testing.T) {
	var testCases = []struct {
		input     string
		arguments []string
	}{
		{
			input:     "",
			arguments: []string{},
		},
		{
			input:     "\n",
			arguments: []string{""},
		},
		{
			input:     "\r\n",
			arguments: []string{""},
		},
		{
			input:     "\r",
			arguments: []string{"\r"},
		},
		{
			input:     "one",
			arguments: []string{"one"},
		},
		{
			input:     "one\n",
			arguments: []string{"one"},
		},
		{
			input:     "one\r\n",
			arguments: []string{"one"},
		},
		{
			input:     "one\r",
			arguments: []string{"one\r"},
		},
		{
			input:     "one\n\ntwo",
			arguments: []string{"one", "", "two"},
		},
		{
			input:     "first\nsecond\nthird",
			arguments: []string{"first", "second", "third"},
		},
		{
			input:     "first\r\nsecond\nthird",
			arguments: []string{"first", "second", "third"},
		},
		{
			input:     "first\nsecond\nthird\n",
			arguments: []string{"first", "second", "third"},
		},
		{
			input:     "first\r\nsecond\r\nthird\r\n",
			arguments: []string{"first", "second", "third"},
		},
		{
			input:     "first\nsecond\nthird\n\n",
			arguments: []string{"first", "second", "third", ""},
		},
		{
			input:     "\nfirst\nsecond\nthird\n",
			arguments: []string{"", "first", "second", "third"},
		},
	}

	for i, tc := range testCases {
		for cut := 0; cut <= len(tc.arguments); cut++ {
			args := newArguments(ioutil.NopCloser(bytes.NewBufferString(tc.input)))
			for j, arg := range tc.arguments[:cut] {
				if !args.Scan() {
					t.Errorf("in test case %d, missing argument %d", i, j)
					continue
				}
				got := args.Argument()
				if got != arg {
					t.Errorf("in test case %d, expected argument %d to be %s, got %s", i, j, arg, got)
				}
				if args.Err() != nil {
					t.Error(args.Err())
				}
			}
			args = newArguments(args)
			// Tests stopping in the middle.
			for j, arg := range tc.arguments[cut:] {
				if !args.Scan() {
					t.Errorf("in test case %d, missing argument %d", i, j+cut)
					continue
				}
				got := args.Argument()
				if got != arg {
					t.Errorf("in test case %d, expected argument %d to be %s, got %s", i, j+cut, arg, got)
				}
				if args.Err() != nil {
					t.Error(args.Err())
				}
			}
			if args.Scan() {
				t.Errorf("in test case %d, got too many arguments", i)
			}
		}
	}
}
