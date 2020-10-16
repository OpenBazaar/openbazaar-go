package miner

import (
	"errors"

	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	xc "github.com/filecoin-project/go-state-types/exitcode"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/specs-actors/v2/actors/util"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

type Partition struct {
	// Sector numbers in this partition, including faulty, unproven, and terminated sectors.
	Sectors bitfield.BitField
	// Unproven sectors in this partition. This bitfield will be cleared on
	// a successful window post (or at the end of the partition's next
	// deadline). At that time, any still unproven sectors will be added to
	// the faulty sector bitfield.
	Unproven bitfield.BitField
	// Subset of sectors detected/declared faulty and not yet recovered (excl. from PoSt).
	// Faults ∩ Terminated = ∅
	Faults bitfield.BitField
	// Subset of faulty sectors expected to recover on next PoSt
	// Recoveries ∩ Terminated = ∅
	Recoveries bitfield.BitField
	// Subset of sectors terminated but not yet removed from partition (excl. from PoSt)
	Terminated bitfield.BitField
	// Maps epochs sectors that expire in or before that epoch.
	// An expiration may be an "on-time" scheduled expiration, or early "faulty" expiration.
	// Keys are quantized to last-in-deadline epochs.
	ExpirationsEpochs cid.Cid // AMT[ChainEpoch]ExpirationSet
	// Subset of terminated that were before their committed expiration epoch, by termination epoch.
	// Termination fees have not yet been calculated or paid and associated deals have not yet been
	// canceled but effective power has already been adjusted.
	// Not quantized.
	EarlyTerminated cid.Cid // AMT[ChainEpoch]BitField

	// Power of not-yet-terminated sectors (incl faulty & unproven).
	LivePower PowerPair
	// Power of yet-to-be-proved sectors (never faulty).
	UnprovenPower PowerPair
	// Power of currently-faulty sectors. FaultyPower <= LivePower.
	FaultyPower PowerPair
	// Power of expected-to-recover sectors. RecoveringPower <= FaultyPower.
	RecoveringPower PowerPair
}

// Value type for a pair of raw and QA power.
type PowerPair struct {
	Raw abi.StoragePower
	QA  abi.StoragePower
}

// A set of sectors associated with a given epoch.
func ConstructPartition(emptyArray cid.Cid) *Partition {
	return &Partition{
		Sectors:           bitfield.New(),
		Unproven:          bitfield.New(),
		Faults:            bitfield.New(),
		Recoveries:        bitfield.New(),
		Terminated:        bitfield.New(),
		ExpirationsEpochs: emptyArray,
		EarlyTerminated:   emptyArray,
		LivePower:         NewPowerPairZero(),
		UnprovenPower:     NewPowerPairZero(),
		FaultyPower:       NewPowerPairZero(),
		RecoveringPower:   NewPowerPairZero(),
	}
}

// Live sectors are those that are not terminated (but may be faulty).
func (p *Partition) LiveSectors() (bitfield.BitField, error) {
	live, err := bitfield.SubtractBitField(p.Sectors, p.Terminated)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("failed to compute live sectors: %w", err)
	}
	return live, nil

}

// Active sectors are those that are neither terminated nor faulty nor unproven, i.e. actively contributing power.
func (p *Partition) ActiveSectors() (bitfield.BitField, error) {
	live, err := p.LiveSectors()
	if err != nil {
		return bitfield.BitField{}, err
	}
	nonFaulty, err := bitfield.SubtractBitField(live, p.Faults)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("failed to compute active sectors: %w", err)
	}
	active, err := bitfield.SubtractBitField(nonFaulty, p.Unproven)
	if err != nil {
		return bitfield.BitField{}, xerrors.Errorf("failed to compute active sectors: %w", err)
	}
	return active, err
}

// Active power is power of non-faulty sectors.
func (p *Partition) ActivePower() PowerPair {
	return p.LivePower.Sub(p.FaultyPower).Sub(p.UnprovenPower)
}

// AddSectors adds new sectors to the partition.
// The sectors are "live", neither faulty, recovering, nor terminated.
// If proven is true, the sectors are assumed to have already been proven.
// Each new sector's expiration is scheduled shortly after its target expiration epoch.
func (p *Partition) AddSectors(
	store adt.Store, proven bool, sectors []*SectorOnChainInfo, ssize abi.SectorSize, quant QuantSpec,
) (powerDelta PowerPair, err error) {
	expirations, err := LoadExpirationQueue(store, p.ExpirationsEpochs, quant)
	if err != nil {
		return NewPowerPairZero(), xerrors.Errorf("failed to load sector expirations: %w", err)
	}
	snos, power, _, err := expirations.AddActiveSectors(sectors, ssize)
	if err != nil {
		return NewPowerPairZero(), xerrors.Errorf("failed to record new sector expirations: %w", err)
	}
	if p.ExpirationsEpochs, err = expirations.Root(); err != nil {
		return NewPowerPairZero(), xerrors.Errorf("failed to store sector expirations: %w", err)
	}

	if contains, err := util.BitFieldContainsAny(p.Sectors, snos); err != nil {
		return NewPowerPairZero(), xerrors.Errorf("failed to check if any new sector was already in the partition: %w", err)
	} else if contains {
		return NewPowerPairZero(), xerrors.Errorf("not all added sectors are new")
	}

	// Update other metadata using the calculated totals.
	if p.Sectors, err = bitfield.MergeBitFields(p.Sectors, snos); err != nil {
		return NewPowerPairZero(), xerrors.Errorf("failed to record new sector numbers: %w", err)
	}
	p.LivePower = p.LivePower.Add(power)

	provenPower := power
	if !proven {
		p.UnprovenPower = p.UnprovenPower.Add(power)
		if p.Unproven, err = bitfield.MergeBitFields(p.Unproven, snos); err != nil {
			return NewPowerPairZero(), xerrors.Errorf("failed to update unproven sectors bitfield: %w", err)
		}
		// Only return the power if proven. Otherwise, it'll "go live" on the next window PoSt.
		provenPower = NewPowerPairZero()
	}

	// check invariants
	if err := p.ValidateState(); err != nil {
		return NewPowerPairZero(), err
	}

	// No change to faults, recoveries, or terminations.
	// No change to faulty or recovering power.
	return provenPower, nil
}

