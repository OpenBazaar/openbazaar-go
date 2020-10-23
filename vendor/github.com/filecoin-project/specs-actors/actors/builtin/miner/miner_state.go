package miner

import (
	"fmt"
	"reflect"
	"sort"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/dline"
	xc "github.com/filecoin-project/go-state-types/exitcode"
	cid "github.com/ipfs/go-cid"
	errors "github.com/pkg/errors"
	xerrors "golang.org/x/xerrors"

	"github.com/filecoin-project/specs-actors/actors/builtin"
	. "github.com/filecoin-project/specs-actors/actors/util"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
)

// Balance of Miner Actor should be greater than or equal to
// the sum of PreCommitDeposits and LockedFunds.
// It is possible for balance to fall below the sum of
// PCD, LF and InitialPledgeRequirements, and this is a bad
// state (IP Debt) that limits a miner actor's behavior (i.e. no balance withdrawals)
// Excess balance as computed by st.GetAvailableBalance will be
// withdrawable or usable for pre-commit deposit or pledge lock-up.
type State struct {
	// Information not related to sectors.
	Info cid.Cid

	PreCommitDeposits abi.TokenAmount // Total funds locked as PreCommitDeposits
	LockedFunds       abi.TokenAmount // Total rewards and added funds locked in vesting table

	VestingFunds cid.Cid // VestingFunds (Vesting Funds schedule for the miner).

	InitialPledgeRequirement abi.TokenAmount // Sum of initial pledge requirements of all active sectors

	// Sectors that have been pre-committed but not yet proven.
	PreCommittedSectors cid.Cid // Map, HAMT[SectorNumber]SectorPreCommitOnChainInfo

	// PreCommittedSectorsExpiry maintains the state required to expire PreCommittedSectors.
	PreCommittedSectorsExpiry cid.Cid // BitFieldQueue (AMT[Epoch]*BitField)

	// Allocated sector IDs. Sector IDs can never be reused once allocated.
	AllocatedSectors cid.Cid // BitField

	// Information for all proven and not-yet-garbage-collected sectors.
	//
	// Sectors are removed from this AMT when the partition to which the
	// sector belongs is compacted.
	Sectors cid.Cid // Array, AMT[SectorNumber]SectorOnChainInfo (sparse)

	// The first epoch in this miner's current proving period. This is the first epoch in which a PoSt for a
	// partition at the miner's first deadline may arrive. Alternatively, it is after the last epoch at which
	// a PoSt for the previous window is valid.
	// Always greater than zero, this may be greater than the current epoch for genesis miners in the first
	// WPoStProvingPeriod epochs of the chain; the epochs before the first proving period starts are exempt from Window
	// PoSt requirements.
	// Updated at the end of every period by a cron callback.
	ProvingPeriodStart abi.ChainEpoch

	// Index of the deadline within the proving period beginning at ProvingPeriodStart that has not yet been
	// finalized.
	// Updated at the end of each deadline window by a cron callback.
	CurrentDeadline uint64

	// The sector numbers due for PoSt at each deadline in the current proving period, frozen at period start.
	// New sectors are added and expired ones removed at proving period boundary.
	// Faults are not subtracted from this in state, but on the fly.
	Deadlines cid.Cid

	// Deadlines with outstanding fees for early sector termination.
	EarlyTerminations bitfield.BitField
}

type MinerInfo struct {
	// Account that owns this miner.
	// - Income and returned collateral are paid to this address.
	// - This address is also allowed to change the worker address for the miner.
	Owner addr.Address // Must be an ID-address.

	// Worker account for this miner.
	// The associated pubkey-type address is used to sign blocks and messages on behalf of this miner.
	Worker addr.Address // Must be an ID-address.

	// Additional addresses that are permitted to submit messages controlling this actor (optional).
	ControlAddresses []addr.Address // Must all be ID addresses.

	PendingWorkerKey *WorkerKeyChange

	// Byte array representing a Libp2p identity that should be used when connecting to this miner.
	PeerId abi.PeerID

	// Slice of byte arrays representing Libp2p multi-addresses used for establishing a connection with this miner.
	Multiaddrs []abi.Multiaddrs

	// The proof type used by this miner for sealing sectors.
	SealProofType abi.RegisteredSealProof

	// Amount of space in each sector committed by this miner.
	// This is computed from the proof type and represented here redundantly.
	SectorSize abi.SectorSize

	// The number of sectors in each Window PoSt partition (proof).
	// This is computed from the proof type and represented here redundantly.
	WindowPoStPartitionSectors uint64
}

type WorkerKeyChange struct {
	NewWorker   addr.Address // Must be an ID address
	EffectiveAt abi.ChainEpoch
}

