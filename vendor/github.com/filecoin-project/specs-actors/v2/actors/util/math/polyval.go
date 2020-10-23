package math

import "math/big"

// note: all coefficients for which Polyval is used would need to be updated if this precision changes
const Precision128 = 128

// polyval evaluates a polynomial given by coefficients `p` in Q.128 format
// at point `x` in Q.128 format. Output is in Q.128.
// Coefficients should be ordered from the highest order coefficient to the lowest.
func Polyval(p []*big.Int, x *big.Int) *big.Int {
	// evaluation using Horner's method
	res := new(big.Int).Set(p[0]) // Q.128
	tmp := new(big.Int)           // big.Int.Mul doesn't like when input is reused as output
	for _, c := range p[1:] {
		tmp = tmp.Mul(res, x)            // Q.128 * Q.128 => Q.256
		res = res.Rsh(tmp, Precision128) // Q.256 >> 128 => Q.128
		res = res.Add(res, c)
	}

	return res
}
