package jwt_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
)

var (
	es256PrivateKey1, es256PublicKey1 = genECDSAKeys(elliptic.P256())
	es256PrivateKey2, es256PublicKey2 = genECDSAKeys(elliptic.P256())

	es384PrivateKey1, es384PublicKey1 = genECDSAKeys(elliptic.P384())
	es384PrivateKey2, es384PublicKey2 = genECDSAKeys(elliptic.P384())

	es512PrivateKey1, es512PublicKey1 = genECDSAKeys(elliptic.P521())
	es512PrivateKey2, es512PublicKey2 = genECDSAKeys(elliptic.P521())
)

func genECDSAKeys(c elliptic.Curve) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	priv, err := ecdsa.GenerateKey(c, rand.Reader)
	if err != nil {
		panic(err)
	}
	return priv, &priv.PublicKey
}
