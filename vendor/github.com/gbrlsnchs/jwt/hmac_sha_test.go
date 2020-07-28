package jwt_test

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/gbrlsnchs/jwt/v3"
	"github.com/gbrlsnchs/jwt/v3/internal"
	"github.com/google/go-cmp/cmp"
)

var (
	hmacKey1 = []byte("secret")
	hmacKey2 = []byte("terces")

	hmacTestCases = []testCase{
		{
			alg:       jwt.NewHS256(hmacKey1),
			payload:   tp,
			verifyAlg: jwt.NewHS256(hmacKey1),
			wantHeader: jwt.Header{
				Algorithm: "HS256",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewHS256(hmacKey1),
			payload:   tp,
			verifyAlg: jwt.NewHS256(hmacKey2),
			wantHeader: jwt.Header{
				Algorithm: "HS256",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrHMACVerification,
		},
		{
			alg:       jwt.NewHS256(hmacKey1),
			payload:   tp,
			verifyAlg: jwt.NewHS384(hmacKey1),
			wantHeader: jwt.Header{
				Algorithm: "HS256",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrHMACVerification,
		},
		{
			alg:       jwt.NewHS384(hmacKey1),
			payload:   tp,
			verifyAlg: jwt.NewHS384(hmacKey1),
			wantHeader: jwt.Header{
				Algorithm: "HS384",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewHS384(hmacKey1),
			payload:   tp,
			verifyAlg: jwt.NewHS384(hmacKey2),
			wantHeader: jwt.Header{
				Algorithm: "HS384",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrHMACVerification,
		},
		{
			alg:       jwt.NewHS384(hmacKey1),
			payload:   tp,
			verifyAlg: jwt.NewHS256(hmacKey1),
			wantHeader: jwt.Header{
				Algorithm: "HS384",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrHMACVerification,
		},
		{
			alg:       jwt.NewHS512(hmacKey1),
			payload:   tp,
			verifyAlg: jwt.NewHS512(hmacKey1),
			wantHeader: jwt.Header{
				Algorithm: "HS512",
				Type:      "JWT",
			},
			wantPayload: tp,
			signErr:     nil,
			verifyErr:   nil,
		},
		{
			alg:       jwt.NewHS512(hmacKey1),
			payload:   tp,
			verifyAlg: jwt.NewHS512(hmacKey2),
			wantHeader: jwt.Header{
				Algorithm: "HS512",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrHMACVerification,
		},
		{
			alg:       jwt.NewHS512(hmacKey1),
			payload:   tp,
			verifyAlg: jwt.NewHS256(hmacKey1),
			wantHeader: jwt.Header{
				Algorithm: "HS512",
				Type:      "JWT",
			},
			wantPayload: testPayload{},
			signErr:     nil,
			verifyErr:   jwt.ErrHMACVerification,
		},
	}
)

func TestNewHMACSHA(t *testing.T) {
	testCases := []struct {
		builder func([]byte) *jwt.HMACSHA
		key     []byte
		err     error
	}{
		{jwt.NewHS256, nil, jwt.ErrHMACMissingKey},
		{jwt.NewHS256, []byte(""), jwt.ErrHMACMissingKey},
		{jwt.NewHS256, []byte("a"), nil},
		{jwt.NewHS384, nil, jwt.ErrHMACMissingKey},
		{jwt.NewHS384, []byte(""), jwt.ErrHMACMissingKey},
		{jwt.NewHS384, []byte("a"), nil},
		{jwt.NewHS512, nil, jwt.ErrHMACMissingKey},
		{jwt.NewHS512, []byte(""), jwt.ErrHMACMissingKey},
		{jwt.NewHS512, []byte("a"), nil},
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
			_ = tc.builder(tc.key)
			if tc.err != nil {
				t.Fatalf("jwt.%s didn't panicked", funcName)
			}
		})
	}
}

func funcName(fn interface{}) string {
	return strings.Split(
		runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name(),
		".",
	)[2]
}
