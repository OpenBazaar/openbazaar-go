package miner

import (
	"math"
	"sort"

	"github.com/filecoin-project/go-bitfield"
	"golang.org/x/xerrors"
)

// Maps deadlines to partition maps.
type DeadlineSectorMap map[uint64]PartitionSectorMap

// Maps partitions to sector bitfields.
type PartitionSectorMap map[uint64]bitfield.BitField

// Check validates all bitfields and counts the number of partitions & sectors
// contained within the map, and returns an error if they exceed the given
// maximums.
func (dm DeadlineSectorMap) Check(maxPartitions, maxSectors uint64) error {
	partitionCount, sectorCount, err := dm.Count()
	if err != nil {
		return xerrors.Errorf("failed to count sectors: %w", err)
	}
	if partitionCount > maxPartitions {
		return xerrors.Errorf("too many partitions %d, max %d", partitionCount, maxPartitions)
	}

	if sectorCount > maxSectors {
		return xerrors.Errorf("too many sectors %d, max %d", sectorCount, maxSectors)
	}

	return nil
}

// Count counts the number of partitions & sectors within the map.
func (dm DeadlineSectorMap) Count() (partitions, sectors uint64, err error) {
	for dlIdx, pm := range dm { //nolint:nomaprange
		partCount, sectorCount, err := pm.Count()
		if err != nil {
			return 0, 0, xerrors.Errorf("when counting deadline %d: %w", dlIdx, err)
		}
		if partCount > math.MaxUint64-partitions {
			return 0, 0, xerrors.Errorf("uint64 overflow when counting partitions")
		}

		if sectorCount > math.MaxUint64-sectors {
			return 0, 0, xerrors.Errorf("uint64 overflow when counting sectors")
		}
		sectors += sectorCount
		partitions += partCount
	}
	return partitions, sectors, nil
}

// Add records the given sector bitfield at the given deadline/partition index.
func (dm DeadlineSectorMap) Add(dlIdx, partIdx uint64, sectorNos bitfield.BitField) error {
	if dlIdx >= WPoStPeriodDeadlines {
		return xerrors.Errorf("invalid deadline %d", dlIdx)
	}
	dl, ok := dm[dlIdx]
	if !ok {
		dl = make(PartitionSectorMap)
		dm[dlIdx] = dl
	}
	return dl.Add(partIdx, sectorNos)
}

// AddValues records the given sectors at the given deadline/partition index.
func (dm DeadlineSectorMap) AddValues(dlIdx, partIdx uint64, sectorNos ...uint64) error {
	return dm.Add(dlIdx, partIdx, bitfield.NewFromSet(sectorNos))
}

// Deadlines returns a sorted slice of deadlines in the map.
func (dm DeadlineSectorMap) Deadlines() []uint64 {
	deadlines := make([]uint64, 0, len(dm))
	for dlIdx := range dm { //nolint:nomaprange
		deadlines = append(deadlines, dlIdx)
	}
	sort.Slice(deadlines, func(i, j int) bool {
		return i < j
	})
	return deadlines
}

// ForEach walks the deadlines in deadline order.
func (dm DeadlineSectorMap) ForEach(cb func(dlIdx uint64, pm PartitionSectorMap) error) error {
	for _, dlIdx := range dm.Deadlines() {
		if err := cb(dlIdx, dm[dlIdx]); err != nil {
			return err
		}
	}
	return nil
}

// AddValues records the given sectors at the given partition.
func (pm PartitionSectorMap) AddValues(partIdx uint64, sectorNos ...uint64) error {
	return pm.Add(partIdx, bitfield.NewFromSet(sectorNos))
}

// Add records the given sector bitfield at the given partition index, merging
// it with any existing bitfields if necessary.
func (pm PartitionSectorMap) Add(partIdx uint64, sectorNos bitfield.BitField) error {
	if oldSectorNos, ok := pm[partIdx]; ok {
		var err error
		sectorNos, err = bitfield.MergeBitFields(sectorNos, oldSectorNos)
		if err != nil {
			return xerrors.Errorf("failed to merge sector bitfields: %w", err)
		}
	}
	pm[partIdx] = sectorNos
	return nil
}

// Count counts the number of partitions & sectors within the map.
func (pm PartitionSectorMap) Count() (partitions, sectors uint64, err error) {
	for partIdx, bf := range pm { //nolint:nomaprange
		count, err := bf.Count()
		if err != nil {
			return 0, 0, xerrors.Errorf("failed to parse bitmap for partition %d: %w", partIdx, err)
		}
		if count > math.MaxUint64-sectors {
			return 0, 0, xerrors.Errorf("uint64 overflow when counting sectors")
		}
		sectors += count
	}
	return uint64(len(pm)), sectors, nil
}

// Partitions returns a sorted slice of partitions in the map.
func (pm PartitionSectorMap) Partitions() []uint64 {
	partitions := make([]uint64, 0, len(pm))
	for partIdx := range pm { //nolint:nomaprange
		partitions = append(partitions, partIdx)
	}
	sort.Slice(partitions, func(i, j int) bool {
		return i < j
	})
	return partitions
}

// ForEach walks the partitions in the map, in order of increasing index.
func (pm PartitionSectorMap) ForEach(cb func(partIdx uint64, sectorNos bitfield.BitField) error) error {
	for _, partIdx := range pm.Partitions() {
		if err := cb(partIdx, pm[partIdx]); err != nil {
			return err
		}
	}
	return nil
}
