package miner

import (
	"container/heap"

	"golang.org/x/xerrors"
)

// Helper types for deadline assignment.
type deadlineAssignmentInfo struct {
	index        int
	liveSectors  uint64
	totalSectors uint64
}

func (dai *deadlineAssignmentInfo) partitionsAfterAssignment(partitionSize uint64) uint64 {
	sectorCount := dai.totalSectors + 1 // after assignment
	fullPartitions := sectorCount / partitionSize
	if (sectorCount % partitionSize) == 0 {
		return fullPartitions
	}
	return fullPartitions + 1 // +1 for partial partition.
}

func (dai *deadlineAssignmentInfo) compactPartitionsAfterAssignment(partitionSize uint64) uint64 {
	sectorCount := dai.liveSectors + 1 // after assignment
	fullPartitions := sectorCount / partitionSize
	if (sectorCount % partitionSize) == 0 {
		return fullPartitions
	}
	return fullPartitions + 1 // +1 for partial partition.
}

func (dai *deadlineAssignmentInfo) isFullNow(partitionSize uint64) bool {
	return (dai.totalSectors % partitionSize) == 0
}

func (dai *deadlineAssignmentInfo) maxPartitionsReached(partitionSize, maxPartitions uint64) bool {
	return dai.totalSectors >= partitionSize*maxPartitions
}

type deadlineAssignmentHeap struct {
	maxPartitions uint64
	partitionSize uint64
	deadlines     []*deadlineAssignmentInfo
}

func (dah *deadlineAssignmentHeap) Len() int {
	return len(dah.deadlines)
}

func (dah *deadlineAssignmentHeap) Swap(i, j int) {
	dah.deadlines[i], dah.deadlines[j] = dah.deadlines[j], dah.deadlines[i]
}

func (dah *deadlineAssignmentHeap) Less(i, j int) bool {
	a, b := dah.deadlines[i], dah.deadlines[j]

	// If one of the deadlines has already reached it's limit for the maximum number of partitions and
	// the other hasn't, we directly pick the deadline that hasn't reached it's limit.
	aMaxPartitionsreached := a.maxPartitionsReached(dah.partitionSize, dah.maxPartitions)
	bMaxPartitionsReached := b.maxPartitionsReached(dah.partitionSize, dah.maxPartitions)
	if aMaxPartitionsreached != bMaxPartitionsReached {
		return !aMaxPartitionsreached
	}

	// Otherwise:-
	// When assigning partitions to deadlines, we're trying to optimize the
	// following:
	//
	// First, avoid increasing the maximum number of partitions in any
	// deadline, across all deadlines, after compaction. This would
	// necessitate buying a new GPU.
	//
	// Second, avoid forcing the miner to repeatedly compact partitions. A
	// miner would be "forced" to compact a partition when a the number of
	// partitions in any given deadline goes above the current maximum
	// number of partitions across all deadlines, and compacting that
	// deadline would then reduce the number of partitions, reducing the
	// maximum.
	//
	// At the moment, the only "forced" compaction happens when either:
	//
	// 1. Assignment of the sector into any deadline would force a
	//    compaction.
	// 2. The chosen deadline has at least one full partition's worth of
	//    terminated sectors and at least one fewer partition (after
	//    compaction) than any other deadline.
	//
	// Third, we attempt to assign "runs" of sectors to the same partition
	// to reduce the size of the bitfields.
	//
	// Finally, we try to balance the number of sectors (thus partitions)
	// assigned to any given deadline over time.

	// Summary:
	//
	// 1. Assign to the deadline that will have the _least_ number of
	//    post-compaction partitions (after sector assignment).
	// 2. Assign to the deadline that will have the _least_ number of
	//    pre-compaction partitions (after sector assignment).
	// 3. Assign to a deadline with a non-full partition.
	//    - If both have non-full partitions, assign to the most full one (stable assortment).
	// 4. Assign to the deadline with the least number of live sectors.
	// 5. Assign sectors to the deadline with the lowest index first.

	// If one deadline would end up with fewer partitions (after
	// compacting), assign to that one. This ensures we keep the maximum
	// number of partitions in any given deadline to a minimum.
	//
	// Technically, this could increase the maximum number of partitions
	// before compaction. However, that can only happen if the deadline in
	// question could save an entire partition by compacting. At that point,
	// the miner should compact the deadline.
	aCompactPartitionsAfterAssignment := a.compactPartitionsAfterAssignment(dah.partitionSize)
	bCompactPartitionsAfterAssignment := b.compactPartitionsAfterAssignment(dah.partitionSize)
	if aCompactPartitionsAfterAssignment != bCompactPartitionsAfterAssignment {
		return aCompactPartitionsAfterAssignment < bCompactPartitionsAfterAssignment
	}

	// If, after assignment, neither deadline would have fewer
	// post-compaction partitions, assign to the deadline with the fewest
	// pre-compaction partitions (after assignment). This will put off
	// compaction as long as possible.
	aPartitionsAfterAssignment := a.partitionsAfterAssignment(dah.partitionSize)
	bPartitionsAfterAssignment := b.partitionsAfterAssignment(dah.partitionSize)
	if aPartitionsAfterAssignment != bPartitionsAfterAssignment {
		return aPartitionsAfterAssignment < bPartitionsAfterAssignment
	}

	// Ok, we'll end up with the same number of partitions any which way we
	// go. Try to fill up a partition instead of opening a new one.
	aIsFullNow := a.isFullNow(dah.partitionSize)
	bIsFullNow := b.isFullNow(dah.partitionSize)
	if aIsFullNow != bIsFullNow {
		return !aIsFullNow
	}

	// Either we have two open partitions, or neither deadline has an open
	// partition.

	// If we have two open partitions, fill the deadline with the most-full
	// open partition. This helps us assign runs of sequential sectors into
	// the same partition.
	if !aIsFullNow && !bIsFullNow {
		if a.totalSectors != b.totalSectors {
			return a.totalSectors > b.totalSectors
		}
	}

	// Otherwise, assign to the deadline with the least live sectors. This
	// will break the tie in one of the two immediately preceding
	// conditions.
	if a.liveSectors != b.liveSectors {
		return a.liveSectors < b.liveSectors
	}

	// Finally, fallback on the deadline index.
	// TODO: Randomize by index instead of simply sorting.
	// https://github.com/filecoin-project/specs-actors/issues/432
	return a.index < b.index
}

