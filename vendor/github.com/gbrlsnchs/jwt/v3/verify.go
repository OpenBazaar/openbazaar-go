package jwt

import (
	"bytes"
	"errors"
)

// ErrAlgValidation indicates an incoming JWT's "alg" field mismatches the Validator's.
var ErrAlgValidation = errors.New(`"alg" field mismatch`)

// VerifyOption is a functional option for verifying.
type VerifyOption func(*RawToken) error

// Verify verifies a token's signature using alg. Before verification, opts is iterated and
// each option in it is run.
func Verify(token []byte, alg Algorithm, payload interface{}, opts ...VerifyOption) (Header, error) {
	rt := &RawToken{
		alg: alg,
	}

	sep1 := bytes.IndexByte(token, '.')
	if sep1 < 0 {
		return rt.hd, ErrMalformed
	}

	cbytes := token[sep1+1:]
	sep2 := bytes.IndexByte(cbytes, '.')
	if sep2 < 0 {
		return rt.hd, ErrMalformed
	}
	rt.setToken(token, sep1, sep2)

	var err error
	if err = rt.decodeHeader(); err != nil {
		return rt.hd, err
	}
	if rv, ok := alg.(Resolver); ok {
		if err = rv.Resolve(rt.hd); err != nil {
			return rt.hd, err
		}
	}
	for _, opt := range opts {
		if err = opt(rt); err != nil {
			return rt.hd, err
		}
	}
	if err = alg.Verify(rt.headerPayload(), rt.sig()); err != nil {
		return rt.hd, err
	}
	return rt.hd, rt.decode(payload)
}

// ValidateHeader checks whether the algorithm contained
// in the JOSE header is the same used by the algorithm.
func ValidateHeader(rt *RawToken) error {
	if rt.alg.Name() != rt.hd.Algorithm {
		return ErrAlgValidation
	}
	return nil
}

// ValidatePayload runs validators against a Payload after it's been decoded.
func ValidatePayload(pl *Payload, vds ...Validator) VerifyOption {
	return func(rt *RawToken) error {
		rt.pl = pl
		rt.vds = vds
		return nil
	}
}

// Compile-time checks.
var _ VerifyOption = ValidateHeader
