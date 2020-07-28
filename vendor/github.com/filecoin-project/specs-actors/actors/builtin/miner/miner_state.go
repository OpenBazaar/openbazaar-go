package miner

import (
	"fmt"
	"reflect"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	cid "github.com/ipfs/go-cid"
	errors "github.com/pkg/errors"
	xerrors "golang.org/x/xerrors"

	abi "github.com/filecoin-project/specs-actors/actors/abi"
	big "github.com/filecoin-project/specs-actors/actors/abi/big"
	. "github.com/filecoin-project/specs-actors/actors/util"
	adt "github.com/filecoin-project/specs-actors/actors/util/adt"
)

// Balance of Miner Actor should be greater than or equal to
// the sum of PreCommitDeposits and LockedFunds.
// Excess balance as computed by st.GetAvailableBalance will be
// withdrawable or usable for pre-commit deposit or pledge lock-up.
type State struct {
	// Information not related to sectors.
	Info cid.Cid

	PreCommitDeposits        abi.TokenAmount // Total funds locked as PreCommitDeposits
	LockedFunds              abi.TokenAmount // Total unvested funds locked as pledge collateral
	VestingFunds             cid.Cid         // Array, AMT[ChainEpoch]TokenAmount
	InitialPledgeRequirement abi.TokenAmount // Sum of initial pledge requirements of all active sectors

	// Sectors that have been pre-committed but not yet proven.
	PreCommittedSectors cid.Cid // Map, HAMT[SectorNumber]SectorPreCommitOnChainInfo

	// Information for all proven and not-yet-expired sectors.
	Sectors cid.Cid // Array, AMT[SectorNumber]SectorOnChainInfo (sparse)

	// The first epoch in this miner's current proving period. This is the first epoch in which a PoSt for a
	// partition at the miner's first deadline may arrive. Alternatively, it is after the last epoch at which
	// a PoSt for the previous window is valid.
	// Always greater than zero, his may be greater than the current epoch for genesis miners in the first
	// WPoStProvingPeriod epochs of the chain; the epochs before the first proving period starts are exempt from Window
	// PoSt requirements.
	// Updated at the end of every period by a power actor cron event.
	ProvingPeriodStart abi.ChainEpoch

	// Sector numbers prove-committed since period start, to be added to Deadlines at next proving period boundary.
	NewSectors *abi.BitField

	// Sector numbers indexed by expiry epoch (which are on proving period boundaries).
	// Invariant: Keys(Sectors) == union(SectorExpirations.Values())
	SectorExpirations cid.Cid // Array, AMT[ChainEpoch]Bitfield

	// The sector numbers due for PoSt at each deadline in the current proving period, frozen at period start.
	// New sectors are added and expired ones removed at proving period boundary.
	// Faults are not subtracted from this in state, but on the fly.
	Deadlines cid.Cid

	// All currently known faulty sectors, mutated eagerly.
	// These sectors are exempt from inclusion in PoSt.
	Faults *abi.BitField

	// Faulty sector numbers indexed by the start epoch of the proving period in which detected.
	// Used to track fault durations for eventual sector termination.
	// At most 14 entries, b/c sectors faulty longer expire.
	// Invariant: Faults == union(FaultEpochs.Values())
	FaultEpochs cid.Cid // AMT[ChainEpoch]Bitfield

	// Faulty sectors that will recover when next included in a valid PoSt.
	// Invariant: Recoveries ⊆ Faults.
	Recoveries *abi.BitField

	// Records successful PoSt submission in the current proving period by partition number.
	// The presence of a partition number indicates on-time PoSt received.
	PostSubmissions *abi.BitField

	// The index of the next deadline for which faults should been detected and processed (after it's closed).
	// The proving period cron handler will always reset this to 0, for the subsequent period.
	// Eager fault detection processing on fault/recovery declarations or PoSt may set a smaller number,
	// indicating partial progress, from which subsequent processing should continue.
	// In the range [0, WPoStProvingPeriodDeadlines).
	NextDeadlineToProcessFaults uint64
}

