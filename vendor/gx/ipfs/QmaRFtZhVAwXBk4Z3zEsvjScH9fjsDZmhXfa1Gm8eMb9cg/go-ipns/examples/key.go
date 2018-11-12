package examples

import (
	crypto "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
)

// GenerateRSAKeyPair is used to generate an RSA key pair
func GenerateRSAKeyPair(bits int) (crypto.PrivKey, error) {
	priv, _, err := crypto.GenerateKeyPair(crypto.RSA, bits)
	if err != nil {
		return nil, err
	}
	return priv, nil
}

// GenerateEDKeyPair is used to generate an ED25519 keypair
func GenerateEDKeyPair() (crypto.PrivKey, error) {
	// ED25519 ignores the bit param and uses 256bit keys
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	if err != nil {
		return nil, err
	}
	return priv, nil
}