// marks a set of sectors faulty
func (p *Partition) addFaults(
	store adt.Store, sectorNos bitfield.BitField, sectors []*SectorOnChainInfo, faultExpiration abi.ChainEpoch,
	ssize abi.SectorSize, quant QuantSpec,
) (powerDelta, newFaultyPower PowerPair, err error) {
	// Load expiration queue
	queue, err := LoadExpirationQueue(store, p.ExpirationsEpochs, quant)
	if err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), xerrors.Errorf("failed to load partition queue: %w", err)
	}

	// Reschedule faults
	newFaultyPower, err = queue.RescheduleAsFaults(faultExpiration, sectors, ssize)
	if err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), xerrors.Errorf("failed to add faults to partition queue: %w", err)
	}

	// Save expiration queue
	if p.ExpirationsEpochs, err = queue.Root(); err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), err
	}

	// Update partition metadata
	if p.Faults, err = bitfield.MergeBitFields(p.Faults, sectorNos); err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), err
	}

	// The sectors must not have been previously faulty or recovering.
	// No change to recoveries or terminations.
	p.FaultyPower = p.FaultyPower.Add(newFaultyPower)

	// Once marked faulty, sectors are moved out of the unproven set.
	unproven, err := bitfield.IntersectBitField(sectorNos, p.Unproven)
	if err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), xerrors.Errorf("failed to intersect faulty sector IDs with unproven sector IDs: %w", err)
	}
	p.Unproven, err = bitfield.SubtractBitField(p.Unproven, unproven)
	if err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), xerrors.Errorf("failed to subtract faulty sectors from unproven sector IDs: %w", err)
	}

	powerDelta = newFaultyPower.Neg()
	if unprovenInfos, err := selectSectors(sectors, unproven); err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), xerrors.Errorf("failed to select unproven sectors: %w", err)
	} else if len(unprovenInfos) > 0 {
		lostUnprovenPower := PowerForSectors(ssize, unprovenInfos)
		p.UnprovenPower = p.UnprovenPower.Sub(lostUnprovenPower)
		powerDelta = powerDelta.Add(lostUnprovenPower)
	}

	// check invariants
	if err := p.ValidateState(); err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), err
	}

	// No change to live or recovering power.
	return powerDelta, newFaultyPower, nil
}

