package jwt_test

import (
	"crypto/rand"
	"crypto/rsa"
)

var (
	rsaPrivateKey1, rsaPublicKey1 = genRSAKeys()
	rsaPrivateKey2, rsaPublicKey2 = genRSAKeys()
)

func genRSAKeys() (*rsa.PrivateKey, *rsa.PublicKey) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return priv, &priv.PublicKey
}