// Information provided by a miner when pre-committing a sector.
type SectorPreCommitInfo struct {
	SealProof       abi.RegisteredSealProof
	SectorNumber    abi.SectorNumber
	SealedCID       cid.Cid `checked:"true"` // CommR
	SealRandEpoch   abi.ChainEpoch
	DealIDs         []abi.DealID
	Expiration      abi.ChainEpoch
	ReplaceCapacity bool // Whether to replace a "committed capacity" no-deal sector (requires non-empty DealIDs)
	// The committed capacity sector to replace, and it's deadline/partition location
	ReplaceSectorDeadline  uint64
	ReplaceSectorPartition uint64
	ReplaceSectorNumber    abi.SectorNumber
}

// Information stored on-chain for a pre-committed sector.
type SectorPreCommitOnChainInfo struct {
	Info               SectorPreCommitInfo
	PreCommitDeposit   abi.TokenAmount
	PreCommitEpoch     abi.ChainEpoch
	DealWeight         abi.DealWeight // Integral of active deals over sector lifetime
	VerifiedDealWeight abi.DealWeight // Integral of active verified deals over sector lifetime
}

// Information stored on-chain for a proven sector.
type SectorOnChainInfo struct {
	SectorNumber          abi.SectorNumber
	SealProof             abi.RegisteredSealProof // The seal proof type implies the PoSt proof/s
	SealedCID             cid.Cid                 // CommR
	DealIDs               []abi.DealID
	Activation            abi.ChainEpoch  // Epoch during which the sector proof was accepted
	Expiration            abi.ChainEpoch  // Epoch during which the sector expires
	DealWeight            abi.DealWeight  // Integral of active deals over sector lifetime
	VerifiedDealWeight    abi.DealWeight  // Integral of active verified deals over sector lifetime
	InitialPledge         abi.TokenAmount // Pledge collected to commit this sector
	ExpectedDayReward     abi.TokenAmount // Expected one day projection of reward for sector computed at activation time
	ExpectedStoragePledge abi.TokenAmount // Expected twenty day projection of reward for sector computed at activation time
}

func ConstructState(infoCid cid.Cid, periodStart abi.ChainEpoch, emptyBitfieldCid, emptyArrayCid, emptyMapCid, emptyDeadlinesCid cid.Cid,
	emptyVestingFundsCid cid.Cid) (*State, error) {
	return &State{
		Info: infoCid,

		PreCommitDeposits: abi.NewTokenAmount(0),
		LockedFunds:       abi.NewTokenAmount(0),

		VestingFunds: emptyVestingFundsCid,

		InitialPledgeRequirement: abi.NewTokenAmount(0),

		PreCommittedSectors:       emptyMapCid,
		PreCommittedSectorsExpiry: emptyArrayCid,
		AllocatedSectors:          emptyBitfieldCid,
		Sectors:                   emptyArrayCid,
		ProvingPeriodStart:        periodStart,
		CurrentDeadline:           0,
		Deadlines:                 emptyDeadlinesCid,
		EarlyTerminations:         bitfield.New(),
	}, nil
}

func ConstructMinerInfo(owner addr.Address, worker addr.Address, controlAddrs []addr.Address, pid []byte,
	multiAddrs [][]byte, sealProofType abi.RegisteredSealProof) (*MinerInfo, error) {

	sectorSize, err := sealProofType.SectorSize()
	if err != nil {
		return nil, err
	}

	partitionSectors, err := builtin.SealProofWindowPoStPartitionSectors(sealProofType)
	if err != nil {
		return nil, err
	}
	return &MinerInfo{
		Owner:                      owner,
		Worker:                     worker,
		ControlAddresses:           controlAddrs,
		PendingWorkerKey:           nil,
		PeerId:                     pid,
		Multiaddrs:                 multiAddrs,
		SealProofType:              sealProofType,
		SectorSize:                 sectorSize,
		WindowPoStPartitionSectors: partitionSectors,
	}, nil
}

func (st *State) GetInfo(store adt.Store) (*MinerInfo, error) {
	var info MinerInfo
	if err := store.Get(store.Context(), st.Info, &info); err != nil {
		return nil, xerrors.Errorf("failed to get miner info %w", err)
	}
	return &info, nil
}

func (st *State) SaveInfo(store adt.Store, info *MinerInfo) error {
	c, err := store.Put(store.Context(), info)
	if err != nil {
		return err
	}
	st.Info = c
	return nil
}

// Returns deadline calculations for the current (according to state) proving period.
func (st *State) DeadlineInfo(currEpoch abi.ChainEpoch) *dline.Info {
	return NewDeadlineInfo(st.ProvingPeriodStart, st.CurrentDeadline, currEpoch)
}

// Returns deadline calculations for the current (according to state) proving period.
func (st *State) QuantSpecForDeadline(dlIdx uint64) QuantSpec {
	return QuantSpecForDeadline(NewDeadlineInfo(st.ProvingPeriodStart, dlIdx, 0))
}