type MinerInfo struct {
	// Account that owns this miner.
	// - Income and returned collateral are paid to this address.
	// - This address is also allowed to change the worker address for the miner.
	Owner addr.Address // Must be an ID-address.

	// Worker account for this miner.
	// The associated pubkey-type address is used to sign blocks and messages on behalf of this miner.
	Worker addr.Address // Must be an ID-address.

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
	SealedCID       cid.Cid // CommR
	SealRandEpoch   abi.ChainEpoch
	DealIDs         []abi.DealID
	Expiration      abi.ChainEpoch
	ReplaceCapacity bool             // Whether to replace a "committed capacity" no-deal sector (requires non-empty DealIDs)
	ReplaceSector   abi.SectorNumber // The committed capacity sector to replace
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
	SectorNumber       abi.SectorNumber
	SealProof          abi.RegisteredSealProof // The seal proof type implies the PoSt proof/s
	SealedCID          cid.Cid                 // CommR
	DealIDs            []abi.DealID
	Activation         abi.ChainEpoch  // Epoch during which the sector proof was accepted
	Expiration         abi.ChainEpoch  // Epoch during which the sector expires
	DealWeight         abi.DealWeight  // Integral of active deals over sector lifetime
	VerifiedDealWeight abi.DealWeight  // Integral of active verified deals over sector lifetime
	InitialPledge      abi.TokenAmount // Pledge collected to commit this sector
}

func ConstructState(infoCid cid.Cid, periodStart abi.ChainEpoch, emptyArrayCid, emptyMapCid, emptyDeadlinesCid cid.Cid) (*State, error) {

	return &State{
		Info: infoCid,

		PreCommitDeposits:        abi.NewTokenAmount(0),
		LockedFunds:              abi.NewTokenAmount(0),
		VestingFunds:             emptyArrayCid,
		InitialPledgeRequirement: abi.NewTokenAmount(0),

		PreCommittedSectors: emptyMapCid,
		Sectors:             emptyArrayCid,
		ProvingPeriodStart:  periodStart,
		NewSectors:          abi.NewBitField(),
		SectorExpirations:   emptyArrayCid,
		Deadlines:           emptyDeadlinesCid,
		Faults:              abi.NewBitField(),
		FaultEpochs:         emptyArrayCid,
		Recoveries:          abi.NewBitField(),
		PostSubmissions:     abi.NewBitField(),
	}, nil
}

