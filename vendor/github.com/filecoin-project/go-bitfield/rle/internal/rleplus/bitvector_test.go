package rleplus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBitVector(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		var v BitVector

		assert.Equal(t, LSB0, v.BytePacking)
	})

	t.Run("Push", func(t *testing.T) {
		// MSB0 bit numbering
		v := BitVector{BytePacking: MSB0}
		v.Push(1)
		v.Push(0)
		v.Push(1)
		v.Push(1)

		assert.Equal(t, byte(176), v.Buf[0])

		// LSB0 bit numbering
		v = BitVector{BytePacking: LSB0}
		v.Push(1)
		v.Push(0)
		v.Push(1)
		v.Push(1)

		assert.Equal(t, byte(13), v.Buf[0])
	})

	t.Run("Get", func(t *testing.T) {
		bits := []byte{1, 0, 1, 1, 0, 0, 1, 0}

		for _, numbering := range []BitNumbering{MSB0, LSB0} {
			v := BitVector{BytePacking: numbering}

			for _, bit := range bits {
				v.Push(bit)
			}

			for idx, expected := range bits {
				actual, _ := v.Get(uint(idx))
				assert.Equal(t, expected, actual)
			}
		}
	})

	t.Run("Extend", func(t *testing.T) {
		val := byte(171) // 0b10101011

		var v BitVector

		// MSB0 bit numbering
		v = BitVector{}
		v.Extend(val, 4, MSB0)
		assertBitVector(t, []byte{1, 0, 1, 0}, v)
		v.Extend(val, 5, MSB0)
		assertBitVector(t, []byte{1, 0, 1, 0, 1, 0, 1, 0, 1}, v)

		// LSB0 bit numbering
		v = BitVector{}
		v.Extend(val, 4, LSB0)
		assertBitVector(t, []byte{1, 1, 0, 1}, v)
		v.Extend(val, 5, LSB0)
		assertBitVector(t, []byte{1, 1, 0, 1, 1, 1, 0, 1, 0}, v)
	})

	t.Run("invalid counts to Take/Extend/Iterator cause panics", func(t *testing.T) {
		v := BitVector{BytePacking: LSB0}

		assert.Panics(t, func() { v.Extend(0xff, 9, LSB0) })

		assert.Panics(t, func() { v.Take(0, 9, LSB0) })

		next := v.Iterator(LSB0)
		assert.Panics(t, func() { next(9) })
	})

	t.Run("Take", func(t *testing.T) {
		var v BitVector

		bits := []byte{1, 0, 1, 0, 1, 0, 1, 1}
		for _, bit := range bits {
			v.Push(bit)
		}

		assert.Equal(t, byte(176), v.Take(4, 4, MSB0))
		assert.Equal(t, byte(13), v.Take(4, 4, LSB0))
	})

	t.Run("Iterator", func(t *testing.T) {
		var buf []byte

		// make a bitvector of 256 sample bits
		for i := 0; i < 32; i++ {
			buf = append(buf, 128+32)
		}

		v := NewBitVector(buf, LSB0)

		next := v.Iterator(LSB0)

		// compare to Get()
		for i := uint(0); i < v.Len; i++ {
			expected, _ := v.Get(i)
			assert.Equal(t, expected, next(1))
		}

		// out of range should return zero
		assert.Equal(t, byte(0), next(1))
		assert.Equal(t, byte(0), next(8))

		// compare to Take()
		next = v.Iterator(LSB0)
		assert.Equal(t, next(5), v.Take(0, 5, LSB0))
		assert.Equal(t, next(8), v.Take(5, 8, LSB0))
	})
}

// Note: When using this helper assertion, expectedBits should *only* be 0s and 1s.
func assertBitVector(t *testing.T, expectedBits []byte, actual BitVector) {
	assert.Equal(t, uint(len(expectedBits)), actual.Len)

	for idx, bit := range expectedBits {
		actualBit, err := actual.Get(uint(idx))
		assert.NoError(t, err)
		assert.Equal(t, bit, actualBit)
	}
}