// Declares a set of sectors faulty. Already faulty sectors are ignored,
// terminated sectors are skipped, and recovering sectors are reverted to
// faulty.
//
// - New faults are added to the Faults bitfield and the FaultyPower is increased.
// - The sectors' expirations are rescheduled to the fault expiration epoch, as "early" (if not expiring earlier).
//
// Returns the power of the now-faulty sectors.
func (p *Partition) DeclareFaults(
	store adt.Store, sectors Sectors, sectorNos bitfield.BitField, faultExpirationEpoch abi.ChainEpoch,
	ssize abi.SectorSize, quant QuantSpec,
) (newFaults bitfield.BitField, powerDelta, newFaultyPower PowerPair, err error) {
	err = validatePartitionContainsSectors(p, sectorNos)
	if err != nil {
		return bitfield.BitField{}, NewPowerPairZero(), NewPowerPairZero(), xc.ErrIllegalArgument.Wrapf("failed fault declaration: %w", err)
	}

	// Split declarations into declarations of new faults, and retraction of declared recoveries.
	retractedRecoveries, err := bitfield.IntersectBitField(p.Recoveries, sectorNos)
	if err != nil {
		return bitfield.BitField{}, NewPowerPairZero(), NewPowerPairZero(), xerrors.Errorf("failed to intersect sectors with recoveries: %w", err)
	}

	newFaults, err = bitfield.SubtractBitField(sectorNos, retractedRecoveries)
	if err != nil {
		return bitfield.BitField{}, NewPowerPairZero(), NewPowerPairZero(), xerrors.Errorf("failed to subtract recoveries from sectors: %w", err)
	}

	// Ignore any terminated sectors and previously declared or detected faults
	newFaults, err = bitfield.SubtractBitField(newFaults, p.Terminated)
	if err != nil {
		return bitfield.BitField{}, NewPowerPairZero(), NewPowerPairZero(), xerrors.Errorf("failed to subtract terminations from faults: %w", err)
	}
	newFaults, err = bitfield.SubtractBitField(newFaults, p.Faults)
	if err != nil {
		return bitfield.BitField{}, NewPowerPairZero(), NewPowerPairZero(), xerrors.Errorf("failed to subtract existing faults from faults: %w", err)
	}

	// Add new faults to state.
	newFaultyPower = NewPowerPairZero()
	powerDelta = NewPowerPairZero()
	if newFaultSectors, err := sectors.Load(newFaults); err != nil {
		return bitfield.BitField{}, NewPowerPairZero(), NewPowerPairZero(), xerrors.Errorf("failed to load fault sectors: %w", err)
	} else if len(newFaultSectors) > 0 {
		powerDelta, newFaultyPower, err = p.addFaults(store, newFaults, newFaultSectors, faultExpirationEpoch, ssize, quant)
		if err != nil {
			return bitfield.BitField{}, NewPowerPairZero(), NewPowerPairZero(), xerrors.Errorf("failed to add faults: %w", err)
		}
	}

	// Remove faulty recoveries from state.
	if retractedRecoverySectors, err := sectors.Load(retractedRecoveries); err != nil {
		return bitfield.BitField{}, NewPowerPairZero(), NewPowerPairZero(), xerrors.Errorf("failed to load recovery sectors: %w", err)
	} else if len(retractedRecoverySectors) > 0 {
		retractedRecoveryPower := PowerForSectors(ssize, retractedRecoverySectors)
		err = p.removeRecoveries(retractedRecoveries, retractedRecoveryPower)
		if err != nil {
			return bitfield.BitField{}, NewPowerPairZero(), NewPowerPairZero(), xerrors.Errorf("failed to remove recoveries: %w", err)
		}
	}

	// check invariants
	if err := p.ValidateState(); err != nil {
		return bitfield.BitField{}, NewPowerPairZero(), NewPowerPairZero(), err
	}

	return newFaults, powerDelta, newFaultyPower, nil
}

// Removes sector numbers from faults and thus from recoveries.
// The sectors are removed from the Faults and Recovering bitfields, and FaultyPower and RecoveringPower reduced.
// The sectors are re-scheduled for expiration shortly after their target expiration epoch.
// Returns the power of the now-recovered sectors.
func (p *Partition) RecoverFaults(store adt.Store, sectors Sectors, ssize abi.SectorSize, quant QuantSpec) (PowerPair, error) {
	// Process recoveries, assuming the proof will be successful.
	// This similarly updates state.
	recoveredSectors, err := sectors.Load(p.Recoveries)
	if err != nil {
		return NewPowerPairZero(), xerrors.Errorf("failed to load recovered sectors: %w", err)
	}
	// Load expiration queue
	queue, err := LoadExpirationQueue(store, p.ExpirationsEpochs, quant)
	if err != nil {
		return NewPowerPairZero(), xerrors.Errorf("failed to load partition queue: %w", err)
	}
	// Reschedule recovered
	power, err := queue.RescheduleRecovered(recoveredSectors, ssize)
	if err != nil {
		return NewPowerPairZero(), xerrors.Errorf("failed to reschedule faults in partition queue: %w", err)
	}
	// Save expiration queue
	if p.ExpirationsEpochs, err = queue.Root(); err != nil {
		return NewPowerPairZero(), err
	}

	// Update partition metadata
	if newFaults, err := bitfield.SubtractBitField(p.Faults, p.Recoveries); err != nil {
		return NewPowerPairZero(), err
	} else {
		p.Faults = newFaults
	}
	p.Recoveries = bitfield.New()

	// No change to live power.
	// No change to unproven sectors.
	p.FaultyPower = p.FaultyPower.Sub(power)
	p.RecoveringPower = p.RecoveringPower.Sub(power)

	// check invariants
	if err := p.ValidateState(); err != nil {
		return NewPowerPairZero(), err
	}

	return power, err
}

// Activates unproven sectors, returning the activated power.
func (p *Partition) ActivateUnproven() PowerPair {
	newPower := p.UnprovenPower
	p.UnprovenPower = NewPowerPairZero()
	p.Unproven = bitfield.New()
	return newPower
}

