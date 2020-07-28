// +build go1.11

package internal

import "crypto/rsa"

// RSASignatureSize returns the signature size of an RSA signature.
func RSASignatureSize(pub *rsa.PublicKey) int {
	return pub.Size()
}
