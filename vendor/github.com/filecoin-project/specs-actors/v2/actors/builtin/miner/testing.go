package miner

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

type DealSummary struct {
	SectorStart      abi.ChainEpoch
	SectorExpiration abi.ChainEpoch
}

type StateSummary struct {
	LivePower     PowerPair
	ActivePower   PowerPair
	FaultyPower   PowerPair
	Deals         map[abi.DealID]DealSummary
	SealProofType abi.RegisteredSealProof
}

// Checks internal invariants of init state.
func CheckStateInvariants(st *State, store adt.Store, balance abi.TokenAmount) (*StateSummary, *builtin.MessageAccumulator) {
	acc := &builtin.MessageAccumulator{}
	sectorSize := abi.SectorSize(0)
	minerSummary := &StateSummary{
		LivePower:     NewPowerPairZero(),
		ActivePower:   NewPowerPairZero(),
		FaultyPower:   NewPowerPairZero(),
		SealProofType: 0,
	}

	// Load data from linked structures.
	if info, err := st.GetInfo(store); err != nil {
		acc.Addf("error loading miner info: %v", err)
		// Stop here, it's too hard to make other useful checks.
		return minerSummary, acc
	} else {
		minerSummary.SealProofType = info.SealProofType
		sectorSize = info.SectorSize
		CheckMinerInfo(info, acc)
	}

	CheckMinerBalances(st, store, balance, acc)

	var allocatedSectors bitfield.BitField
	var allocatedSectorsMap map[uint64]bool
	if err := store.Get(store.Context(), st.AllocatedSectors, &allocatedSectors); err != nil {
		acc.Addf("error loading allocated sector bitfield: %v", err)
	} else {
		allocatedSectorsMap, err = allocatedSectors.AllMap(1 << 30)
		if err != nil {
			acc.Addf("error expanding allocated sector bitfield: %v", err)
			allocatedSectorsMap = nil
		}
	}

	CheckPreCommits(st, store, allocatedSectorsMap, acc)

	minerSummary.Deals = map[abi.DealID]DealSummary{}
	var allSectors map[abi.SectorNumber]*SectorOnChainInfo
	if sectorsArr, err := adt.AsArray(store, st.Sectors); err != nil {
		acc.Addf("error loading sectors")
	} else {
		allSectors = map[abi.SectorNumber]*SectorOnChainInfo{}
		var sector SectorOnChainInfo
		err = sectorsArr.ForEach(&sector, func(sno int64) error {
			cpy := sector
			allSectors[abi.SectorNumber(sno)] = &cpy
			acc.Require(allocatedSectorsMap == nil || allocatedSectorsMap[uint64(sno)],
				"on chain sector's sector number has not been allocated %d", sno)

			for _, dealID := range sector.DealIDs {
				minerSummary.Deals[dealID] = DealSummary{
					SectorStart:      sector.Activation,
					SectorExpiration: sector.Expiration,
				}
			}

			return nil
		})
		acc.RequireNoError(err, "error iterating sectors")
	}

	// Check deadlines
	acc.Require(st.CurrentDeadline < WPoStPeriodDeadlines,
		"current deadline index is greater than deadlines per period(%d): %d", WPoStPeriodDeadlines, st.CurrentDeadline)

	deadlines, err := st.LoadDeadlines(store)
	if err != nil {
		acc.Addf("error loading deadlines: %v", err)
		deadlines = nil
	}

	if allSectors != nil && deadlines != nil {
		err = deadlines.ForEach(store, func(dlIdx uint64, dl *Deadline) error {
			acc := acc.WithPrefix("deadline %d: ", dlIdx) // Shadow
			quant := st.QuantSpecForDeadline(dlIdx)
			dlSummary := CheckDeadlineStateInvariants(dl, store, quant, sectorSize, allSectors, acc)

			minerSummary.LivePower = minerSummary.LivePower.Add(dlSummary.LivePower)
			minerSummary.ActivePower = minerSummary.ActivePower.Add(dlSummary.ActivePower)
			minerSummary.FaultyPower = minerSummary.FaultyPower.Add(dlSummary.FaultyPower)
			return nil
		})
		acc.RequireNoError(err, "error iterating deadlines")
	}

	return minerSummary, acc
}

type DeadlineStateSummary struct {
	AllSectors        bitfield.BitField
	LiveSectors       bitfield.BitField
	FaultySectors     bitfield.BitField
	RecoveringSectors bitfield.BitField
	UnprovenSectors   bitfield.BitField
	TerminatedSectors bitfield.BitField
	LivePower         PowerPair
	ActivePower       PowerPair
	FaultyPower       PowerPair
}

