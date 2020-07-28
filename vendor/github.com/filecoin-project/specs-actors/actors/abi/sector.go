package abi

import (
	"fmt"
	"strconv"

	cid "github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	big "github.com/filecoin-project/specs-actors/actors/abi/big"
)

// SectorNumber is a numeric identifier for a sector. It is usually relative to a miner.
type SectorNumber uint64

func (s SectorNumber) String() string {
	return strconv.FormatUint(uint64(s), 10)
}

// SectorSize indicates one of a set of possible sizes in the network.
// Ideally, SectorSize would be an enum
// type SectorSize enum {
//   1KiB = 1024
//   1MiB = 1048576
//   1GiB = 1073741824
//   1TiB = 1099511627776
//   1PiB = 1125899906842624
//   1EiB = 1152921504606846976
//   max  = 18446744073709551615
// }
type SectorSize uint64

// Formats the size as a decimal string.
func (s SectorSize) String() string {
	return strconv.FormatUint(uint64(s), 10)
}

// Abbreviates the size as a human-scale number.
// This approximates (truncates) the size unless it is a power of 1024.
func (s SectorSize) ShortString() string {
	var biUnits = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	unit := 0
	for s >= 1024 && unit < len(biUnits)-1 {
		s /= 1024
		unit++
	}
	return fmt.Sprintf("%d%s", s, biUnits[unit])
}

type SectorID struct {
	Miner  ActorID
	Number SectorNumber
}

// The unit of storage power (measured in bytes)
type StoragePower = big.Int

type SectorQuality = big.Int

func NewStoragePower(n int64) StoragePower {
	return big.NewInt(n)
}

type RegisteredProof = int64

// This ordering, defines mappings to UInt in a way which MUST never change.
type RegisteredSealProof RegisteredProof

const (
	RegisteredSealProof_StackedDrg2KiBV1   = RegisteredSealProof(0)
	RegisteredSealProof_StackedDrg8MiBV1   = RegisteredSealProof(1)
	RegisteredSealProof_StackedDrg512MiBV1 = RegisteredSealProof(2)
	RegisteredSealProof_StackedDrg32GiBV1  = RegisteredSealProof(3)
	RegisteredSealProof_StackedDrg64GiBV1  = RegisteredSealProof(4)
)

type RegisteredPoStProof RegisteredProof

const (
	RegisteredPoStProof_StackedDrgWinning2KiBV1   = RegisteredPoStProof(0)
	RegisteredPoStProof_StackedDrgWinning8MiBV1   = RegisteredPoStProof(1)
	RegisteredPoStProof_StackedDrgWinning512MiBV1 = RegisteredPoStProof(2)
	RegisteredPoStProof_StackedDrgWinning32GiBV1  = RegisteredPoStProof(3)
	RegisteredPoStProof_StackedDrgWinning64GiBV1  = RegisteredPoStProof(4)
	RegisteredPoStProof_StackedDrgWindow2KiBV1    = RegisteredPoStProof(5)
	RegisteredPoStProof_StackedDrgWindow8MiBV1    = RegisteredPoStProof(6)
	RegisteredPoStProof_StackedDrgWindow512MiBV1  = RegisteredPoStProof(7)
	RegisteredPoStProof_StackedDrgWindow32GiBV1   = RegisteredPoStProof(8)
	RegisteredPoStProof_StackedDrgWindow64GiBV1   = RegisteredPoStProof(9)
)

func (p RegisteredPoStProof) RegisteredSealProof() (RegisteredSealProof, error) {
	switch p {
	case RegisteredPoStProof_StackedDrgWinning2KiBV1, RegisteredPoStProof_StackedDrgWindow2KiBV1:
		return RegisteredSealProof_StackedDrg2KiBV1, nil
	case RegisteredPoStProof_StackedDrgWinning8MiBV1, RegisteredPoStProof_StackedDrgWindow8MiBV1:
		return RegisteredSealProof_StackedDrg8MiBV1, nil
	case RegisteredPoStProof_StackedDrgWinning512MiBV1, RegisteredPoStProof_StackedDrgWindow512MiBV1:
		return RegisteredSealProof_StackedDrg512MiBV1, nil
	case RegisteredPoStProof_StackedDrgWinning32GiBV1, RegisteredPoStProof_StackedDrgWindow32GiBV1:
		return RegisteredSealProof_StackedDrg32GiBV1, nil
	case RegisteredPoStProof_StackedDrgWinning64GiBV1, RegisteredPoStProof_StackedDrgWindow64GiBV1:
		return RegisteredSealProof_StackedDrg64GiBV1, nil
	default:
		return 0, errors.Errorf("unsupported PoSt proof type: %v", p)
	}
}

func (p RegisteredSealProof) SectorSize() (SectorSize, error) {
	switch p {
	case RegisteredSealProof_StackedDrg2KiBV1:
		return 2 << 10, nil
	case RegisteredSealProof_StackedDrg8MiBV1:
		return 8 << 20, nil
	case RegisteredSealProof_StackedDrg512MiBV1:
		return 512 << 20, nil
	case RegisteredSealProof_StackedDrg32GiBV1:
		return 32 << 30, nil
	case RegisteredSealProof_StackedDrg64GiBV1:
		return 2 * (32 << 30), nil
	default:
		return 0, errors.Errorf("unsupported proof type: %v", p)
	}
}

func (p RegisteredPoStProof) SectorSize() (SectorSize, error) {
	// Resolve to seal proof and then compute size from that.
	sp, err := p.RegisteredSealProof()
	if err != nil {
		return 0, err
	}
	return sp.SectorSize()
}

