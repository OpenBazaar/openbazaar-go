package rlepluslazy

import (
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrRuns(t *testing.T) {
	{
		a, err := RunsFromSlice([]uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 11, 12, 13, 14})
		assert.NoError(t, err)
		b, err := RunsFromSlice([]uint64{0, 1, 2, 3, 9, 10, 16, 17, 18, 50, 51, 70})
		assert.NoError(t, err)

		s, err := Or(a, b)
		assert.NoError(t, err)
		bis, err := SliceFromRuns(s)
		assert.Equal(t, []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 16, 17, 18, 50, 51, 70}, bis)
		assert.NoError(t, err)
	}

	{
		a, err := RunsFromSlice([]uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 11, 12, 13, 14})
		assert.NoError(t, err)
		b, err := RunsFromSlice([]uint64{0, 1, 2, 3, 9, 10, 16, 17, 18, 50, 51, 70})
		assert.NoError(t, err)

		s, err := Or(b, a)
		assert.NoError(t, err)
		bis, err := SliceFromRuns(s)
		assert.Equal(t, []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 16, 17, 18, 50, 51, 70}, bis)
		assert.NoError(t, err)
	}
}

func randomBits(N int, max uint64) []uint64 {
	all := make(map[uint64]struct{})
	for len(all) <= N {
		x := rand.Uint64() % max
		if _, has := all[x]; has {
			continue
		}
		all[x] = struct{}{}
	}

	res := make([]uint64, 0, N)
	for x := range all {
		res = append(res, x)
	}
	sort.Slice(res, func(i, j int) bool { return res[i] < res[j] })

	return res
}

func sum(a, b []uint64) []uint64 {
	all := make(map[uint64]struct{})
	for _, x := range a {
		all[x] = struct{}{}
	}
	for _, x := range b {
		all[x] = struct{}{}
	}
	res := make([]uint64, 0, len(all))
	for x := range all {
		res = append(res, x)
	}
	sort.Slice(res, func(i, j int) bool { return res[i] < res[j] })

	return res
}

func and(a, b []uint64) []uint64 {
	amap := make(map[uint64]struct{})
	for _, x := range a {
		amap[x] = struct{}{}
	}

	res := make([]uint64, 0)
	for _, x := range b {
		if _, ok := amap[x]; ok {
			res = append(res, x)
		}

	}
	sort.Slice(res, func(i, j int) bool { return res[i] < res[j] })

	return res
}

func TestOrRandom(t *testing.T) {
	N := 100
	for i := 0; i < N; i++ {
		abits := randomBits(1000, 1500)
		bbits := randomBits(1000, 1500)
		sumbits := sum(abits, bbits)

		a, err := RunsFromSlice(abits)
		assert.NoError(t, err)
		b, err := RunsFromSlice(bbits)
		assert.NoError(t, err)

		s, err := Or(b, a)
		assert.NoError(t, err)
		bis, err := SliceFromRuns(s)
		assert.NoError(t, err)
		assert.Equal(t, sumbits, bis)
	}
}

func TestIsSet(t *testing.T) {
	set := []uint64{0, 2, 3, 4, 5, 6, 7, 8, 11, 12, 13, 14}
	setMap := make(map[uint64]struct{})
	for _, v := range set {
		setMap[v] = struct{}{}
	}

	for i := uint64(0); i < 30; i++ {
		a, err := RunsFromSlice(set)
		assert.NoError(t, err)
		res, err := IsSet(a, i)
		assert.NoError(t, err)
		_, should := setMap[i]
		assert.Equal(t, should, res, "IsSet result missmatch at: %d", i)

	}
}

func TestCount(t *testing.T) {
	tests := []struct {
		name       string
		runs       []Run
		count      uint64
		shouldFail bool
	}{
		{
			name:  "count-20",
			runs:  []Run{{false, 4}, {true, 7}, {false, 10}, {true, 3}, {false, 13}, {true, 10}},
			count: 20,
		},
		{
			name: "count-2024",
			runs: []Run{{false, 4}, {true, 1000}, {false, 10}, {true, 1000},
				{false, 13}, {true, 24}, {false, 4}},
			count: 2024,
		},
		{
			name: "fail-set-over-max",
			runs: []Run{{false, 4}, {true, math.MaxUint64 / 2}, {false, 10}, {true, math.MaxUint64 / 2},
				{false, 13}, {true, 24}, {false, 4}},
			shouldFail: true,
		},
		{
			name: "length-over-max",
			runs: []Run{{false, math.MaxUint64 / 2}, {true, 4}, {false, 10}, {true, math.MaxUint64 / 2},
				{false, 13}, {true, 24}, {false, 4}},
			shouldFail: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			runs := &RunSliceIterator{Runs: test.runs}
			c, err := Count(runs)
			if test.shouldFail {
				assert.Error(t, err, "test indicated it should fail")
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, test.count, c)
			}
		})
	}
}