func CheckDeadlineStateInvariants(deadline *Deadline, store adt.Store, quant QuantSpec, ssize abi.SectorSize,
	sectors map[abi.SectorNumber]*SectorOnChainInfo, acc *builtin.MessageAccumulator) *DeadlineStateSummary {

	// Load linked structures.
	partitions, err := deadline.PartitionsArray(store)
	if err != nil {
		acc.Addf("error loading partitions: %v", err)
		// Hard to do any useful checks.
		return &DeadlineStateSummary{
			AllSectors:        bitfield.New(),
			LiveSectors:       bitfield.New(),
			FaultySectors:     bitfield.New(),
			RecoveringSectors: bitfield.New(),
			UnprovenSectors:   bitfield.New(),
			TerminatedSectors: bitfield.New(),
			LivePower:         NewPowerPairZero(),
			ActivePower:       NewPowerPairZero(),
			FaultyPower:       NewPowerPairZero(),
		}
	}

	allSectors := bitfield.New()
	var allLiveSectors []bitfield.BitField
	var allFaultySectors []bitfield.BitField
	var allRecoveringSectors []bitfield.BitField
	var allUnprovenSectors []bitfield.BitField
	var allTerminatedSectors []bitfield.BitField
	allLivePower := NewPowerPairZero()
	allActivePower := NewPowerPairZero()
	allFaultyPower := NewPowerPairZero()

	// Check partitions.
	partitionsWithExpirations := map[abi.ChainEpoch][]uint64{}
	var partitionsWithEarlyTerminations []uint64
	partitionCount := uint64(0)
	var partition Partition
	err = partitions.ForEach(&partition, func(i int64) error {
		pIdx := uint64(i)
		// Check sequential partitions.
		acc.Require(pIdx == partitionCount, "Non-sequential partitions, expected index %d, found %d", partitionCount, pIdx)
		partitionCount++

		acc := acc.WithPrefix("partition %d: ", pIdx) // Shadow
		summary := CheckPartitionStateInvariants(&partition, store, quant, ssize, sectors, acc)

		if contains, err := util.BitFieldContainsAny(allSectors, summary.AllSectors); err != nil {
			acc.Addf("error checking bitfield contains: %v", err)
		} else {
			acc.Require(!contains, "duplicate sector in partition %d", pIdx)
		}

		for _, e := range summary.ExpirationEpochs {
			partitionsWithExpirations[e] = append(partitionsWithExpirations[e], pIdx)
		}
		if summary.EarlyTerminationCount > 0 {
			partitionsWithEarlyTerminations = append(partitionsWithEarlyTerminations, pIdx)
		}

		allSectors, err = bitfield.MergeBitFields(allSectors, summary.AllSectors)
		if err != nil {
			acc.Addf("error merging partition sector numbers with all: %v", err)
			allSectors = bitfield.New()
		}
		allLiveSectors = append(allLiveSectors, summary.LiveSectors)
		allFaultySectors = append(allFaultySectors, summary.FaultySectors)
		allRecoveringSectors = append(allRecoveringSectors, summary.RecoveringSectors)
		allUnprovenSectors = append(allUnprovenSectors, summary.UnprovenSectors)
		allTerminatedSectors = append(allTerminatedSectors, summary.TerminatedSectors)
		allLivePower = allLivePower.Add(summary.LivePower)
		allActivePower = allActivePower.Add(summary.ActivePower)
		allFaultyPower = allFaultyPower.Add(summary.FaultyPower)
		return nil
	})
	acc.RequireNoError(err, "error iterating partitions")

	// Check PoSt submissions
	if postSubmissions, err := deadline.PostSubmissions.All(1 << 20); err != nil {
		acc.Addf("error expanding post submissions: %v", err)
	} else {
		for _, p := range postSubmissions {
			acc.Require(p <= partitionCount, "invalid PoSt submission for partition %d of %d", p, partitionCount)
		}
	}

	// Check memoized sector and power values.
	live, err := bitfield.MultiMerge(allLiveSectors...)
	if err != nil {
		acc.Addf("error merging live sector numbers: %v", err)
		live = bitfield.New()
	} else {
		if liveCount, err := live.Count(); err != nil {
			acc.Addf("error counting live sectors: %v", err)
		} else {
			acc.Require(deadline.LiveSectors == liveCount, "deadline live sectors %d != partitions count %d", deadline.LiveSectors, liveCount)
		}
	}

	if allCount, err := allSectors.Count(); err != nil {
		acc.Addf("error counting all sectors: %v", err)
	} else {
		acc.Require(deadline.TotalSectors == allCount, "deadline total sectors %d != partitions count %d", deadline.TotalSectors, allCount)
	}

	faulty, err := bitfield.MultiMerge(allFaultySectors...)
	if err != nil {
		acc.Addf("error merging faulty sector numbers: %v", err)
		faulty = bitfield.New()
	}
	recovering, err := bitfield.MultiMerge(allRecoveringSectors...)
	if err != nil {
		acc.Addf("error merging recovering sector numbers: %v", err)
		recovering = bitfield.New()
	}
	unproven, err := bitfield.MultiMerge(allUnprovenSectors...)
	if err != nil {
		acc.Addf("error merging unproven sector numbers: %v", err)
		unproven = bitfield.New()
	}
	terminated, err := bitfield.MultiMerge(allTerminatedSectors...)
	if err != nil {
		acc.Addf("error merging terminated sector numbers: %v", err)
		terminated = bitfield.New()
	}

	acc.Require(deadline.FaultyPower.Equals(allFaultyPower), "deadline faulty power %v != partitions total %v", deadline.FaultyPower, allFaultyPower)

	{
		// Validate partition expiration queue contains an entry for each partition and epoch with an expiration.
		// The queue may be a superset of the partitions that have expirations because we never remove from it.
		if expirationEpochs, err := adt.AsArray(store, deadline.ExpirationsEpochs); err != nil {
			acc.Addf("error loading expiration queue: %v", err)
		} else {
			for epoch, expiringPIdxs := range partitionsWithExpirations { // nolint:nomaprange
				var bf bitfield.BitField
				if found, err := expirationEpochs.Get(uint64(epoch), &bf); err != nil {
					acc.Addf("error fetching expiration bitfield: %v", err)
				} else {
					acc.Require(found, "expected to find partition expiration entry at epoch %d", epoch)
				}

				if queuedPIdxs, err := bf.AllMap(1 << 20); err != nil {
					acc.Addf("error expanding expirating partitions: %v", err)
				} else {
					for _, p := range expiringPIdxs {
						acc.Require(queuedPIdxs[p], "expected partition %d to be present in deadline expiration queue at epoch %d", p, epoch)
					}
				}
			}
		}
	}
	{
		// Validate the early termination queue contains exactly the partitions with early terminations.
		expected := bitfield.NewFromSet(partitionsWithEarlyTerminations)
		requireEqual(expected, deadline.EarlyTerminations, acc, "deadline early terminations doesn't match expected partitions")
	}

	return &DeadlineStateSummary{
		AllSectors:        allSectors,
		LiveSectors:       live,
		FaultySectors:     faulty,
		RecoveringSectors: recovering,
		UnprovenSectors:   unproven,
		TerminatedSectors: terminated,
		LivePower:         allLivePower,
		ActivePower:       allActivePower,
		FaultyPower:       allFaultyPower,
	}
}

