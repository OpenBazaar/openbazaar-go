package jwt_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/gbrlsnchs/jwt/v3"
	"github.com/gbrlsnchs/jwt/v3/internal"
	"github.com/google/go-cmp/cmp"
)

type testPayload struct {
	jwt.Payload
	String string `json:"string,omitempty"`
	Int    int    `json:"int,omitempty"`
}

type testCase struct {
	alg     jwt.Algorithm
	payload interface{}

	verifyAlg   jwt.Algorithm
	opts        []func(*jwt.RawToken)
	wantHeader  jwt.Header
	wantPayload testPayload

	signErr   error
	verifyErr error
}

var (
	now = time.Now()
	tp  = testPayload{
		Payload: jwt.Payload{
			Issuer:         "gbrlsnchs",
			Subject:        "someone",
			Audience:       jwt.Audience{"https://golang.org", "https://jwt.io"},
			ExpirationTime: jwt.NumericDate(now.Add(24 * 30 * 12 * time.Hour)),
			NotBefore:      jwt.NumericDate(now.Add(30 * time.Minute)),
			IssuedAt:       jwt.NumericDate(now),
			JWTID:          "foobar",
		},
		String: "foobar",
		Int:    1337,
	}
)

func TestVerify(t *testing.T) {
	testCases := map[string][]testCase{
		"HMAC":    hmacTestCases,
		"RSA":     rsaTestCases,
		"RSA-PSS": rsaPSSTestCases,
		"ECDSA":   ecdsaTestCases,
		"Ed25519": ed25519TestCases,
	}
	for k, v := range testCases {
		t.Run(k, func(t *testing.T) {
			for _, tc := range v {
				t.Run(tc.verifyAlg.Name(), func(t *testing.T) {
					token, err := jwt.Sign(tc.payload, tc.alg)
					if err != nil {
						t.Fatal(err)
					}
					var pl testPayload
					hd, err := jwt.Verify(token, tc.verifyAlg, &pl)
					if want, got := tc.verifyErr, err; got != want {
						t.Errorf("jwt.Verify err mismatch (-want +got):\n%s", cmp.Diff(want, got))
					}
					if want, got := tc.wantHeader, hd; !cmp.Equal(got, want) {
						t.Errorf("jwt.Verify header mismatch (-want +got):\n%s", cmp.Diff(want, got))
					}
					if want, got := tc.wantPayload, pl; !cmp.Equal(got, want) {
						t.Errorf("jwt.Verify payload mismatch (-want +got):\n%s", cmp.Diff(want, got))
					}
				})
			}
		})
	}

	t.Run("non-JSON payload", func(t *testing.T) {
		var (
			header  = "eyJ0eXAiOiJKV1QiLCJhbGciOiJub25lIn0" // {"typ":"JWT","alg":"none"}
			payload = "NTcwMDU"                             // 57005
			token   = fmt.Sprintf("%s.%s.", header, payload)
			v       interface{}
		)
		_, err := jwt.Verify([]byte(token), jwt.None(), &v)
		if want, got := jwt.ErrNotJSONObject, err; !internal.ErrorIs(got, want) {
			t.Errorf("jwt.Verify JSON payload mismatch (-want +got):\n%s", cmp.Diff(want, got))
		}
	})
}

func TestValidatePayload(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		pl  *jwt.Payload
		vds []jwt.Validator
		err error
	}{
		{
			pl: &jwt.Payload{
				ExpirationTime: jwt.NumericDate(now.Add(1 * time.Second)),
			},
			vds: []jwt.Validator{jwt.ExpirationTimeValidator(now)},
			err: nil,
		},
		{
			pl: &jwt.Payload{
				ExpirationTime: jwt.NumericDate(now.Add(1 * time.Second)),
			},
			vds: []jwt.Validator{jwt.ExpirationTimeValidator(now.Add(15 * time.Second))},
			err: jwt.ErrExpValidation,
		},
		{
			pl: &jwt.Payload{
				Subject:        "test",
				ExpirationTime: jwt.NumericDate(now.Add(1 * time.Second)),
			},
			vds: []jwt.Validator{
				jwt.SubjectValidator("test"),
				jwt.ExpirationTimeValidator(now),
			},
			err: nil,
		},
		{
			pl: &jwt.Payload{
				Subject:        "foo",
				ExpirationTime: jwt.NumericDate(now.Add(1 * time.Second)),
			},
			vds: []jwt.Validator{
				jwt.SubjectValidator("bar"),
				jwt.ExpirationTimeValidator(now),
			},
			err: jwt.ErrSubValidation,
		},
		{
			pl: &jwt.Payload{
				Subject:        "test",
				ExpirationTime: jwt.NumericDate(now.Add(1 * time.Second)),
			},
			vds: []jwt.Validator{
				jwt.SubjectValidator("test"),
				jwt.ExpirationTimeValidator(now.Add(15 * time.Second)),
			},
			err: jwt.ErrExpValidation,
		},
	}
	hs256 := jwt.NewHS256([]byte("secret"))
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			token, err := jwt.Sign(tc.pl, hs256)
			if err != nil {
				t.Fatal(err)
			}
			_, err = jwt.Verify(token, hs256, tc.pl, jwt.ValidatePayload(tc.pl, tc.vds...))
			if want, got := tc.err, err; !internal.ErrorIs(got, want) {
				t.Errorf("jwt.Verify with validators mismatch (-want +got):\n%s", cmp.Diff(want, got))
			}
		})
	}
}
