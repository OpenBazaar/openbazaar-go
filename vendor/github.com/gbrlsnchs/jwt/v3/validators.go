package jwt

import (
	"errors"
	"time"
)

var (
	// ErrAudValidation is the error for an invalid "aud" claim.
	ErrAudValidation = errors.New("jwt: aud claim is invalid")
	// ErrExpValidation is the error for an invalid "exp" claim.
	ErrExpValidation = errors.New("jwt: exp claim is invalid")
	// ErrIatValidation is the error for an invalid "iat" claim.
	ErrIatValidation = errors.New("jwt: iat claim is invalid")
	// ErrIssValidation is the error for an invalid "iss" claim.
	ErrIssValidation = errors.New("jwt: iss claim is invalid")
	// ErrJtiValidation is the error for an invalid "jti" claim.
	ErrJtiValidation = errors.New("jwt: jti claim is invalid")
	// ErrNbfValidation is the error for an invalid "nbf" claim.
	ErrNbfValidation = errors.New("jwt: nbf claim is invalid")
	// ErrSubValidation is the error for an invalid "sub" claim.
	ErrSubValidation = errors.New("jwt: sub claim is invalid")
)

// Validator is a function that validates a Payload pointer.
type Validator func(*Payload) error

// AudienceValidator validates the "aud" claim.
// It checks if at least one of the audiences in the JWT's payload is listed in aud.
func AudienceValidator(aud Audience) Validator {
	return func(pl *Payload) error {
		for _, serverAud := range aud {
			for _, clientAud := range pl.Audience {
				if clientAud == serverAud {
					return nil
				}
			}
		}
		return ErrAudValidation
	}
}

// ExpirationTimeValidator validates the "exp" claim.
func ExpirationTimeValidator(now time.Time) Validator {
	return func(pl *Payload) error {
		if pl.ExpirationTime == nil || NumericDate(now).After(pl.ExpirationTime.Time) {
			return ErrExpValidation
		}
		return nil
	}
}

// IssuedAtValidator validates the "iat" claim.
func IssuedAtValidator(now time.Time) Validator {
	return func(pl *Payload) error {
		if pl.IssuedAt != nil && NumericDate(now).Before(pl.IssuedAt.Time) {
			return ErrIatValidation
		}
		return nil
	}
}

// IssuerValidator validates the "iss" claim.
func IssuerValidator(iss string) Validator {
	return func(pl *Payload) error {
		if pl.Issuer != iss {
			return ErrIssValidation
		}
		return nil
	}
}

// IDValidator validates the "jti" claim.
func IDValidator(jti string) Validator {
	return func(pl *Payload) error {
		if pl.JWTID != jti {
			return ErrJtiValidation
		}
		return nil
	}
}

// NotBeforeValidator validates the "nbf" claim.
func NotBeforeValidator(now time.Time) Validator {
	return func(pl *Payload) error {
		if pl.NotBefore != nil && NumericDate(now).Before(pl.NotBefore.Time) {
			return ErrNbfValidation
		}
		return nil
	}
}

// SubjectValidator validates the "sub" claim.
func SubjectValidator(sub string) Validator {
	return func(pl *Payload) error {
		if pl.Subject != sub {
			return ErrSubValidation
		}
		return nil
	}
}