type PartitionStateSummary struct {
	AllSectors            bitfield.BitField
	LiveSectors           bitfield.BitField
	FaultySectors         bitfield.BitField
	RecoveringSectors     bitfield.BitField
	UnprovenSectors       bitfield.BitField
	TerminatedSectors     bitfield.BitField
	LivePower             PowerPair
	ActivePower           PowerPair
	FaultyPower           PowerPair
	RecoveringPower       PowerPair
	ExpirationEpochs      []abi.ChainEpoch // Epochs at which some sector is scheduled to expire.
	EarlyTerminationCount int
}

func CheckPartitionStateInvariants(
	partition *Partition,
	store adt.Store,
	quant QuantSpec,
	sectorSize abi.SectorSize,
	sectors map[abi.SectorNumber]*SectorOnChainInfo,
	acc *builtin.MessageAccumulator,
) *PartitionStateSummary {
	irrecoverable := false // State is so broken we can't make useful checks.
	live, err := partition.LiveSectors()
	if err != nil {
		acc.Addf("error computing live sectors: %v", err)
		irrecoverable = true
	}
	active, err := partition.ActiveSectors()
	if err != nil {
		acc.Addf("error computing active sectors: %v", err)
		irrecoverable = true
	}

	if irrecoverable {
		return &PartitionStateSummary{
			AllSectors:            partition.Sectors,
			LiveSectors:           bitfield.New(),
			FaultySectors:         partition.Faults,
			RecoveringSectors:     partition.Recoveries,
			UnprovenSectors:       partition.Unproven,
			TerminatedSectors:     partition.Terminated,
			LivePower:             partition.LivePower,
			ActivePower:           partition.ActivePower(),
			FaultyPower:           partition.FaultyPower,
			RecoveringPower:       partition.RecoveringPower,
			ExpirationEpochs:      nil,
			EarlyTerminationCount: 0,
		}
	}

	// Live contains all active sectors.
	requireContainsAll(live, active, acc, "live does not contain active")

	// Live contains all faults.
	requireContainsAll(live, partition.Faults, acc, "live does not contain faults")

	// Live contains all unproven.
	requireContainsAll(live, partition.Unproven, acc, "live does not contain unproven")

	// Active contains no faults
	requireContainsNone(active, partition.Faults, acc, "active includes faults")

	// Active contains no unproven
	requireContainsNone(active, partition.Unproven, acc, "active includes unproven")

	// Faults contains all recoveries.
	requireContainsAll(partition.Faults, partition.Recoveries, acc, "faults do not contain recoveries")

	// Live contains no terminated sectors
	requireContainsNone(live, partition.Terminated, acc, "live includes terminations")

	// Unproven contains no faults
	requireContainsNone(partition.Faults, partition.Unproven, acc, "unproven includes faults")

	// All terminated sectors are part of the partition.
	requireContainsAll(partition.Sectors, partition.Terminated, acc, "sectors do not contain terminations")

	// Validate power
	var liveSectors map[abi.SectorNumber]*SectorOnChainInfo
	var missing []abi.SectorNumber
	livePower := NewPowerPairZero()
	faultyPower := NewPowerPairZero()
	unprovenPower := NewPowerPairZero()

	if liveSectors, missing, err = selectSectorsMap(sectors, live); err != nil {
		acc.Addf("error selecting live sectors: %v", err)
	} else if len(missing) > 0 {
		acc.Addf("live sectors missing from all sectors: %v", missing)
	} else {
		livePower = powerForSectors(liveSectors, sectorSize)
		acc.Require(partition.LivePower.Equals(livePower), "live power was %v, expected %v", partition.LivePower, livePower)
	}

	if unprovenSectors, missing, err := selectSectorsMap(sectors, partition.Unproven); err != nil {
		acc.Addf("error selecting unproven sectors: %v", err)
	} else if len(missing) > 0 {
		acc.Addf("unproven sectors missing from all sectors: %v", missing)
	} else {
		unprovenPower = powerForSectors(unprovenSectors, sectorSize)
		acc.Require(partition.UnprovenPower.Equals(unprovenPower), "unproven power was %v, expected %v", partition.UnprovenPower, unprovenPower)
	}

	if faultySectors, missing, err := selectSectorsMap(sectors, partition.Faults); err != nil {
		acc.Addf("error selecting faulty sectors: %v", err)
	} else if len(missing) > 0 {
		acc.Addf("faulty sectors missing from all sectors: %v", missing)
	} else {
		faultyPower = powerForSectors(faultySectors, sectorSize)
		acc.Require(partition.FaultyPower.Equals(faultyPower), "faulty power was %v, expected %v", partition.FaultyPower, faultyPower)
	}

	if recoveringSectors, missing, err := selectSectorsMap(sectors, partition.Recoveries); err != nil {
		acc.Addf("error selecting recovering sectors: %v", err)
	} else if len(missing) > 0 {
		acc.Addf("recovering sectors missing from all sectors: %v", missing)
	} else {
		recoveringPower := powerForSectors(recoveringSectors, sectorSize)
		acc.Require(partition.RecoveringPower.Equals(recoveringPower), "recovering power was %v, expected %v", partition.RecoveringPower, recoveringPower)
	}

	activePower := livePower.Sub(faultyPower).Sub(unprovenPower)
	partitionActivePower := partition.ActivePower()
	acc.Require(partitionActivePower.Equals(activePower), "active power was %v, expected %v", partitionActivePower, activePower)

	// Validate the expiration queue.
	var expirationEpochs []abi.ChainEpoch
	if expQ, err := LoadExpirationQueue(store, partition.ExpirationsEpochs, quant); err != nil {
		acc.Addf("error loading expiration queue: %v", err)
	} else if liveSectors != nil {
		qsummary := CheckExpirationQueue(expQ, liveSectors, partition.Faults, quant, sectorSize, acc)
		expirationEpochs = qsummary.ExpirationEpochs

		// Check the queue is compatible with partition fields
		if qSectors, err := bitfield.MergeBitFields(qsummary.OnTimeSectors, qsummary.EarlySectors); err != nil {
			acc.Addf("error merging summary on-time and early sectors: %v", err)
		} else {
			requireEqual(live, qSectors, acc, "live does not equal all expirations")
		}
	}

	// Validate the early termination queue.
	earlyTerminationCount := 0
	if earlyQ, err := LoadBitfieldQueue(store, partition.EarlyTerminated, NoQuantization); err != nil {
		acc.Addf("error loading early termination queue: %v", err)
	} else {
		earlyTerminationCount = CheckEarlyTerminationQueue(earlyQ, partition.Terminated, acc)
	}

	return &PartitionStateSummary{
		AllSectors:            partition.Sectors,
		LiveSectors:           live,
		FaultySectors:         partition.Faults,
		RecoveringSectors:     partition.Recoveries,
		UnprovenSectors:       partition.Unproven,
		TerminatedSectors:     partition.Terminated,
		LivePower:             livePower,
		ActivePower:           activePower,
		FaultyPower:           partition.FaultyPower,
		RecoveringPower:       partition.RecoveringPower,
		ExpirationEpochs:      expirationEpochs,
		EarlyTerminationCount: earlyTerminationCount,
	}
}

