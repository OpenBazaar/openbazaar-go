package jwt_test

import (
	"errors"
	"testing"

	"github.com/gbrlsnchs/jwt/v3"
	"github.com/gbrlsnchs/jwt/v3/internal"
	"github.com/gbrlsnchs/jwt/v3/jwtutil"
	"github.com/google/go-cmp/cmp"
)

var testErr = errors.New("test")

func TestSign(t *testing.T) {
	testCases := []struct {
		payload interface{}
		alg     jwt.Algorithm
		opts    []jwt.SignOption
		err     error
	}{
		{
			payload: jwt.Payload{},
			alg:     jwt.None(),
			opts:    nil,
			err:     nil,
		},
		{
			payload: nil,
			alg:     &jwt.HMACSHA{},
			opts:    nil,
			err:     jwt.ErrHMACMissingKey,
		},
		{
			payload: nil,
			alg:     jwt.NewHS256([]byte("secret")),
			opts:    nil,
			err:     nil,
		},
		{
			payload: nil,
			alg:     jwt.NewHS384([]byte("secret")),
			opts:    nil,
			err:     nil,
		},
		{
			payload: nil,
			alg:     jwt.NewHS512([]byte("secret")),
			opts:    nil,
			err:     nil,
		},
		{
			payload: nil,
			alg:     &jwt.RSASHA{},
			opts:    nil,
			err:     jwt.ErrRSANilPrivKey,
		},
		{
			payload: nil,
			alg:     jwt.NewRS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			opts:    nil,
			err:     nil,
		},
		{
			payload: nil,
			alg:     jwt.NewRS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			opts:    nil,
			err:     nil,
		},
		{
			payload: nil,
			alg:     jwt.NewRS512(jwt.RSAPrivateKey(rsaPrivateKey1)),
			opts:    nil,
			err:     nil,
		},
		{
			payload: nil,
			alg:     &jwt.ECDSASHA{},
			opts:    nil,
			err:     jwt.ErrECDSANilPrivKey,
		},
		{
			payload: nil,
			alg:     jwt.NewES256(jwt.ECDSAPrivateKey(es256PrivateKey1)),
			opts:    nil,
			err:     nil,
		},
		{
			payload: nil,
			alg:     jwt.NewES384(jwt.ECDSAPrivateKey(es256PrivateKey1)),
			opts:    nil,
			err:     nil,
		},
		{
			payload: nil,
			alg:     jwt.NewES512(jwt.ECDSAPrivateKey(es256PrivateKey1)),
			opts:    nil,
			err:     nil,
		},
		{
			payload: nil,
			alg:     &jwt.Ed25519{},
			opts:    nil,
			err:     jwt.ErrEd25519NilPrivKey,
		},
		{
			payload: nil,
			alg:     jwt.NewEd25519(jwt.Ed25519PrivateKey(ed25519PrivateKey1)),
			opts:    nil,
			err:     nil,
		},
		{
			payload: 0xDEAD,
			alg:     jwt.NewHS256([]byte("secret")),
			opts:    nil,
			err:     jwt.ErrNotJSONObject,
		},
		{
			payload: jwt.Payload{},
			alg: &jwtutil.Resolver{New: func(hd jwt.Header) (jwt.Algorithm, error) {
				return jwt.NewHS256([]byte("secret")), nil
			}},
			opts: nil,
			err:  nil,
		},
		{
			payload: jwt.Payload{},
			alg: &jwtutil.Resolver{New: func(hd jwt.Header) (jwt.Algorithm, error) {
				return nil, testErr
			}},
			opts: nil,
			err:  testErr,
		},
		{
			payload: jwt.Payload{},
			alg: &jwtutil.Resolver{New: func(hd jwt.Header) (jwt.Algorithm, error) {
				return nil, nil
			}},
			opts: nil,
			err:  jwtutil.ErrNilAlg,
		},
		{
			payload: jwt.Payload{},
			alg:     &jwtutil.Resolver{},
			opts:    nil,
			err:     jwtutil.ErrNilAlg,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.alg.Name(), func(t *testing.T) {
			token, err := jwt.Sign(tc.payload, tc.alg, tc.opts...)
			if want, got := tc.err, err; !internal.ErrorIs(got, want) {
				t.Fatalf("jwt.Sign error mismatch (-want +got):\n%s", cmp.Diff(want, got))
			}
			if err == nil && len(token) == 0 {
				t.Fatalf("jwt.Sign return value is empty")
			}
		})
	}
}
