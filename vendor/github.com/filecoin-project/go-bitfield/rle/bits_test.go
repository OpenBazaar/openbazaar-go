package rlepluslazy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunsFromBits(t *testing.T) {
	expected := []Run{{Val: false, Len: 0x1},
		{Val: true, Len: 0x3},
		{Val: false, Len: 0x2},
		{Val: true, Len: 0x3},
	}
	rit, err := RunsFromBits(BitsFromSlice([]uint64{1, 2, 3, 6, 7, 8}))
	assert.NoError(t, err)
	i := 10
	output := make([]Run, 0, 4)
	for rit.HasNext() && i > 0 {
		run, err := rit.NextRun()
		assert.NoError(t, err)
		i--
		output = append(output, run)
	}
	assert.NotEqual(t, 0, i, "too many iterations")
	assert.Equal(t, expected, output)
}

func TestNthSlice(t *testing.T) {
	testIter(t, func(t *testing.T, bits []uint64) BitIterator {
		iter := BitsFromSlice(bits)
		return iter
	})
}

func TestNthRuns(t *testing.T) {
	testIter(t, func(t *testing.T, bits []uint64) BitIterator {
		riter, err := RunsFromSlice(bits)
		assert.NoError(t, err)
		biter, err := BitsFromRuns(riter)
		assert.NoError(t, err)
		return biter
	})
}

func testIter(t *testing.T, ctor func(t *testing.T, bits []uint64) BitIterator) {
	for i := 0; i < 10; i++ {
		bits := randomBits(1000, 1500)
		iter := ctor(t, bits)

		n, err := iter.Nth(10)
		assert.NoError(t, err)
		assert.Equal(t, bits[10], n)

		n, err = iter.Nth(0)
		assert.NoError(t, err)
		assert.Equal(t, bits[11], n)

		n, err = iter.Nth(1)
		assert.NoError(t, err)
		assert.Equal(t, bits[13], n)

		n, err = iter.Next()
		assert.NoError(t, err)
		assert.Equal(t, bits[14], n)

		runs, err := RunsFromBits(iter)
		assert.NoError(t, err)

		remainingBits, err := SliceFromRuns(runs)
		assert.NoError(t, err)

		assert.Equal(t, bits[15:], remainingBits)
	}
	for i := 0; i < 10; i++ {
		bits := randomBits(1000, 1500)
		iter := ctor(t, bits)

		last, err := iter.Nth(uint64(len(bits) - 1))
		assert.NoError(t, err)
		assert.Equal(t, bits[len(bits)-1], last)
		assert.False(t, iter.HasNext())
		_, err = iter.Nth(0)
		assert.Equal(t, ErrEndOfIterator, err)
	}
}
