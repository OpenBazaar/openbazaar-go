package power

import (
	abi "github.com/filecoin-project/specs-actors/actors/abi"
)

// Minimum number of registered miners for the minimum miner size limit to effectively limit consensus power.
const ConsensusMinerMinMiners = 3

// Minimum power of an individual miner to meet the threshold for leader election.
var ConsensusMinerMinPower = abi.NewStoragePower(1 << 40) // PARAM_FINISH
