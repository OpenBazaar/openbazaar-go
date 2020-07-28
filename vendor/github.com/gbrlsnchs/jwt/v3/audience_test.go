package jwt_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/gbrlsnchs/jwt/v3"
)

func TestAudienceMarshal(t *testing.T) {
	t.Run("omitempty", func(t *testing.T) {
		var (
			b   []byte
			err error
			v   = struct {
				Audience jwt.Audience `json:"aud,omitempty"`
			}{}
		)
		if b, err = json.Marshal(v); err != nil {
			t.Fatal(err)
		}
		checkAudMarshal(t, "{}", b)

	})

	testCases := []struct {
		aud      jwt.Audience
		expected string
	}{
		{jwt.Audience{"foo"}, `"foo"`},
		{jwt.Audience{"foo", "bar"}, `["foo","bar"]`},
		{nil, `""`},
		{jwt.Audience{}, `""`},
		{jwt.Audience{""}, `""`},
	}
	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			var (
				b   []byte
				err error
			)
			if tc.aud != nil {
				if b, err = tc.aud.MarshalJSON(); err != nil {
					t.Fatal(err)
				}
				checkAudMarshal(t, tc.expected, b)
			}
			if b, err = json.Marshal(tc.aud); err != nil {
				t.Fatal(err)
			}
			checkAudMarshal(t, tc.expected, b)
		})
	}
}

func TestAudienceUnmarshal(t *testing.T) {
	testCases := []struct {
		jstr     []byte
		expected jwt.Audience
	}{
		{[]byte(`"foo"`), jwt.Audience{"foo"}},
		{[]byte(`["foo","bar"]`), jwt.Audience{"foo", "bar"}},
		{[]byte("[]"), jwt.Audience{}},
	}
	for _, tc := range testCases {
		t.Run(string(tc.jstr), func(t *testing.T) {
			var aud jwt.Audience
			if err := aud.UnmarshalJSON(tc.jstr); err != nil {
				t.Fatal(err)
			}
			checkAudUnmarshal(t, tc.expected, aud)
			if err := json.Unmarshal(tc.jstr, &aud); err != nil {
				t.Fatal(err)
			}
			checkAudUnmarshal(t, tc.expected, aud)
		})
	}
}

func checkAudMarshal(t *testing.T, want string, got []byte) {
	if want != string(got) {
		t.Errorf("want %q, got %q", want, got)
	}
}

func checkAudUnmarshal(t *testing.T, want, got jwt.Audience) {
	if !reflect.DeepEqual(want, got) {
		t.Errorf("want %v, got %v", want, got)
	}
}
