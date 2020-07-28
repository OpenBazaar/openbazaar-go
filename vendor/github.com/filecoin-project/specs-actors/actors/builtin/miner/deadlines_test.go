package miner_test

import (
	"testing"

	"github.com/filecoin-project/go-bitfield"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
)

func TestProvingPeriodDeadlines(t *testing.T) {
	PP := miner.WPoStProvingPeriod
	CW := miner.WPoStChallengeWindow
	DLS := miner.WPoStPeriodDeadlines

	t.Run("pre-open", func(t *testing.T) {
		curr := abi.ChainEpoch(0) // Current is before the period opens.
		{
			periodStart := miner.FaultDeclarationCutoff + 1
			di := miner.ComputeProvingPeriodDeadline(periodStart, curr)
			assert.Equal(t, uint64(0), di.Index)
			assert.Equal(t, periodStart, di.Open)

			assert.False(t, di.PeriodStarted())
			assert.False(t, di.IsOpen())
			assert.False(t, di.HasElapsed())
			assert.False(t, di.FaultCutoffPassed())
			assert.Equal(t, periodStart+miner.WPoStProvingPeriod-1, di.PeriodEnd())
			assert.Equal(t, periodStart+miner.WPoStProvingPeriod, di.NextPeriodStart())
		}
		{
			periodStart := miner.FaultDeclarationCutoff - 1
			di := miner.ComputeProvingPeriodDeadline(periodStart, curr)
			assert.True(t, di.FaultCutoffPassed())
		}
	})

	t.Run("offset zero", func(t *testing.T) {
		firstPeriodStart := abi.ChainEpoch(0)

		// First proving period.
		di := assertDeadlineInfo(t, 0, firstPeriodStart, 0, 0)
		assert.Equal(t, -miner.WPoStChallengeLookback, di.Challenge)
		assert.Equal(t, -miner.FaultDeclarationCutoff, di.FaultCutoff)
		assert.True(t, di.IsOpen())
		assert.True(t, di.FaultCutoffPassed())

		assertDeadlineInfo(t, 1, firstPeriodStart, 0, 0)
		// Final epoch of deadline 0.
		assertDeadlineInfo(t, CW-1, firstPeriodStart, 0, 0)
		// First epoch of deadline 1
		assertDeadlineInfo(t, CW, firstPeriodStart, 1, CW)
		assertDeadlineInfo(t, CW+1, firstPeriodStart, 1, CW)
		// Final epoch of deadline 1
		assertDeadlineInfo(t, CW*2-1, firstPeriodStart, 1, CW)
		// First epoch of deadline 2
		assertDeadlineInfo(t, CW*2, firstPeriodStart, 2, CW*2)

		// Last epoch of last deadline
		assertDeadlineInfo(t, PP-1, firstPeriodStart, DLS-1, PP-CW)

		// Second proving period
		// First epoch of deadline 0
		secondPeriodStart := PP
		di = assertDeadlineInfo(t, PP, secondPeriodStart, 0, PP)
		assert.Equal(t, PP-miner.WPoStChallengeLookback, di.Challenge)
		assert.Equal(t, PP-miner.FaultDeclarationCutoff, di.FaultCutoff)

		// Final epoch of deadline 0.
		assertDeadlineInfo(t, PP+CW-1, secondPeriodStart, 0, PP+0)
		// First epoch of deadline 1
		assertDeadlineInfo(t, PP+CW, secondPeriodStart, 1, PP+CW)
		assertDeadlineInfo(t, PP+CW+1, secondPeriodStart, 1, PP+CW)
	})

	t.Run("offset non-zero", func(t *testing.T) {
		offset := CW*2 + 2 // Arbitrary not aligned with challenge window.
		initialPPStart := offset - PP
		firstDlIndex := miner.WPoStPeriodDeadlines - uint64(offset/CW) - 1
		firstDlOpen := initialPPStart + CW*abi.ChainEpoch(firstDlIndex)

		require.True(t, offset < PP)
		require.True(t, initialPPStart < 0)
		require.True(t, firstDlOpen < 0)

		// Incomplete initial proving period.
		// At epoch zero, the initial deadlines in the period have already passed and we're part way through
		// another one.
		di := assertDeadlineInfo(t, 0, initialPPStart, firstDlIndex, firstDlOpen)
		assert.Equal(t, firstDlOpen-miner.WPoStChallengeLookback, di.Challenge)
		assert.Equal(t, firstDlOpen-miner.FaultDeclarationCutoff, di.FaultCutoff)
		assert.True(t, di.IsOpen())
		assert.True(t, di.FaultCutoffPassed())

		// Epoch 1
		assertDeadlineInfo(t, 1, initialPPStart, firstDlIndex, firstDlOpen)

		// Epoch 2 rolls over to third-last challenge window
		assertDeadlineInfo(t, 2, initialPPStart, firstDlIndex+1, firstDlOpen+CW)
		assertDeadlineInfo(t, 3, initialPPStart, firstDlIndex+1, firstDlOpen+CW)

		// Last epoch of second-last window.
		assertDeadlineInfo(t, 2+CW-1, initialPPStart, firstDlIndex+1, firstDlOpen+CW)
		// First epoch of last challenge window.
		assertDeadlineInfo(t, 2+CW, initialPPStart, firstDlIndex+2, firstDlOpen+CW*2)
		// Last epoch of last challenge window.
		assert.Equal(t, miner.WPoStPeriodDeadlines-1, firstDlIndex+2)
		assertDeadlineInfo(t, 2+2*CW-1, initialPPStart, firstDlIndex+2, firstDlOpen+CW*2)

		// First epoch of next proving period.
		assertDeadlineInfo(t, 2+2*CW, initialPPStart+PP, 0, initialPPStart+PP)
		assertDeadlineInfo(t, 2+2*CW+1, initialPPStart+PP, 0, initialPPStart+PP)
	})

	t.Run("period expired", func(t *testing.T) {
		offset := abi.ChainEpoch(1)
		d := miner.ComputeProvingPeriodDeadline(offset, offset+miner.WPoStProvingPeriod)
		assert.True(t, d.PeriodStarted())
		assert.True(t, d.PeriodElapsed())
		assert.Equal(t, miner.WPoStPeriodDeadlines, d.Index)
		assert.False(t, d.IsOpen())
		assert.True(t, d.HasElapsed())
		assert.True(t, d.FaultCutoffPassed())
		assert.Equal(t, offset+miner.WPoStProvingPeriod-1, d.PeriodEnd())
		assert.Equal(t, offset+miner.WPoStProvingPeriod, d.NextPeriodStart())
	})
}

