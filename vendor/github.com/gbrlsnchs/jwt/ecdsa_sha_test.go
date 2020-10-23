package jwt_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"

	"testing"

	"github.com/gbrlsnchs/jwt/v3"
	"github.com/gbrlsnchs/jwt/v3/internal"
	"github.com/google/go-cmp/cmp"
)

var (
	es256PrivateKey1, es256PublicKey1 = genECDSAKeys(elliptic.P256())
	es256PrivateKey2, es256PublicKey2 = genECDSAKeys(elliptic.P256())

	es384PrivateKey1, es384PublicKey1 = genECDSAKeys(elliptic.P384())
	es384PrivateKey2, es384PublicKey2 = genECDSAKeys(elliptic.P384())

	es512PrivateKey1, es512PublicKey1 = genECDSAKeys(elliptic.P521())
	es512PrivateKey2, es512PublicKey2 = genECDSAKeys(elliptic.P521())

	ecdsaTestCases = []testCase{
		{
			alg:       jwt.NewES256(jwt.ECDSAPrivateKey(es256PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewES256(jwt.ECDSAPublicKey(es256PublicKey1)),
			wantHeader: jwt.Header{
				Algorithm: "ES256",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewES256(jwt.ECDSAPrivateKey(es256PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewES384(jwt.ECDSAPublicKey(es256PublicKey1)),
			wantHeader: jwt.Header{
				Algorithm: "ES256",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrECDSAVerification,
		},
		{
			alg:       jwt.NewES256(jwt.ECDSAPrivateKey(es256PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewES256(jwt.ECDSAPublicKey(es256PublicKey2)),
			wantHeader: jwt.Header{
				Algorithm: "ES256",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrECDSAVerification,
		},
		{
			alg:       jwt.NewES256(jwt.ECDSAPrivateKey(es256PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewES256(jwt.ECDSAPrivateKey(es256PrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "ES256",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewES256(jwt.ECDSAPrivateKey(es256PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewES256(jwt.ECDSAPrivateKey(es256PrivateKey2)),
			wantHeader: jwt.Header{
				Algorithm: "ES256",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrECDSAVerification,
		},
		{
			alg:       jwt.NewES384(jwt.ECDSAPrivateKey(es384PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewES384(jwt.ECDSAPublicKey(es384PublicKey1)),
			wantHeader: jwt.Header{
				Algorithm: "ES384",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewES384(jwt.ECDSAPrivateKey(es384PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewES256(jwt.ECDSAPublicKey(es384PublicKey1)),
			wantHeader: jwt.Header{
				Algorithm: "ES384",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrECDSAVerification,
		},
		{
			alg:       jwt.NewES384(jwt.ECDSAPrivateKey(es384PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewES384(jwt.ECDSAPublicKey(es384PublicKey2)),
			wantHeader: jwt.Header{
				Algorithm: "ES384",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrECDSAVerification,
		},
		{
			alg:       jwt.NewES384(jwt.ECDSAPrivateKey(es384PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewES384(jwt.ECDSAPrivateKey(es384PrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "ES384",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewES384(jwt.ECDSAPrivateKey(es384PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewES384(jwt.ECDSAPrivateKey(es384PrivateKey2)),
			wantHeader: jwt.Header{
				Algorithm: "ES384",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrECDSAVerification,
		},
		{
			alg:       jwt.NewES512(jwt.ECDSAPrivateKey(es512PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewES512(jwt.ECDSAPublicKey(es512PublicKey1)),
			wantHeader: jwt.Header{
				Algorithm: "ES512",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewES512(jwt.ECDSAPrivateKey(es512PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewES384(jwt.ECDSAPublicKey(es512PublicKey1)),
			wantHeader: jwt.Header{
				Algorithm: "ES512",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrECDSAVerification,
		},
		{
			alg:       jwt.NewES512(jwt.ECDSAPrivateKey(es512PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewES512(jwt.ECDSAPublicKey(es512PublicKey2)),
			wantHeader: jwt.Header{
				Algorithm: "ES512",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrECDSAVerification,
		},
		{
			alg:       jwt.NewES512(jwt.ECDSAPrivateKey(es512PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewES512(jwt.ECDSAPrivateKey(es512PrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "ES512",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewES512(jwt.ECDSAPrivateKey(es512PrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewES512(jwt.ECDSAPrivateKey(es512PrivateKey2)),
			wantHeader: jwt.Header{
				Algorithm: "ES512",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrECDSAVerification,
		},
	}
)

func TestNewECDSASHA(t *testing.T) {
	testCases := []struct {
		builder func(...func(*jwt.ECDSASHA)) *jwt.ECDSASHA
		opts    func(*jwt.ECDSASHA)
		err     error
	}{
		{jwt.NewES256, nil, jwt.ErrECDSANilPrivKey},
		{jwt.NewES256, jwt.ECDSAPrivateKey(nil), jwt.ErrECDSANilPrivKey},
		{jwt.NewES256, jwt.ECDSAPrivateKey(es256PrivateKey1), nil},
		{jwt.NewES256, jwt.ECDSAPublicKey(es256PublicKey1), nil},
		{jwt.NewES384, nil, jwt.ErrECDSANilPrivKey},
		{jwt.NewES384, jwt.ECDSAPrivateKey(nil), jwt.ErrECDSANilPrivKey},
		{jwt.NewES384, jwt.ECDSAPrivateKey(es384PrivateKey1), nil},
		{jwt.NewES384, jwt.ECDSAPublicKey(es384PublicKey1), nil},
		{jwt.NewES512, nil, jwt.ErrECDSANilPrivKey},
		{jwt.NewES512, jwt.ECDSAPrivateKey(nil), jwt.ErrECDSANilPrivKey},
		{jwt.NewES512, jwt.ECDSAPrivateKey(es512PrivateKey1), nil},
		{jwt.NewES512, jwt.ECDSAPublicKey(es512PublicKey1), nil},
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

func genECDSAKeys(c elliptic.Curve) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	priv, err := ecdsa.GenerateKey(c, rand.Reader)
	if err != nil {
		panic(err)
	}
	return priv, &priv.PublicKey
}