type ExpirationQueueStateSummary struct {
	OnTimeSectors    bitfield.BitField
	EarlySectors     bitfield.BitField
	ActivePower      PowerPair
	FaultyPower      PowerPair
	OnTimePledge     abi.TokenAmount
	ExpirationEpochs []abi.ChainEpoch
}

// Checks the expiration queue for consistency.
func CheckExpirationQueue(expQ ExpirationQueue, liveSectors map[abi.SectorNumber]*SectorOnChainInfo,
	partitionFaults bitfield.BitField, quant QuantSpec, sectorSize abi.SectorSize, acc *builtin.MessageAccumulator) *ExpirationQueueStateSummary {
	partitionFaultsMap, err := partitionFaults.AllMap(1 << 30)
	if err != nil {
		acc.Addf("error loading partition faults map: %v", err)
		partitionFaultsMap = nil
	}

	seenSectors := make(map[abi.SectorNumber]bool)
	var allOnTime []bitfield.BitField
	var allEarly []bitfield.BitField
	var expirationEpochs []abi.ChainEpoch
	allActivePower := NewPowerPairZero()
	allFaultyPower := NewPowerPairZero()
	allOnTimePledge := big.Zero()
	firstQueueEpoch := abi.ChainEpoch(-1)
	var exp ExpirationSet
	err = expQ.ForEach(&exp, func(e int64) error {
		epoch := abi.ChainEpoch(e)
		acc := acc.WithPrefix("expiration epoch %d: ", epoch)
		acc.Require(quant.QuantizeUp(epoch) == epoch,
			"expiration queue key %d is not quantized, expected %d", epoch, quant.QuantizeUp(epoch))
		if firstQueueEpoch == abi.ChainEpoch(-1) {
			firstQueueEpoch = epoch
		}
		expirationEpochs = append(expirationEpochs, epoch)

		onTimeSectorsPledge := big.Zero()
		err := exp.OnTimeSectors.ForEach(func(n uint64) error {
			sno := abi.SectorNumber(n)
			// Check sectors are present only once.
			acc.Require(!seenSectors[sno], "sector %d in expiration queue twice", sno)
			seenSectors[sno] = true

			// Check expiring sectors are still alive.
			if sector, ok := liveSectors[sno]; ok {
				// The sector can be "on time" either at its target expiration epoch, or in the first queue entry
				// (a CC-replaced sector moved forward).
				target := quant.QuantizeUp(sector.Expiration)
				acc.Require(epoch == target || epoch == firstQueueEpoch, "invalid expiration %d for sector %d, expected %d or %d",
					epoch, sector.SectorNumber, firstQueueEpoch, target)

				onTimeSectorsPledge = big.Add(onTimeSectorsPledge, sector.InitialPledge)
			} else {
				acc.Addf("on-time expiration sector %d isn't live", n)
			}
			return nil
		})
		acc.RequireNoError(err, "error iterating on-time sectors")

		err = exp.EarlySectors.ForEach(func(n uint64) error {
			sno := abi.SectorNumber(n)
			// Check sectors are present only once.
			acc.Require(!seenSectors[sno], "sector %d in expiration queue twice", sno)
			seenSectors[sno] = true

			// Check early sectors are faulty
			acc.Require(partitionFaultsMap == nil || partitionFaultsMap[n], "sector %d expiring early but not faulty", sno)

			// Check expiring sectors are still alive.
			if sector, ok := liveSectors[sno]; ok {
				target := quant.QuantizeUp(sector.Expiration)
				acc.Require(epoch < target, "invalid early expiration %d for sector %d, expected < %d",
					epoch, sector.SectorNumber, target)
			} else {
				acc.Addf("on-time expiration sector %d isn't live", n)
			}
			return nil
		})
		acc.RequireNoError(err, "error iterating early sectors")

		// Validate power and pledge.
		var activeSectors, faultySectors map[abi.SectorNumber]*SectorOnChainInfo
		var missing []abi.SectorNumber

		all, err := bitfield.MergeBitFields(exp.OnTimeSectors, exp.EarlySectors)
		if err != nil {
			acc.Addf("error merging all on-time and early bitfields: %v", err)
		} else {
			if allActive, err := bitfield.SubtractBitField(all, partitionFaults); err != nil {
				acc.Addf("error computing active sectors: %v", err)
			} else {
				activeSectors, missing, err = selectSectorsMap(liveSectors, allActive)
				if err != nil {
					acc.Addf("error selecting active sectors: %v", err)
					activeSectors = nil
				} else if len(missing) > 0 {
					acc.Addf("active sectors missing from live: %v", missing)
				}
			}

			if allFaulty, err := bitfield.IntersectBitField(all, partitionFaults); err != nil {
				acc.Addf("error computing faulty sectors: %v", err)
			} else {
				faultySectors, missing, err = selectSectorsMap(liveSectors, allFaulty)
				if err != nil {
					acc.Addf("error selecting faulty sectors: %v", err)
					faultySectors = nil
				} else if len(missing) > 0 {
					acc.Addf("faulty sectors missing from live: %v", missing)
				}
			}
		}

		if activeSectors != nil && faultySectors != nil {
			activeSectorsPower := powerForSectors(activeSectors, sectorSize)
			acc.Require(exp.ActivePower.Equals(activeSectorsPower), "active power recorded %v doesn't match computed %v", exp.ActivePower, activeSectorsPower)

			faultySectorsPower := powerForSectors(faultySectors, sectorSize)
			acc.Require(exp.FaultyPower.Equals(faultySectorsPower), "faulty power recorded %v doesn't match computed %v", exp.FaultyPower, faultySectorsPower)
		}

		acc.Require(exp.OnTimePledge.Equals(onTimeSectorsPledge), "on time pledge recorded %v doesn't match computed %v", exp.OnTimePledge, onTimeSectorsPledge)

		allOnTime = append(allOnTime, exp.OnTimeSectors)
		allEarly = append(allEarly, exp.EarlySectors)
		allActivePower = allActivePower.Add(exp.ActivePower)
		allFaultyPower = allFaultyPower.Add(exp.FaultyPower)
		allOnTimePledge = big.Add(allOnTimePledge, exp.OnTimePledge)
		return nil
	})
	acc.RequireNoError(err, "error iterating expiration queue")

	unionOnTime, err := bitfield.MultiMerge(allOnTime...)
	if err != nil {
		acc.Addf("error merging on-time sector numbers: %v", err)
		unionOnTime = bitfield.New()
	}
	unionEarly, err := bitfield.MultiMerge(allEarly...)
	if err != nil {
		acc.Addf("error merging early sector numbers: %v", err)
		unionEarly = bitfield.New()
	}
	return &ExpirationQueueStateSummary{
		OnTimeSectors:    unionOnTime,
		EarlySectors:     unionEarly,
		ActivePower:      allActivePower,
		FaultyPower:      allFaultyPower,
		OnTimePledge:     allOnTimePledge,
		ExpirationEpochs: expirationEpochs,
	}
}

