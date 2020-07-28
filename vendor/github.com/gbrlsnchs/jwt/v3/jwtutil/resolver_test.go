package jwtutil_test

import (
	"errors"
	"testing"

	"github.com/gbrlsnchs/jwt/v3"
	"github.com/gbrlsnchs/jwt/v3/jwtutil"
)

var hs256 = jwt.NewHS256([]byte("resolver"))

func TestResolver(t *testing.T) {
	testCases := []struct {
		signer   jwt.Algorithm
		signOpts []jwt.SignOption
		verifier jwt.Algorithm
	}{
		{
			signer: hs256,
			verifier: &jwtutil.Resolver{
				New: func(hd jwt.Header) (jwt.Algorithm, error) {
					return hs256, nil
				},
			},
		},
		{
			signer:   hs256,
			signOpts: []jwt.SignOption{jwt.KeyID("test")},
			verifier: &jwtutil.Resolver{
				New: func(hd jwt.Header) (jwt.Algorithm, error) {
					if hd.KeyID != "test" {
						return nil, errors.New(`wrong "kid"`)
					}
					return hs256, nil
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			token, err := jwt.Sign(jwt.Payload{}, tc.signer, tc.signOpts...)
			if err != nil {
				t.Fatal(err)
			}
			var pl jwt.Payload
			if _, err = jwt.Verify(token, tc.verifier, &pl); err != nil {
				t.Fatal(err)
			}
		})
	}
}
