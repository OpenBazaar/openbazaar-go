package miner

import (
	"fmt"
	"sort"

	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/specs-actors/v2/actors/util"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

// ExpirationSet is a collection of sector numbers that are expiring, either due to
// expected "on-time" expiration at the end of their life, or unexpected "early" termination
// due to being faulty for too long consecutively.
// Note that there is not a direct correspondence between on-time sectors and active power;
// a sector may be faulty but expiring on-time if it faults just prior to expected termination.
// Early sectors are always faulty, and active power always represents on-time sectors.
type ExpirationSet struct {
	OnTimeSectors bitfield.BitField // Sectors expiring "on time" at the end of their committed life
	EarlySectors  bitfield.BitField // Sectors expiring "early" due to being faulty for too long
	OnTimePledge  abi.TokenAmount   // Pledge total for the on-time sectors
	ActivePower   PowerPair         // Power that is currently active (not faulty)
	FaultyPower   PowerPair         // Power that is currently faulty
}

func NewExpirationSetEmpty() *ExpirationSet {
	return NewExpirationSet(bitfield.New(), bitfield.New(), big.Zero(), NewPowerPairZero(), NewPowerPairZero())
}

func NewExpirationSet(onTimeSectors, earlySectors bitfield.BitField, onTimePledge abi.TokenAmount, activePower, faultyPower PowerPair) *ExpirationSet {
	return &ExpirationSet{
		OnTimeSectors: onTimeSectors,
		EarlySectors:  earlySectors,
		OnTimePledge:  onTimePledge,
		ActivePower:   activePower,
		FaultyPower:   faultyPower,
	}
}

// Adds sectors and power to the expiration set in place.
func (es *ExpirationSet) Add(onTimeSectors, earlySectors bitfield.BitField, onTimePledge abi.TokenAmount, activePower, faultyPower PowerPair) error {
	var err error
	if es.OnTimeSectors, err = bitfield.MergeBitFields(es.OnTimeSectors, onTimeSectors); err != nil {
		return err
	}
	if es.EarlySectors, err = bitfield.MergeBitFields(es.EarlySectors, earlySectors); err != nil {
		return err
	}
	es.OnTimePledge = big.Add(es.OnTimePledge, onTimePledge)
	es.ActivePower = es.ActivePower.Add(activePower)
	es.FaultyPower = es.FaultyPower.Add(faultyPower)
	return es.ValidateState()
}

// Removes sectors and power from the expiration set in place.
func (es *ExpirationSet) Remove(onTimeSectors, earlySectors bitfield.BitField, onTimePledge abi.TokenAmount, activePower, faultyPower PowerPair) error {
	// Check for sector intersection. This could be cheaper with a combined intersection/difference method used below.
	if found, err := util.BitFieldContainsAll(es.OnTimeSectors, onTimeSectors); err != nil {
		return err
	} else if !found {
		return xerrors.Errorf("removing on-time sectors %v not contained in %v", onTimeSectors, es.OnTimeSectors)
	}
	if found, err := util.BitFieldContainsAll(es.EarlySectors, earlySectors); err != nil {
		return err
	} else if !found {
		return xerrors.Errorf("removing early sectors %v not contained in %v", earlySectors, es.EarlySectors)
	}

	var err error
	if es.OnTimeSectors, err = bitfield.SubtractBitField(es.OnTimeSectors, onTimeSectors); err != nil {
		return err
	}
	if es.EarlySectors, err = bitfield.SubtractBitField(es.EarlySectors, earlySectors); err != nil {
		return err
	}
	es.OnTimePledge = big.Sub(es.OnTimePledge, onTimePledge)
	es.ActivePower = es.ActivePower.Sub(activePower)
	es.FaultyPower = es.FaultyPower.Sub(faultyPower)
	// Check underflow.
	if es.OnTimePledge.LessThan(big.Zero()) {
		return xerrors.Errorf("expiration set pledge underflow: %v", es)
	}
	if es.ActivePower.QA.LessThan(big.Zero()) || es.FaultyPower.QA.LessThan(big.Zero()) {
		return xerrors.Errorf("expiration set power underflow: %v", es)
	}
	return es.ValidateState()
}

// A set is empty if it has no sectors.
// The power and pledge are not checked, but expected to be zero.
func (es *ExpirationSet) IsEmpty() (empty bool, err error) {
	if empty, err = es.OnTimeSectors.IsEmpty(); err != nil {
		return false, err
	} else if empty {
		if empty, err = es.EarlySectors.IsEmpty(); err != nil {
			return false, err
		}
		return empty, nil
	} else {
		return false, nil
	}
}

// Counts all sectors in the expiration set.
func (es *ExpirationSet) Count() (count uint64, err error) {
	onTime, err := es.OnTimeSectors.Count()
	if err != nil {
		return 0, err
	}

	early, err := es.EarlySectors.Count()
	if err != nil {
		return 0, err
	}

	return onTime + early, nil
}

// validates a set of assertions that must hold for expiration sets
func (es *ExpirationSet) ValidateState() error {
	if es.OnTimePledge.LessThan(big.Zero()) {
		return xerrors.Errorf("ESet left with negative pledge: %+v", es)
	}

	if es.ActivePower.Raw.LessThan(big.Zero()) {
		return xerrors.Errorf("ESet left with negative raw active power: %+v", es)
	}

	if es.ActivePower.QA.LessThan(big.Zero()) {
		return xerrors.Errorf("ESet left with negative qa active power: %+v", es)
	}

	if es.FaultyPower.Raw.LessThan(big.Zero()) {
		return xerrors.Errorf("ESet left with negative raw faulty power: %+v", es)
	}

	if es.FaultyPower.QA.LessThan(big.Zero()) {
		return xerrors.Errorf("ESet left with negative qa faulty power: %+v", es)
	}

	return nil
}

// A queue of expiration sets by epoch, representing the on-time or early termination epoch for a collection of sectors.
// Wraps an AMT[ChainEpoch]*ExpirationSet.
// Keys in the queue are quantized (upwards), modulo some offset, to reduce the cardinality of keys.
type ExpirationQueue struct {
	*adt.Array
	quant QuantSpec
}

// An internal limit on the cardinality of a bitfield in a queue entry.
// This must be at least large enough to support the maximum number of sectors in a partition.
// It would be a bit better to derive this number from an enumeration over all partition sizes.
const entrySectorsMax = 10_000

// Loads a queue root.
// Epochs provided to subsequent method calls will be quantized upwards to quanta mod offsetSeed before being
// written to/read from queue entries.
func LoadExpirationQueue(store adt.Store, root cid.Cid, quant QuantSpec) (ExpirationQueue, error) {
	arr, err := adt.AsArray(store, root)
	if err != nil {
		return ExpirationQueue{}, xerrors.Errorf("failed to load epoch queue %v: %w", root, err)
	}
	return ExpirationQueue{arr, quant}, nil
}

// Adds a collection of sectors to their on-time target expiration entries (quantized).
// The sectors are assumed to be active (non-faulty).
// Returns the sector numbers, power, and pledge added.
func (q ExpirationQueue) AddActiveSectors(sectors []*SectorOnChainInfo, ssize abi.SectorSize) (bitfield.BitField, PowerPair, abi.TokenAmount, error) {
	totalPower := NewPowerPairZero()
	totalPledge := big.Zero()
	var totalSectors []bitfield.BitField
	noEarlySectors := bitfield.New()
	noFaultyPower := NewPowerPairZero()
	for _, group := range groupNewSectorsByDeclaredExpiration(ssize, sectors, q.quant) {
		snos := bitfield.NewFromSet(group.sectors)
		if err := q.add(group.epoch, snos, noEarlySectors, group.power, noFaultyPower, group.pledge); err != nil {
			return bitfield.BitField{}, NewPowerPairZero(), big.Zero(), xerrors.Errorf("failed to record new sector expirations: %w", err)
		}
		totalSectors = append(totalSectors, snos)
		totalPower = totalPower.Add(group.power)
		totalPledge = big.Add(totalPledge, group.pledge)
	}
	snos, err := bitfield.MultiMerge(totalSectors...)
	if err != nil {
		return bitfield.BitField{}, NewPowerPairZero(), big.Zero(), err
	}
	return snos, totalPower, totalPledge, nil
}

// Reschedules some sectors to a new (quantized) expiration epoch.
// The sectors being rescheduled are assumed to be not faulty, and hence are removed from and re-scheduled for on-time
// rather than early expiration.
// The sectors' power and pledge are assumed not to change, despite the new expiration.
func (q ExpirationQueue) RescheduleExpirations(newExpiration abi.ChainEpoch, sectors []*SectorOnChainInfo, ssize abi.SectorSize) error {
	if len(sectors) == 0 {
		return nil
	}

	snos, power, pledge, err := q.removeActiveSectors(sectors, ssize)
	if err != nil {
		return xerrors.Errorf("failed to remove sector expirations: %w", err)
	}
	if err = q.add(newExpiration, snos, bitfield.New(), power, NewPowerPairZero(), pledge); err != nil {
		return xerrors.Errorf("failed to record new sector expirations: %w", err)
	}
	return nil
}

// Re-schedules sectors to expire at an early expiration epoch (quantized), if they wouldn't expire before then anyway.
// The sectors must not be currently faulty, so must be registered as expiring on-time rather than early.
// The pledge for the now-early sectors is removed from the queue.
// Returns the total power represented by the sectors.
func (q ExpirationQueue) RescheduleAsFaults(newExpiration abi.ChainEpoch, sectors []*SectorOnChainInfo, ssize abi.SectorSize) (PowerPair, error) {
	var sectorsTotal []uint64
	expiringPower := NewPowerPairZero()
	rescheduledPower := NewPowerPairZero()

	// Group sectors by their target expiration, then remove from existing queue entries according to those groups.
	groups, err := q.findSectorsByExpiration(ssize, sectors)
	if err != nil {
		return NewPowerPairZero(), err
	}

	for _, group := range groups {
		var err error
		if group.epoch <= q.quant.QuantizeUp(newExpiration) {
			// Don't reschedule sectors that are already due to expire on-time before the fault-driven expiration,
			// but do represent their power as now faulty.
			// Their pledge remains as "on-time".
			group.expirationSet.ActivePower = group.expirationSet.ActivePower.Sub(group.power)
			group.expirationSet.FaultyPower = group.expirationSet.FaultyPower.Add(group.power)
			expiringPower = expiringPower.Add(group.power)
		} else {
			// Remove sectors from on-time expiry and active power.
			sectorsBf := bitfield.NewFromSet(group.sectors)
			if group.expirationSet.OnTimeSectors, err = bitfield.SubtractBitField(group.expirationSet.OnTimeSectors, sectorsBf); err != nil {
				return NewPowerPairZero(), err
			}
			group.expirationSet.OnTimePledge = big.Sub(group.expirationSet.OnTimePledge, group.pledge)
			group.expirationSet.ActivePower = group.expirationSet.ActivePower.Sub(group.power)

			// Accumulate the sectors and power removed.
			sectorsTotal = append(sectorsTotal, group.sectors...)
			rescheduledPower = rescheduledPower.Add(group.power)
		}
		if err = q.mustUpdateOrDelete(group.epoch, group.expirationSet); err != nil {
			return NewPowerPairZero(), err
		}

		if err = group.expirationSet.ValidateState(); err != nil {
			return NewPowerPairZero(), err
		}
	}

	if len(sectorsTotal) > 0 {
		// Add sectors to new expiration as early-terminating and faulty.
		earlySectors := bitfield.NewFromSet(sectorsTotal)
		noOnTimeSectors := bitfield.New()
		noOnTimePledge := abi.NewTokenAmount(0)
		noActivePower := NewPowerPairZero()
		if err := q.add(newExpiration, noOnTimeSectors, earlySectors, noActivePower, rescheduledPower, noOnTimePledge); err != nil {
			return NewPowerPairZero(), err
		}
	}

	return rescheduledPower.Add(expiringPower), nil
}

// Re-schedules *all* sectors to expire at an early expiration epoch, if they wouldn't expire before then anyway.
func (q ExpirationQueue) RescheduleAllAsFaults(faultExpiration abi.ChainEpoch) error {
	var rescheduledEpochs []uint64
	var rescheduledSectors []bitfield.BitField
	rescheduledPower := NewPowerPairZero()

	var es ExpirationSet
	if err := q.Array.ForEach(&es, func(e int64) error {
		epoch := abi.ChainEpoch(e)
		if epoch <= q.quant.QuantizeUp(faultExpiration) {
			// Regardless of whether the sectors were expiring on-time or early, all the power is now faulty.
			// Pledge is still on-time.
			es.FaultyPower = es.FaultyPower.Add(es.ActivePower)
			es.ActivePower = NewPowerPairZero()
			if err := q.mustUpdate(epoch, &es); err != nil {
				return err
			}
		} else {
			rescheduledEpochs = append(rescheduledEpochs, uint64(epoch))
			// sanity check to make sure we're not trying to re-schedule already faulty sectors.
			if isEmpty, err := es.EarlySectors.IsEmpty(); err != nil {
				return xerrors.Errorf("failed to determine if epoch had early expirations: %w", err)
			} else if !isEmpty {
				return xerrors.Errorf("attempted to re-schedule early expirations to an even earlier epoch")
			}
			rescheduledSectors = append(rescheduledSectors, es.OnTimeSectors)
			rescheduledPower = rescheduledPower.Add(es.ActivePower)
			rescheduledPower = rescheduledPower.Add(es.FaultyPower)
		}

		if err := es.ValidateState(); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	// If we didn't reschedule anything, we're done.
	if len(rescheduledEpochs) == 0 {
		return nil
	}

	// Add rescheduled sectors to new expiration as early-terminating and faulty.
	allRescheduled, err := bitfield.MultiMerge(rescheduledSectors...)
	if err != nil {
		return xerrors.Errorf("failed to merge rescheduled sectors: %w", err)
	}
	noOnTimeSectors := bitfield.New()
	noActivePower := NewPowerPairZero()
	noOnTimePledge := abi.NewTokenAmount(0)
	if err = q.add(faultExpiration, noOnTimeSectors, allRescheduled, noActivePower, rescheduledPower, noOnTimePledge); err != nil {
		return err
	}

	// Trim the rescheduled epochs from the queue.
	if err = q.BatchDelete(rescheduledEpochs); err != nil {
		return err
	}

	return nil
}

// Removes sectors from any queue entries in which they appear that are earlier then their scheduled expiration epoch,
// and schedules them at their expected termination epoch.
// Pledge for the sectors is re-added as on-time.
// Power for the sectors is changed from faulty to active (whether rescheduled or not).
// Returns the newly-recovered power. Fails if any sectors are not found in the queue.
func (q ExpirationQueue) RescheduleRecovered(sectors []*SectorOnChainInfo, ssize abi.SectorSize) (PowerPair, error) {
	remaining := make(map[abi.SectorNumber]struct{}, len(sectors))
	for _, s := range sectors {
		remaining[s.SectorNumber] = struct{}{}
	}

	// Traverse the expiration queue once to find each recovering sector and remove it from early/faulty there.
	// We expect this to find all recovering sectors within the first FaultMaxAge/WPoStProvingPeriod entries
	// (i.e. 14 for 14-day faults), but if something has gone wrong it's safer not to fail if that's not met.
	var sectorsRescheduled []*SectorOnChainInfo
	recoveredPower := NewPowerPairZero()
	if err := q.traverseMutate(func(epoch abi.ChainEpoch, es *ExpirationSet) (changed, keepGoing bool, err error) {
		onTimeSectors, err := es.OnTimeSectors.AllMap(entrySectorsMax)
		if err != nil {
			return false, false, err
		}
		earlySectors, err := es.EarlySectors.AllMap(entrySectorsMax)
		if err != nil {
			return false, false, err
		}

		// This loop could alternatively be done by constructing bitfields and intersecting them, but it's not
		// clear that would be much faster (O(max(N, M)) vs O(N+M)).
		// If faults are correlated, the first queue entry likely has them all anyway.
		// The length of sectors has a maximum of one partition size.
		for _, sector := range sectors {
			sno := uint64(sector.SectorNumber)
			power := PowerForSector(ssize, sector)
			var found bool
			if _, found = onTimeSectors[sno]; found {
				// If the sector expires on-time at this epoch, leave it here but change faulty power to active.
				// The pledge is already part of the on-time pledge at this entry.
				es.FaultyPower = es.FaultyPower.Sub(power)
				es.ActivePower = es.ActivePower.Add(power)
			} else if _, found = earlySectors[sno]; found {
				// If the sector expires early at this epoch, remove it for re-scheduling.
				// It's not part of the on-time pledge number here.
				es.EarlySectors.Unset(sno)
				es.FaultyPower = es.FaultyPower.Sub(power)
				sectorsRescheduled = append(sectorsRescheduled, sector)
			}
			if found {
				recoveredPower = recoveredPower.Add(power)
				delete(remaining, sector.SectorNumber)
				changed = true
			}
		}

		if err = es.ValidateState(); err != nil {
			return false, false, err
		}

		return changed, len(remaining) > 0, nil
	}); err != nil {
		return NewPowerPairZero(), err
	}
	if len(remaining) > 0 {
		return NewPowerPairZero(), xerrors.Errorf("sectors not found in expiration queue: %v", remaining)
	}

	// Re-schedule the removed sectors to their target expiration.
	if _, _, _, err := q.AddActiveSectors(sectorsRescheduled, ssize); err != nil {
		return NewPowerPairZero(), err
	}
	return recoveredPower, nil
}

// Removes some sectors and adds some others.
// The sectors being replaced must not be faulty, so must be scheduled for on-time rather than early expiration.
// The sectors added are assumed to be not faulty.
// Returns the old a new sector number bitfields, and delta to power and pledge, new minus old.
func (q ExpirationQueue) ReplaceSectors(oldSectors, newSectors []*SectorOnChainInfo, ssize abi.SectorSize) (bitfield.BitField, bitfield.BitField, PowerPair, abi.TokenAmount, error) {
	oldSnos, oldPower, oldPledge, err := q.removeActiveSectors(oldSectors, ssize)
	if err != nil {
		return bitfield.BitField{}, bitfield.BitField{}, NewPowerPairZero(), big.Zero(), xerrors.Errorf("failed to remove replaced sectors: %w", err)
	}
	newSnos, newPower, newPledge, err := q.AddActiveSectors(newSectors, ssize)
	if err != nil {
		return bitfield.BitField{}, bitfield.BitField{}, NewPowerPairZero(), big.Zero(), xerrors.Errorf("failed to add replacement sectors: %w", err)
	}
	return oldSnos, newSnos, newPower.Sub(oldPower), big.Sub(newPledge, oldPledge), nil
}

// Remove some sectors from the queue.
// The sectors may be active or faulty, and scheduled either for on-time or early termination.
// Returns the aggregate of removed sectors and power, and recovering power.
// Fails if any sectors are not found in the queue.
func (q ExpirationQueue) RemoveSectors(sectors []*SectorOnChainInfo, faults bitfield.BitField, recovering bitfield.BitField,
	ssize abi.SectorSize) (*ExpirationSet, PowerPair, error) {
	remaining := make(map[abi.SectorNumber]struct{}, len(sectors))
	for _, s := range sectors {
		remaining[s.SectorNumber] = struct{}{}
	}
	faultsMap, err := faults.AllMap(AddressedSectorsMax)
	if err != nil {
		return nil, NewPowerPairZero(), xerrors.Errorf("failed to expand faults: %w", err)
	}
	recoveringMap, err := recovering.AllMap(AddressedSectorsMax)
	if err != nil {
		return nil, NewPowerPairZero(), xerrors.Errorf("failed to expand recoveries: %w", err)
	}

	// results
	removed := NewExpirationSetEmpty()
	recoveringPower := NewPowerPairZero()

	// Split into faulty and non-faulty. We process non-faulty sectors first
	// because they always expire on-time so we know where to find them.
	var (
		nonFaultySectors []*SectorOnChainInfo
		faultySectors    []*SectorOnChainInfo
	)
	for _, sector := range sectors {
		if _, found := faultsMap[uint64(sector.SectorNumber)]; found {
			faultySectors = append(faultySectors, sector)
			continue
		}
		nonFaultySectors = append(nonFaultySectors, sector)
		// remove them from "remaining", we're going to process them below.
		delete(remaining, sector.SectorNumber)
	}

	// Remove non-faulty sectors.
	removed.OnTimeSectors, removed.ActivePower, removed.OnTimePledge, err = q.removeActiveSectors(nonFaultySectors, ssize)
	if err != nil {
		return nil, NewPowerPairZero(), xerrors.Errorf("failed to remove on-time recoveries: %w", err)
	}

	// Finally, remove faulty sectors (on time and not). These sectors can
	// only appear within the first 14 days (fault max age). Given that this
	// queue is quantized, we should be able to stop traversing the queue
	// after 14 entries.
	if err = q.traverseMutate(func(epoch abi.ChainEpoch, es *ExpirationSet) (changed, keepGoing bool, err error) {
		onTimeSectors, err := es.OnTimeSectors.AllMap(entrySectorsMax)
		if err != nil {
			return false, false, err
		}
		earlySectors, err := es.EarlySectors.AllMap(entrySectorsMax)
		if err != nil {
			return false, false, err
		}

		// This loop could alternatively be done by constructing bitfields and intersecting them, but it's not
		// clear that would be much faster (O(max(N, M)) vs O(N+M)).
		// The length of sectors has a maximum of one partition size.
		for _, sector := range faultySectors {
			sno := uint64(sector.SectorNumber)
			var found bool
			if _, found = onTimeSectors[sno]; found {
				es.OnTimeSectors.Unset(sno)
				removed.OnTimeSectors.Set(sno)
				es.OnTimePledge = big.Sub(es.OnTimePledge, sector.InitialPledge)
				removed.OnTimePledge = big.Add(removed.OnTimePledge, sector.InitialPledge)
			} else if _, found = earlySectors[sno]; found {
				es.EarlySectors.Unset(sno)
				removed.EarlySectors.Set(sno)
			}
			if found {
				power := PowerForSector(ssize, sector)
				if _, f := faultsMap[sno]; f {
					es.FaultyPower = es.FaultyPower.Sub(power)
					removed.FaultyPower = removed.FaultyPower.Add(power)
				} else {
					es.ActivePower = es.ActivePower.Sub(power)
					removed.ActivePower = removed.ActivePower.Add(power)
				}
				if _, r := recoveringMap[sno]; r {
					recoveringPower = recoveringPower.Add(power)
				}
				delete(remaining, sector.SectorNumber)
				changed = true
			}
		}

		if err = es.ValidateState(); err != nil {
			return false, false, err
		}

		return changed, len(remaining) > 0, nil
	}); err != nil {
		return nil, recoveringPower, err
	}
	if len(remaining) > 0 {
		return NewExpirationSetEmpty(), NewPowerPairZero(), xerrors.Errorf("sectors not found in expiration queue: %v", remaining)
	}

	return removed, recoveringPower, nil
}

// Removes and aggregates entries from the queue up to and including some epoch.
func (q ExpirationQueue) PopUntil(until abi.ChainEpoch) (*ExpirationSet, error) {
	var onTimeSectors []bitfield.BitField
	var earlySectors []bitfield.BitField
	activePower := NewPowerPairZero()
	faultyPower := NewPowerPairZero()
	onTimePledge := big.Zero()

	var poppedKeys []uint64
	var thisValue ExpirationSet
	stopErr := fmt.Errorf("stop")
	if err := q.Array.ForEach(&thisValue, func(i int64) error {
		if abi.ChainEpoch(i) > until {
			return stopErr
		}
		poppedKeys = append(poppedKeys, uint64(i))
		onTimeSectors = append(onTimeSectors, thisValue.OnTimeSectors)
		earlySectors = append(earlySectors, thisValue.EarlySectors)
		activePower = activePower.Add(thisValue.ActivePower)
		faultyPower = faultyPower.Add(thisValue.FaultyPower)
		onTimePledge = big.Add(onTimePledge, thisValue.OnTimePledge)
		return nil
	}); err != nil && err != stopErr {
		return nil, err
	}

	if err := q.Array.BatchDelete(poppedKeys); err != nil {
		return nil, err
	}

	allOnTime, err := bitfield.MultiMerge(onTimeSectors...)
	if err != nil {
		return nil, err
	}
	allEarly, err := bitfield.MultiMerge(earlySectors...)
	if err != nil {
		return nil, err
	}
	return NewExpirationSet(allOnTime, allEarly, onTimePledge, activePower, faultyPower), nil
}

func (q ExpirationQueue) add(rawEpoch abi.ChainEpoch, onTimeSectors, earlySectors bitfield.BitField, activePower, faultyPower PowerPair,
	pledge abi.TokenAmount) error {
	epoch := q.quant.QuantizeUp(rawEpoch)
	es, err := q.mayGet(epoch)
	if err != nil {
		return err
	}

	if err = es.Add(onTimeSectors, earlySectors, pledge, activePower, faultyPower); err != nil {
		return xerrors.Errorf("failed to add expiration values for epoch %v: %w", epoch, err)
	}

	return q.mustUpdate(epoch, es)
}

func (q ExpirationQueue) remove(rawEpoch abi.ChainEpoch, onTimeSectors, earlySectors bitfield.BitField, activePower, faultyPower PowerPair,
	pledge abi.TokenAmount) error {
	epoch := q.quant.QuantizeUp(rawEpoch)
	var es ExpirationSet
	if found, err := q.Array.Get(uint64(epoch), &es); err != nil {
		return xerrors.Errorf("failed to lookup queue epoch %v: %w", epoch, err)
	} else if !found {
		return xerrors.Errorf("missing expected expiration set at epoch %v", epoch)
	}

	if err := es.Remove(onTimeSectors, earlySectors, pledge, activePower, faultyPower); err != nil {
		return xerrors.Errorf("failed to remove expiration values for queue epoch %v: %w", epoch, err)
	}

	return q.mustUpdateOrDelete(epoch, &es)
}

func (q ExpirationQueue) removeActiveSectors(sectors []*SectorOnChainInfo, ssize abi.SectorSize) (bitfield.BitField, PowerPair, abi.TokenAmount, error) {
	removedSnos := bitfield.New()
	removedPower := NewPowerPairZero()
	removedPledge := big.Zero()
	noEarlySectors := bitfield.New()
	noFaultyPower := NewPowerPairZero()

	// Group sectors by their expiration, then remove from existing queue entries according to those groups.
	groups, err := q.findSectorsByExpiration(ssize, sectors)
	if err != nil {
		return bitfield.BitField{}, NewPowerPairZero(), big.Zero(), err
	}

	for _, group := range groups {
		sectorsBf := bitfield.NewFromSet(group.sectors)
		if err := q.remove(group.epoch, sectorsBf, noEarlySectors, group.power, noFaultyPower, group.pledge); err != nil {
			return bitfield.BitField{}, NewPowerPairZero(), big.Zero(), err
		}
		for _, n := range group.sectors {
			removedSnos.Set(n)
		}
		removedPower = removedPower.Add(group.power)
		removedPledge = big.Add(removedPledge, group.pledge)
	}
	return removedSnos, removedPower, removedPledge, nil
}

// Traverses the entire queue with a callback function that may mutate entries.
// Iff the function returns that it changed an entry, the new entry will be re-written in the queue. Any changed
// entries that become empty are removed after iteration completes.
func (q ExpirationQueue) traverseMutate(f func(epoch abi.ChainEpoch, es *ExpirationSet) (changed, keepGoing bool, err error)) error {
	var es ExpirationSet
	var epochsEmptied []uint64
	errStop := fmt.Errorf("stop")
	if err := q.Array.ForEach(&es, func(epoch int64) error {
		changed, keepGoing, err := f(abi.ChainEpoch(epoch), &es)
		if err != nil {
			return err
		} else if changed {
			if emptied, err := es.IsEmpty(); err != nil {
				return err
			} else if emptied {
				epochsEmptied = append(epochsEmptied, uint64(epoch))
			} else if err = q.mustUpdate(abi.ChainEpoch(epoch), &es); err != nil {
				return err
			}
		}

		if !keepGoing {
			return errStop
		}
		return nil
	}); err != nil && err != errStop {
		return err
	}
	if err := q.Array.BatchDelete(epochsEmptied); err != nil {
		return err
	}
	return nil
}

func (q ExpirationQueue) traverse(f func(epoch abi.ChainEpoch, es *ExpirationSet) (keepGoing bool, err error)) error {
	return q.traverseMutate(func(epoch abi.ChainEpoch, es *ExpirationSet) (bool, bool, error) {
		keepGoing, err := f(epoch, es)
		return false, keepGoing, err
	})
}

func (q ExpirationQueue) mayGet(key abi.ChainEpoch) (*ExpirationSet, error) {
	es := NewExpirationSetEmpty()
	if _, err := q.Array.Get(uint64(key), es); err != nil {
		return nil, xerrors.Errorf("failed to lookup queue epoch %v: %w", key, err)
	}
	return es, nil
}

func (q ExpirationQueue) mustUpdate(epoch abi.ChainEpoch, es *ExpirationSet) error {
	if err := q.Array.Set(uint64(epoch), es); err != nil {
		return xerrors.Errorf("failed to set queue epoch %v: %w", epoch, err)
	}
	return nil
}

// Since this might delete the node, it's not safe for use inside an iteration.
func (q ExpirationQueue) mustUpdateOrDelete(epoch abi.ChainEpoch, es *ExpirationSet) error {
	if empty, err := es.IsEmpty(); err != nil {
		return err
	} else if empty {
		if err = q.Array.Delete(uint64(epoch)); err != nil {
			return xerrors.Errorf("failed to delete queue epoch %d: %w", epoch, err)
		}
	} else if err = q.Array.Set(uint64(epoch), es); err != nil {
		return xerrors.Errorf("failed to set queue epoch %v: %w", epoch, err)
	}
	return nil
}

type sectorEpochSet struct {
	epoch   abi.ChainEpoch
	sectors []uint64
	power   PowerPair
	pledge  abi.TokenAmount
}

type sectorExpirationSet struct {
	sectorEpochSet
	expirationSet *ExpirationSet
}

// Takes a slice of sector infos and returns sector info sets grouped and
// sorted by expiration epoch, quantized.
//
// Note: While the result is sorted by epoch, the order of per-epoch sectors is maintained.
func groupNewSectorsByDeclaredExpiration(sectorSize abi.SectorSize, sectors []*SectorOnChainInfo, quant QuantSpec) []sectorEpochSet {
	sectorsByExpiration := make(map[abi.ChainEpoch][]*SectorOnChainInfo)

	for _, sector := range sectors {
		qExpiration := quant.QuantizeUp(sector.Expiration)
		sectorsByExpiration[qExpiration] = append(sectorsByExpiration[qExpiration], sector)
	}

	sectorEpochSets := make([]sectorEpochSet, 0, len(sectorsByExpiration))

	// This map iteration is non-deterministic but safe because we sort by epoch below.
	for expiration, epochSectors := range sectorsByExpiration { //nolint:nomaprange // result is subsequently sorted
		sectorNumbers := make([]uint64, len(epochSectors))
		totalPower := NewPowerPairZero()
		totalPledge := big.Zero()
		for i, sector := range epochSectors {
			sectorNumbers[i] = uint64(sector.SectorNumber)
			totalPower = totalPower.Add(PowerForSector(sectorSize, sector))
			totalPledge = big.Add(totalPledge, sector.InitialPledge)
		}
		sectorEpochSets = append(sectorEpochSets, sectorEpochSet{
			epoch:   expiration,
			sectors: sectorNumbers,
			power:   totalPower,
			pledge:  totalPledge,
		})
	}

	sort.Slice(sectorEpochSets, func(i, j int) bool {
		return sectorEpochSets[i].epoch < sectorEpochSets[j].epoch
	})
	return sectorEpochSets
}

// Groups sectors into sets based on their Expiration field.
// If sectors are not found in the expiration set corresponding to their expiration field
// (i.e. they have been rescheduled) traverse expiration sets to for groups where these
// sectors actually expire.
// Groups will be returned in expiration order, earliest first.
func (q *ExpirationQueue) findSectorsByExpiration(sectorSize abi.SectorSize, sectors []*SectorOnChainInfo) ([]sectorExpirationSet, error) {
	declaredExpirations := make(map[abi.ChainEpoch]bool, len(sectors))
	sectorsByNumber := make(map[uint64]*SectorOnChainInfo, len(sectors))
	allRemaining := make(map[uint64]struct{})
	expirationGroups := make([]sectorExpirationSet, 0, len(declaredExpirations))

	for _, sector := range sectors {
		qExpiration := q.quant.QuantizeUp(sector.Expiration)
		declaredExpirations[qExpiration] = true
		allRemaining[uint64(sector.SectorNumber)] = struct{}{}
		sectorsByNumber[uint64(sector.SectorNumber)] = sector
	}

	// Traverse expiration sets first by expected expirations. This will find all groups if no sectors have been rescheduled.
	// This map iteration is non-deterministic but safe because we sort by epoch below.
	for expiration := range declaredExpirations { //nolint:nomaprange // result is subsequently sorted
		es, err := q.mayGet(expiration)
		if err != nil {
			return nil, err
		}

		// create group from overlap
		var group sectorExpirationSet
		group, allRemaining, err = groupExpirationSet(sectorSize, sectorsByNumber, allRemaining, es, expiration)
		if err != nil {
			return nil, err
		}
		if len(group.sectors) > 0 {
			expirationGroups = append(expirationGroups, group)
		}
	}

	// If sectors remain, traverse next in epoch order. Remaining sectors should be rescheduled to expire soon, so
	// this traversal should exit early.
	if len(allRemaining) > 0 {
		err := q.traverse(func(epoch abi.ChainEpoch, es *ExpirationSet) (bool, error) {
			// If this set's epoch is one of our declared epochs, we've already processed it in the loop above,
			// so skip processing here. Sectors rescheduled to this epoch would have been included in the earlier processing.
			if _, found := declaredExpirations[epoch]; found {
				return true, nil
			}

			// Sector should not be found in EarlyExpirations which holds faults. An implicit assumption
			// of grouping is that it only returns sectors with active power. ExpirationQueue should not
			// provide operations that allow this to happen.
			if err := assertNoEarlySectors(allRemaining, es); err != nil {
				return true, err
			}

			var group sectorExpirationSet
			var err error
			group, allRemaining, err = groupExpirationSet(sectorSize, sectorsByNumber, allRemaining, es, epoch)
			if err != nil {
				return false, err
			}
			if len(group.sectors) > 0 {
				expirationGroups = append(expirationGroups, group)
			}

			return len(allRemaining) > 0, nil
		})
		if err != nil {
			return nil, err
		}
	}

	if len(allRemaining) > 0 {
		return nil, xerrors.New("some sectors not found in expiration queue")
	}

	// sort groups, earliest first.
	sort.Slice(expirationGroups, func(i, j int) bool {
		return expirationGroups[i].epoch < expirationGroups[j].epoch
	})
	return expirationGroups, nil
}

// Takes a slice of sector infos a bitfield of sector numbers and returns a single group for all bitfield sectors
// Also returns a bitfield containing sectors not found in expiration set.
// This method mutates includeSet by removing sector numbers of sectors found in expiration set.
func groupExpirationSet(sectorSize abi.SectorSize, sectors map[uint64]*SectorOnChainInfo,
	includeSet map[uint64]struct{}, es *ExpirationSet, expiration abi.ChainEpoch,
) (sectorExpirationSet, map[uint64]struct{}, error) {
	var sectorNumbers []uint64
	totalPower := NewPowerPairZero()
	totalPledge := big.Zero()
	err := es.OnTimeSectors.ForEach(func(u uint64) error {
		if _, found := includeSet[u]; found {
			sector := sectors[u]
			sectorNumbers = append(sectorNumbers, u)
			totalPower = totalPower.Add(PowerForSector(sectorSize, sector))
			totalPledge = big.Add(totalPledge, sector.InitialPledge)
			delete(includeSet, u)
		}
		return nil
	})
	if err != nil {
		return sectorExpirationSet{}, nil, err
	}

	return sectorExpirationSet{
		sectorEpochSet: sectorEpochSet{
			epoch:   expiration,
			sectors: sectorNumbers,
			power:   totalPower,
			pledge:  totalPledge,
		},
		expirationSet: es,
	}, includeSet, nil
}

// assertNoEarlySectors checks for an invalid overlap between a bitfield an a set's early sectors.
func assertNoEarlySectors(set map[uint64]struct{}, es *ExpirationSet) error {
	return es.EarlySectors.ForEach(func(u uint64) error {
		if _, found := set[u]; found {
			return xerrors.Errorf("Invalid attempt to group sector %d with an early expiration", u)
		}
		return nil
	})
}