func assertDeadlineInfo(t *testing.T, current, periodStart abi.ChainEpoch, expectedIndex uint64, expectedDeadlineOpen abi.ChainEpoch) *miner.DeadlineInfo {
	expected := makeDeadline(current, periodStart, expectedIndex, expectedDeadlineOpen)
	actual := miner.ComputeProvingPeriodDeadline(periodStart, current)
	assert.True(t, actual.PeriodStarted())
	assert.True(t, actual.IsOpen())
	assert.False(t, actual.HasElapsed())
	assert.Equal(t, expected, actual)
	return actual
}

func makeDeadline(currEpoch, periodStart abi.ChainEpoch, index uint64, deadlineOpen abi.ChainEpoch) *miner.DeadlineInfo {
	return &miner.DeadlineInfo{
		CurrentEpoch: currEpoch,
		PeriodStart:  periodStart,
		Index:        index,
		Open:         deadlineOpen,
		Close:        deadlineOpen + miner.WPoStChallengeWindow,
		Challenge:    deadlineOpen - miner.WPoStChallengeLookback,
		FaultCutoff:  deadlineOpen - miner.FaultDeclarationCutoff,
	}
}

func TestPartitionsForDeadline(t *testing.T) {
	const partSize = uint64(1000)

	t.Run("empty deadlines", func(t *testing.T) {
		dl := buildDeadlines(t, []uint64{})
		firstIndex, sectorCount, err := miner.PartitionsForDeadline(dl, partSize, 0)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, uint64(0), sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, miner.WPoStPeriodDeadlines-1)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, uint64(0), sectorCount)
	})

	t.Run("single sector at first deadline", func(t *testing.T) {
		dl := buildDeadlines(t, []uint64{1})
		firstIndex, sectorCount, err := miner.PartitionsForDeadline(dl, partSize, 0)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, uint64(1), sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 1)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), firstIndex)
		assert.Zero(t, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, miner.WPoStPeriodDeadlines-1)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), firstIndex)
		assert.Zero(t, sectorCount)
	})

	t.Run("single sector at non-first deadline", func(t *testing.T) {
		dl := buildDeadlines(t, []uint64{0, 1})
		firstIndex, sectorCount, err := miner.PartitionsForDeadline(dl, partSize, 0)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, uint64(0), sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 1)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, uint64(1), sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 2)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), firstIndex)
		assert.Equal(t, uint64(0), sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, miner.WPoStPeriodDeadlines-1)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), firstIndex)
		assert.Equal(t, uint64(0), sectorCount)
	})

	t.Run("deadlines with one full partitions", func(t *testing.T) {
		dl := NewDeadlinesBuilder(t).addToAll(partSize).Deadlines
		firstIndex, sectorCount, err := miner.PartitionsForDeadline(dl, partSize, 0)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, partSize, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 1)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), firstIndex)
		assert.Equal(t, partSize, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, miner.WPoStPeriodDeadlines-1)
		require.NoError(t, err)
		assert.Equal(t, miner.WPoStPeriodDeadlines-1, firstIndex)
		assert.Equal(t, partSize, sectorCount)
	})

	t.Run("partial partitions", func(t *testing.T) {
		dl := buildDeadlines(t, []uint64{
			0: partSize - 1,
			1: partSize,
			2: partSize - 2,
			3: partSize,
			4: partSize - 3,
			5: partSize,
		})
		firstIndex, sectorCount, err := miner.PartitionsForDeadline(dl, partSize, 0)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, partSize-1, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 1)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), firstIndex)
		assert.Equal(t, partSize, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 2)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), firstIndex)
		assert.Equal(t, partSize-2, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 5)
		require.NoError(t, err)
		assert.Equal(t, uint64(5), firstIndex)
		assert.Equal(t, partSize, sectorCount)
	})

	t.Run("multiple partitions", func(t *testing.T) {
		dl := buildDeadlines(t, []uint64{
			0: partSize,       // 1 partition 1 total
			1: partSize * 2,   // 2 partitions 3 total
			2: partSize*4 - 1, // 4 partitions 7 total
			3: partSize * 6,   // 6 partitions 13 total
			4: partSize*8 - 1, // 8 partitions 21 total
			5: partSize * 9,   // 9 partitions 30 total
		})

		firstIndex, sectorCount, err := miner.PartitionsForDeadline(dl, partSize, 0)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, partSize, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 1)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), firstIndex)
		assert.Equal(t, partSize*2, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 2)
		require.NoError(t, err)
		assert.Equal(t, uint64(3), firstIndex)
		assert.Equal(t, partSize*4-1, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 3)
		require.NoError(t, err)
		assert.Equal(t, uint64(7), firstIndex)
		assert.Equal(t, partSize*6, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 4)
		require.NoError(t, err)
		assert.Equal(t, uint64(13), firstIndex)
		assert.Equal(t, partSize*8-1, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 5)
		require.NoError(t, err)
		assert.Equal(t, uint64(21), firstIndex)
		assert.Equal(t, partSize*9, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, miner.WPoStPeriodDeadlines-1)
		require.NoError(t, err)
		assert.Equal(t, uint64(30), firstIndex)
		assert.Equal(t, uint64(0), sectorCount)
	})
}

