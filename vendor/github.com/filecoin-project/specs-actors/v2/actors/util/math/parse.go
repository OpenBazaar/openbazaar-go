package math

import "math/big"

// Parse a slice of strings (representing integers in decimal)
// Convention: this function is to be applied to strings representing Q.128 fixed-point numbers, and thus returns numbers in binary Q.128 representation
func Parse(coefs []string) []*big.Int {
	out := make([]*big.Int, len(coefs))
	for i, coef := range coefs {
		c, ok := new(big.Int).SetString(coef, 10)
		if !ok {
			panic("could not parse q128 parameter")
		}
		// << 128 (Q.0 to Q.128) >> 128 to transform integer params to coefficients
		out[i] = c
	}
	return out
}
