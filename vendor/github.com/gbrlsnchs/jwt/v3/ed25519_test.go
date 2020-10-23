package jwt_test

import "github.com/gbrlsnchs/jwt/v3/internal"

var (
	ed25519PrivateKey1, ed25519PublicKey1 = internal.GenerateEd25519Keys()
	ed25519PrivateKey2, ed25519PublicKey2 = internal.GenerateEd25519Keys()
)