func ConstructMinerInfo(owner addr.Address, worker addr.Address, pid []byte, multiAddrs [][]byte, sealProofType abi.RegisteredSealProof) (*MinerInfo, error) {

	sectorSize, err := sealProofType.SectorSize()
	if err != nil {
		return nil, err
	}

	partitionSectors, err := sealProofType.WindowPoStPartitionSectors()
	if err != nil {
		return nil, err
	}
	return &MinerInfo{
		Owner:                      owner,
		Worker:                     worker,
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

// Returns deadline calculations for the current proving period.
func (st *State) DeadlineInfo(currEpoch abi.ChainEpoch) *DeadlineInfo {
	return ComputeProvingPeriodDeadline(st.ProvingPeriodStart, currEpoch)
}

func (st *State) GetSectorCount(store adt.Store) (uint64, error) {
	arr, err := adt.AsArray(store, st.Sectors)
	if err != nil {
		return 0, err
	}

	return arr.Length(), nil
}

func (st *State) GetMaxAllowedFaults(store adt.Store) (uint64, error) {
	sectorCount, err := st.GetSectorCount(store)
	if err != nil {
		return 0, err
	}
	return 2 * sectorCount, nil
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
	sectors, err := adt.AsArray(store, st.Sectors)
	if err != nil {
		return false, err
	}

	var info SectorOnChainInfo
	found, err := sectors.Get(uint64(sectorNo), &info)
	if err != nil {
		return false, xerrors.Errorf("failed to get sector %v: %w", sectorNo, err)
	}
	return found, nil
}

func (st *State) PutSectors(store adt.Store, newSectors ...*SectorOnChainInfo) error {
	sectors, err := adt.AsArray(store, st.Sectors)
	if err != nil {
		return xerrors.Errorf("failed to load sectors: %w", err)
	}

	for _, sector := range newSectors {
		if err := sectors.Set(uint64(sector.SectorNumber), sector); err != nil {
			return xerrors.Errorf("failed to put sector %v: %w", sector, err)
		}
	}

	st.Sectors, err = sectors.Root()
	if err != nil {
		return xerrors.Errorf("failed to persist sectors: %w", err)
	}
	return nil
}

func (st *State) GetSector(store adt.Store, sectorNo abi.SectorNumber) (*SectorOnChainInfo, bool, error) {
	sectors, err := adt.AsArray(store, st.Sectors)
	if err != nil {
		return nil, false, err
	}

	var info SectorOnChainInfo
	found, err := sectors.Get(uint64(sectorNo), &info)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to get sector %v", sectorNo)
	}
	return &info, found, nil
}

func (st *State) DeleteSectors(store adt.Store, sectorNos *abi.BitField) error {
	sectors, err := adt.AsArray(store, st.Sectors)
	if err != nil {
		return err
	}
	err = sectorNos.ForEach(func(sectorNo uint64) error {
		if err = sectors.Delete(sectorNo); err != nil {
			return errors.Wrapf(err, "failed to delete sector %v", sectorNos)
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
	sectors, err := adt.AsArray(store, st.Sectors)
	if err != nil {
		return err
	}
	var sector SectorOnChainInfo
	return sectors.ForEach(&sector, func(idx int64) error {
		f(&sector)
		return nil
	})
}

// Adds some sector numbers to the new sectors bitfield.
func (st *State) AddNewSectors(sectorNos ...abi.SectorNumber) (err error) {
	ns := abi.NewBitField()
	for _, sector := range sectorNos {
		ns.Set(uint64(sector))
	}
	st.NewSectors, err = bitfield.MergeBitFields(st.NewSectors, ns)
	if err != nil {
		return err
	}

	count, err := st.NewSectors.Count()
	if err != nil {
		return err
	}
	if count > NewSectorsPerPeriodMax {
		return fmt.Errorf("too many new sectors %d, max %d", count, NewSectorsPerPeriodMax)
	}
	return nil
}

// Removes some sector numbers from the new sectors bitfield, if present.
func (st *State) RemoveNewSectors(sectorNos *abi.BitField) (err error) {
	st.NewSectors, err = bitfield.SubtractBitField(st.NewSectors, sectorNos)
	return err
}

// Gets the sector numbers expiring at some epoch.
func (st *State) GetSectorExpirations(store adt.Store, expiry abi.ChainEpoch) (*abi.BitField, error) {
	arr, err := adt.AsArray(store, st.SectorExpirations)
	if err != nil {
		return nil, err
	}

	bf := abi.NewBitField()
	_, err = arr.Get(uint64(expiry), bf)
	if err != nil {
		return nil, err
	}

	return bf, nil
}

// Iterates sector expiration groups in order.
// Note that the sectors bitfield provided to the callback is not safe to store.
func (st *State) ForEachSectorExpiration(store adt.Store, f func(expiry abi.ChainEpoch, sectors *abi.BitField) error) error {
	arr, err := adt.AsArray(store, st.SectorExpirations)
	if err != nil {
		return err
	}

	var bf bitfield.BitField
	return arr.ForEach(&bf, func(i int64) error {
		bfCopy, err := bf.Copy()
		if err != nil {
			return err
		}
		return f(abi.ChainEpoch(i), bfCopy)
	})
}

// Adds some sector numbers to the set expiring at an epoch.
// The sector numbers are given as uint64s to avoid pointless conversions.
func (st *State) AddSectorExpirations(store adt.Store, sectors ...*SectorOnChainInfo) error {
	if len(sectors) == 0 {
		return nil
	}

	arr, err := adt.AsArray(store, st.SectorExpirations)
	if err != nil {
		return xerrors.Errorf("failed to load sector expirations: %w", err)
	}

	for _, epochSet := range groupSectorsByExpiration(sectors) {
		bf := new(abi.BitField)
		_, err = arr.Get(uint64(epochSet.epoch), bf)
		if err != nil {
			return xerrors.Errorf("failed to lookup sector expirations at epoch %v: %w", epochSet.epoch, err)
		}

		bf, err = bitfield.MergeBitFields(bf, bitfield.NewFromSet(epochSet.sectors))
		if err != nil {
			return xerrors.Errorf("failed to update sector expirations at epoch %v: %w", epochSet.epoch, err)
		}
		count, err := bf.Count()
		if err != nil {
			return xerrors.Errorf("failed to count sector expirations at epoch %v: %w", epochSet.epoch, err)
		}
		if count > SectorsMax {
			return fmt.Errorf("too many sectors at expiration %d, %d, max %d", epochSet.epoch, count, SectorsMax)
		}

		if err = arr.Set(uint64(epochSet.epoch), bf); err != nil {
			return xerrors.Errorf("failed to set sector expirations at epoch %v: %w", epochSet.epoch, err)
		}
	}

	st.SectorExpirations, err = arr.Root()
	if err != nil {
		return xerrors.Errorf("failed to persist sector expirations: %w", err)
	}
	return nil
}

// Removes some sector numbers from the set expiring at an epoch.
func (st *State) RemoveSectorExpirations(store adt.Store, sectors ...*SectorOnChainInfo) error {
	if len(sectors) == 0 {
		return nil
	}

	arr, err := adt.AsArray(store, st.SectorExpirations)
	if err != nil {
		return xerrors.Errorf("failed to load sector expirations: %w", err)
	}

	for _, epochSet := range groupSectorsByExpiration(sectors) {
		bf := new(abi.BitField)
		_, err = arr.Get(uint64(epochSet.epoch), bf)
		if err != nil {
			return xerrors.Errorf("failed to lookup sector expirations at epoch %v: %w", epochSet.epoch, err)
		}

		bf, err = bitfield.SubtractBitField(bf, bitfield.NewFromSet(epochSet.sectors))
		if err != nil {
			return xerrors.Errorf("failed to update sector expirations at epoch %v: %w", epochSet.epoch, err)
		}

		if empty, err := bf.IsEmpty(); err != nil {
			return xerrors.Errorf("corrupted sector expirations bitfield at epoch %v: %w", epochSet.epoch, err)
		} else if empty {
			if err := arr.Delete(uint64(epochSet.epoch)); err != nil {
				return xerrors.Errorf("failed to delete sector expirations at epoch %v: %w", epochSet.epoch, err)
			}
		} else if err = arr.Set(uint64(epochSet.epoch), bf); err != nil {
			return xerrors.Errorf("failed to set sector expirations at epoch %v: %w", epochSet.epoch, err)
		}
	}

	st.SectorExpirations, err = arr.Root()
	if err != nil {
		return xerrors.Errorf("failed to persist sector expirations: %w", err)
	}
	return nil
}

// Removes all sector numbers from the set expiring some epochs.
func (st *State) ClearSectorExpirations(store adt.Store, expirations ...abi.ChainEpoch) error {
	arr, err := adt.AsArray(store, st.SectorExpirations)
	if err != nil {
		return err
	}

	for _, exp := range expirations {
		err = arr.Delete(uint64(exp))
		if err != nil {
			return err
		}
	}

	st.SectorExpirations, err = arr.Root()
	return err
}

// Adds sectors numbers to faults and fault epochs.
func (st *State) AddFaults(store adt.Store, sectorNos *abi.BitField, faultEpoch abi.ChainEpoch) (err error) {
	empty, err := sectorNos.IsEmpty()
	if err != nil {
		return err
	}
	if empty {
		return nil
	}

	{
		st.Faults, err = bitfield.MergeBitFields(st.Faults, sectorNos)
		if err != nil {
			return err
		}

		count, err := st.Faults.Count()
		if err != nil {
			return err
		}
		if count > SectorsMax {
			return fmt.Errorf("too many faults %d, max %d", count, SectorsMax)
		}
	}

	{
		epochFaultArr, err := adt.AsArray(store, st.FaultEpochs)
		if err != nil {
			return err
		}

		bf := abi.NewBitField()
		_, err = epochFaultArr.Get(uint64(faultEpoch), bf)
		if err != nil {
			return err
		}

		bf, err = bitfield.MergeBitFields(bf, sectorNos)
		if err != nil {
			return err
		}

		if err = epochFaultArr.Set(uint64(faultEpoch), bf); err != nil {
			return err
		}

		st.FaultEpochs, err = epochFaultArr.Root()
		if err != nil {
			return err
		}
	}

	return nil
}

// Removes sector numbers from faults and fault epochs, if present.
func (st *State) RemoveFaults(store adt.Store, sectorNos *abi.BitField) error {
	if empty, err := sectorNos.IsEmpty(); err != nil {
		return err
	} else if empty {
		return nil
	}

	if newFaults, err := bitfield.SubtractBitField(st.Faults, sectorNos); err != nil {
		return err
	} else {
		st.Faults = newFaults
	}

	arr, err := adt.AsArray(store, st.FaultEpochs)
	if err != nil {
		return err
	}

	type change struct {
		index uint64
		value *abi.BitField
	}

	var (
		epochsChanged []change
		epochsDeleted []uint64
	)

	epochFaultsOld := &abi.BitField{}
	err = arr.ForEach(epochFaultsOld, func(i int64) error {
		countOld, err := epochFaultsOld.Count()
		if err != nil {
			return err
		}

		epochFaultsNew, err := bitfield.SubtractBitField(epochFaultsOld, sectorNos)
		if err != nil {
			return err
		}

		countNew, err := epochFaultsNew.Count()
		if err != nil {
			return err
		}

		if countNew == 0 {
			epochsDeleted = append(epochsDeleted, uint64(i))
		} else if countOld != countNew {
			epochsChanged = append(epochsChanged, change{index: uint64(i), value: epochFaultsNew})
		}

		return nil
	})
	if err != nil {
		return err
	}

	err = arr.BatchDelete(epochsDeleted)
	if err != nil {
		return err
	}

	for _, change := range epochsChanged {
		err = arr.Set(change.index, change.value)
		if err != nil {
			return err
		}
	}

	st.FaultEpochs, err = arr.Root()
	return err
}

// Iterates faults by declaration epoch, in order.
func (st *State) ForEachFaultEpoch(store adt.Store, cb func(epoch abi.ChainEpoch, faults *abi.BitField) error) error {
	arr, err := adt.AsArray(store, st.FaultEpochs)
	if err != nil {
		return err
	}

	var bf bitfield.BitField
	return arr.ForEach(&bf, func(i int64) error {
		bfCopy, err := bf.Copy()
		if err != nil {
			return err
		}
		return cb(abi.ChainEpoch(i), bfCopy)
	})
}

func (st *State) ClearFaultEpochs(store adt.Store, epochs ...abi.ChainEpoch) error {
	arr, err := adt.AsArray(store, st.FaultEpochs)
	if err != nil {
		return err
	}

	for _, exp := range epochs {
		err = arr.Delete(uint64(exp))
		if err != nil {
			return nil
		}
	}

	st.FaultEpochs, err = arr.Root()
	return err
}

// Adds sectors to recoveries.
func (st *State) AddRecoveries(sectorNos *abi.BitField) (err error) {
	empty, err := sectorNos.IsEmpty()
	if err != nil {
		return err
	}
	if empty {
		return nil
	}
	st.Recoveries, err = bitfield.MergeBitFields(st.Recoveries, sectorNos)
	if err != nil {
		return err
	}

	count, err := st.Recoveries.Count()
	if err != nil {
		return err
	}
	if count > SectorsMax {
		return fmt.Errorf("too many recoveries %d, max %d", count, SectorsMax)
	}
	return nil
}

// Removes sectors from recoveries, if present.
func (st *State) RemoveRecoveries(sectorNos *abi.BitField) (err error) {
	empty, err := sectorNos.IsEmpty()
	if err != nil {
		return err
	}
	if empty {
		return nil
	}
	st.Recoveries, err = bitfield.SubtractBitField(st.Recoveries, sectorNos)
	return err
}

// Loads sector info for a sequence of sectors.
func (st *State) LoadSectorInfos(store adt.Store, sectors *abi.BitField) ([]*SectorOnChainInfo, error) {
	var sectorInfos []*SectorOnChainInfo
	err := sectors.ForEach(func(i uint64) error {
		sectorOnChain, found, err := st.GetSector(store, abi.SectorNumber(i))
		if err != nil {
			return xerrors.Errorf("failed to load sector %d: %w", i, err)
		} else if !found {
			return fmt.Errorf("can't find sector %d", i)
		}
		sectorInfos = append(sectorInfos, sectorOnChain)
		return nil
	})
	return sectorInfos, err
}

// Loads info for a set of sectors to be proven.
// If any of the sectors are declared faulty and not to be recovered, info for the first non-faulty sector is substituted instead.
// If any of the sectors are declared recovered, they are returned from this method.
func (st *State) LoadSectorInfosForProof(store adt.Store, provenSectors *abi.BitField) (sectorInfos []*SectorOnChainInfo, recoveries *bitfield.BitField, err error) {
	// Extract a fault set relevant to the sectors being submitted, for expansion into a map.
	declaredFaults, err := bitfield.IntersectBitField(provenSectors, st.Faults)
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to intersect proof sectors with faults: %w", err)
	}

	recoveries, err = bitfield.IntersectBitField(declaredFaults, st.Recoveries)
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to intersect recoveries with faults: %w", err)
	}

	expectedFaults, err := bitfield.SubtractBitField(declaredFaults, recoveries)
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to subtract recoveries from faults: %w", err)
	}

	nonFaults, err := bitfield.SubtractBitField(provenSectors, expectedFaults)
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to diff bitfields: %w", err)
	}

	empty, err := nonFaults.IsEmpty()
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to check if bitfield was empty: %w", err)
	}
	if empty {
		return nil, nil, xerrors.Errorf("no non-faulty sectors in partitions: %w", err)
	}

	// Select a non-faulty sector as a substitute for faulty ones.
	goodSectorNo, err := nonFaults.First()
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to get first good sector: %w", err)
	}

	// Load sector infos
	sectorInfos, err = st.LoadSectorInfosWithFaultMask(store, provenSectors, expectedFaults, abi.SectorNumber(goodSectorNo))
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to load sector infos: %w", err)
	}
	return
}

