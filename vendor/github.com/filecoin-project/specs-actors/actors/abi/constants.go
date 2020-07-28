package abi

import "github.com/filecoin-project/specs-actors/actors/abi/big"

// Number of token units in an abstract "FIL" token.
// The network works purely in the indivisible token amounts. This constant converts to a fixed decimal with more
// human-friendly scale.
var TokenPrecision = big.NewIntUnsigned(1_000_000_000_000_000_000)

// The maximum supply of Filecoin that will ever exist (in token units)
var TotalFilecoin = big.Mul(big.NewIntUnsigned(2_000_000_000), TokenPrecision)
