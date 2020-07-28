package jwt_test

import (
	"encoding/json"
	"testing"

	"github.com/gbrlsnchs/jwt/v3"
	"github.com/google/go-cmp/cmp"
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
		checkAudMarshal(t, b, "{}")

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
				checkAudMarshal(t, b, tc.expected)
			}
			if b, err = json.Marshal(tc.aud); err != nil {
				t.Fatal(err)
			}
			checkAudMarshal(t, b, tc.expected)
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
			checkAudUnmarshal(t, aud, tc.expected)
			if err := json.Unmarshal(tc.jstr, &aud); err != nil {
				t.Fatal(err)
			}
			checkAudUnmarshal(t, aud, tc.expected)
		})
	}
}

func checkAudMarshal(t *testing.T, got []byte, want string) {
	if string(got) != want {
		t.Errorf("jwt.Audience.Marshal mismatch (-want +got):\n%s", cmp.Diff(want, got))
	}
}

func checkAudUnmarshal(t *testing.T, got, want jwt.Audience) {
	if !cmp.Equal(got, want) {
		t.Errorf("jwt.Audience.Unmarshal mismatch (-want +got):\n%s", cmp.Diff(want, got))
	}
}