// Returns the partition size, in sectors, associated with a proof type.
// The partition size is the number of sectors proved in a single PoSt proof.
func (p RegisteredSealProof) WindowPoStPartitionSectors() (uint64, error) {
	// These numbers must match those used by the proofs library.
	// See https://github.com/filecoin-project/rust-fil-proofs/blob/master/filecoin-proofs/src/constants.rs#L85
	switch p {
	case RegisteredSealProof_StackedDrg64GiBV1:
		return 2300, nil
	case RegisteredSealProof_StackedDrg32GiBV1:
		return 2349, nil
	case RegisteredSealProof_StackedDrg2KiBV1:
		return 2, nil
	case RegisteredSealProof_StackedDrg8MiBV1:
		return 2, nil
	case RegisteredSealProof_StackedDrg512MiBV1:
		return 2, nil
	default:
		return 0, errors.Errorf("unsupported proof type: %v", p)
	}
}

// Returns the partition size, in sectors, associated with a proof type.
// The partition size is the number of sectors proved in a single PoSt proof.
func (p RegisteredPoStProof) WindowPoStPartitionSectors() (uint64, error) {
	// Resolve to seal proof and then compute size from that.
	sp, err := p.RegisteredSealProof()
	if err != nil {
		return 0, err
	}
	return sp.WindowPoStPartitionSectors()
}

// RegisteredWinningPoStProof produces the PoSt-specific RegisteredProof corresponding
// to the receiving RegisteredProof.
func (p RegisteredSealProof) RegisteredWinningPoStProof() (RegisteredPoStProof, error) {
	switch p {
	case RegisteredSealProof_StackedDrg64GiBV1:
		return RegisteredPoStProof_StackedDrgWinning64GiBV1, nil
	case RegisteredSealProof_StackedDrg32GiBV1:
		return RegisteredPoStProof_StackedDrgWinning32GiBV1, nil
	case RegisteredSealProof_StackedDrg2KiBV1:
		return RegisteredPoStProof_StackedDrgWinning2KiBV1, nil
	case RegisteredSealProof_StackedDrg8MiBV1:
		return RegisteredPoStProof_StackedDrgWinning8MiBV1, nil
	case RegisteredSealProof_StackedDrg512MiBV1:
		return RegisteredPoStProof_StackedDrgWinning512MiBV1, nil
	default:
		return 0, errors.Errorf("unsupported mapping from %+v to PoSt-specific RegisteredProof", p)
	}
}

// RegisteredWindowPoStProof produces the PoSt-specific RegisteredProof corresponding
// to the receiving RegisteredProof.
func (p RegisteredSealProof) RegisteredWindowPoStProof() (RegisteredPoStProof, error) {
	switch p {
	case RegisteredSealProof_StackedDrg64GiBV1:
		return RegisteredPoStProof_StackedDrgWindow64GiBV1, nil
	case RegisteredSealProof_StackedDrg32GiBV1:
		return RegisteredPoStProof_StackedDrgWindow32GiBV1, nil
	case RegisteredSealProof_StackedDrg2KiBV1:
		return RegisteredPoStProof_StackedDrgWindow2KiBV1, nil
	case RegisteredSealProof_StackedDrg8MiBV1:
		return RegisteredPoStProof_StackedDrgWindow8MiBV1, nil
	case RegisteredSealProof_StackedDrg512MiBV1:
		return RegisteredPoStProof_StackedDrgWindow512MiBV1, nil
	default:
		return 0, errors.Errorf("unsupported mapping from %+v to PoSt-specific RegisteredProof", p)
	}
}

// SectorMaximumLifetime is the maximum duration a sector sealed with this proof may exist between activation and expiration
func (p RegisteredSealProof) SectorMaximumLifetime() ChainEpoch {
	// For all Stacked DRG sectors, the max is 5 years
	epochsPerYear := 1_262_277
	fiveYears := 5 * epochsPerYear
	return ChainEpoch(fiveYears)
}

///
/// Sealing
///

type SealRandomness Randomness
type InteractiveSealRandomness Randomness

// Information needed to verify a seal proof.
type SealVerifyInfo struct {
	SealProof RegisteredSealProof
	SectorID
	DealIDs               []DealID
	Randomness            SealRandomness
	InteractiveRandomness InteractiveSealRandomness
	Proof                 []byte
	SealedCID             cid.Cid // CommR
	UnsealedCID           cid.Cid // CommD
}

///
/// PoSting
///

type PoStRandomness Randomness

// Information about a sector necessary for PoSt verification.
type SectorInfo struct {
	SealProof    RegisteredSealProof // RegisteredProof used when sealing - needs to be mapped to PoSt registered proof when used to verify a PoSt
	SectorNumber SectorNumber
	SealedCID    cid.Cid // CommR
}

type PoStProof struct {
	PoStProof  RegisteredPoStProof
	ProofBytes []byte
}

// Information needed to verify a Winning PoSt attached to a block header.
// Note: this is not used within the state machine, but by the consensus/election mechanisms.
type WinningPoStVerifyInfo struct {
	Randomness        PoStRandomness
	Proofs            []PoStProof
	ChallengedSectors []SectorInfo
	Prover            ActorID // used to derive 32-byte prover ID
}

// Information needed to verify a Window PoSt submitted directly to a miner actor.
type WindowPoStVerifyInfo struct {
	Randomness        PoStRandomness
	Proofs            []PoStProof
	ChallengedSectors []SectorInfo
	Prover            ActorID // used to derive 32-byte prover ID
}
