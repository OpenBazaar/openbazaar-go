package abi

import big "github.com/filecoin-project/specs-actors/actors/abi/big"

type DealID uint64

// BigInt types are aliases rather than new types because the latter introduce incredible amounts of noise converting to
// and from types in order to manipulate values. We give up some type safety for ergonomics.
type DealWeight = big.Int // units: byte-epochs