// Checks the early termination queue for consistency.
// Returns the number of sectors in the queue.
func CheckEarlyTerminationQueue(earlyQ BitfieldQueue, terminated bitfield.BitField, acc *builtin.MessageAccumulator) int {
	seenMap := make(map[uint64]bool)
	seenBf := bitfield.New()
	err := earlyQ.ForEach(func(epoch abi.ChainEpoch, bf bitfield.BitField) error {
		acc := acc.WithPrefix("early termination epoch %d: ", epoch)
		err := bf.ForEach(func(i uint64) error {
			acc.Require(!seenMap[i], "sector %v in early termination queue twice", i)
			seenMap[i] = true
			seenBf.Set(i)
			return nil
		})
		acc.RequireNoError(err, "error iterating early termination bitfield")
		return nil
	})
	acc.RequireNoError(err, "error iterating early termination queue")

	requireContainsAll(terminated, seenBf, acc, "terminated sectors missing early termination entry")
	return len(seenMap)
}

func CheckMinerInfo(info *MinerInfo, acc *builtin.MessageAccumulator) {
	acc.Require(info.Owner.Protocol() == addr.ID, "owner address %v is not an ID address", info.Owner)
	acc.Require(info.Worker.Protocol() == addr.ID, "worker address %v is not an ID address", info.Worker)
	for _, a := range info.ControlAddresses {
		acc.Require(a.Protocol() == addr.ID, "control address %v is not an ID address", a)
	}

	if info.PendingWorkerKey != nil {
		acc.Require(info.PendingWorkerKey.NewWorker.Protocol() == addr.ID,
			"pending worker address %v is not an ID address", info.PendingWorkerKey.NewWorker)
		acc.Require(info.PendingWorkerKey.NewWorker != info.Worker,
			"pending worker key %v is same as existing worker %v", info.PendingWorkerKey.NewWorker, info.Worker)
	}

	if info.PendingOwnerAddress != nil {
		acc.Require(info.PendingOwnerAddress.Protocol() == addr.ID,
			"pending owner address %v is not an ID address", info.PendingOwnerAddress)
		acc.Require(*info.PendingOwnerAddress != info.Owner,
			"pending owner address %v is same as existing owner %v", info.PendingOwnerAddress, info.Owner)
	}

	sealProofInfo, found := abi.SealProofInfos[info.SealProofType]
	acc.Require(found, "miner has unrecognized seal proof type %d", info.SealProofType)
	if found {
		acc.Require(sealProofInfo.SectorSize == info.SectorSize,
			"sector size %d is wrong for seal proof type %d: %d", info.SectorSize, info.SealProofType, sealProofInfo.SectorSize)
	}
	sealProofPolicy, found := builtin.SealProofPolicies[info.SealProofType]
	acc.Require(found, "no seal proof policy exists for proof type %d", info.SealProofType)
	if found {
		acc.Require(sealProofPolicy.WindowPoStPartitionSectors == info.WindowPoStPartitionSectors,
			"miner partition sectors %d does not match partition sectors %d for seal proof type %d",
			info.WindowPoStPartitionSectors, sealProofPolicy.WindowPoStPartitionSectors, info.SealProofType)
	}
}

