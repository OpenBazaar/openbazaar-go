package btcutil

import "crypto/ecdsa"
import "math/big"

// Multiplies the base G by a large integer.  The resulting
// point is represented as an ECDSA public key since that's
// typically how they're used.
func ScalarBaseMult(k *big.Int) *ecdsa.PublicKey {
	key := new(ecdsa.PublicKey)
	key.Curve = Secp256k1()
	key.X, key.Y = Secp256k1().ScalarBaseMult(k.Bytes())
	return key
}

// Multiply a large integer and a point.  The resulting point
// is represented as an ECDSA public key.
func ScalarMult(k *big.Int, B *ecdsa.PublicKey) *ecdsa.PublicKey {
	key := new(ecdsa.PublicKey)
	key.Curve = Secp256k1()
	key.X, key.Y = Secp256k1().ScalarMult(B.X, B.Y, k.Bytes())
	return key
}

// Adds two points to create a third.  Points are represented as
// ECDSA public keys.
func Add(a, b *ecdsa.PublicKey) *ecdsa.PublicKey {
	key := new(ecdsa.PublicKey)
	key.Curve = Secp256k1()
	key.X, key.Y = Secp256k1().Add(a.X, a.Y, b.X, b.Y)
	return key
}
