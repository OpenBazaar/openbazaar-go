package jwt_test

import (
	"crypto/rand"
	"crypto/rsa"

	"testing"

	"github.com/gbrlsnchs/jwt/v3"
	"github.com/gbrlsnchs/jwt/v3/internal"
	"github.com/google/go-cmp/cmp"
)

var (
	rsaPrivateKey1, rsaPublicKey1 = genRSAKeys()
	rsaPrivateKey2, rsaPublicKey2 = genRSAKeys()

	rsaTestCases = []testCase{
		{
			alg:       jwt.NewRS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "RS256",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewRS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "RS256",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewRS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS256(jwt.RSAPrivateKey(rsaPrivateKey2)),
			wantHeader: jwt.Header{
				Algorithm: "RS256",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewRS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS256(jwt.RSAPublicKey(rsaPublicKey1)),
			wantHeader: jwt.Header{
				Algorithm: "RS256",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewRS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS256(jwt.RSAPublicKey(rsaPublicKey2)),
			wantHeader: jwt.Header{
				Algorithm: "RS256",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewRS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "RS384",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewRS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "RS384",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewRS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS384(jwt.RSAPrivateKey(rsaPrivateKey2)),
			wantHeader: jwt.Header{
				Algorithm: "RS384",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewRS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS384(jwt.RSAPublicKey(rsaPublicKey1)),
			wantHeader: jwt.Header{
				Algorithm: "RS384",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewRS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS384(jwt.RSAPublicKey(rsaPublicKey2)),
			wantHeader: jwt.Header{
				Algorithm: "RS384",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewRS512(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS512(jwt.RSAPrivateKey(rsaPrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "RS512",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewRS512(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "RS512",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewRS512(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS512(jwt.RSAPrivateKey(rsaPrivateKey2)),
			wantHeader: jwt.Header{
				Algorithm: "RS512",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewRS512(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS512(jwt.RSAPublicKey(rsaPublicKey1)),
			wantHeader: jwt.Header{
				Algorithm: "RS512",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewRS512(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS512(jwt.RSAPublicKey(rsaPublicKey2)),
			wantHeader: jwt.Header{
				Algorithm: "RS512",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
	}
	rsaPSSTestCases = []testCase{
		{
			alg:       jwt.NewPS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewPS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "PS256",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewPS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "PS256",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewPS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewPS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "PS256",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewPS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewPS256(jwt.RSAPrivateKey(rsaPrivateKey2)),
			wantHeader: jwt.Header{
				Algorithm: "PS256",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewPS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewPS256(jwt.RSAPublicKey(rsaPublicKey1)),
			wantHeader: jwt.Header{
				Algorithm: "PS256",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewPS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewPS256(jwt.RSAPublicKey(rsaPublicKey2)),
			wantHeader: jwt.Header{
				Algorithm: "PS256",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewPS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewPS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "PS384",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewPS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "PS384",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewPS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewPS256(jwt.RSAPrivateKey(rsaPrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "PS384",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewPS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewPS384(jwt.RSAPrivateKey(rsaPrivateKey2)),
			wantHeader: jwt.Header{
				Algorithm: "PS384",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewPS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewPS384(jwt.RSAPublicKey(rsaPublicKey1)),
			wantHeader: jwt.Header{
				Algorithm: "PS384",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewPS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewPS384(jwt.RSAPublicKey(rsaPublicKey2)),
			wantHeader: jwt.Header{
				Algorithm: "PS384",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewPS512(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewPS512(jwt.RSAPrivateKey(rsaPrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "PS512",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewPS512(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewRS512(jwt.RSAPrivateKey(rsaPrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "PS512",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewPS512(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewPS384(jwt.RSAPrivateKey(rsaPrivateKey1)),
			wantHeader: jwt.Header{
				Algorithm: "PS512",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewPS512(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewPS512(jwt.RSAPrivateKey(rsaPrivateKey2)),
			wantHeader: jwt.Header{
				Algorithm: "PS512",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
		{
			alg:       jwt.NewPS512(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewPS512(jwt.RSAPublicKey(rsaPublicKey1)),
			wantHeader: jwt.Header{
				Algorithm: "PS512",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewPS512(jwt.RSAPrivateKey(rsaPrivateKey1)),
			payload:   tp,
			verifyAlg: jwt.NewPS512(jwt.RSAPublicKey(rsaPublicKey2)),
			wantHeader: jwt.Header{
				Algorithm: "PS512",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrRSAVerification,
		},
	}
)

func TestNewRSASHA(t *testing.T) {
	testCases := []struct {
		builder func(...func(*jwt.RSASHA)) *jwt.RSASHA
		opts    func(*jwt.RSASHA)
		err     error
	}{
		{jwt.NewRS256, nil, jwt.ErrRSANilPrivKey},
		{jwt.NewRS256, jwt.RSAPrivateKey(nil), jwt.ErrRSANilPrivKey},
		{jwt.NewRS256, jwt.RSAPrivateKey(rsaPrivateKey1), nil},
		{jwt.NewRS256, jwt.RSAPublicKey(rsaPublicKey1), nil},
		{jwt.NewRS384, nil, jwt.ErrRSANilPrivKey},
		{jwt.NewRS384, jwt.RSAPrivateKey(nil), jwt.ErrRSANilPrivKey},
		{jwt.NewRS384, jwt.RSAPrivateKey(rsaPrivateKey1), nil},
		{jwt.NewRS384, jwt.RSAPublicKey(rsaPublicKey1), nil},
		{jwt.NewRS512, nil, jwt.ErrRSANilPrivKey},
		{jwt.NewRS512, jwt.RSAPrivateKey(nil), jwt.ErrRSANilPrivKey},
		{jwt.NewRS512, jwt.RSAPrivateKey(rsaPrivateKey1), nil},
		{jwt.NewRS512, jwt.RSAPublicKey(rsaPublicKey1), nil},
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

func genRSAKeys() (*rsa.PrivateKey, *rsa.PublicKey) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return priv, &priv.PublicKey
}
