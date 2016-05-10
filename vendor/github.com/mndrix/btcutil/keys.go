package btcutil

import "crypto/ecdsa"
import "io"

// GenerateKey generates a public and private key pair
func GenerateKey(rand io.Reader) (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(Secp256k1(), rand)
}

func KeysEqual(a, b *ecdsa.PublicKey) bool {
	return a.X.Cmp(b.X) == 0 && a.Y.Cmp(b.Y) == 0
}
