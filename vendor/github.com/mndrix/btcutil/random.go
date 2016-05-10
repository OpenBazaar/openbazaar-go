package btcutil

import "io"
import "math/big"

var one = new(big.Int).SetInt64(1)

// RandFieldElement returns a random element of the field underlying the given
// curve using the procedure given in [NSA] A.2.1.
//
// Implementation copied from Go's crypto/ecdsa package since
// the function wasn't public.  Modified to always use secp256k1 curve.
func RandFieldElement(rand io.Reader) (k *big.Int, err error) {
	params := Secp256k1().Params()
	b := make([]byte, params.BitSize/8+8)
	_, err = io.ReadFull(rand, b)
	if err != nil {
		return
	}

	k = new(big.Int).SetBytes(b)
	n := new(big.Int).Sub(params.N, one)
	k.Mod(k, n)
	k.Add(k, one)
	return
}