func TestComputePartitionsSectors(t *testing.T) {
	const partSize = uint64(1000)

	t.Run("no partitions due at empty deadline", func(t *testing.T) {
		dls := miner.ConstructDeadlines()
		dls.Due[1] = bfSeq(0, 1)

		// No partitions at deadline 0
		_, err := miner.ComputePartitionsSectors(dls, partSize, 0, []uint64{0})
		require.Error(t, err)

		// No partitions at deadline 2
		_, err = miner.ComputePartitionsSectors(dls, partSize, 2, []uint64{0})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 2, []uint64{1})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 2, []uint64{2})
		require.Error(t, err)
	})
	t.Run("single sector", func(t *testing.T) {
		dls := miner.ConstructDeadlines()
		dls.Due[1] = bfSeq(0, 1)
		partitions, err := miner.ComputePartitionsSectors(dls, partSize, 1, []uint64{0})
		require.NoError(t, err)
		assert.Equal(t, 1, len(partitions))
		assertBfEqual(t, bfSeq(0, 1), partitions[0])
	})
	t.Run("full partition", func(t *testing.T) {
		dls := miner.ConstructDeadlines()
		dls.Due[10] = bfSeq(1234, partSize)
		partitions, err := miner.ComputePartitionsSectors(dls, partSize, 10, []uint64{0})
		require.NoError(t, err)
		assert.Equal(t, 1, len(partitions))
		assertBfEqual(t, bfSeq(1234, partSize), partitions[0])
	})
	t.Run("full plus partial partition", func(t *testing.T) {
		dls := miner.ConstructDeadlines()
		dls.Due[10] = bfSeq(5555, partSize+1)
		partitions, err := miner.ComputePartitionsSectors(dls, partSize, 10, []uint64{0}) // First partition
		require.NoError(t, err)
		assert.Equal(t, 1, len(partitions))
		assertBfEqual(t, bfSeq(5555, partSize), partitions[0])

		partitions, err = miner.ComputePartitionsSectors(dls, partSize, 10, []uint64{1}) // Second partition
		require.NoError(t, err)
		assert.Equal(t, 1, len(partitions))
		assertBfEqual(t, bfSeq(5555+partSize, 1), partitions[0])

		partitions, err = miner.ComputePartitionsSectors(dls, partSize, 10, []uint64{0, 1}) // Both partitions
		require.NoError(t, err)
		assert.Equal(t, 2, len(partitions))
		assertBfEqual(t, bfSeq(5555, partSize), partitions[0])
		assertBfEqual(t, bfSeq(5555+partSize, 1), partitions[1])
	})
	t.Run("multiple partitions", func(t *testing.T) {
		dls := miner.ConstructDeadlines()
		dls.Due[1] = bfSeq(0, 3*partSize+1)
		partitions, err := miner.ComputePartitionsSectors(dls, partSize, 1, []uint64{0, 1, 2, 3})
		require.NoError(t, err)
		assert.Equal(t, 4, len(partitions))
		assertBfEqual(t, bfSeq(0, partSize), partitions[0])
		assertBfEqual(t, bfSeq(1*partSize, partSize), partitions[1])
		assertBfEqual(t, bfSeq(2*partSize, partSize), partitions[2])
		assertBfEqual(t, bfSeq(3*partSize, 1), partitions[3])
	})
	t.Run("partitions numbered across deadlines", func(t *testing.T) {
		dls := miner.ConstructDeadlines()
		dls.Due[1] = bfSeq(0, 3*partSize+1)
		dls.Due[3] = bfSeq(3*partSize+1, 1)
		dls.Due[5] = bfSeq(3*partSize+1+1, 2*partSize)

		partitions, err := miner.ComputePartitionsSectors(dls, partSize, 1, []uint64{0, 1, 2, 3})
		require.NoError(t, err)
		assert.Equal(t, 4, len(partitions))

		partitions, err = miner.ComputePartitionsSectors(dls, partSize, 3, []uint64{4})
		require.NoError(t, err)
		assert.Equal(t, 1, len(partitions))
		assertBfEqual(t, bfSeq(3*partSize+1, 1), partitions[0])

		partitions, err = miner.ComputePartitionsSectors(dls, partSize, 5, []uint64{5, 6})
		require.NoError(t, err)
		assert.Equal(t, 2, len(partitions))
		assertBfEqual(t, bfSeq(3*partSize+1+1, partSize), partitions[0])
		assertBfEqual(t, bfSeq(3*partSize+1+1+partSize, partSize), partitions[1])

		// Mismatched deadline/partition pairs
		_, err = miner.ComputePartitionsSectors(dls, partSize, 1, []uint64{4})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 2, []uint64{4})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 3, []uint64{0})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 3, []uint64{3})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 3, []uint64{5})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 4, []uint64{5})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 5, []uint64{0})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 5, []uint64{7})
		require.Error(t, err)
	})
}

