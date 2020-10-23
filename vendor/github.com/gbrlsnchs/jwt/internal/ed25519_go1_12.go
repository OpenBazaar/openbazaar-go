// +build !go1.13

package internal

import (
	"crypto/rand"

	"golang.org/x/crypto/ed25519"
)

// GenerateEd25519Keys generates a pair of keys for testing purposes.
func GenerateEd25519Keys() (ed25519.PrivateKey, ed25519.PublicKey) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	return priv, pub
}