// Declares sectors as recovering. Non-faulty and already recovering sectors will be skipped.
func (p *Partition) DeclareFaultsRecovered(sectors Sectors, ssize abi.SectorSize, sectorNos bitfield.BitField) (err error) {
	// Check that the declared sectors are actually assigned to the partition.
	err = validatePartitionContainsSectors(p, sectorNos)
	if err != nil {
		return xc.ErrIllegalArgument.Wrapf("failed fault declaration: %w", err)
	}

	// Ignore sectors not faulty or already declared recovered
	recoveries, err := bitfield.IntersectBitField(sectorNos, p.Faults)
	if err != nil {
		return xerrors.Errorf("failed to intersect recoveries with faults: %w", err)
	}
	recoveries, err = bitfield.SubtractBitField(recoveries, p.Recoveries)
	if err != nil {
		return xerrors.Errorf("failed to subtract existing recoveries: %w", err)
	}

	// Record the new recoveries for processing at Window PoSt or deadline cron.
	recoverySectors, err := sectors.Load(recoveries)
	if err != nil {
		return xerrors.Errorf("failed to load recovery sectors: %w", err)
	}

	p.Recoveries, err = bitfield.MergeBitFields(p.Recoveries, recoveries)
	if err != nil {
		return err
	}

	power := PowerForSectors(ssize, recoverySectors)
	p.RecoveringPower = p.RecoveringPower.Add(power)

	// check invariants
	if err := p.ValidateState(); err != nil {
		return err
	}

	// No change to faults, or terminations.
	// No change to faulty power.
	// No change to unproven power/sectors.
	return nil
}

// Removes sectors from recoveries and recovering power. Assumes sectors are currently faulty and recovering..
func (p *Partition) removeRecoveries(sectorNos bitfield.BitField, power PowerPair) (err error) {
	empty, err := sectorNos.IsEmpty()
	if err != nil {
		return err
	}
	if empty {
		return nil
	}
	p.Recoveries, err = bitfield.SubtractBitField(p.Recoveries, sectorNos)
	if err != nil {
		return err
	}
	p.RecoveringPower = p.RecoveringPower.Sub(power)
	// No change to faults, or terminations.
	// No change to faulty power.
	// No change to unproven or unproven power.
	return nil
}

// RescheduleExpirations moves expiring sectors to the target expiration,
// skipping any sectors it can't find.
//
// The power of the rescheduled sectors is assumed to have not changed since
// initial scheduling.
//
// Note: see the docs on State.RescheduleSectorExpirations for details on why we
// skip sectors/partitions we can't find.
func (p *Partition) RescheduleExpirations(
	store adt.Store, sectors Sectors,
	newExpiration abi.ChainEpoch, sectorNos bitfield.BitField,
	ssize abi.SectorSize, quant QuantSpec,
) (replaced []*SectorOnChainInfo, err error) {
	// Ensure these sectors actually belong to this partition.
	present, err := bitfield.IntersectBitField(sectorNos, p.Sectors)
	if err != nil {
		return nil, err
	}

	// Filter out terminated sectors.
	live, err := bitfield.SubtractBitField(present, p.Terminated)
	if err != nil {
		return nil, err
	}

	// Filter out faulty sectors.
	active, err := bitfield.SubtractBitField(live, p.Faults)
	if err != nil {
		return nil, err
	}

	sectorInfos, err := sectors.Load(active)
	if err != nil {
		return nil, err
	}

	expirations, err := LoadExpirationQueue(store, p.ExpirationsEpochs, quant)
	if err != nil {
		return nil, xerrors.Errorf("failed to load sector expirations: %w", err)
	}
	if err = expirations.RescheduleExpirations(newExpiration, sectorInfos, ssize); err != nil {
		return nil, err
	}
	p.ExpirationsEpochs, err = expirations.Root()
	if err != nil {
		return nil, err
	}

	// check invariants
	if err := p.ValidateState(); err != nil {
		return nil, err
	}

	return sectorInfos, nil
}

// Replaces a number of "old" sectors with new ones.
// The old sectors must not be faulty, terminated, or unproven.
// If the same sector is both removed and added, this permits rescheduling *with a change in power*,
// unlike RescheduleExpirations.
// Returns the delta to power and pledge requirement.
func (p *Partition) ReplaceSectors(store adt.Store, oldSectors, newSectors []*SectorOnChainInfo,
	ssize abi.SectorSize, quant QuantSpec) (PowerPair, abi.TokenAmount, error) {
	expirations, err := LoadExpirationQueue(store, p.ExpirationsEpochs, quant)
	if err != nil {
		return NewPowerPairZero(), big.Zero(), xerrors.Errorf("failed to load sector expirations: %w", err)
	}
	oldSnos, newSnos, powerDelta, pledgeDelta, err := expirations.ReplaceSectors(oldSectors, newSectors, ssize)
	if err != nil {
		return NewPowerPairZero(), big.Zero(), xerrors.Errorf("failed to replace sector expirations: %w", err)
	}
	if p.ExpirationsEpochs, err = expirations.Root(); err != nil {
		return NewPowerPairZero(), big.Zero(), xerrors.Errorf("failed to save sector expirations: %w", err)
	}

	// Check the sectors being removed are active (alive, not faulty).
	active, err := p.ActiveSectors()
	if err != nil {
		return NewPowerPairZero(), big.Zero(), err
	}
	allActive, err := util.BitFieldContainsAll(active, oldSnos)
	if err != nil {
		return NewPowerPairZero(), big.Zero(), xerrors.Errorf("failed to check for active sectors: %w", err)
	} else if !allActive {
		return NewPowerPairZero(), big.Zero(), xerrors.Errorf("refusing to replace inactive sectors in %v (active: %v)", oldSnos, active)
	}

	// Update partition metadata.
	if p.Sectors, err = bitfield.SubtractBitField(p.Sectors, oldSnos); err != nil {
		return NewPowerPairZero(), big.Zero(), xerrors.Errorf("failed to remove replaced sectors: %w", err)
	}
	if p.Sectors, err = bitfield.MergeBitFields(p.Sectors, newSnos); err != nil {
		return NewPowerPairZero(), big.Zero(), xerrors.Errorf("failed to add replaced sectors: %w", err)
	}
	p.LivePower = p.LivePower.Add(powerDelta)

	// check invariants
	if err := p.ValidateState(); err != nil {
		return NewPowerPairZero(), big.Zero(), err
	}

	// No change to faults, recoveries, or terminations.
	// No change to faulty or recovering power.
	return powerDelta, pledgeDelta, nil
}