func TestAssignNewSectors(t *testing.T) {
	partSize := uint64(4)
	seed := abi.Randomness([]byte{})

	assign := func(deadlines *miner.Deadlines, sectors []uint64) *miner.Deadlines {
		require.NoError(t, miner.AssignNewSectors(deadlines, partSize, sectors, seed))
		return deadlines
	}

	t.Run("initial assignment", func(t *testing.T) {
		{
			deadlines := assign(miner.ConstructDeadlines(), seq(0, 0))
			NewDeadlinesBuilder(t).verify(deadlines)
		}
		{
			deadlines := assign(miner.ConstructDeadlines(), seq(0, 1))
			NewDeadlinesBuilder(t, 0, 1).verify(deadlines)
		}
		{
			deadlines := assign(miner.ConstructDeadlines(), seq(0, 15))
			NewDeadlinesBuilder(t, 0, 4, 4, 4, 3).verify(deadlines)
		}
		{
			deadlines := assign(miner.ConstructDeadlines(), seq(0, (miner.WPoStPeriodDeadlines-1)*partSize+1))
			NewDeadlinesBuilder(t).addToAllFrom(1, partSize).addTo(1, 1).verify(deadlines)
		}
	})
	t.Run("incremental assignment", func(t *testing.T) {
		{
			// Add one sector at a time.
			deadlines := NewDeadlinesBuilder(t, 0, 1).Deadlines
			assign(deadlines, seq(1, 1))
			NewDeadlinesBuilder(t, 0, 2).verify(deadlines)
			assign(deadlines, seq(2, 1))
			NewDeadlinesBuilder(t, 0, 3).verify(deadlines)
			assign(deadlines, seq(3, 1))
			NewDeadlinesBuilder(t, 0, 4).verify(deadlines)
			assign(deadlines, seq(4, 1))
			NewDeadlinesBuilder(t, 0, 4, 1).verify(deadlines)
		}
		{
			// Add one partition at a time.
			deadlines := miner.ConstructDeadlines()
			assign(deadlines, seq(0, 4))
			NewDeadlinesBuilder(t, 0, 4).verify(deadlines)
			assign(deadlines, seq(4, 4))
			NewDeadlinesBuilder(t, 0, 4, 4).verify(deadlines)
			assign(deadlines, seq(2*4, 4))
			NewDeadlinesBuilder(t, 0, 4, 4, 4).verify(deadlines)
			assign(deadlines, seq(3*4, 4))
			NewDeadlinesBuilder(t, 0, 4, 4, 4, 4).verify(deadlines)
		}
		{
			// Add lots
			deadlines := miner.ConstructDeadlines()
			assign(deadlines, seq(0, 2*partSize+1))
			NewDeadlinesBuilder(t, 0, partSize, partSize, 1).verify(deadlines)
			assign(deadlines, seq(2*partSize+1, partSize))
			NewDeadlinesBuilder(t, 0, partSize, partSize, partSize, 1).verify(deadlines)
		}
	})
	t.Run("fill partial partitions first", func(t *testing.T) {
		{
			b := NewDeadlinesBuilder(t, 0, 4, 3, 1)
			deadlines := assign(b.Deadlines, seq(b.NextSectorIdx, 4))

			NewDeadlinesBuilder(t, 0, 4, 3, 1).
				addTo(2, 1). // Fill the first partial partition
				addTo(3, 3). // Fill the next partial partition
				verify(deadlines)
		}
		{
			b := NewDeadlinesBuilder(t, 0, 9, 8, 7, 4, 1)
			deadlines := assign(b.Deadlines, seq(b.NextSectorIdx, 7))

			NewDeadlinesBuilder(t, 0, 9, 8, 7, 4, 1).
				addTo(1, 3). // Fill the first partial partition, in deadline 1
				addTo(3, 1). // Fill the next partial partition
				addTo(5, 3). // Fill the final partial partition
				verify(deadlines)
		}
	})
	t.Run("fill less full deadlines first", func(t *testing.T) {
		{
			b := NewDeadlinesBuilder(t, 0, 12, 4, 4, 8).
				addToAllFrom(5, 100) // Fill  trailing deadlines so we can just use the first few.
			deadlines := b.Deadlines
			assign(deadlines, seq(b.NextSectorIdx, 20))

			NewDeadlinesBuilder(t, 0, 12, 4, 4, 8).
				addToAllFrom(5, 100).
				addTo(2, 4).
				addTo(3, 4).
				addTo(2, 4).
				addTo(3, 4).
				addTo(4, 4).
				verify(deadlines)
		}
	})
	// TODO: a final test including partial and full partitions that exercises both filling the partials first,
	// then prioritising the less full deadlines.
	// https://github.com/filecoin-project/specs-actors/issues/439
}

