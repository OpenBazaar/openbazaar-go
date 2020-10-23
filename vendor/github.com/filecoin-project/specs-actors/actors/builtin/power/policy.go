package power

import (
	abi "github.com/filecoin-project/go-state-types/abi"
)

// Minimum number of registered miners for the minimum miner size limit to effectively limit consensus power.
const ConsensusMinerMinMiners = 3

// Minimum power of an individual miner to meet the threshold for leader election.
var ConsensusMinerMinPower = abi.NewStoragePower(1 << 40) // PARAM_FINISH

// Maximum number of prove commits a miner can submit in one epoch
//
// We bound this to 200 to limit the number of prove partitions we may need to update in a given epoch to 200.
//
// To support onboarding 1EiB/year, we need to allow at least 32 prove commits per epoch.
const MaxMinerProveCommitsPerEpoch = 200
