package cmds

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

type TestOutput struct {
	Foo, Bar string
	Baz      int
}

func eqStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func TestMarshalling(t *testing.T) {
	cmd := &Command{}

	req, err := NewRequest(context.Background(), nil, map[string]interface{}{
		EncLong: JSON,
	}, nil, nil, cmd)
	if err != nil {
		t.Error(err, "Should have passed")
	}

	buf := bytes.NewBuffer(nil)
	wc := writecloser{Writer: buf, Closer: nopCloser{}}
	re, err := NewWriterResponseEmitter(wc, req)
	if err != nil {
		t.Fatal(err)
	}

	err = re.Emit(TestOutput{"beep", "boop", 1337})
	if err != nil {
		t.Error(err, "Should have passed")
	}

	output := buf.String()
	if removeWhitespace(output) != "{\"Foo\":\"beep\",\"Bar\":\"boop\",\"Baz\":1337}" {
		t.Log("expected: {\"Foo\":\"beep\",\"Bar\":\"boop\",\"Baz\":1337}")
		t.Log("got:", removeWhitespace(buf.String()))
		t.Error("Incorrect JSON output")
	}

	buf.Reset()

	re.Close()
}

func removeWhitespace(input string) string {
	input = strings.Replace(input, " ", "", -1)
	input = strings.Replace(input, "\t", "", -1)
	input = strings.Replace(input, "\n", "", -1)
	return strings.Replace(input, "\r", "", -1)
}