func (st *State) AllocateSectorNumber(store adt.Store, sectorNo abi.SectorNumber) error {
	// This will likely already have been checked, but this is a good place
	// to catch any mistakes.
	if sectorNo > abi.MaxSectorNumber {
		return xc.ErrIllegalArgument.Wrapf("sector number out of range: %d", sectorNo)
	}

	var allocatedSectors bitfield.BitField
	if err := store.Get(store.Context(), st.AllocatedSectors, &allocatedSectors); err != nil {
		return xc.ErrIllegalState.Wrapf("failed to load allocated sectors bitfield: %w", err)
	}
	if allocated, err := allocatedSectors.IsSet(uint64(sectorNo)); err != nil {
		return xc.ErrIllegalState.Wrapf("failed to lookup sector number in allocated sectors bitfield: %w", err)
	} else if allocated {
		return xc.ErrIllegalArgument.Wrapf("sector number %d has already been allocated", sectorNo)
	}
	allocatedSectors.Set(uint64(sectorNo))

	if root, err := store.Put(store.Context(), allocatedSectors); err != nil {
		return xc.ErrIllegalArgument.Wrapf("failed to store allocated sectors bitfield after adding sector %d: %w", sectorNo, err)
	} else {
		st.AllocatedSectors = root
	}
	return nil
}

func (st *State) MaskSectorNumbers(store adt.Store, sectorNos bitfield.BitField) error {
	lastSectorNo, err := sectorNos.Last()
	if err != nil {
		return xc.ErrIllegalArgument.Wrapf("invalid mask bitfield: %w", err)
	}

	if lastSectorNo > abi.MaxSectorNumber {
		return xc.ErrIllegalArgument.Wrapf("masked sector number %d exceeded max sector number", lastSectorNo)
	}

	var allocatedSectors bitfield.BitField
	if err := store.Get(store.Context(), st.AllocatedSectors, &allocatedSectors); err != nil {
		return xc.ErrIllegalState.Wrapf("failed to load allocated sectors bitfield: %w", err)
	}

	allocatedSectors, err = bitfield.MergeBitFields(allocatedSectors, sectorNos)
	if err != nil {
		return xc.ErrIllegalState.Wrapf("failed to merge allocated bitfield with mask: %w", err)
	}

	if root, err := store.Put(store.Context(), allocatedSectors); err != nil {
		return xc.ErrIllegalArgument.Wrapf("failed to mask allocated sectors bitfield: %w", err)
	} else {
		st.AllocatedSectors = root
	}
	return nil
}

func (st *State) PutPrecommittedSector(store adt.Store, info *SectorPreCommitOnChainInfo) error {
	precommitted, err := adt.AsMap(store, st.PreCommittedSectors)
	if err != nil {
		return err
	}

	err = precommitted.Put(SectorKey(info.Info.SectorNumber), info)
	if err != nil {
		return errors.Wrapf(err, "failed to store precommitment for %v", info)
	}
	st.PreCommittedSectors, err = precommitted.Root()
	return err
}

func (st *State) GetPrecommittedSector(store adt.Store, sectorNo abi.SectorNumber) (*SectorPreCommitOnChainInfo, bool, error) {
	precommitted, err := adt.AsMap(store, st.PreCommittedSectors)
	if err != nil {
		return nil, false, err
	}

	var info SectorPreCommitOnChainInfo
	found, err := precommitted.Get(SectorKey(sectorNo), &info)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to load precommitment for %v", sectorNo)
	}
	return &info, found, nil
}

// This method gets and returns the requested pre-committed sectors, skipping
// missing sectors.
func (st *State) FindPrecommittedSectors(store adt.Store, sectorNos ...abi.SectorNumber) ([]*SectorPreCommitOnChainInfo, error) {
	precommitted, err := adt.AsMap(store, st.PreCommittedSectors)
	if err != nil {
		return nil, err
	}

	result := make([]*SectorPreCommitOnChainInfo, 0, len(sectorNos))

	for _, sectorNo := range sectorNos {
		var info SectorPreCommitOnChainInfo
		found, err := precommitted.Get(SectorKey(sectorNo), &info)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load precommitment for %v", sectorNo)
		}
		if !found {
			// TODO #564 log: "failed to get precommitted sector on sector %d, dropping from prove commit set"
			continue
		}
		result = append(result, &info)
	}

	return result, nil
}

func (st *State) DeletePrecommittedSectors(store adt.Store, sectorNos ...abi.SectorNumber) error {
	precommitted, err := adt.AsMap(store, st.PreCommittedSectors)
	if err != nil {
		return err
	}

	for _, sectorNo := range sectorNos {
		err = precommitted.Delete(SectorKey(sectorNo))
		if err != nil {
			return xerrors.Errorf("failed to delete precommitment for %v: %w", sectorNo, err)
		}
	}
	st.PreCommittedSectors, err = precommitted.Root()
	return err
}

func (st *State) HasSectorNo(store adt.Store, sectorNo abi.SectorNumber) (bool, error) {
	sectors, err := LoadSectors(store, st.Sectors)
	if err != nil {
		return false, err
	}

	_, found, err := sectors.Get(sectorNo)
	if err != nil {
		return false, xerrors.Errorf("failed to get sector %v: %w", sectorNo, err)
	}
	return found, nil
}

