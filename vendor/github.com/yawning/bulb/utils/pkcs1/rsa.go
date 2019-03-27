//
// rsa.go - PKCS#1 RSA key related helpers.
//
// To the extent possible under law, Yawning Angel has waived all copyright and
// related or neighboring rights to bulb, using the creative commons
// "cc0" public domain dedication. See LICENSE or
// <http://creativecommons.org/publicdomain/zero/1.0/> for full details.

// Package pkcs1 implements PKCS#1 RSA key marshalling/unmarshalling,
// compatibile with Tor's usage.
package pkcs1

import (
	"crypto/rsa"
	"crypto/sha1"
	"encoding/asn1"
	"encoding/base32"
	"math/big"
	"strings"
)

type pkcs1RSAPrivKey struct {
	Version int      // version
	N       *big.Int // modulus
	E       int      // publicExponent
	D       *big.Int // privateExponent
	P       *big.Int // prime1
	Q       *big.Int // prime2
	Dp      *big.Int // exponent1: d mod (p-1)
	Dq      *big.Int // exponent2: d mod (q-1)
	Qinv    *big.Int // coefficient: (inverse of q) mod p
}

// EncodePrivateKeyDER returns the PKCS#1 DER encoding of a rsa.PrivateKey.
func EncodePrivateKeyDER(sk *rsa.PrivateKey) ([]byte, error) {
	// The crypto.RSA structure has a slightly different layout than PKCS#1
	// private keys, so directly marshaling does not work.  Pull out the values
	// into a strucuture with the correct layout and marshal.
	sk.Precompute() // Ensure that the structure is fully populated.
	k := pkcs1RSAPrivKey{
		Version: 0,
		N:       sk.N,
		E:       sk.E,
		D:       sk.D,
		P:       sk.Primes[0],
		Q:       sk.Primes[1],
		Dp:      sk.Precomputed.Dp,
		Dq:      sk.Precomputed.Dq,
		Qinv:    sk.Precomputed.Qinv,
	}
	return asn1.Marshal(k)
}

// DecodePrivateKeyDER returns the rsa.PrivateKey decoding of a PKCS#1 DER blob.
func DecodePrivateKeyDER(b []byte) (*rsa.PrivateKey, []byte, error) {
	var k pkcs1RSAPrivKey
	rest, err := asn1.Unmarshal(b, &k)
	if err == nil {
		sk := &rsa.PrivateKey{}
		sk.Primes = make([]*big.Int, 2)
		sk.N = k.N
		sk.E = k.E
		sk.D = k.D
		sk.Primes[0] = k.P
		sk.Primes[1] = k.Q

		// Ignore the precomputed values and just rederive them.
		sk.Precompute()
		return sk, rest, nil
	}
	return nil, rest, err
}

// EncodePublicKeyDER returns the PKCS#1 DER encoding of a rsa.PublicKey.
func EncodePublicKeyDER(pk *rsa.PublicKey) ([]byte, error) {
	// The crypto.RSA structure is exactly the same as the PCKS#1 public keys,
	// when the encoding/asn.1 marshaller is done with it.
	//
	// DER encoding of (SEQUENCE | INTEGER(n) | INTEGER(e))
	return asn1.Marshal(*pk)
}

// DecodePublicKeyDER returns the rsa.PublicKey decoding of a PKCS#1 DER blob.
func DecodePublicKeyDER(b []byte) (*rsa.PublicKey, []byte, error) {
	pk := &rsa.PublicKey{}
	rest, err := asn1.Unmarshal(b, pk)
	return pk, rest, err
}

// OnionAddr returns the Tor Onion Service address corresponding to a given
// rsa.PublicKey.
func OnionAddr(pk *rsa.PublicKey) (string, error) {
	der, err := EncodePublicKeyDER(pk)
	if err != nil {
		return "", err
	}
	h := sha1.Sum(der)
	hb32 := base32.StdEncoding.EncodeToString(h[:10])

	return strings.ToLower(hb32), nil
}
