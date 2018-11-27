package misc

// The copy-paste history of this document is long:
// Originally:
//   Copyright (c) 2013-2014 Conformal Systems LLC.
//   Use of this source code is governed by an ISC
//   license that can be found in the LICENSE file
//   at https://github.com/btcsuite/btcutil .
// Modified by Juan Benet (juan@benet.ai),
//   at https://github.com/jbenet/go-base58 .
// Now here, modified by Eric Myhre; consider it
//   either ISC, or Apachev2, or MIT at your convenience
//   and legal ability to keep a straight face.

// Changes from upstream: method renames, less exports,
// fewer parameters.  This is only here as a necessity,
// and generally used to create strings that are thereafter
// considered opaque.

import (
	"math/big"
	"strings"
)

// This alphabet is the modified base58 alphabet used by Bitcoin.
// ("modified" compared to what?  I don't know.  There's not an RFC, afaict.)
// It's also the defacto choice of IPFS systems.
// It's also the order of the characters in ascii -- uppercase precedes lower.
const b58alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var bigRadix = big.NewInt(58)
var bigZero = big.NewInt(0)

func Base58Decode(b string) []byte {
	answer := big.NewInt(0)
	j := big.NewInt(1)

	for i := len(b) - 1; i >= 0; i-- {
		tmp := strings.IndexAny(b58alphabet, string(b[i]))
		if tmp == -1 {
			return []byte("")
		}
		idx := big.NewInt(int64(tmp))
		tmp1 := big.NewInt(0)
		tmp1.Mul(j, idx)

		answer.Add(answer, tmp1)
		j.Mul(j, bigRadix)
	}

	tmpval := answer.Bytes()

	var numZeros int
	for numZeros = 0; numZeros < len(b); numZeros++ {
		if b[numZeros] != b58alphabet[0] {
			break
		}
	}
	flen := numZeros + len(tmpval)
	val := make([]byte, flen, flen)
	copy(val[numZeros:], tmpval)

	return val
}

func Base58Encode(b []byte) string {
	x := new(big.Int)
	x.SetBytes(b)

	answer := make([]byte, 0, len(b)*136/100)
	for x.Cmp(bigZero) > 0 {
		mod := new(big.Int)
		x.DivMod(x, bigRadix, mod)
		answer = append(answer, b58alphabet[mod.Int64()])
	}

	// leading zero bytes
	for _, i := range b {
		if i != 0 {
			break
		}
		answer = append(answer, b58alphabet[0])
	}

	// reverse
	alen := len(answer)
	for i := 0; i < alen/2; i++ {
		answer[i], answer[alen-1-i] = answer[alen-1-i], answer[i]
	}

	return string(answer)
}