func (st *State) PutSectors(store adt.Store, newSectors ...*SectorOnChainInfo) error {
	sectors, err := LoadSectors(store, st.Sectors)
	if err != nil {
		return xerrors.Errorf("failed to load sectors: %w", err)
	}

	err = sectors.Store(newSectors...)
	if err != nil {
		return err
	}

	st.Sectors, err = sectors.Root()
	if err != nil {
		return xerrors.Errorf("failed to persist sectors: %w", err)
	}
	return nil
}

func (st *State) GetSector(store adt.Store, sectorNo abi.SectorNumber) (*SectorOnChainInfo, bool, error) {
	sectors, err := LoadSectors(store, st.Sectors)
	if err != nil {
		return nil, false, err
	}

	return sectors.Get(sectorNo)
}

func (st *State) DeleteSectors(store adt.Store, sectorNos bitfield.BitField) error {
	sectors, err := LoadSectors(store, st.Sectors)
	if err != nil {
		return err
	}
	err = sectorNos.ForEach(func(sectorNo uint64) error {
		if err = sectors.Delete(sectorNo); err != nil {
			return xerrors.Errorf("failed to delete sector %v: %w", sectorNos, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	st.Sectors, err = sectors.Root()
	return err
}

// Iterates sectors.
// The pointer provided to the callback is not safe for re-use. Copy the pointed-to value in full to hold a reference.
func (st *State) ForEachSector(store adt.Store, f func(*SectorOnChainInfo)) error {
	sectors, err := LoadSectors(store, st.Sectors)
	if err != nil {
		return err
	}
	var sector SectorOnChainInfo
	return sectors.ForEach(&sector, func(idx int64) error {
		f(&sector)
		return nil
	})
}

func (st *State) FindSector(store adt.Store, sno abi.SectorNumber) (uint64, uint64, error) {
	deadlines, err := st.LoadDeadlines(store)
	if err != nil {
		return 0, 0, err
	}
	return FindSector(store, deadlines, sno)
}

// Schedules each sector to expire at its next deadline end. If it can't find
// any given sector, it skips it.
//
// This method assumes that each sector's power has not changed, despite the rescheduling.
//
// Note: this method is used to "upgrade" sectors, rescheduling the now-replaced
// sectors to expire at the end of the next deadline. Given the expense of
// sealing a sector, this function skips missing/faulty/terminated "upgraded"
// sectors instead of failing. That way, the new sectors can still be proved.
func (st *State) RescheduleSectorExpirations(
	store adt.Store, currEpoch abi.ChainEpoch, ssize abi.SectorSize,
	deadlineSectors DeadlineSectorMap,
) error {
	deadlines, err := st.LoadDeadlines(store)
	if err != nil {
		return err
	}
	sectors, err := LoadSectors(store, st.Sectors)
	if err != nil {
		return err
	}

	if err = deadlineSectors.ForEach(func(dlIdx uint64, pm PartitionSectorMap) error {
		dlInfo := NewDeadlineInfo(st.ProvingPeriodStart, dlIdx, currEpoch).NextNotElapsed()
		newExpiration := dlInfo.Last()

		dl, err := deadlines.LoadDeadline(store, dlIdx)
		if err != nil {
			return err
		}

		if err := dl.RescheduleSectorExpirations(store, sectors, newExpiration, pm, ssize, QuantSpecForDeadline(dlInfo)); err != nil {
			return err
		}

		if err := deadlines.UpdateDeadline(store, dlIdx, dl); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}
	return st.SaveDeadlines(store, deadlines)
}

// Assign new sectors to deadlines.
func (st *State) AssignSectorsToDeadlines(
	store adt.Store,
	currentEpoch abi.ChainEpoch,
	sectors []*SectorOnChainInfo,
	partitionSize uint64,
	sectorSize abi.SectorSize,
) (PowerPair, error) {
	deadlines, err := st.LoadDeadlines(store)
	if err != nil {
		return NewPowerPairZero(), err
	}

	// Sort sectors by number to get better runs in partition bitfields.
	sort.Slice(sectors, func(i, j int) bool {
		return sectors[i].SectorNumber < sectors[j].SectorNumber
	})

	var deadlineArr [WPoStPeriodDeadlines]*Deadline
	err = deadlines.ForEach(store, func(idx uint64, dl *Deadline) error {
		// Skip deadlines that aren't currently mutable.
		if deadlineIsMutable(st.ProvingPeriodStart, idx, currentEpoch) {
			deadlineArr[int(idx)] = dl
		}
		return nil
	})
	if err != nil {
		return NewPowerPairZero(), err
	}

	newPower := NewPowerPairZero()
	for dlIdx, deadlineSectors := range assignDeadlines(partitionSize, &deadlineArr, sectors) {
		if len(deadlineSectors) == 0 {
			continue
		}

		quant := st.QuantSpecForDeadline(uint64(dlIdx))
		dl := deadlineArr[dlIdx]

		deadlineNewPower, err := dl.AddSectors(store, partitionSize, deadlineSectors, sectorSize, quant)
		if err != nil {
			return NewPowerPairZero(), err
		}

		newPower = newPower.Add(deadlineNewPower)

		err = deadlines.UpdateDeadline(store, uint64(dlIdx), dl)
		if err != nil {
			return NewPowerPairZero(), err
		}
	}

	err = st.SaveDeadlines(store, deadlines)
	if err != nil {
		return NewPowerPairZero(), err
	}
	return newPower, nil
}

// Pops up to max early terminated sectors from all deadlines.
//
// Returns hasMore if we still have more early terminations to process.
func (st *State) PopEarlyTerminations(store adt.Store, maxPartitions, maxSectors uint64) (result TerminationResult, hasMore bool, err error) {
	stopErr := errors.New("stop error")

	// Anything to do? This lets us avoid loading the deadlines if there's nothing to do.
	noEarlyTerminations, err := st.EarlyTerminations.IsEmpty()
	if err != nil {
		return TerminationResult{}, false, xerrors.Errorf("failed to count deadlines with early terminations: %w", err)
	} else if noEarlyTerminations {
		return TerminationResult{}, false, nil
	}

	// Load deadlines
	deadlines, err := st.LoadDeadlines(store)
	if err != nil {
		return TerminationResult{}, false, xerrors.Errorf("failed to load deadlines: %w", err)
	}

	// Process early terminations.
	if err = st.EarlyTerminations.ForEach(func(dlIdx uint64) error {
		// Load deadline + partitions.
		dl, err := deadlines.LoadDeadline(store, dlIdx)
		if err != nil {
			return xerrors.Errorf("failed to load deadline %d: %w", dlIdx, err)
		}

		deadlineResult, more, err := dl.PopEarlyTerminations(store, maxPartitions-result.PartitionsProcessed, maxSectors-result.SectorsProcessed)
		if err != nil {
			return xerrors.Errorf("failed to pop early terminations for deadline %d: %w", dlIdx, err)
		}

		err = result.Add(deadlineResult)
		if err != nil {
			return xerrors.Errorf("failed to merge result from popping early terminations from deadline: %w", err)
		}

		if !more {
			// safe to do while iterating.
			st.EarlyTerminations.Unset(dlIdx)
		}

		// Save the deadline
		err = deadlines.UpdateDeadline(store, dlIdx, dl)
		if err != nil {
			return xerrors.Errorf("failed to store deadline %d: %w", dlIdx, err)
		}

		if result.BelowLimit(maxPartitions, maxSectors) {
			return nil
		}

		return stopErr
	}); err != nil && err != stopErr {
		return TerminationResult{}, false, xerrors.Errorf("failed to walk early terminations bitfield for deadlines: %w", err)
	}

	// Save back the deadlines.
	err = st.SaveDeadlines(store, deadlines)
	if err != nil {
		return TerminationResult{}, false, xerrors.Errorf("failed to save deadlines: %w", err)
	}

	// Ok, check to see if we've handled all early terminations.
	noEarlyTerminations, err = st.EarlyTerminations.IsEmpty()
	if err != nil {
		return TerminationResult{}, false, xerrors.Errorf("failed to count remaining early terminations deadlines")
	}

	return result, !noEarlyTerminations, nil
}

// Returns an error if the target sector cannot be found and/or is faulty/terminated.
func (st *State) CheckSectorHealth(store adt.Store, dlIdx, pIdx uint64, sector abi.SectorNumber) error {
	dls, err := st.LoadDeadlines(store)
	if err != nil {
		return err
	}

	dl, err := dls.LoadDeadline(store, dlIdx)
	if err != nil {
		return err
	}

	partition, err := dl.LoadPartition(store, pIdx)
	if err != nil {
		return err
	}

	if exists, err := partition.Sectors.IsSet(uint64(sector)); err != nil {
		return xc.ErrIllegalState.Wrapf("failed to decode sectors bitfield (deadline %d, partition %d): %w", dlIdx, pIdx, err)
	} else if !exists {
		return xc.ErrNotFound.Wrapf("sector %d not a member of partition %d, deadline %d", sector, pIdx, dlIdx)
	}

	if faulty, err := partition.Faults.IsSet(uint64(sector)); err != nil {
		return xc.ErrIllegalState.Wrapf("failed to decode faults bitfield (deadline %d, partition %d): %w", dlIdx, pIdx, err)
	} else if faulty {
		return xc.ErrForbidden.Wrapf("sector %d of partition %d, deadline %d is faulty", sector, pIdx, dlIdx)
	}

	if terminated, err := partition.Terminated.IsSet(uint64(sector)); err != nil {
		return xc.ErrIllegalState.Wrapf("failed to decode terminated bitfield (deadline %d, partition %d): %w", dlIdx, pIdx, err)
	} else if terminated {
		return xc.ErrNotFound.Wrapf("sector %d of partition %d, deadline %d is terminated", sector, pIdx, dlIdx)
	}

	return nil
}

// Loads sector info for a sequence of sectors.
func (st *State) LoadSectorInfos(store adt.Store, sectors bitfield.BitField) ([]*SectorOnChainInfo, error) {
	sectorsArr, err := LoadSectors(store, st.Sectors)
	if err != nil {
		return nil, err
	}
	return sectorsArr.Load(sectors)
}

// Loads info for a set of sectors to be proven.
// If any of the sectors are declared faulty and not to be recovered, info for the first non-faulty sector is substituted instead.
// If any of the sectors are declared recovered, they are returned from this method.
func (st *State) LoadSectorInfosForProof(store adt.Store, provenSectors, expectedFaults bitfield.BitField) ([]*SectorOnChainInfo, error) {
	nonFaults, err := bitfield.SubtractBitField(provenSectors, expectedFaults)
	if err != nil {
		return nil, xerrors.Errorf("failed to diff bitfields: %w", err)
	}

	// Return empty if no non-faults
	if empty, err := nonFaults.IsEmpty(); err != nil {
		return nil, xerrors.Errorf("failed to check if bitfield was empty: %w", err)
	} else if empty {
		return nil, nil
	}

	// Select a non-faulty sector as a substitute for faulty ones.
	goodSectorNo, err := nonFaults.First()
	if err != nil {
		return nil, xerrors.Errorf("failed to get first good sector: %w", err)
	}

	// Load sector infos
	sectorInfos, err := st.LoadSectorInfosWithFaultMask(store, provenSectors, expectedFaults, abi.SectorNumber(goodSectorNo))
	if err != nil {
		return nil, xerrors.Errorf("failed to load sector infos: %w", err)
	}
	return sectorInfos, nil
}

// Loads sector info for a sequence of sectors, substituting info for a stand-in sector for any that are faulty.
func (st *State) LoadSectorInfosWithFaultMask(store adt.Store, sectors bitfield.BitField, faults bitfield.BitField, faultStandIn abi.SectorNumber) ([]*SectorOnChainInfo, error) {
	sectorArr, err := LoadSectors(store, st.Sectors)
	if err != nil {
		return nil, xerrors.Errorf("failed to load sectors array: %w", err)
	}
	standInInfo, err := sectorArr.MustGet(faultStandIn)
	if err != nil {
		return nil, fmt.Errorf("failed to load stand-in sector %d: %v", faultStandIn, err)
	}

	// Expand faults into a map for quick lookups.
	// The faults bitfield should already be a subset of the sectors bitfield.
	sectorCount, err := sectors.Count()
	if err != nil {
		return nil, err
	}
	faultSet, err := faults.AllMap(sectorCount)
	if err != nil {
		return nil, fmt.Errorf("failed to expand faults: %w", err)
	}

	// Load the sector infos, masking out fault sectors with a good one.
	sectorInfos := make([]*SectorOnChainInfo, 0, sectorCount)
	err = sectors.ForEach(func(i uint64) error {
		sector := standInInfo
		faulty := faultSet[i]
		if !faulty {
			sectorOnChain, err := sectorArr.MustGet(abi.SectorNumber(i))
			if err != nil {
				return xerrors.Errorf("failed to load sector %d: %w", i, err)
			}
			sector = sectorOnChain
		}
		sectorInfos = append(sectorInfos, sector)
		return nil
	})
	return sectorInfos, err
}

func (st *State) LoadDeadlines(store adt.Store) (*Deadlines, error) {
	var deadlines Deadlines
	if err := store.Get(store.Context(), st.Deadlines, &deadlines); err != nil {
		return nil, xc.ErrIllegalState.Wrapf("failed to load deadlines (%s): %w", st.Deadlines, err)
	}

	return &deadlines, nil
}

func (st *State) SaveDeadlines(store adt.Store, deadlines *Deadlines) error {
	c, err := store.Put(store.Context(), deadlines)
	if err != nil {
		return err
	}
	st.Deadlines = c
	return nil
}

// LoadVestingFunds loads the vesting funds table from the store
func (st *State) LoadVestingFunds(store adt.Store) (*VestingFunds, error) {
	var funds VestingFunds
	if err := store.Get(store.Context(), st.VestingFunds, &funds); err != nil {
		return nil, xerrors.Errorf("failed to load vesting funds (%s): %w", st.VestingFunds, err)
	}

	return &funds, nil
}

// SaveVestingFunds saves the vesting table to the store
func (st *State) SaveVestingFunds(store adt.Store, funds *VestingFunds) error {
	c, err := store.Put(store.Context(), funds)
	if err != nil {
		return err
	}
	st.VestingFunds = c
	return nil
}

//
// Funds and vesting
//

func (st *State) AddPreCommitDeposit(amount abi.TokenAmount) {
	newTotal := big.Add(st.PreCommitDeposits, amount)
	AssertMsg(newTotal.GreaterThanEqual(big.Zero()), "negative pre-commit deposit %s after adding %s to prior %s",
		newTotal, amount, st.PreCommitDeposits)
	st.PreCommitDeposits = newTotal
}

func (st *State) AddInitialPledgeRequirement(amount abi.TokenAmount) {
	newTotal := big.Add(st.InitialPledgeRequirement, amount)
	AssertMsg(newTotal.GreaterThanEqual(big.Zero()), "negative initial pledge requirement %s after adding %s to prior %s",
		newTotal, amount, st.InitialPledgeRequirement)
	st.InitialPledgeRequirement = newTotal
}

// AddLockedFunds first vests and unlocks the vested funds AND then locks the given funds in the vesting table.
func (st *State) AddLockedFunds(store adt.Store, currEpoch abi.ChainEpoch, vestingSum abi.TokenAmount, spec *VestSpec) (vested abi.TokenAmount, err error) {
	AssertMsg(vestingSum.GreaterThanEqual(big.Zero()), "negative vesting sum %s", vestingSum)

	vestingFunds, err := st.LoadVestingFunds(store)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to load vesting funds: %w", err)
	}

	// unlock vested funds first
	amountUnlocked := vestingFunds.unlockVestedFunds(currEpoch)
	st.LockedFunds = big.Sub(st.LockedFunds, amountUnlocked)
	Assert(st.LockedFunds.GreaterThanEqual(big.Zero()))

	// add locked funds now
	vestingFunds.addLockedFunds(currEpoch, vestingSum, st.ProvingPeriodStart, spec)
	st.LockedFunds = big.Add(st.LockedFunds, vestingSum)

	// save the updated vesting table state
	if err := st.SaveVestingFunds(store, vestingFunds); err != nil {
		return big.Zero(), xerrors.Errorf("failed to save vesting funds: %w", err)
	}

	return amountUnlocked, nil
}

// PenalizeFundsInPriorityOrder first unlocks unvested funds from the vesting table.
// If the target is not yet hit it deducts funds from the (new) available balance.
// Returns the amount unlocked from the vesting table and the amount taken from current balance.
// If the penalty exceeds the total amount available in the vesting table and unlocked funds
// the penalty is reduced to match.  This must be fixed when handling bankrupcy:
// https://github.com/filecoin-project/specs-actors/issues/627
func (st *State) PenalizeFundsInPriorityOrder(store adt.Store, currEpoch abi.ChainEpoch, target, unlockedBalance abi.TokenAmount) (fromVesting abi.TokenAmount, fromBalance abi.TokenAmount, err error) {
	fromVesting, err = st.UnlockUnvestedFunds(store, currEpoch, target)
	if err != nil {
		return abi.NewTokenAmount(0), abi.NewTokenAmount(0), err
	}
	if fromVesting.Equals(target) {
		return fromVesting, abi.NewTokenAmount(0), nil
	}

	// unlocked funds were just deducted from available, so track that
	remaining := big.Sub(target, fromVesting)

	fromBalance = big.Min(unlockedBalance, remaining)
	return fromVesting, fromBalance, nil
}

// Unlocks an amount of funds that have *not yet vested*, if possible.
// The soonest-vesting entries are unlocked first.
// Returns the amount actually unlocked.
func (st *State) UnlockUnvestedFunds(store adt.Store, currEpoch abi.ChainEpoch, target abi.TokenAmount) (abi.TokenAmount, error) {
	vestingFunds, err := st.LoadVestingFunds(store)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed tp load vesting funds: %w", err)
	}

	amountUnlocked := vestingFunds.unlockUnvestedFunds(currEpoch, target)

	st.LockedFunds = big.Sub(st.LockedFunds, amountUnlocked)
	Assert(st.LockedFunds.GreaterThanEqual(big.Zero()))

	if err := st.SaveVestingFunds(store, vestingFunds); err != nil {
		return big.Zero(), xerrors.Errorf("failed to save vesting funds: %w", err)
	}

	return amountUnlocked, nil
}

// Unlocks all vesting funds that have vested before the provided epoch.
// Returns the amount unlocked.
func (st *State) UnlockVestedFunds(store adt.Store, currEpoch abi.ChainEpoch) (abi.TokenAmount, error) {
	vestingFunds, err := st.LoadVestingFunds(store)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to load vesting funds: %w", err)
	}

	amountUnlocked := vestingFunds.unlockVestedFunds(currEpoch)
	st.LockedFunds = big.Sub(st.LockedFunds, amountUnlocked)
	Assert(st.LockedFunds.GreaterThanEqual(big.Zero()))

	err = st.SaveVestingFunds(store, vestingFunds)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to save vesing funds: %w", err)
	}

	return amountUnlocked, nil
}

// CheckVestedFunds returns the amount of vested funds that have vested before the provided epoch.
func (st *State) CheckVestedFunds(store adt.Store, currEpoch abi.ChainEpoch) (abi.TokenAmount, error) {
	vestingFunds, err := st.LoadVestingFunds(store)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to load vesting funds: %w", err)
	}

	amountVested := abi.NewTokenAmount(0)

	for i := range vestingFunds.Funds {
		vf := vestingFunds.Funds[i]
		epoch := vf.Epoch
		amount := vf.Amount

		if epoch >= currEpoch {
			break
		}

		amountVested = big.Add(amountVested, amount)
	}

	return amountVested, nil
}