// Record the epoch of any sectors expiring early, for termination fee calculation later.
func (p *Partition) recordEarlyTermination(store adt.Store, epoch abi.ChainEpoch, sectors bitfield.BitField) error {
	etQueue, err := LoadBitfieldQueue(store, p.EarlyTerminated, NoQuantization)
	if err != nil {
		return xerrors.Errorf("failed to load early termination queue: %w", err)
	}
	if err = etQueue.AddToQueue(epoch, sectors); err != nil {
		return xerrors.Errorf("failed to add to early termination queue: %w", err)
	}
	if p.EarlyTerminated, err = etQueue.Root(); err != nil {
		return xerrors.Errorf("failed to save early termination queue: %w", err)
	}
	return nil
}

// Marks a collection of sectors as terminated.
// The sectors are removed from Faults and Recoveries.
// The epoch of termination is recorded for future termination fee calculation.
func (p *Partition) TerminateSectors(
	store adt.Store, sectors Sectors, epoch abi.ChainEpoch, sectorNos bitfield.BitField,
	ssize abi.SectorSize, quant QuantSpec) (*ExpirationSet, error) {
	liveSectors, err := p.LiveSectors()
	if err != nil {
		return nil, err
	}
	if contains, err := util.BitFieldContainsAll(liveSectors, sectorNos); err != nil {
		return nil, xc.ErrIllegalArgument.Wrapf("failed to intersect live sectors with terminating sectors: %w", err)
	} else if !contains {
		return nil, xc.ErrIllegalArgument.Wrapf("can only terminate live sectors")
	}

	sectorInfos, err := sectors.Load(sectorNos)
	if err != nil {
		return nil, err
	}
	expirations, err := LoadExpirationQueue(store, p.ExpirationsEpochs, quant)
	if err != nil {
		return nil, xerrors.Errorf("failed to load sector expirations: %w", err)
	}
	removed, removedRecovering, err := expirations.RemoveSectors(sectorInfos, p.Faults, p.Recoveries, ssize)
	if err != nil {
		return nil, xerrors.Errorf("failed to remove sector expirations: %w", err)
	}
	if p.ExpirationsEpochs, err = expirations.Root(); err != nil {
		return nil, xerrors.Errorf("failed to save sector expirations: %w", err)
	}

	removedSectors, err := bitfield.MergeBitFields(removed.OnTimeSectors, removed.EarlySectors)
	if err != nil {
		return nil, err
	}

	// Record early termination.
	err = p.recordEarlyTermination(store, epoch, removedSectors)
	if err != nil {
		return nil, xerrors.Errorf("failed to record early sector termination: %w", err)
	}

	unprovenNos, err := bitfield.IntersectBitField(removedSectors, p.Unproven)
	if err != nil {
		return nil, xerrors.Errorf("failed to determine unproven sectors: %w", err)
	}

	// Update partition metadata.
	if p.Faults, err = bitfield.SubtractBitField(p.Faults, removedSectors); err != nil {
		return nil, xerrors.Errorf("failed to remove terminated sectors from faults: %w", err)
	}
	if p.Recoveries, err = bitfield.SubtractBitField(p.Recoveries, removedSectors); err != nil {
		return nil, xerrors.Errorf("failed to remove terminated sectors from recoveries: %w", err)
	}
	if p.Terminated, err = bitfield.MergeBitFields(p.Terminated, removedSectors); err != nil {
		return nil, xerrors.Errorf("failed to add terminated sectors: %w", err)
	}
	if p.Unproven, err = bitfield.SubtractBitField(p.Unproven, unprovenNos); err != nil {
		return nil, xerrors.Errorf("failed to remove unproven sectors: %w", err)
	}

	p.LivePower = p.LivePower.Sub(removed.ActivePower).Sub(removed.FaultyPower)
	p.FaultyPower = p.FaultyPower.Sub(removed.FaultyPower)
	p.RecoveringPower = p.RecoveringPower.Sub(removedRecovering)
	if unprovenInfos, err := selectSectors(sectorInfos, unprovenNos); err != nil {
		return nil, xerrors.Errorf("failed to select unproven sectors: %w", err)
	} else {
		removedUnprovenPower := PowerForSectors(ssize, unprovenInfos)
		p.UnprovenPower = p.UnprovenPower.Sub(removedUnprovenPower)
		removed.ActivePower = removed.ActivePower.Sub(removedUnprovenPower)
	}

	// check invariants
	if err := p.ValidateState(); err != nil {
		return nil, err
	}

	return removed, nil
}