// Loads sector info for a sequence of sectors, substituting info for a stand-in sector for any that are faulty.
func (st *State) LoadSectorInfosWithFaultMask(store adt.Store, sectors *abi.BitField, faults *abi.BitField, faultStandIn abi.SectorNumber) ([]*SectorOnChainInfo, error) {
	standInInfo, found, err := st.GetSector(store, faultStandIn)
	if err != nil {
		return nil, fmt.Errorf("failed to load stand-in sector %d: %v", faultStandIn, err)
	} else if !found {
		return nil, fmt.Errorf("can't find stand-in sector %d", faultStandIn)
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
			sectorOnChain, found, err := st.GetSector(store, abi.SectorNumber(i))
			if err != nil {
				return xerrors.Errorf("failed to load sector %d: %w", i, err)
			} else if !found {
				return fmt.Errorf("can't find sector %d", i)
			}
			sector = sectorOnChain
		}
		sectorInfos = append(sectorInfos, sector)
		return nil
	})
	return sectorInfos, err
}

// Adds partition numbers to the set of PoSt submissions
func (st *State) AddPoStSubmissions(partitionNos *abi.BitField) (err error) {
	st.PostSubmissions, err = bitfield.MergeBitFields(st.PostSubmissions, partitionNos)
	return err
}