func CheckMinerBalances(st *State, store adt.Store, balance abi.TokenAmount, acc *builtin.MessageAccumulator) {
	acc.Require(balance.GreaterThanEqual(big.Zero()), "miner actor balance is less than zero: %v", balance)
	acc.Require(st.LockedFunds.GreaterThanEqual(big.Zero()), "miner locked funds is less than zero: %v", st.LockedFunds)
	acc.Require(st.PreCommitDeposits.GreaterThanEqual(big.Zero()), "miner precommit deposit is less than zero: %v", st.PreCommitDeposits)
	acc.Require(st.InitialPledge.GreaterThanEqual(big.Zero()), "miner initial pledge is less than zero: %v", st.InitialPledge)
	acc.Require(st.FeeDebt.GreaterThanEqual(big.Zero()), "miner fee debt is less than zero: %v", st.FeeDebt)

	acc.Require(big.Subtract(balance, st.LockedFunds, st.PreCommitDeposits, st.InitialPledge).GreaterThanEqual(big.Zero()),
		"miner balance (%v) is less than sum of locked funds (%v), precommit deposit (%v), and initial pledge (%v)",
		balance, st.LockedFunds, st.PreCommitDeposits, st.InitialPledge)

	// locked funds must be sum of vesting table and vesting table payments must be quantized
	vestingSum := big.Zero()
	if funds, err := st.LoadVestingFunds(store); err != nil {
		acc.Addf("error loading vesting funds: %v", err)
	} else {
		quant := st.QuantSpecEveryDeadline()
		for _, entry := range funds.Funds {
			acc.Require(entry.Amount.GreaterThan(big.Zero()), "non-positive amount in miner vesting table entry %v", entry)
			vestingSum = big.Add(vestingSum, entry.Amount)

			quantized := quant.QuantizeUp(entry.Epoch)
			acc.Require(entry.Epoch == quantized, "vesting table entry has non-quantized epoch %d (should be %d)", entry.Epoch, quantized)
		}
	}

	acc.Require(st.LockedFunds.Equals(vestingSum),
		"locked funds %d is not sum of vesting table entries %d", st.LockedFunds, vestingSum)
}

