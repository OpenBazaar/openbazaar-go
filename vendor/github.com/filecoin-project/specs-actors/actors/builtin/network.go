package builtin

import (
	"fmt"

	big "github.com/filecoin-project/go-state-types/big"
)

// The duration of a chain epoch.
// This is used for deriving epoch-denominated periods that are more naturally expressed in clock time.
// TODO: In lieu of a real configuration mechanism for this value, we'd like to make it a var so that implementations
// can override it at runtime. Doing so requires changing all the static references to it in this repo to go through
// late-binding function calls, or they'll see the "wrong" value.
// https://github.com/filecoin-project/specs-actors/issues/353
// If EpochDurationSeconds is changed, update `BaselineExponent`, `lambda`, and // `expLamSubOne` in ./reward/reward_logic.go
// You can re-calculate these constants by changing the epoch duration in ./reward/reward_calc.py and running it.
const EpochDurationSeconds = 30
const SecondsInHour = 60 * 60
const SecondsInDay = 24 * SecondsInHour
const EpochsInHour = SecondsInHour / EpochDurationSeconds
const EpochsInDay = SecondsInDay / EpochDurationSeconds

// The expected number of block producers in each epoch.
var ExpectedLeadersPerEpoch = int64(5)

func init() {
	//noinspection GoBoolExpressions
	if SecondsInHour%EpochDurationSeconds != 0 {
		// This even division is an assumption that other code might unwittingly make.
		// Don't rely on it on purpose, though.
		// While we're pretty sure everything will still work fine, we're safer maintaining this invariant anyway.
		panic(fmt.Sprintf("epoch duration %d does not evenly divide one hour (%d)", EpochDurationSeconds, SecondsInHour))
	}
}

// Number of token units in an abstract "FIL" token.
// The network works purely in the indivisible token amounts. This constant converts to a fixed decimal with more
// human-friendly scale.
var TokenPrecision = big.NewIntUnsigned(1_000_000_000_000_000_000)

// The maximum supply of Filecoin that will ever exist (in token units)
var TotalFilecoin = big.Mul(big.NewIntUnsigned(2_000_000_000), TokenPrecision)

// Quality multiplier for committed capacity (no deals) in a sector
var QualityBaseMultiplier = big.NewInt(10)

// Quality multiplier for unverified deals in a sector
var DealWeightMultiplier = big.NewInt(10)

// Quality multiplier for verified deals in a sector
var VerifiedDealWeightMultiplier = big.NewInt(100)

// Precision used for making QA power calculations
const SectorQualityPrecision = 20