// Removes all PoSt submissions
func (st *State) ClearPoStSubmissions() error {
	st.PostSubmissions = abi.NewBitField()
	return nil
}

func (st *State) LoadDeadlines(store adt.Store) (*Deadlines, error) {
	var deadlines Deadlines
	if err := store.Get(store.Context(), st.Deadlines, &deadlines); err != nil {
		return nil, fmt.Errorf("failed to load deadlines (%s): %w", st.Deadlines, err)
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

//
// PoSt Deadlines and partitions
//

type Deadlines struct {
	// A bitfield of sector numbers due at each deadline.
	// The sectors for each deadline are logically grouped into sequential partitions for proving.
	Due [WPoStPeriodDeadlines]*abi.BitField
}

func ConstructDeadlines() *Deadlines {
	d := &Deadlines{Due: [WPoStPeriodDeadlines]*abi.BitField{}}
	for i := range d.Due {
		d.Due[i] = abi.NewBitField()
	}
	return d
}

// Adds sector numbers to a deadline.
// The sector numbers are given as uint64 to avoid pointless conversions for bitfield use.
func (d *Deadlines) AddToDeadline(deadline uint64, newSectors ...uint64) (err error) {
	ns := bitfield.NewFromSet(newSectors)
	d.Due[deadline], err = bitfield.MergeBitFields(d.Due[deadline], ns)
	return err
}

// Removes sector numbers from all deadlines.
func (d *Deadlines) RemoveFromAllDeadlines(sectorNos *abi.BitField) (err error) {
	for i := range d.Due {
		d.Due[i], err = bitfield.SubtractBitField(d.Due[i], sectorNos)
		if err != nil {
			return err
		}
	}
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
	AssertMsg(newTotal.GreaterThanEqual(big.Zero()), "negative initial pledge %s after adding %s to prior %s",
		newTotal, amount, st.InitialPledgeRequirement)
	st.InitialPledgeRequirement = newTotal
}

func (st *State) AddLockedFunds(store adt.Store, currEpoch abi.ChainEpoch, vestingSum abi.TokenAmount, spec *VestSpec) error {
	AssertMsg(vestingSum.GreaterThanEqual(big.Zero()), "negative vesting sum %s", vestingSum)
	vestingFunds, err := adt.AsArray(store, st.VestingFunds)
	if err != nil {
		return err
	}

	// Nothing unlocks here, this is just the start of the clock.
	vestBegin := currEpoch + spec.InitialDelay
	vestPeriod := big.NewInt(int64(spec.VestPeriod))
	vestedSoFar := big.Zero()
	for e := vestBegin + spec.StepDuration; vestedSoFar.LessThan(vestingSum); e += spec.StepDuration {
		vestEpoch := quantizeUp(e, spec.Quantization, st.ProvingPeriodStart)
		elapsed := vestEpoch - vestBegin

		targetVest := big.Zero() //nolint:ineffassign
		if elapsed < spec.VestPeriod {
			// Linear vesting, PARAM_FINISH
			targetVest = big.Div(big.Mul(vestingSum, big.NewInt(int64(elapsed))), vestPeriod)
		} else {
			targetVest = vestingSum
		}

		vestThisTime := big.Sub(targetVest, vestedSoFar)
		vestedSoFar = targetVest

		// Load existing entry, else set a new one
		key := EpochKey(vestEpoch)
		lockedFundEntry := big.Zero()
		_, err = vestingFunds.Get(key, &lockedFundEntry)
		if err != nil {
			return err
		}

		lockedFundEntry = big.Add(lockedFundEntry, vestThisTime)
		err = vestingFunds.Set(key, &lockedFundEntry)
		if err != nil {
			return err
		}
	}

	st.VestingFunds, err = vestingFunds.Root()
	if err != nil {
		return err
	}
	st.LockedFunds = big.Add(st.LockedFunds, vestingSum)

	return nil
}

// Unlocks an amount of funds that have *not yet vested*, if possible.
// The soonest-vesting entries are unlocked first.
// Returns the amount actually unlocked.
func (st *State) UnlockUnvestedFunds(store adt.Store, currEpoch abi.ChainEpoch, target abi.TokenAmount) (abi.TokenAmount, error) {
	vestingFunds, err := adt.AsArray(store, st.VestingFunds)
	if err != nil {
		return big.Zero(), err
	}

	amountUnlocked := abi.NewTokenAmount(0)
	lockedEntry := abi.NewTokenAmount(0)
	var toDelete []uint64
	var finished = fmt.Errorf("finished")

	// Iterate vestingFunds are in order of release.
	err = vestingFunds.ForEach(&lockedEntry, func(k int64) error {
		if amountUnlocked.LessThan(target) {
			if k >= int64(currEpoch) {
				unlockAmount := big.Min(big.Sub(target, amountUnlocked), lockedEntry)
				amountUnlocked = big.Add(amountUnlocked, unlockAmount)
				lockedEntry = big.Sub(lockedEntry, unlockAmount)

				if lockedEntry.IsZero() {
					toDelete = append(toDelete, uint64(k))
				} else {
					if err = vestingFunds.Set(uint64(k), &lockedEntry); err != nil {
						return err
					}
				}
			}
			return nil
		} else {
			return finished
		}
	})

	if err != nil && err != finished {
		return big.Zero(), err
	}

	err = deleteMany(vestingFunds, toDelete)
	if err != nil {
		return big.Zero(), errors.Wrapf(err, "failed to delete locked fund during slash: %v", err)
	}

	st.LockedFunds = big.Sub(st.LockedFunds, amountUnlocked)
	Assert(st.LockedFunds.GreaterThanEqual(big.Zero()))
	st.VestingFunds, err = vestingFunds.Root()
	if err != nil {
		return big.Zero(), err
	}

	return amountUnlocked, nil
}

// Unlocks all vesting funds that have vested before the provided epoch.
// Returns the amount unlocked.
func (st *State) UnlockVestedFunds(store adt.Store, currEpoch abi.ChainEpoch) (abi.TokenAmount, error) {
	vestingFunds, err := adt.AsArray(store, st.VestingFunds)
	if err != nil {
		return big.Zero(), err
	}

	amountUnlocked := abi.NewTokenAmount(0)
	lockedEntry := abi.NewTokenAmount(0)
	var toDelete []uint64
	var finished = fmt.Errorf("finished")

	// Iterate vestingFunds  in order of release.
	err = vestingFunds.ForEach(&lockedEntry, func(k int64) error {
		if k < int64(currEpoch) {
			amountUnlocked = big.Add(amountUnlocked, lockedEntry)
			toDelete = append(toDelete, uint64(k))
		} else {
			return finished // stop iterating
		}
		return nil
	})

	if err != nil && err != finished {
		return big.Zero(), err
	}

	err = deleteMany(vestingFunds, toDelete)
	if err != nil {
		return big.Zero(), errors.Wrapf(err, "failed to delete locked fund during vest: %v", err)
	}

	st.LockedFunds = big.Sub(st.LockedFunds, amountUnlocked)
	Assert(st.LockedFunds.GreaterThanEqual(big.Zero()))
	st.VestingFunds, err = vestingFunds.Root()
	if err != nil {
		return big.Zero(), err
	}

	return amountUnlocked, nil
}

// CheckVestedFunds returns the amount of vested funds that have vested before the provided epoch.
func (st *State) CheckVestedFunds(store adt.Store, currEpoch abi.ChainEpoch) (abi.TokenAmount, error) {
	vestingFunds, err := adt.AsArray(store, st.VestingFunds)
	if err != nil {
		return big.Zero(), err
	}

	amountUnlocked := abi.NewTokenAmount(0)
	lockedEntry := abi.NewTokenAmount(0)
	var finished = fmt.Errorf("finished")

	// Iterate vestingFunds  in order of release.
	err = vestingFunds.ForEach(&lockedEntry, func(k int64) error {
		if k < int64(currEpoch) {
			amountUnlocked = big.Add(amountUnlocked, lockedEntry)
		} else {
			return finished // stop iterating
		}
		return nil
	})
	if err != nil && err != finished {
		return big.Zero(), err
	}

	return amountUnlocked, nil
}

func (st *State) GetAvailableBalance(actorBalance abi.TokenAmount) abi.TokenAmount {
	availableBal := big.Sub(big.Sub(actorBalance, st.LockedFunds), st.PreCommitDeposits)
	Assert(availableBal.GreaterThanEqual(big.Zero()))
	return availableBal
}

func (st *State) AssertBalanceInvariants(balance abi.TokenAmount) {
	Assert(st.PreCommitDeposits.GreaterThanEqual(big.Zero()))
	Assert(st.LockedFunds.GreaterThanEqual(big.Zero()))
	Assert(balance.GreaterThanEqual(big.Add(st.PreCommitDeposits, st.LockedFunds)))
}

//
// Sectors
//

func (s *SectorOnChainInfo) AsSectorInfo() abi.SectorInfo {
	return abi.SectorInfo{
		SealProof:    s.SealProof,
		SectorNumber: s.SectorNumber,
		SealedCID:    s.SealedCID,
	}
}

//
// Misc helpers
//

func deleteMany(arr *adt.Array, keys []uint64) error {
	// If AMT exposed a batch delete we could save some writes here.
	for _, i := range keys {
		err := arr.Delete(i)
		if err != nil {
			return err
		}
	}
	return nil
}

// Rounds e to the nearest exact multiple of the quantization unit offset by
// offsetSeed % unit, rounding up.
// This function is equivalent to `unit * ceil(e - (offsetSeed % unit) / unit) + (offsetSeed % unit)`
// with the variables/operations are over real numbers instead of ints.
// Precondition: unit >= 0 else behaviour is undefined
func quantizeUp(e abi.ChainEpoch, unit abi.ChainEpoch, offsetSeed abi.ChainEpoch) abi.ChainEpoch {
	offset := offsetSeed % unit

	remainder := (e - offset) % unit
	quotient := (e - offset) / unit
	// Don't round if epoch falls on a quantization epoch
	if remainder == 0 {
		return unit*quotient + offset
	}
	// Negative truncating division rounds up
	if e-offset < 0 {
		return unit*quotient + offset
	}
	return unit*(quotient+1) + offset

}

func SectorKey(e abi.SectorNumber) adt.Keyer {
	return adt.UIntKey(uint64(e))
}

func EpochKey(e abi.ChainEpoch) uint64 {
	return uint64(e)
}

func init() {
	// Check that ChainEpoch is indeed an unsigned integer to confirm that SectorKey is making the right interpretation.
	var e abi.SectorNumber
	if reflect.TypeOf(e).Kind() != reflect.Uint64 {
		panic("incorrect sector number encoding")
	}
}
