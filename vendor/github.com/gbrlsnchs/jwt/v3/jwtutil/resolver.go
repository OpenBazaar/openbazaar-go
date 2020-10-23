package jwtutil

import (
	"errors"

	"github.com/gbrlsnchs/jwt/v3"
)

// Resolver is an Algorithm resolver.
type Resolver struct {
	New func(jwt.Header) (jwt.Algorithm, error)
	alg jwt.Algorithm
}

// Name returns an Algorithm's name.
func (rv *Resolver) Name() string {
	return rv.alg.Name()
}

// Resolve sets an Algorithm based on a JOSE Header.
func (rv *Resolver) Resolve(hd jwt.Header) error {
	if rv.alg != nil {
		return nil
	}
	alg, err := rv.New(hd)
	if err != nil {
		return err
	}
	rv.alg = alg
	return nil
}

// Sign returns an error since Resolver doesn't support signing.
func (rv *Resolver) Sign(_ []byte) ([]byte, error) {
	return nil, errors.New("jwtutil: Resolver can only verify")
}

// Size returns an Algorithm's size.
func (rv *Resolver) Size() int {
	return rv.alg.Size()
}

// Verify resolves and Algorithm and verifies using it.
func (rv *Resolver) Verify(headerPayload, sig []byte) error {
	return rv.alg.Verify(headerPayload, sig)
}
