package builtin

import "fmt"

// The duration of a chain epoch.
// This is used for deriving epoch-denominated periods that are more naturally expressed in clock time.
// TODO: In lieu of a real configuration mechanism for this value, we'd like to make it a var so that implementations
// can override it at runtime. Doing so requires changing all the static references to it in this repo to go through
// late-binding function calls, or they'll see the "wrong" value.
// https://github.com/filecoin-project/specs-actors/issues/353
const EpochDurationSeconds = 25
const SecondsInHour = 3600
const SecondsInDay = 86400
const SecondsInYear = 31556925
const EpochsInHour = SecondsInHour / EpochDurationSeconds
const EpochsInDay = SecondsInDay / EpochDurationSeconds
const EpochsInYear = SecondsInYear / EpochDurationSeconds

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