func CheckPreCommits(st *State, store adt.Store, allocatedSectors map[uint64]bool, acc *builtin.MessageAccumulator) {
	quant := st.QuantSpecEveryDeadline()

	// invert pre-commit expiry queue into a lookup by sector number
	expireEpochs := make(map[uint64]abi.ChainEpoch)
	if expiryQ, err := LoadBitfieldQueue(store, st.PreCommittedSectorsExpiry, st.QuantSpecEveryDeadline()); err != nil {
		acc.Addf("error loading pre-commit expiry queue: %v", err)
	} else {
		err = expiryQ.ForEach(func(epoch abi.ChainEpoch, bf bitfield.BitField) error {
			quantized := quant.QuantizeUp(epoch)
			acc.Require(quantized == epoch, "precommit expiration %d is not quantized", epoch)
			if err = bf.ForEach(func(secNum uint64) error {
				expireEpochs[secNum] = epoch
				return nil
			}); err != nil {
				acc.Addf("error iteration pre-commit expiration bitfield: %v", err)
			}
			return nil
		})
		acc.RequireNoError(err, "error iterating pre-commit expiry queue")
	}

	precommitTotal := big.Zero()
	if precommitted, err := adt.AsMap(store, st.PreCommittedSectors); err != nil {
		acc.Addf("error loading precommitted sectors: %v", err)
	} else {
		var precommit SectorPreCommitOnChainInfo
		err = precommitted.ForEach(&precommit, func(key string) error {
			secNum, err := abi.ParseUIntKey(key)
			if err != nil {
				acc.Addf("error parsing pre-commit key as uint: %v", err)
				return nil
			}

			acc.Require(allocatedSectors[secNum], "pre-committed sector number has not been allocated %d", secNum)

			_, found := expireEpochs[secNum]
			acc.Require(found, "no expiry epoch for pre-commit at %d", precommit.PreCommitEpoch)

			precommitTotal = big.Add(precommitTotal, precommit.PreCommitDeposit)
			return nil
		})
		acc.RequireNoError(err, "error iterating pre-committed sectors")
	}

	acc.Require(st.PreCommitDeposits.Equals(precommitTotal),
		"sum of precommit deposits %v does not equal recorded precommit deposit %v", precommitTotal, st.PreCommitDeposits)
}

