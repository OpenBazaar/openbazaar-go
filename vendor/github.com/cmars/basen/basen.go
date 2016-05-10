// Copyright (c) 2014 Casey Marshall. See LICENSE file for details.

package basen

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"unicode/utf8"
)

var zero = big.NewInt(int64(0))

// Encoding represents a given base-N encoding.
type Encoding struct {
	alphabet string
	index    map[byte]*big.Int
	base     *big.Int
}

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// Base62 represents bytes as a base-62 number [0-9A-Za-z].
var Base62 = NewEncoding(base62Alphabet)

const base58Alphabet = "123456789abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ"

// Base58 represents bytes as a base-58 number [1-9A-GHJ-LM-Za-z].
var Base58 = NewEncoding(base58Alphabet)

// NewEncoding creates a new base-N representation from the given alphabet.
// Panics if the alphabet is not unique. Only ASCII characters are supported.
func NewEncoding(alphabet string) *Encoding {
	return &Encoding{
		alphabet: alphabet,
		index:    newAlphabetMap(alphabet),
		base:     big.NewInt(int64(len(alphabet))),
	}
}

func newAlphabetMap(s string) map[byte]*big.Int {
	if utf8.RuneCountInString(s) != len(s) {
		panic("multi-byte characters not supported")
	}
	result := make(map[byte]*big.Int)
	for i := range s {
		result[s[i]] = big.NewInt(int64(i))
	}
	if len(result) != len(s) {
		panic("alphabet contains non-unique characters")
	}
	return result
}

// Random returns the base-encoded representation of n random bytes.
func (enc *Encoding) Random(n int) (string, error) {
	buf := make([]byte, n)
	_, err := rand.Reader.Read(buf)
	if err != nil {
		return "", err
	}
	return enc.EncodeToString(buf), nil
}

// MustRandom returns the base-encoded representation of n random bytes,
// panicking in the unlikely event of a read error from the random source.
func (enc *Encoding) MustRandom(n int) string {
	s, err := enc.Random(n)
	if err != nil {
		panic(err)
	}
	return s
}

// Base returns the number base of the encoding.
func (enc *Encoding) Base() int {
	return len(enc.alphabet)
}

// EncodeToString returns the base-encoded string representation
// of the given bytes.
func (enc *Encoding) EncodeToString(b []byte) string {
	n := new(big.Int)
	r := new(big.Int)
	n.SetBytes(b)
	var result []byte
	for n.Cmp(zero) > 0 {
		n, r = n.DivMod(n, enc.base, r)
		result = append([]byte{enc.alphabet[r.Int64()]}, result...)
	}
	return string(result)
}

// DecodeString returns the bytes for the given base-encoded string.
func (enc *Encoding) DecodeString(s string) ([]byte, error) {
	result := new(big.Int)
	for i := range s {
		n, ok := enc.index[s[i]]
		if !ok {
			return nil, fmt.Errorf("invalid character %q at index %d", s[i], i)
		}
		result = result.Add(result.Mul(result, enc.base), n)
	}
	return result.Bytes(), nil
}

// DecodeStringN returns N bytes for the given base-encoded string.
// Use this method to ensure the value is left-padded with zeroes.
func (enc *Encoding) DecodeStringN(s string, n int) ([]byte, error) {
	value, err := enc.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(value) > n {
		return nil, fmt.Errorf("value is too large")
	}
	pad := make([]byte, n-len(value))
	return append(pad, value...), nil
}
