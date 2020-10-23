package jwt_test

import (
	"testing"

	"github.com/gbrlsnchs/jwt/v3"
	"github.com/gbrlsnchs/jwt/v3/internal"
	"github.com/google/go-cmp/cmp"
)

var (
	ed25519PrivateKey1, ed25519PublicKey1 = internal.GenerateEd25519Keys()
	ed25519PrivateKey2, ed25519PublicKey2 = internal.GenerateEd25519Keys()

	ed25519TestCases = []testCase{
		{
			alg:       jwt.NewEd25519(jwt.Ed25519PrivateKey(ed25519PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewEd25519(jwt.Ed25519PrivateKey(ed25519PrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "Ed25519",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewEd25519(jwt.Ed25519PrivateKey(ed25519PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewEd25519(jwt.Ed25519PublicKey(ed25519PublicKey1)),
			wantHeader: jwt.Header{
				Algorithm: "Ed25519",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewEd25519(jwt.Ed25519PrivateKey(ed25519PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewEd25519(jwt.Ed25519PrivateKey(ed25519PrivateKey2)),
			wantHeader: jwt.Header{
				Algorithm: "Ed25519",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrEd25519Verification,
		},
		{
			alg:       jwt.NewEd25519(jwt.Ed25519PrivateKey(ed25519PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewEd25519(jwt.Ed25519PublicKey(ed25519PublicKey2)),
			wantHeader: jwt.Header{
				Algorithm: "Ed25519",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrEd25519Verification,
		},
	}
)

func TestNewEd25519(t *testing.T) {
	testCases := []struct {
		builder func(...func(*jwt.Ed25519)) *jwt.Ed25519
		opts    func(*jwt.Ed25519)
		err     error
	}{
		{jwt.NewEd25519, nil, jwt.ErrEd25519NilPrivKey},
		{jwt.NewEd25519, jwt.Ed25519PrivateKey(nil), jwt.ErrEd25519NilPrivKey},
		{jwt.NewEd25519, jwt.Ed25519PrivateKey(ed25519PrivateKey1), nil},
		{jwt.NewEd25519, jwt.Ed25519PublicKey(ed25519PublicKey1), nil},
	}
	for _, tc := range testCases {
		funcName := funcName(tc.builder)
		t.Run(funcName, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						t.Fatal("r is not an error")
					}
					if want, got := tc.err, err; !internal.ErrorIs(got, want) {
						t.Fatalf("jwt.%s err mismatch (-want +got):\n%s", funcName, cmp.Diff(want, got))
					}
				}
			}()
			_ = tc.builder(tc.opts)
			if tc.err != nil {
				t.Fatalf("jwt.%s didn't panicked", funcName)
			}
		})
	}
}
