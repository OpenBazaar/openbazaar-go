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

type StateSummary struct {
	LivePower     PowerPair
	ActivePower   PowerPair
	FaultyPower   PowerPair
	SealProofType abi.RegisteredSealProof
}

// Checks internal invariants of init state.
func CheckStateInvariants(st *State, store adt.Store, balance abi.TokenAmount) (*StateSummary, *builtin.MessageAccumulator, error) {
	acc := &builtin.MessageAccumulator{}

	// Load data from linked structures.
	info, err := st.GetInfo(store)
	if err != nil {
		return nil, nil, err
	}
	msgs, err := CheckMinerInfo(info)
	acc.AddAll(msgs)
	if err != nil {
		return nil, acc, err
	}

	msgs, err = CheckMinerBalances(st, store, balance)
	acc.AddAll(msgs)
	if err != nil {
		return nil, acc, err
	}

	var allocatedSectors bitfield.BitField
	if err := store.Get(store.Context(), st.AllocatedSectors, &allocatedSectors); err != nil {
		return nil, acc, err
	}
	allocatedSectorsMap, err := allocatedSectors.AllMap(1 << 30)
	if err != nil {
		return nil, nil, err
	}

	msgs, err = CheckPreCommits(st, store, allocatedSectorsMap)
	acc.AddAll(msgs)
	if err != nil {
		return nil, acc, err
	}

	allSectors := map[abi.SectorNumber]*SectorOnChainInfo{}
	if sectorsArr, err := adt.AsArray(store, st.Sectors); err != nil {
		return nil, nil, err
	} else {
		var sector SectorOnChainInfo
		if err = sectorsArr.ForEach(&sector, func(sno int64) error {
			cpy := sector
			allSectors[abi.SectorNumber(sno)] = &cpy
			acc.Require(allocatedSectorsMap[uint64(sno)], "on chain sector's sector number has not been allocated %d", sno)
			return nil
		}); err != nil {
			return nil, nil, err
		}
	}

	deadlines, err := st.LoadDeadlines(store)
	if err != nil {
		return nil, nil, err
	}

	livePower := NewPowerPairZero()
	activePower := NewPowerPairZero()
	faultyPower := NewPowerPairZero()

	// Check deadlines
	acc.Require(st.CurrentDeadline < WPoStPeriodDeadlines,
		"current deadline index is greater than deadlines per period(%d): %d", WPoStPeriodDeadlines, st.CurrentDeadline)

	if err := deadlines.ForEach(store, func(dlIdx uint64, dl *Deadline) error {
		acc := acc.WithPrefix("deadline %d: ", dlIdx) // Shadow
		quant := st.QuantSpecForDeadline(dlIdx)
		summary, msgs, err := CheckDeadlineStateInvariants(dl, store, quant, info.SectorSize, allSectors)
		if err != nil {
			return err
		}
		acc.AddAll(msgs)

		livePower = livePower.Add(summary.LivePower)
		activePower = activePower.Add(summary.ActivePower)
		faultyPower = faultyPower.Add(summary.FaultyPower)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	return &StateSummary{
		LivePower:     livePower,
		ActivePower:   activePower,
		FaultyPower:   faultyPower,
		SealProofType: info.SealProofType,
	}, acc, nil
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

func CheckDeadlineStateInvariants(deadline *Deadline, store adt.Store, quant QuantSpec, ssize abi.SectorSize, sectors map[abi.SectorNumber]*SectorOnChainInfo) (*DeadlineStateSummary, *builtin.MessageAccumulator, error) {
	acc := &builtin.MessageAccumulator{}

	// Load linked structures.
	partitions, err := deadline.PartitionsArray(store)
	if err != nil {
		return nil, nil, err
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
	if err = partitions.ForEach(&partition, func(i int64) error {
		pIdx := uint64(i)
		// Check sequential partitions.
		acc.Require(pIdx == partitionCount, "Non-sequential partitions, expected index %d, found %d", partitionCount, pIdx)
		partitionCount++

		acc := acc.WithPrefix("partition %d: ", pIdx) // Shadow
		summary, msgs, err := CheckPartitionStateInvariants(&partition, store, quant, ssize, sectors)
		if err != nil {
			return err
		}
		acc.AddAll(msgs)

		if contains, err := util.BitFieldContainsAny(allSectors, summary.AllSectors); err != nil {
			return err
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
			return err
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
	}); err != nil {
		return nil, nil, err
	}

	// Check PoSt submissions
	postSubmissions, err := deadline.PostSubmissions.All(1 << 20)
	if err != nil {
		return nil, nil, err
	}
	for _, p := range postSubmissions {
		acc.Require(p <= partitionCount, "invalid PoSt submission for partition %d of %d", p, partitionCount)
	}

	// Check memoized sector and power values.
	live, err := bitfield.MultiMerge(allLiveSectors...)
	if err != nil {
		return nil, nil, err
	}
	if liveCount, err := live.Count(); err != nil {
		return nil, nil, err
	} else {
		acc.Require(deadline.LiveSectors == liveCount, "deadline live sectors %d != partitions count %d", deadline.LiveSectors, liveCount)
	}

	if allCount, err := allSectors.Count(); err != nil {
		return nil, nil, err
	} else {
		acc.Require(deadline.TotalSectors == allCount, "deadline total sectors %d != partitions count %d", deadline.TotalSectors, allCount)
	}

	faulty, err := bitfield.MultiMerge(allFaultySectors...)
	if err != nil {
		return nil, nil, err
	}
	recovering, err := bitfield.MultiMerge(allRecoveringSectors...)
	if err != nil {
		return nil, nil, err
	}
	unproven, err := bitfield.MultiMerge(allUnprovenSectors...)
	if err != nil {
		return nil, nil, err
	}
	terminated, err := bitfield.MultiMerge(allTerminatedSectors...)
	if err != nil {
		return nil, nil, err
	}

	acc.Require(deadline.FaultyPower.Equals(allFaultyPower), "deadline faulty power %v != partitions total %v", deadline.FaultyPower, allFaultyPower)

	{
		// Validate partition expiration queue contains an entry for each partition and epoch with an expiration.
		// The queue may be a superset of the partitions that have expirations because we never remove from it.
		expirationEpochs, err := adt.AsArray(store, deadline.ExpirationsEpochs)
		if err != nil {
			return nil, nil, err
		}
		for epoch, expiringPIdxs := range partitionsWithExpirations { // nolint:nomaprange
			var bf bitfield.BitField
			found, err := expirationEpochs.Get(uint64(epoch), &bf)
			if err != nil {
				return nil, nil, err
			}
			acc.Require(found, "expected to find partitions with expirations at epoch %d", epoch)

			queuedPIdxs, err := bf.AllMap(1 << 20)
			if err != nil {
				return nil, nil, err
			}

			for _, p := range expiringPIdxs {
				acc.Require(queuedPIdxs[p], "expected partition %d to be present in deadline expiration queue at epoch %d", p, epoch)
			}
		}
	}
	{
		// Validate the early termination queue contains exactly the partitions with early terminations.
		expected := bitfield.NewFromSet(partitionsWithEarlyTerminations)
		if err = requireEqual(expected, deadline.EarlyTerminations, acc, "deadline early terminations doesn't match expected partitions"); err != nil {
			return nil, nil, err
		}
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
	}, acc, nil
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
	ExpirationEpochs      []abi.ChainEpoch // Epochs at which some sector is scheduled to expire.
	EarlyTerminationCount int
}

func CheckPartitionStateInvariants(
	partition *Partition,
	store adt.Store,
	quant QuantSpec,
	sectorSize abi.SectorSize,
	sectors map[abi.SectorNumber]*SectorOnChainInfo,
) (*PartitionStateSummary, *builtin.MessageAccumulator, error) {
	acc := &builtin.MessageAccumulator{}

	live, err := partition.LiveSectors()
	if err != nil {
		return nil, nil, err
	}
	active, err := partition.ActiveSectors()
	if err != nil {
		return nil, nil, err
	}

	// Live contains all active sectors.
	if err = requireContainsAll(live, active, acc, "live does not contain active"); err != nil {
		return nil, nil, err
	}

	// Live contains all faults.
	if err = requireContainsAll(live, partition.Faults, acc, "live does not contain faults"); err != nil {
		return nil, nil, err
	}

	// Live contains all unproven.
	if err = requireContainsAll(live, partition.Unproven, acc, "live does not contain unproven"); err != nil {
		return nil, nil, err
	}

	// Active contains no faults
	if err = requireContainsNone(active, partition.Faults, acc, "active includes faults"); err != nil {
		return nil, nil, err
	}

	// Active contains no unproven
	if err = requireContainsNone(active, partition.Unproven, acc, "active includes unproven"); err != nil {
		return nil, nil, err
	}

	// Faults contains all recoveries.
	if err = requireContainsAll(partition.Faults, partition.Recoveries, acc, "faults do not contain recoveries"); err != nil {
		return nil, nil, err
	}

	// Live contains no terminated sectors
	if err = requireContainsNone(live, partition.Terminated, acc, "live includes terminations"); err != nil {
		return nil, nil, err
	}

	// Unproven contains no faults
	if err = requireContainsNone(partition.Faults, partition.Unproven, acc, "unproven includes faults"); err != nil {
		return nil, nil, err
	}

	// All terminated sectors are part of the partition.
	if err = requireContainsAll(partition.Sectors, partition.Terminated, acc, "sectors do not contain terminations"); err != nil {
		return nil, nil, err
	}

	liveSectors, missing, err := selectSectorsMap(sectors, live)
	if err != nil {
		return nil, nil, err
	} else if len(missing) > 0 {
		acc.Addf("live sectors missing from all sectors: %v", missing)
	}
	unprovenSectors, missing, err := selectSectorsMap(sectors, partition.Unproven)
	if err != nil {
		return nil, nil, err
	} else if len(missing) > 0 {
		acc.Addf("unproven sectors missing from all sectors: %v", missing)
	}

	// Validate power
	faultySectors, missing, err := selectSectorsMap(sectors, partition.Faults)
	if err != nil {
		return nil, nil, err
	} else if len(missing) > 0 {
		acc.Addf("faulty sectors missing from all sectors: %v", missing)
	}
	faultyPower := powerForSectors(faultySectors, sectorSize)
	acc.Require(partition.FaultyPower.Equals(faultyPower), "faulty power was %v, expected %v", partition.FaultyPower, faultyPower)

	recoveringSectors, missing, err := selectSectorsMap(sectors, partition.Recoveries)
	if err != nil {
		return nil, nil, err
	} else if len(missing) > 0 {
		acc.Addf("recovering sectors missing from all sectors: %v", missing)
	}
	recoveringPower := powerForSectors(recoveringSectors, sectorSize)
	acc.Require(partition.RecoveringPower.Equals(recoveringPower), "recovering power was %v, expected %v", partition.RecoveringPower, recoveringPower)

	livePower := powerForSectors(liveSectors, sectorSize)
	acc.Require(partition.LivePower.Equals(livePower), "live power was %v, expected %v", partition.LivePower, livePower)

	unprovenPower := powerForSectors(unprovenSectors, sectorSize)
	acc.Require(partition.UnprovenPower.Equals(unprovenPower), "unproven power was %v, expected %v", partition.UnprovenPower, unprovenPower)

	activePower := livePower.Sub(faultyPower).Sub(unprovenPower)
	partitionActivePower := partition.ActivePower()
	acc.Require(partitionActivePower.Equals(activePower), "active power was %v, expected %v", partitionActivePower, activePower)

	// Validate the expiration queue.
	expQ, err := LoadExpirationQueue(store, partition.ExpirationsEpochs, quant)
	if err != nil {
		return nil, nil, err
	}
	qsummary, err := CheckExpirationQueue(expQ, liveSectors, partition.Faults, quant, sectorSize, acc)
	if err != nil {
		return nil, nil, err
	}

	// Check the queue is compatible with partition fields
	qSectors, err := bitfield.MergeBitFields(qsummary.OnTimeSectors, qsummary.EarlySectors)
	if err != nil {
		return nil, nil, err
	}
	if err = requireEqual(live, qSectors, acc, "live does not equal all expirations"); err != nil {
		return nil, nil, err
	}

	// Validate the early termination queue.
	earlyQ, err := LoadBitfieldQueue(store, partition.EarlyTerminated, NoQuantization)
	if err != nil {
		return nil, nil, err
	}
	earlyTerminationCount, err := CheckEarlyTerminationQueue(earlyQ, partition.Terminated, acc)
	if err != nil {
		return nil, nil, err
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
		EarlyTerminationCount: earlyTerminationCount,
	}, acc, nil
}

type ExpirationQueueStateSummary struct {
	OnTimeSectors bitfield.BitField
	EarlySectors  bitfield.BitField
	ActivePower   PowerPair
	FaultyPower   PowerPair
	OnTimePledge  abi.TokenAmount
}

// Checks the expiration queue for consistency.
func CheckExpirationQueue(expQ ExpirationQueue, liveSectors map[abi.SectorNumber]*SectorOnChainInfo,
	partitionFaults bitfield.BitField, quant QuantSpec, sectorSize abi.SectorSize, acc *builtin.MessageAccumulator) (*ExpirationQueueStateSummary, error) {
	partitionFaultsMap, err := partitionFaults.AllMap(1 << 30)
	if err != nil {
		return nil, err
	}

	seenSectors := make(map[abi.SectorNumber]bool)
	var allOnTime []bitfield.BitField
	var allEarly []bitfield.BitField
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

		onTimeSectorsPledge := big.Zero()
		if err := exp.OnTimeSectors.ForEach(func(n uint64) error {
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
		}); err != nil {
			return err
		}

		if err := exp.EarlySectors.ForEach(func(n uint64) error {
			sno := abi.SectorNumber(n)
			// Check sectors are present only once.
			acc.Require(!seenSectors[sno], "sector %d in expiration queue twice", sno)
			seenSectors[sno] = true

			// Check early sectors are faulty
			acc.Require(partitionFaultsMap[n], "sector %d expiring early but not faulty", sno)

			// Check expiring sectors are still alive.
			if sector, ok := liveSectors[sno]; ok {
				target := quant.QuantizeUp(sector.Expiration)
				acc.Require(epoch < target, "invalid early expiration %d for sector %d, expected < %d",
					epoch, sector.SectorNumber, target)
			} else {
				acc.Addf("on-time expiration sector %d isn't live", n)
			}

			return nil
		}); err != nil {
			return err
		}

		// Validate power and pledge.
		all, err := bitfield.MergeBitFields(exp.OnTimeSectors, exp.EarlySectors)
		if err != nil {
			return err
		}
		allActive, err := bitfield.SubtractBitField(all, partitionFaults)
		if err != nil {
			return err
		}
		allFaulty, err := bitfield.IntersectBitField(all, partitionFaults)
		if err != nil {
			return err
		}
		activeSectors, missing, err := selectSectorsMap(liveSectors, allActive)
		if err != nil {
			return err
		} else if len(missing) > 0 {
			acc.Addf("active sectors missing from live: %v", missing)
		}
		faultySectors, missing, err := selectSectorsMap(liveSectors, allFaulty)
		if err != nil {
			return err
		} else if len(missing) > 0 {
			acc.Addf("faulty sectors missing from live: %v", missing)
		}
		activeSectorsPower := powerForSectors(activeSectors, sectorSize)
		acc.Require(exp.ActivePower.Equals(activeSectorsPower), "active power recorded %v doesn't match computed %v", exp.ActivePower, activeSectorsPower)

		faultySectorsPower := powerForSectors(faultySectors, sectorSize)
		acc.Require(exp.FaultyPower.Equals(faultySectorsPower), "faulty power recorded %v doesn't match computed %v", exp.FaultyPower, faultySectorsPower)

		acc.Require(exp.OnTimePledge.Equals(onTimeSectorsPledge), "on time pledge recorded %v doesn't match computed %v", exp.OnTimePledge, onTimeSectorsPledge)

		allOnTime = append(allOnTime, exp.OnTimeSectors)
		allEarly = append(allEarly, exp.EarlySectors)
		allActivePower = allActivePower.Add(exp.ActivePower)
		allFaultyPower = allFaultyPower.Add(exp.FaultyPower)
		allOnTimePledge = big.Add(allOnTimePledge, exp.OnTimePledge)
		return nil
	})
	if err != nil {
		return nil, err
	}

	unionOnTime, err := bitfield.MultiMerge(allOnTime...)
	if err != nil {
		return nil, err
	}
	unionEarly, err := bitfield.MultiMerge(allEarly...)
	if err != nil {
		return nil, err
	}
	return &ExpirationQueueStateSummary{
		OnTimeSectors: unionOnTime,
		EarlySectors:  unionEarly,
		ActivePower:   allActivePower,
		FaultyPower:   allFaultyPower,
		OnTimePledge:  allOnTimePledge,
	}, nil
}

// Checks the early termination queue for consistency.
// Returns the number of sectors in the queue.
func CheckEarlyTerminationQueue(earlyQ BitfieldQueue, terminated bitfield.BitField, acc *builtin.MessageAccumulator) (int, error) {
	seenMap := make(map[uint64]bool)
	seenBf := bitfield.New()
	if err := earlyQ.ForEach(func(epoch abi.ChainEpoch, bf bitfield.BitField) error {
		acc := acc.WithPrefix("early termination epoch %d: ", epoch)
		return bf.ForEach(func(i uint64) error {
			acc.Require(!seenMap[i], "sector %v in early termination queue twice", i)
			seenMap[i] = true
			seenBf.Set(i)
			return nil
		})
	}); err != nil {
		return 0, err
	}

	if err := requireContainsAll(terminated, seenBf, acc, "terminated sectors missing early termination entry"); err != nil {
		return 0, err
	}
	return len(seenMap), nil
}

func CheckMinerInfo(info *MinerInfo) (*builtin.MessageAccumulator, error) {
	acc := &builtin.MessageAccumulator{}

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

	return acc, nil
}

func CheckMinerBalances(st *State, store adt.Store, balance abi.TokenAmount) (*builtin.MessageAccumulator, error) {
	acc := &builtin.MessageAccumulator{}

	acc.Require(balance.GreaterThanEqual(big.Zero()), "miner actor balance is less than zero: %v", balance)
	acc.Require(st.LockedFunds.GreaterThanEqual(big.Zero()), "miner locked funds is less than zero: %v", st.LockedFunds)
	acc.Require(st.PreCommitDeposits.GreaterThanEqual(big.Zero()), "miner precommit deposit is less than zero: %v", st.PreCommitDeposits)
	acc.Require(st.InitialPledge.GreaterThanEqual(big.Zero()), "miner initial pledge is less than zero: %v", st.InitialPledge)
	acc.Require(st.FeeDebt.GreaterThanEqual(big.Zero()), "miner fee debt is less than zero: %v", st.FeeDebt)

	acc.Require(big.Subtract(balance, st.LockedFunds, st.PreCommitDeposits, st.InitialPledge).GreaterThanEqual(big.Zero()),
		"miner balance (%v) is less than sum of locked funds (%v), precommit deposit (%v), and initial pledge (%v)",
		balance, st.LockedFunds, st.PreCommitDeposits, st.InitialPledge)

	// locked funds must be sum of vesting table and vesting table payments must be quantized
	funds, err := st.LoadVestingFunds(store)
	if err != nil {
		return acc, err
	}

	vestingSum := big.Zero()
	quant := st.QuantSpecEveryDeadline()
	for _, entry := range funds.Funds {
		acc.Require(entry.Amount.GreaterThan(big.Zero()), "non-positive amount in miner vesting table entry %v", entry)
		vestingSum = big.Add(vestingSum, entry.Amount)

		quantized := quant.QuantizeUp(entry.Epoch)
		acc.Require(entry.Epoch == quantized, "vesting table entry has non-quantized epoch %d (should be %d)", entry.Epoch, quantized)
	}

	acc.Require(st.LockedFunds.Equals(vestingSum),
		"locked funds %d is not sum of vesting table entries %d", st.LockedFunds, vestingSum)

	return acc, nil
}

func CheckPreCommits(st *State, store adt.Store, allocatedSectors map[uint64]bool) (*builtin.MessageAccumulator, error) {
	acc := &builtin.MessageAccumulator{}

	// expire pre-committed sectors
	expiryQ, err := LoadBitfieldQueue(store, st.PreCommittedSectorsExpiry, st.QuantSpecEveryDeadline())
	if err != nil {
		return acc, err
	}

	quant := st.QuantSpecEveryDeadline()

	// invert bitfield queue into a lookup by sector number
	expireEpochs := make(map[uint64]abi.ChainEpoch)
	err = expiryQ.ForEach(func(epoch abi.ChainEpoch, bf bitfield.BitField) error {
		quantized := quant.QuantizeUp(epoch)
		acc.Require(quantized == epoch, "precommit expiration %d is not quantized", epoch)

		return bf.ForEach(func(secNum uint64) error {
			expireEpochs[secNum] = epoch
			return nil
		})
	})
	if err != nil {
		return acc, err
	}

	precommitted, err := adt.AsMap(store, st.PreCommittedSectors)
	if err != nil {
		return nil, err
	}

	var precommit SectorPreCommitOnChainInfo
	precommitTotal := big.Zero()
	err = precommitted.ForEach(&precommit, func(key string) error {
		secNum, err := abi.ParseUIntKey(key)
		if err != nil {
			return err
		}

		acc.Require(allocatedSectors[secNum], "precommited sector number has not been allocated %d", secNum)

		_, found := expireEpochs[secNum]
		acc.Require(found, "no expiry epoch for precommit at %d", precommit.PreCommitEpoch)

		precommitTotal = big.Add(precommitTotal, precommit.PreCommitDeposit)

		return nil
	})
	if err != nil {
		return acc, err
	}

	acc.Require(st.PreCommitDeposits.Equals(precommitTotal),
		"sum of precommit deposits %v does not equal recorded precommit deposit %v", precommitTotal, st.PreCommitDeposits)

	return acc, nil
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

func requireContainsAll(superset, subset bitfield.BitField, acc *builtin.MessageAccumulator, msg string) error {
	contains, err := util.BitFieldContainsAll(superset, subset)
	if err != nil {
		return err
	}
	if !contains {
		acc.Addf(msg+": %v, %v", superset, subset)
		// Verbose output for debugging
		//sup, err := superset.All(1 << 20)
		//if err != nil {
		//	return err
		//}
		//sub, err := subset.All(1 << 20)
		//if err != nil {
		//	return err
		//}
		//acc.Addf(msg+": %v, %v", sup, sub)
	}
	return nil
}

func requireContainsNone(superset, subset bitfield.BitField, acc *builtin.MessageAccumulator, msg string) error {
	contains, err := util.BitFieldContainsAny(superset, subset)
	if err != nil {
		return err
	}
	if contains {
		acc.Addf(msg+": %v, %v", superset, subset)
		// Verbose output for debugging
		//sup, err := superset.All(1 << 20)
		//if err != nil {
		//	return err
		//}
		//sub, err := subset.All(1 << 20)
		//if err != nil {
		//	return err
		//}
		//acc.Addf(msg+": %v, %v", sup, sub)
	}
	return nil
}

func requireEqual(a, b bitfield.BitField, acc *builtin.MessageAccumulator, msg string) error {
	if err := requireContainsAll(a, b, acc, msg); err != nil {
		return err
	}
	if err := requireContainsAll(b, a, acc, msg); err != nil {
		return err
	}
	return nil
}
