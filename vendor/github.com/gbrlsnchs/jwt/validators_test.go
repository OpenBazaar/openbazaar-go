package jwt_test

import (
	"testing"
	"time"

	"github.com/gbrlsnchs/jwt/v3"
	"github.com/gbrlsnchs/jwt/v3/internal"
	"github.com/google/go-cmp/cmp"
)

func TestValidators(t *testing.T) {
	now := time.Now()
	iat := jwt.NumericDate(now)
	exp := jwt.NumericDate(now.Add(24 * time.Hour))
	nbf := jwt.NumericDate(now.Add(15 * time.Second))
	jti := "jti"
	aud := jwt.Audience{"aud", "aud1", "aud2", "aud3"}
	sub := "sub"
	iss := "iss"
	testCases := []struct {
		claim string
		pl    *jwt.Payload
		vl    jwt.Validator
		err   error
	}{
		{"iss", &jwt.Payload{Issuer: iss}, jwt.IssuerValidator("iss"), nil},
		{"iss", &jwt.Payload{Issuer: iss}, jwt.IssuerValidator("not_iss"), jwt.ErrIssValidation},
		{"sub", &jwt.Payload{Subject: sub}, jwt.SubjectValidator("sub"), nil},
		{"sub", &jwt.Payload{Subject: sub}, jwt.SubjectValidator("not_sub"), jwt.ErrSubValidation},
		{"aud", &jwt.Payload{Audience: aud}, jwt.AudienceValidator(jwt.Audience{"aud"}), nil},
		{"aud", &jwt.Payload{Audience: aud}, jwt.AudienceValidator(jwt.Audience{"foo", "aud1"}), nil},
		{"aud", &jwt.Payload{Audience: aud}, jwt.AudienceValidator(jwt.Audience{"bar", "aud2"}), nil},
		{"aud", &jwt.Payload{Audience: aud}, jwt.AudienceValidator(jwt.Audience{"baz", "aud3"}), nil},
		{"aud", &jwt.Payload{Audience: aud}, jwt.AudienceValidator(jwt.Audience{"qux", "aud4"}), jwt.ErrAudValidation},
		{"aud", &jwt.Payload{Audience: aud}, jwt.AudienceValidator(jwt.Audience{"not_aud"}), jwt.ErrAudValidation},
		{"exp", &jwt.Payload{ExpirationTime: exp}, jwt.ExpirationTimeValidator(now), nil},
		{"exp", &jwt.Payload{ExpirationTime: exp}, jwt.ExpirationTimeValidator(time.Unix(now.Unix()-int64(24*time.Hour), 0)), nil},
		{"exp", &jwt.Payload{ExpirationTime: exp}, jwt.ExpirationTimeValidator(time.Unix(now.Unix()+int64(24*time.Hour), 0)), jwt.ErrExpValidation},
		{"exp", &jwt.Payload{}, jwt.ExpirationTimeValidator(time.Now()), jwt.ErrExpValidation},
		{"nbf", &jwt.Payload{NotBefore: nbf}, jwt.NotBeforeValidator(now), jwt.ErrNbfValidation},
		{"nbf", &jwt.Payload{NotBefore: nbf}, jwt.NotBeforeValidator(time.Unix(now.Unix()+int64(15*time.Second), 0)), nil},
		{"nbf", &jwt.Payload{NotBefore: nbf}, jwt.NotBeforeValidator(time.Unix(now.Unix()-int64(15*time.Second), 0)), jwt.ErrNbfValidation},
		{"nbf", &jwt.Payload{}, jwt.NotBeforeValidator(time.Now()), nil},
		{"iat", &jwt.Payload{IssuedAt: iat}, jwt.IssuedAtValidator(now), nil},
		{"iat", &jwt.Payload{IssuedAt: iat}, jwt.IssuedAtValidator(time.Unix(now.Unix()+1, 0)), nil},
		{"iat", &jwt.Payload{IssuedAt: iat}, jwt.IssuedAtValidator(time.Unix(now.Unix()-1, 0)), jwt.ErrIatValidation},
		{"iat", &jwt.Payload{}, jwt.IssuedAtValidator(time.Now()), nil},
		{"jti", &jwt.Payload{JWTID: jti}, jwt.IDValidator("jti"), nil},
		{"jti", &jwt.Payload{JWTID: jti}, jwt.IDValidator("not_jti"), jwt.ErrJtiValidation},
	}
	for _, tc := range testCases {
		t.Run(tc.claim, func(t *testing.T) {
			if want, got := tc.err, tc.vl(tc.pl); !internal.ErrorIs(got, want) {
				t.Errorf(cmp.Diff(want, got))
			}
		})
	}
}
