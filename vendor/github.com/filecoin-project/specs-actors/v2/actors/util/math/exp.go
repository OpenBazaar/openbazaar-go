package math

import "github.com/filecoin-project/go-state-types/big"

// ExpBySquaring takes a Q.128 base b and an int64 exponent n and computes n^b
// using the exponentiation by squaring method, returning a Q.128 value.
func ExpBySquaring(base big.Int, n int64) big.Int {
	one := big.Lsh(big.NewInt(1), Precision128)
	// Base cases
	if n == 0 {
		return one
	}
	if n == 1 {
		return base
	}

	// Recurse
	if n < 0 {
		inverseBase := big.Div(big.Lsh(one, Precision128), base) // Q.256 / Q.128 => Q.128
		return ExpBySquaring(inverseBase, -n)
	}
	baseSquared := big.Mul(base, base)               // Q.128 * Q.128 => Q.256
	baseSquared = big.Rsh(baseSquared, Precision128) // Q.256 => Q.128
	if n%2 == 0 {
		return ExpBySquaring(baseSquared, n/2)
	}
	result := big.Mul(base, ExpBySquaring(baseSquared, (n-1)/2)) // Q.128 * Q.128 => Q.256
	return big.Rsh(result, Precision128)                         // Q.256 => Q.128
}