//
// Deadlines Utils
//

func assertBfEqual(t *testing.T, expected, actual *bitfield.BitField) {
	ex, err := expected.All(1 << 20)
	require.NoError(t, err)
	ac, err := actual.All(1 << 20)
	require.NoError(t, err)
	assert.Equal(t, ex, ac)
}

func assertDeadlinesEqual(t *testing.T, expected, actual *miner.Deadlines) {
	for i := range expected.Due {
		ex, err := expected.Due[i].All(1 << 20)
		require.NoError(t, err)
		ac, err := actual.Due[i].All(1 << 20)
		require.NoError(t, err)
		assert.Equal(t, ex, ac, "mismatched deadlines at index %d", i)
	}
}

// Creates a bitfield with a sequence of `count` values from `first.
func bfSeq(first uint64, count uint64) *abi.BitField {
	values := seq(first, count)
	return bitfield.NewFromSet(values)
}

// Creates a slice of integers with a sequence of `count` values from `first.
func seq(first uint64, count uint64) []uint64 {
	values := make([]uint64, count)
	for i := range values {
		values[i] = first + uint64(i)
	}
	return values
}

// accepts an array were the value at each index indicates how many sectors are in the partition of the returned Deadlines
// Example:
// gen := [miner.WPoStPeriodDeadlines]uint64{1, 42, 89, 0} returns a deadline with:
// 1  sectors at deadlineIdx 0
// 42 sectors at deadlineIdx 1
// 89 sectors at deadlineIdx 2
// 0  sectors at deadlineIdx 3-47
func buildDeadlines(t *testing.T, gen []uint64) *miner.Deadlines {
	return NewDeadlinesBuilder(t).addToFrom(0, gen...).Deadlines
}