// Unclaimed funds that are not locked -- includes funds used to cover initial pledge requirement
func (st *State) GetUnlockedBalance(actorBalance abi.TokenAmount) abi.TokenAmount {
	unlockedBalance := big.Subtract(actorBalance, st.LockedFunds, st.PreCommitDeposits)
	Assert(unlockedBalance.GreaterThanEqual(big.Zero()))
	return unlockedBalance
}

// Unclaimed funds.  Actor balance - (locked funds, precommit deposit, ip requirement)
// Can go negative if the miner is in IP debt
func (st *State) GetAvailableBalance(actorBalance abi.TokenAmount) abi.TokenAmount {
	availableBalance := st.GetUnlockedBalance(actorBalance)
	return big.Sub(availableBalance, st.InitialPledgeRequirement)
}

func (st *State) AssertBalanceInvariants(balance abi.TokenAmount) {
	Assert(st.PreCommitDeposits.GreaterThanEqual(big.Zero()))
	Assert(st.LockedFunds.GreaterThanEqual(big.Zero()))
	Assert(balance.GreaterThanEqual(big.Sum(st.PreCommitDeposits, st.LockedFunds)))
}

func (st *State) MeetsInitialPledgeCondition(balance abi.TokenAmount) bool {
	available := st.GetUnlockedBalance(balance)
	return available.GreaterThanEqual(st.InitialPledgeRequirement)
}

