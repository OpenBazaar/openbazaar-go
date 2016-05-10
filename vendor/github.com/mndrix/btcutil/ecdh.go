package btcutil

import "crypto/ecdsa"
import "math/big"

// Calculate a shared secret using elliptic curve Diffie-Hellman
func ECDH(priv *ecdsa.PrivateKey, pub *ecdsa.PublicKey) *big.Int {
	x, _ := Secp256k1().ScalarMult(pub.X, pub.Y, priv.D.Bytes())
	return x
}