// A builder for initialising a Deadlines with sectors assigned.
type DeadlinesBuilder struct {
	Deadlines     *miner.Deadlines
	NextSectorIdx uint64
	t             *testing.T
}

// Creates a new builder, with optional initial sector counts.
func NewDeadlinesBuilder(t *testing.T, counts ...uint64) *DeadlinesBuilder {
	b := &DeadlinesBuilder{miner.ConstructDeadlines(), 0, t}
	b.addToFrom(0, counts...)
	return b
}

// Assigns count new sectors to deadline idx.
func (b *DeadlinesBuilder) addTo(idx uint64, count uint64) *DeadlinesBuilder {
	nums := seq(b.NextSectorIdx, count)
	b.NextSectorIdx += count
	require.NoError(b.t, b.Deadlines.AddToDeadline(idx, nums...))
	return b
}

// Assigns counts[i] new sectors to deadlines sequentially from first.
func (b *DeadlinesBuilder) addToFrom(first uint64, counts ...uint64) *DeadlinesBuilder {
	for i, c := range counts {
		b.addTo(first+uint64(i), c)
	}
	return b
}

// Assigns count new sectors to every deadline.
func (b *DeadlinesBuilder) addToAll(count uint64) *DeadlinesBuilder {
	for i := range b.Deadlines.Due {
		b.addTo(uint64(i), count)
	}
	return b
}

// Assigns count new sectors to every deadline from first until the last.
func (b *DeadlinesBuilder) addToAllFrom(first uint64, count uint64) *DeadlinesBuilder {
	for i := first; i < miner.WPoStPeriodDeadlines; i++ {
		b.addTo(i, count)
	}
	return b
}

// Verifies that deadlines match this builder as expected values.
func (b *DeadlinesBuilder) verify(actual *miner.Deadlines) {
	assertDeadlinesEqual(b.t, b.Deadlines, actual)
}