func (dah *deadlineAssignmentHeap) Push(x interface{}) {
	dah.deadlines = append(dah.deadlines, x.(*deadlineAssignmentInfo))
}

func (dah *deadlineAssignmentHeap) Pop() interface{} {
	last := dah.deadlines[len(dah.deadlines)-1]
	dah.deadlines[len(dah.deadlines)-1] = nil
	dah.deadlines = dah.deadlines[:len(dah.deadlines)-1]
	return last
}

// Assigns partitions to deadlines, first filling partial partitions, then
// adding new partitions to deadlines with the fewest live sectors.
func assignDeadlines(
	maxPartitions uint64,
	partitionSize uint64,
	deadlines *[WPoStPeriodDeadlines]*Deadline,
	sectors []*SectorOnChainInfo,
) (changes [WPoStPeriodDeadlines][]*SectorOnChainInfo, err error) {
	// Build a heap
	dlHeap := deadlineAssignmentHeap{
		maxPartitions: maxPartitions,
		partitionSize: partitionSize,
		deadlines:     make([]*deadlineAssignmentInfo, 0, len(deadlines)),
	}

	for dlIdx, dl := range deadlines {
		if dl != nil {
			dlHeap.deadlines = append(dlHeap.deadlines, &deadlineAssignmentInfo{
				index:        dlIdx,
				liveSectors:  dl.LiveSectors,
				totalSectors: dl.TotalSectors,
			})
		}
	}

	heap.Init(&dlHeap)

	// Assign sectors to deadlines.
	for _, sector := range sectors {
		info := dlHeap.deadlines[0]

		if info.maxPartitionsReached(partitionSize, maxPartitions) {
			return changes, xerrors.Errorf("maxPartitions limit %d reached for all deadlines", maxPartitions)
		}

		changes[info.index] = append(changes[info.index], sector)
		info.liveSectors++
		info.totalSectors++

		// Update heap.
		heap.Fix(&dlHeap, 0)
	}

	return changes, nil
}
