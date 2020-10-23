// +build !go1.13

package jwt

import (
	"errors"

	"github.com/gbrlsnchs/jwt/v3/internal"
	"golang.org/x/crypto/ed25519"
)

var (
	// ErrEd25519PrivKey is the error for trying to sign a JWT with a nil private key.
	ErrEd25519PrivKey = errors.New("jwt: Ed25519 private key is nil")
	// ErrEd25519PubKey is the error for trying to verify a JWT with a nil public key.
	ErrEd25519PubKey = errors.New("jwt: Ed25519 public key is nil")
	// ErrEd25519Verification is the error for when verification with Ed25519 fails.
	ErrEd25519Verification = errors.New("jwt: Ed25519 verification failed")

	_ Algorithm = new(Ed25519)
)

// Ed25519PrivateKey is an option to set a private key to the Ed25519 algorithm.
func Ed25519PrivateKey(priv ed25519.PrivateKey) func(*Ed25519) {
	return func(ed *Ed25519) {
		ed.priv = priv
	}
}

// Ed25519PublicKey is an option to set a public key to the Ed25519 algorithm.
func Ed25519PublicKey(pub ed25519.PublicKey) func(*Ed25519) {
	return func(ed *Ed25519) {
		ed.pub = pub
	}
}

// Ed25519 is an algorithm that uses EdDSA to sign SHA-512 hashes.
type Ed25519 struct {
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
}

// NewEd25519 creates a new algorithm using EdDSA and SHA-512.
func NewEd25519(opts ...func(*Ed25519)) *Ed25519 {
	var ed Ed25519
	for _, opt := range opts {
		opt(&ed)
	}
	if ed.pub == nil {
		ed.pub = ed.priv.Public().(ed25519.PublicKey)
	}
	return &ed
}

// Name returns the algorithm's name.
func (*Ed25519) Name() string {
	return "Ed25519"
}

// Sign signs headerPayload using the Ed25519 algorithm.
func (ed *Ed25519) Sign(headerPayload []byte) ([]byte, error) {
	if ed.priv == nil {
		return nil, ErrEd25519PrivKey
	}
	return ed25519.Sign(ed.priv, headerPayload), nil
}

// Size returns the signature byte size.
func (*Ed25519) Size() int {
	return ed25519.SignatureSize
}

// Verify verifies a payload and a signature.
func (ed *Ed25519) Verify(payload, sig []byte) (err error) {
	if ed.pub == nil {
		return ErrEd25519PubKey
	}
	if sig, err = internal.DecodeToBytes(sig); err != nil {
		return err
	}
	if !ed25519.Verify(ed.pub, payload, sig) {
		return ErrEd25519Verification
	}
	return nil
}
