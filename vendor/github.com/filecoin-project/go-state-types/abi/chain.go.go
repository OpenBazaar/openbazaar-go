package abi

import (
	"strconv"

	"github.com/filecoin-project/go-state-types/big"
)

// Epoch number of the chain state, which acts as a proxy for time within the VM.
type ChainEpoch int64

func (e ChainEpoch) String() string {
	return strconv.FormatInt(int64(e), 10)
}

// TokenAmount is an amount of Filecoin tokens. This type is used within
// the VM in message execution, to account movement of tokens, payment
// of VM gas, and more.
//
// BigInt types are aliases rather than new types because the latter introduce incredible amounts of noise converting to
// and from types in order to manipulate values. We give up some type safety for ergonomics.
type TokenAmount = big.Int

func NewTokenAmount(t int64) TokenAmount {
	return big.NewInt(t)
}

// Randomness is a string of random bytes
type Randomness []byte

// RandomnessLength is the length of the randomness slice.
const RandomnessLength = 32