// pre-commit expiry
func (st *State) QuantSpecEveryDeadline() QuantSpec {
	return NewQuantSpec(WPoStChallengeWindow, st.ProvingPeriodStart)
}

func (st *State) AddPreCommitExpiry(store adt.Store, expireEpoch abi.ChainEpoch, sectorNum abi.SectorNumber) error {
	// Load BitField Queue for sector expiry
	quant := st.QuantSpecEveryDeadline()
	queue, err := LoadBitfieldQueue(store, st.PreCommittedSectorsExpiry, quant)
	if err != nil {
		return xerrors.Errorf("failed to load pre-commit expiry queue: %w", err)
	}

	// add entry for this sector to the queue
	if err := queue.AddToQueueValues(expireEpoch, uint64(sectorNum)); err != nil {
		return xerrors.Errorf("failed to add pre-commit sector expiry to queue: %w", err)
	}

	st.PreCommittedSectorsExpiry, err = queue.Root()
	if err != nil {
		return xerrors.Errorf("failed to save pre-commit sector queue: %w", err)
	}

	return nil
}

func (st *State) checkPrecommitExpiry(store adt.Store, sectors bitfield.BitField) (depositToBurn abi.TokenAmount, err error) {
	depositToBurn = abi.NewTokenAmount(0)

	var precommitsToDelete []abi.SectorNumber
	if err = sectors.ForEach(func(i uint64) error {
		sectorNo := abi.SectorNumber(i)
		sector, found, err := st.GetPrecommittedSector(store, sectorNo)
		if err != nil {
			return err
		}
		if !found {
			// already committed/deleted
			return nil
		}

		// mark it for deletion
		precommitsToDelete = append(precommitsToDelete, sectorNo)

		// increment deposit to burn
		depositToBurn = big.Add(depositToBurn, sector.PreCommitDeposit)
		return nil
	}); err != nil {
		return big.Zero(), xerrors.Errorf("failed to check pre-commit expiries: %w", err)
	}

	// Actually delete it.
	if len(precommitsToDelete) > 0 {
		if err := st.DeletePrecommittedSectors(store, precommitsToDelete...); err != nil {
			return big.Zero(), fmt.Errorf("failed to delete pre-commits: %w", err)
		}
	}

	st.PreCommitDeposits = big.Sub(st.PreCommitDeposits, depositToBurn)
	Assert(st.PreCommitDeposits.GreaterThanEqual(big.Zero()))

	// This deposit was locked separately to pledge collateral so there's no pledge change here.
	return depositToBurn, nil
}

//
// Misc helpers
//

func SectorKey(e abi.SectorNumber) abi.Keyer {
	return abi.UIntKey(uint64(e))
}

func init() {
	// Check that ChainEpoch is indeed an unsigned integer to confirm that SectorKey is making the right interpretation.
	var e abi.SectorNumber
	if reflect.TypeOf(e).Kind() != reflect.Uint64 {
		panic("incorrect sector number encoding")
	}
}
