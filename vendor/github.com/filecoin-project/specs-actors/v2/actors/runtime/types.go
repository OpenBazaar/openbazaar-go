package runtime

import (
	"github.com/filecoin-project/go-state-types/rt"
	runtime0 "github.com/filecoin-project/specs-actors/actors/runtime"
)

// Concrete types associated with the runtime interface.

// Result of checking two headers for a consensus fault.
type ConsensusFault = runtime0.ConsensusFault

//type ConsensusFault struct {
//	// Address of the miner at fault (always an ID address).
//	Target addr.Address
//	// Epoch of the fault, which is the higher epoch of the two blocks causing it.
//	Epoch abi.ChainEpoch
//	// Type of fault.
//	Type ConsensusFaultType
//}

type ConsensusFaultType = runtime0.ConsensusFaultType

const (
	ConsensusFaultDoubleForkMining = runtime0.ConsensusFaultDoubleForkMining
	ConsensusFaultParentGrinding   = runtime0.ConsensusFaultParentGrinding
	ConsensusFaultTimeOffsetMining = runtime0.ConsensusFaultTimeOffsetMining
)

type VMActor = rt.VMActor
