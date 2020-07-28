// +build !go1.11

package internal

import "crypto/rsa"

// RSASignatureSize returns the signature size of an RSA signature.
func RSASignatureSize(pub *rsa.PublicKey) int {
	// As defined at https://golang.org/src/crypto/rsa/rsa.go?s=1609:1641#L39.
	return (pub.N.BitLen() + 7) / 8
}
