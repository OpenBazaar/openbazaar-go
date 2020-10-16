package builtin

import (
	stabi "github.com/filecoin-project/go-state-types/abi"
	"github.com/pkg/errors"
)

// Metadata about a seal proof type.
type SealProofPolicy struct {
	WindowPoStPartitionSectors uint64
	SectorMaxLifetime          stabi.ChainEpoch
	ConsensusMinerMinPower     stabi.StoragePower
}

// For all Stacked DRG sectors, the max is 5 years
const epochsPerYear = 1_051_200
const fiveYears = stabi.ChainEpoch(5 * epochsPerYear)

// Partition sizes must match those used by the proofs library.
// See https://github.com/filecoin-project/rust-fil-proofs/blob/master/filecoin-proofs/src/constants.rs#L85
var SealProofPolicies = map[stabi.RegisteredSealProof]*SealProofPolicy{
	stabi.RegisteredSealProof_StackedDrg2KiBV1: {
		WindowPoStPartitionSectors: 2,
		SectorMaxLifetime:          fiveYears,
		ConsensusMinerMinPower:     stabi.NewStoragePower(0),
	},
	stabi.RegisteredSealProof_StackedDrg8MiBV1: {
		WindowPoStPartitionSectors: 2,
		SectorMaxLifetime:          fiveYears,
		ConsensusMinerMinPower:     stabi.NewStoragePower(16 << 20),
	},
	stabi.RegisteredSealProof_StackedDrg512MiBV1: {
		WindowPoStPartitionSectors: 2,
		SectorMaxLifetime:          fiveYears,
		ConsensusMinerMinPower:     stabi.NewStoragePower(1 << 30),
	},
	stabi.RegisteredSealProof_StackedDrg32GiBV1: {

		WindowPoStPartitionSectors: 2349,
		SectorMaxLifetime:          fiveYears,
		ConsensusMinerMinPower:     stabi.NewStoragePower(10 << 40),
	},
	stabi.RegisteredSealProof_StackedDrg64GiBV1: {
		WindowPoStPartitionSectors: 2300,
		SectorMaxLifetime:          fiveYears,
		ConsensusMinerMinPower:     stabi.NewStoragePower(20 << 40),
	},
}

// Returns the partition size, in sectors, associated with a proof type.
// The partition size is the number of sectors proved in a single PoSt proof.
func SealProofWindowPoStPartitionSectors(p stabi.RegisteredSealProof) (uint64, error) {
	info, ok := SealProofPolicies[p]
	if !ok {
		return 0, errors.Errorf("unsupported proof type: %v", p)
	}
	return info.WindowPoStPartitionSectors, nil
}

// SectorMaximumLifetime is the maximum duration a sector sealed with this proof may exist between activation and expiration
func SealProofSectorMaximumLifetime(p stabi.RegisteredSealProof) (stabi.ChainEpoch, error) {
	info, ok := SealProofPolicies[p]
	if !ok {
		return 0, errors.Errorf("unsupported proof type: %v", p)
	}
	return info.SectorMaxLifetime, nil
}

// The minimum power of an individual miner to meet the threshold for leader election (in bytes).
// Motivation:
// - Limits sybil generation
// - Improves consensus fault detection
// - Guarantees a minimum fee for consensus faults
// - Ensures that a specific soundness for the power table
// Note: We may be able to reduce this in the future, addressing consensus faults with more complicated penalties,
// sybil generation with crypto-economic mechanism, and PoSt soundness by increasing the challenges for small miners.
func ConsensusMinerMinPower(p stabi.RegisteredSealProof) (stabi.StoragePower, error) {
	info, ok := SealProofPolicies[p]
	if !ok {
		return stabi.NewStoragePower(0), errors.Errorf("unsupported proof type: %v", p)
	}
	return info.ConsensusMinerMinPower, nil
}

// Returns the partition size, in sectors, associated with a proof type.
// The partition size is the number of sectors proved in a single PoSt proof.
func PoStProofWindowPoStPartitionSectors(p stabi.RegisteredPoStProof) (uint64, error) {
	sp, err := p.RegisteredSealProof()
	if err != nil {
		return 0, err
	}
	return SealProofWindowPoStPartitionSectors(sp)
}
