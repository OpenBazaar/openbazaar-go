package internal_test

import (
	"encoding/base64"
	"testing"

	"github.com/gbrlsnchs/jwt/v3/internal"
)

var (
	stdEnc    = base64.StdEncoding
	rawURLEnc = base64.RawURLEncoding
)

type decodeTest struct {
	X string `json:"x,omitempty"`
}

func TestDecode(t *testing.T) {
	testCases := []struct {
		encoding *base64.Encoding
		json     string
		expected string
		errors   bool
	}{
		{rawURLEnc, "{}", "", false},
		{rawURLEnc, `{"x":"test"}`, "test", false},
		{stdEnc, "{}", "", true},
		{stdEnc, `{"x":"test"}`, "test", false}, // the output is the same as with RawURLEncoding
		{nil, "{}", "", true},
		{nil, `{"x":"test"}`, "", true},
	}
	for _, tc := range testCases {
		t.Run(tc.json, func(t *testing.T) {
			b64 := tc.json
			if tc.encoding != nil {
				b64 = tc.encoding.EncodeToString([]byte(tc.json))
			}
			t.Logf("b64: %s", b64)
			var (
				dt  decodeTest
				err = internal.Decode([]byte(b64), &dt)
			)
			if want, got := tc.errors, err != nil; want != got {
				t.Fatalf("want %t, got %t: %v", want, got, err)
			}
			if want, got := tc.expected, dt.X; want != got {
				t.Errorf("want %q, got %q", want, got)
			}
		})
	}
}
