package btcutil

import "crypto/ecdsa"
import "crypto/rand"
import "math/big"

type BlindRequesterState struct {
	// secret stuff
	a, b, bInv, c *big.Int

	// shareable stuff
	F    *ecdsa.PublicKey
	X0   *big.Int //
	Mhat *big.Int // called m̂ in the paper
}

// Calculates a blinded version of message m
func BlindMessage(rState *BlindRequesterState, Q, R *ecdsa.PublicKey, m *big.Int) *big.Int {
	crv := Secp256k1().Params()

	// generate F which is not equal to O (§4.2)
	var err error
	F := new(ecdsa.PublicKey)
	for F.X == nil && F.Y == nil {
		// requester's three blinding factors (§4.2)
		rState.a, err = RandFieldElement(rand.Reader)
		maybePanic(err)
		rState.b, err = RandFieldElement(rand.Reader)
		maybePanic(err)
		rState.c, err = RandFieldElement(rand.Reader)
		maybePanic(err)
		rState.bInv = new(big.Int).ModInverse(rState.b, crv.N)

		// requester calculates point F (§4.2)
		abInv := new(big.Int).Mul(rState.a, rState.bInv)
		abInv.Mod(abInv, crv.N)
		bInvR := ScalarMult(rState.bInv, R)
		abInvQ := ScalarMult(abInv, Q)
		cG := ScalarBaseMult(rState.c)
		F = Add(bInvR, abInvQ)
		F = Add(F, cG)
	}
	rState.F = F

	// calculate r and m̂
	r := new(big.Int).Mod(F.X, crv.N)
	mHat := new(big.Int).Mul(rState.b, r)
	mHat.Mul(mHat, m)
	mHat.Add(mHat, rState.a)
	mHat.Mod(mHat, crv.N)
	rState.Mhat = mHat

	return rState.Mhat
}

// Extract true signature from the blind signature
func BlindExtract(rState *BlindRequesterState, sHat *big.Int) *BlindSignature {
	crv := Secp256k1().Params()

	// requester extracts the real signature (§4.4)
	s := new(big.Int).Mul(rState.bInv, sHat)
	s.Add(s, rState.c)
	s.Mod(s, crv.N)
	sig := &BlindSignature{S: s, F: rState.F}
	return sig
}

func maybePanic(err error) {
	if err != nil {
		panic(err)
	}
}