// PopExpiredSectors traverses the expiration queue up to and including some epoch, and marks all expiring
// sectors as terminated.
//
// This cannot be called while there are unproven sectors.
//
// Returns the expired sector aggregates.
func (p *Partition) PopExpiredSectors(store adt.Store, until abi.ChainEpoch, quant QuantSpec) (*ExpirationSet, error) {
	// This is a sanity check to make sure we handle proofs _before_
	// handling sector expirations.
	if noUnproven, err := p.Unproven.IsEmpty(); err != nil {
		return nil, xerrors.Errorf("failed to determine if partition has unproven sectors: %w", err)
	} else if !noUnproven {
		return nil, xerrors.Errorf("cannot pop expired sectors from a partition with unproven sectors: %w", err)
	}

	expirations, err := LoadExpirationQueue(store, p.ExpirationsEpochs, quant)
	if err != nil {
		return nil, xerrors.Errorf("failed to load expiration queue: %w", err)
	}
	popped, err := expirations.PopUntil(until)
	if err != nil {
		return nil, xerrors.Errorf("failed to pop expiration queue until %d: %w", until, err)
	}
	if p.ExpirationsEpochs, err = expirations.Root(); err != nil {
		return nil, err
	}

	expiredSectors, err := bitfield.MergeBitFields(popped.OnTimeSectors, popped.EarlySectors)
	if err != nil {
		return nil, err
	}

	// There shouldn't be any recovering sectors or power if this is invoked at deadline end.
	// Either the partition was PoSted and the recovering became recovered, or the partition was not PoSted
	// and all recoveries retracted.
	// No recoveries may be posted until the deadline is closed.
	noRecoveries, err := p.Recoveries.IsEmpty()
	if err != nil {
		return nil, err
	} else if !noRecoveries {
		return nil, xerrors.Errorf("unexpected recoveries while processing expirations")
	}
	if !p.RecoveringPower.IsZero() {
		return nil, xerrors.Errorf("unexpected recovering power while processing expirations")
	}
	// Nothing expiring now should have already terminated.
	alreadyTerminated, err := util.BitFieldContainsAny(p.Terminated, expiredSectors)
	if err != nil {
		return nil, err
	} else if alreadyTerminated {
		return nil, xerrors.Errorf("expiring sectors already terminated")
	}

	// Mark the sectors as terminated and subtract sector power.
	if p.Terminated, err = bitfield.MergeBitFields(p.Terminated, expiredSectors); err != nil {
		return nil, xerrors.Errorf("failed to merge expired sectors: %w", err)
	}
	if p.Faults, err = bitfield.SubtractBitField(p.Faults, expiredSectors); err != nil {
		return nil, err
	}
	p.LivePower = p.LivePower.Sub(popped.ActivePower.Add(popped.FaultyPower))
	p.FaultyPower = p.FaultyPower.Sub(popped.FaultyPower)

	// Record the epoch of any sectors expiring early, for termination fee calculation later.
	err = p.recordEarlyTermination(store, until, popped.EarlySectors)
	if err != nil {
		return nil, xerrors.Errorf("failed to record early terminations: %w", err)
	}

	// check invariants
	if err := p.ValidateState(); err != nil {
		return nil, err
	}

	return popped, nil
}

// Marks all non-faulty sectors in the partition as faulty and clears recoveries, updating power memos appropriately.
// All sectors' expirations are rescheduled to the fault expiration, as "early" (if not expiring earlier)
// Returns the power delta, power that should be penalized (new faults + failed recoveries), and newly faulty power.
func (p *Partition) RecordMissedPost(
	store adt.Store, faultExpiration abi.ChainEpoch, quant QuantSpec,
) (powerDelta, penalizedPower, newFaultyPower PowerPair, err error) {
	// Collapse tail of queue into the last entry, and mark all power faulty.
	// Load expiration queue
	queue, err := LoadExpirationQueue(store, p.ExpirationsEpochs, quant)
	if err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), xerrors.Errorf("failed to load partition queue: %w", err)
	}
	if err = queue.RescheduleAllAsFaults(faultExpiration); err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), xerrors.Errorf("failed to reschedule all as faults: %w", err)
	}
	// Save expiration queue
	if p.ExpirationsEpochs, err = queue.Root(); err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), err
	}

	// Compute power changes.

	// New faulty power is the total power minus already faulty.
	newFaultyPower = p.LivePower.Sub(p.FaultyPower)
	// Penalized power is the newly faulty power, plus the failed recovery power.
	penalizedPower = p.RecoveringPower.Add(newFaultyPower)
	// The power delta is -(newFaultyPower-unproven), because unproven power
	// was never activated in the first place.
	powerDelta = newFaultyPower.Sub(p.UnprovenPower).Neg()

	// Update partition metadata
	allFaults, err := p.LiveSectors()
	if err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), err
	}
	p.Faults = allFaults
	p.Recoveries = bitfield.New()
	p.Unproven = bitfield.New()
	p.FaultyPower = p.LivePower
	p.RecoveringPower = NewPowerPairZero()
	p.UnprovenPower = NewPowerPairZero()

	// check invariants
	if err := p.ValidateState(); err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), err
	}

	return powerDelta, penalizedPower, newFaultyPower, nil
}

