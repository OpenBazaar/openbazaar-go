package jwtutil

import (
	"github.com/gbrlsnchs/jwt/v3"
	"github.com/gbrlsnchs/jwt/v3/internal"
)

// Resolver is an Algorithm resolver.
type Resolver struct {
	New func(jwt.Header) (jwt.Algorithm, error)
	alg jwt.Algorithm
}

// ErrNilAlg is the error for when an algorithm can't be resolved.
var ErrNilAlg = internal.NewError("algorithm is nil")

// Name returns an Algorithm's name.
func (rv *Resolver) Name() string {
	if rv.alg == nil {
		return ""
	}
	return rv.alg.Name()
}

// Resolve sets an Algorithm based on a JOSE Header.
func (rv *Resolver) Resolve(hd jwt.Header) error {
	if rv.alg != nil {
		return nil
	}
	if rv.New == nil {
		return ErrNilAlg
	}
	alg, err := rv.New(hd)
	if err != nil {
		return err
	}
	if alg == nil {
		return ErrNilAlg
	}
	rv.alg = alg
	return nil
}

// Sign returns an error since Resolver doesn't support signing.
func (rv *Resolver) Sign(headerPayload []byte) ([]byte, error) {
	return rv.alg.Sign(headerPayload)
}

// Size returns an Algorithm's size.
func (rv *Resolver) Size() int {
	return rv.alg.Size()
}

// Verify resolves and Algorithm and verifies using it.
func (rv *Resolver) Verify(headerPayload, sig []byte) error {
	return rv.alg.Verify(headerPayload, sig)
}
