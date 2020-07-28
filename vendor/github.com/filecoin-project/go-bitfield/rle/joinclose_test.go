package rlepluslazy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinClose(t *testing.T) {
	inBits := []uint64{0, 1, 4, 5, 9, 14}
	var tests = []struct {
		name      string
		given     []uint64
		expected  []uint64
		closeness uint64
	}{
		{"closeness 0", inBits, []uint64{0, 1, 4, 5, 9, 14}, 0},
		{"closeness 2", inBits, []uint64{0, 1, 2, 3, 4, 5, 9, 14}, 2},
		{"closeness 3", inBits, []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 14}, 3},
		{"closeness 4", inBits, []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}, 4},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			a, err := RunsFromSlice(tt.given)
			assert.NoError(t, err)
			jc, err := JoinClose(a, tt.closeness)
			assert.NoError(t, err)
			bits, err := SliceFromRuns(jc)
			assert.Equal(t, tt.expected, bits)
		})
	}

}