func (p *Partition) PopEarlyTerminations(store adt.Store, maxSectors uint64) (result TerminationResult, hasMore bool, err error) {
	stopErr := errors.New("stop iter")

	// Load early terminations.
	earlyTerminatedQ, err := LoadBitfieldQueue(store, p.EarlyTerminated, NoQuantization)
	if err != nil {
		return TerminationResult{}, false, err
	}

	var (
		processed        []uint64
		hasRemaining     bool
		remainingSectors bitfield.BitField
		remainingEpoch   abi.ChainEpoch
	)

	result.PartitionsProcessed = 1
	result.Sectors = make(map[abi.ChainEpoch]bitfield.BitField)

	if err = earlyTerminatedQ.ForEach(func(epoch abi.ChainEpoch, sectors bitfield.BitField) error {
		toProcess := sectors
		count, err := sectors.Count()
		if err != nil {
			return xerrors.Errorf("failed to count early terminations: %w", err)
		}

		limit := maxSectors - result.SectorsProcessed

		if limit < count {
			toProcess, err = sectors.Slice(0, limit)
			if err != nil {
				return xerrors.Errorf("failed to slice early terminations: %w", err)
			}

			rest, err := bitfield.SubtractBitField(sectors, toProcess)
			if err != nil {
				return xerrors.Errorf("failed to subtract processed early terminations: %w", err)
			}
			hasRemaining = true
			remainingSectors = rest
			remainingEpoch = epoch

			result.SectorsProcessed += limit
		} else {
			processed = append(processed, uint64(epoch))
			result.SectorsProcessed += count
		}

		result.Sectors[epoch] = toProcess

		if result.SectorsProcessed < maxSectors {
			return nil
		}
		return stopErr
	}); err != nil && err != stopErr {
		return TerminationResult{}, false, xerrors.Errorf("failed to walk early terminations queue: %w", err)
	}

	// Update early terminations
	err = earlyTerminatedQ.BatchDelete(processed)
	if err != nil {
		return TerminationResult{}, false, xerrors.Errorf("failed to remove entries from early terminations queue: %w", err)
	}

	if hasRemaining {
		err = earlyTerminatedQ.Set(uint64(remainingEpoch), remainingSectors)
		if err != nil {
			return TerminationResult{}, false, xerrors.Errorf("failed to update remaining entry early terminations queue: %w", err)
		}
	}

	// Save early terminations.
	p.EarlyTerminated, err = earlyTerminatedQ.Root()
	if err != nil {
		return TerminationResult{}, false, xerrors.Errorf("failed to store early terminations queue: %w", err)
	}

	// check invariants
	if err := p.ValidateState(); err != nil {
		return TerminationResult{}, false, err
	}

	return result, earlyTerminatedQ.Length() > 0, nil
}

// Discovers how skipped faults declared during post intersect with existing faults and recoveries, records the
// new faults in state.
// Returns the amount of power newly faulty, or declared recovered but faulty again.
//
// - Skipped faults that are not in the provided partition triggers an error.
// - Skipped faults that are already declared (but not delcared recovered) are ignored.
func (p *Partition) RecordSkippedFaults(
	store adt.Store, sectors Sectors, ssize abi.SectorSize, quant QuantSpec, faultExpiration abi.ChainEpoch, skipped bitfield.BitField,
) (powerDelta, newFaultPower, retractedRecoveryPower PowerPair, hasNewFaults bool, err error) {
	empty, err := skipped.IsEmpty()
	if err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), false, xc.ErrIllegalArgument.Wrapf("failed to check if skipped sectors is empty: %w", err)
	}
	if empty {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), false, nil
	}

	// Check that the declared sectors are actually in the partition.
	contains, err := util.BitFieldContainsAll(p.Sectors, skipped)
	if err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), false, xerrors.Errorf("failed to check if skipped faults are in partition: %w", err)
	} else if !contains {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), false, xc.ErrIllegalArgument.Wrapf("skipped faults contains sectors outside partition")
	}

	// Find all skipped faults that have been labeled recovered
	retractedRecoveries, err := bitfield.IntersectBitField(p.Recoveries, skipped)
	if err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), false, xerrors.Errorf("failed to intersect sectors with recoveries: %w", err)
	}
	retractedRecoverySectors, err := sectors.Load(retractedRecoveries)
	if err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), false, xerrors.Errorf("failed to load sectors: %w", err)
	}
	retractedRecoveryPower = PowerForSectors(ssize, retractedRecoverySectors)

	// Ignore skipped faults that are already faults or terminated.
	newFaults, err := bitfield.SubtractBitField(skipped, p.Terminated)
	if err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), false, xerrors.Errorf("failed to subtract terminations from skipped: %w", err)
	}
	newFaults, err = bitfield.SubtractBitField(newFaults, p.Faults)
	if err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), false, xerrors.Errorf("failed to subtract existing faults from skipped: %w", err)
	}
	newFaultSectors, err := sectors.Load(newFaults)
	if err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), false, xerrors.Errorf("failed to load sectors: %w", err)
	}

	// Record new faults
	powerDelta, newFaultPower, err = p.addFaults(store, newFaults, newFaultSectors, faultExpiration, ssize, quant)
	if err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), false, xerrors.Errorf("failed to add skipped faults: %w", err)
	}

	// Remove faulty recoveries
	err = p.removeRecoveries(retractedRecoveries, retractedRecoveryPower)
	if err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), false, xerrors.Errorf("failed to remove recoveries: %w", err)
	}

	// check invariants
	if err := p.ValidateState(); err != nil {
		return NewPowerPairZero(), NewPowerPairZero(), NewPowerPairZero(), false, err
	}

	return powerDelta, newFaultPower, retractedRecoveryPower, len(newFaultSectors) > 0, nil
}