// Selects a subset of sectors from a map by sector number.
// Returns the selected sectors, and a slice of any sector numbers not found.
func selectSectorsMap(sectors map[abi.SectorNumber]*SectorOnChainInfo, include bitfield.BitField) (map[abi.SectorNumber]*SectorOnChainInfo, []abi.SectorNumber, error) {
	included := map[abi.SectorNumber]*SectorOnChainInfo{}
	missing := []abi.SectorNumber{}
	if err := include.ForEach(func(n uint64) error {
		if s, ok := sectors[abi.SectorNumber(n)]; ok {
			included[abi.SectorNumber(n)] = s
		} else {
			missing = append(missing, abi.SectorNumber(n))
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}
	return included, missing, nil
}

func powerForSectors(sectors map[abi.SectorNumber]*SectorOnChainInfo, ssize abi.SectorSize) PowerPair {
	qa := big.Zero()
	for _, s := range sectors { // nolint:nomaprange
		qa = big.Add(qa, QAPowerForSector(ssize, s))
	}

	return PowerPair{
		Raw: big.Mul(big.NewIntUnsigned(uint64(ssize)), big.NewIntUnsigned(uint64(len(sectors)))),
		QA:  qa,
	}
}

func requireContainsAll(superset, subset bitfield.BitField, acc *builtin.MessageAccumulator, msg string) {
	if contains, err := util.BitFieldContainsAll(superset, subset); err != nil {
		acc.Addf("error in BitfieldContainsAll(): %v", err)
	} else if !contains {
		acc.Addf(msg+": %v, %v", superset, subset)
		// Verbose output for debugging
		//sup, err := superset.All(1 << 20)
		//if err != nil {
		//	acc.Addf("error in Bitfield.All(): %v", err)
		//	return
		//}
		//sub, err := subset.All(1 << 20)
		//if err != nil {
		//	acc.Addf("error in Bitfield.All(): %v", err)
		//	return
		//}
		//acc.Addf(msg+": %v, %v", sup, sub)
	}
}

func requireContainsNone(superset, subset bitfield.BitField, acc *builtin.MessageAccumulator, msg string) {
	if contains, err := util.BitFieldContainsAny(superset, subset); err != nil {
		acc.Addf("error in BitfieldContainsAny(): %v", err)
	} else if contains {
		acc.Addf(msg+": %v, %v", superset, subset)
		// Verbose output for debugging
		//sup, err := superset.All(1 << 20)
		//if err != nil {
		//	acc.Addf("error in Bitfield.All(): %v", err)
		//	return
		//}
		//sub, err := subset.All(1 << 20)
		//if err != nil {
		//	acc.Addf("error in Bitfield.All(): %v", err)
		//	return
		//}
		//acc.Addf(msg+": %v, %v", sup, sub)
	}
}

func requireEqual(a, b bitfield.BitField, acc *builtin.MessageAccumulator, msg string) {
	requireContainsAll(a, b, acc, msg)
	requireContainsAll(b, a, acc, msg)
}
