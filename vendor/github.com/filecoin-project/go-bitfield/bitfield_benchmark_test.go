package bitfield

import (
	"fmt"
	"testing"
)

func benchmark(b *testing.B, cb func(b *testing.B, bf *BitField)) {
	for _, size := range []int{
		0,
		1,
		10,
		1000,
		1000000,
	} {
		benchmarkSize(b, size, cb)
	}
}

func benchmarkSize(b *testing.B, size int, cb func(b *testing.B, bf *BitField)) {
	b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {
		vals := getRandIndexSet(size)
		bf := NewFromSet(vals)
		b.Run("basic", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				cb(b, bf)
			}
		})

		if size < 1 {
			return
		}

		// Set and unset some bits
		i := uint64(size / 10)
		bf.Set(i)
		bf.Set(i + 1)
		bf.Set(i * 2)
		bf.Unset(i / 2)
		bf.Unset(uint64(size) - 1)

		b.Run("modified", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				cb(b, bf)
			}
		})
	})
}

func BenchmarkCount(b *testing.B) {
	benchmark(b, func(b *testing.B, bf *BitField) {
		_, err := bf.Count()
		if err != nil {
			b.Fatal(err)
		}
	})
}

func BenchmarkIsEmpty(b *testing.B) {
	benchmark(b, func(b *testing.B, bf *BitField) {
		_, err := bf.IsEmpty()
		if err != nil {
			b.Fatal(err)
		}
	})
}