// Test that invariants about partition power hold
func (p *Partition) ValidatePowerState() error {
	if p.LivePower.Raw.LessThan(big.Zero()) || p.LivePower.QA.LessThan(big.Zero()) {
		return xerrors.Errorf("Partition left with negative live power: %v", p)
	}

	if p.UnprovenPower.Raw.LessThan(big.Zero()) || p.UnprovenPower.QA.LessThan(big.Zero()) {
		return xerrors.Errorf("Partition left with negative unproven power: %v", p)
	}

	if p.FaultyPower.Raw.LessThan(big.Zero()) || p.FaultyPower.QA.LessThan(big.Zero()) {
		return xerrors.Errorf("Partition left with negative faulty power: %v", p)
	}

	if p.RecoveringPower.Raw.LessThan(big.Zero()) || p.RecoveringPower.QA.LessThan(big.Zero()) {
		return xerrors.Errorf("Partition left with negative recovering power: %v", p)
	}

	if p.UnprovenPower.Raw.GreaterThan(p.LivePower.Raw) {
		return xerrors.Errorf("Partition left with invalid unproven power: %v", p)
	}

	if p.FaultyPower.Raw.GreaterThan(p.LivePower.Raw) {
		return xerrors.Errorf("Partition left with invalid faulty power: %v", p)
	}

	if p.RecoveringPower.Raw.GreaterThan(p.LivePower.Raw) || p.RecoveringPower.Raw.GreaterThan(p.FaultyPower.Raw) {
		return xerrors.Errorf("Partition left with invalid recovering power: %v", p)
	}

	return nil
}

// Test that invariants about sector bitfields hold
func (p *Partition) ValidateBFState() error {
	// Merge unproven and faults for checks
	merge, err := bitfield.MultiMerge(p.Unproven, p.Faults)
	if err != nil {
		return err
	}

	// Unproven or faulty sectors should not be in terminated
	if containsAny, err := util.BitFieldContainsAny(p.Terminated, merge); err != nil {
		return err
	} else if containsAny {
		return xerrors.Errorf("Partition left with terminated sectors in multiple states: %v", p)
	}

	// Merge terminated into set for checks
	merge, err = bitfield.MergeBitFields(merge, p.Terminated)
	if err != nil {
		return err
	}

	// All merged sectors should exist in p.Sectors
	if containsAll, err := util.BitFieldContainsAll(p.Sectors, merge); err != nil {
		return err
	} else if !containsAll {
		return xerrors.Errorf("Partition left with invalid sector state: %v", p)
	}

	// All recoveries should exist in p.Faults
	if containsAll, err := util.BitFieldContainsAll(p.Faults, p.Recoveries); err != nil {
		return err
	} else if !containsAll {
		return xerrors.Errorf("Partition left with invalid recovery state: %v", p)
	}

	return nil
}

// Test all invariants hold
func (p *Partition) ValidateState() error {
	var err error
	if err = p.ValidatePowerState(); err != nil {
		return err
	}

	if err = p.ValidateBFState(); err != nil {
		return err
	}

	return nil
}

//
// PowerPair
//

func NewPowerPairZero() PowerPair {
	return NewPowerPair(big.Zero(), big.Zero())
}

func NewPowerPair(raw, qa abi.StoragePower) PowerPair {
	return PowerPair{Raw: raw, QA: qa}
}

func (pp PowerPair) IsZero() bool {
	return pp.Raw.IsZero() && pp.QA.IsZero()
}

func (pp PowerPair) Add(other PowerPair) PowerPair {
	return PowerPair{
		Raw: big.Add(pp.Raw, other.Raw),
		QA:  big.Add(pp.QA, other.QA),
	}
}

func (pp PowerPair) Sub(other PowerPair) PowerPair {
	return PowerPair{
		Raw: big.Sub(pp.Raw, other.Raw),
		QA:  big.Sub(pp.QA, other.QA),
	}
}

func (pp PowerPair) Neg() PowerPair {
	return PowerPair{
		Raw: pp.Raw.Neg(),
		QA:  pp.QA.Neg(),
	}
}

func (pp *PowerPair) Equals(other PowerPair) bool {
	return pp.Raw.Equals(other.Raw) && pp.QA.Equals(other.QA)
}
