package bitfield

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"math/bits"
	"testing"
)

func TestExhaustive24(t *testing.T) {
	bf := NewBitfield(24)
	max := 1 << 24

	bint := new(big.Int)

	bts := make([]byte, 4)
	for j := 0; j < max; j++ {
		binary.BigEndian.PutUint32(bts, uint32(j))
		bint.SetBytes(bts[1:])
		bf.SetBytes(nil)
		for i := 0; i < 24; i++ {
			if bf.Bit(i) {
				t.Fatalf("bit %d should have been false", i)
			}
			if bint.Bit(i) == 1 {
				bf.SetBit(i)
				bf.SetBit(i)
			} else {
				bf.UnsetBit(i)
				bf.UnsetBit(i)
			}
			if bf.Bit(i) != (bint.Bit(i) == 1) {
				t.Fatalf("bit %d should have been true", i)
			}
		}
		if !bytes.Equal(bint.Bytes(), bf.Bytes()) {
			t.Fatal("big int and bitfield not equal")
		}
		for i := 0; i < 24; i++ {
			if (bint.Bit(i) == 1) != bf.Bit(i) {
				t.Fatalf("bit %d wrong", i)
			}
		}
		for i := 0; i < 24; i++ {
			if bf.OnesBefore(i) != bits.OnesCount32(uint32(j)<<(32-uint(i))) {
				t.Fatalf("wrong bit count")
			}
			if bf.OnesAfter(i) != bits.OnesCount32(uint32(j)>>uint(i)) {
				t.Fatalf("wrong bit count")
			}
			if bf.Ones() != bits.OnesCount32(uint32(j)) {
				t.Fatalf("wrong bit count")
			}
		}
	}
}

func TestBitfield(t *testing.T) {
	bf := NewBitfield(128)
	if bf.OnesBefore(20) != 0 {
		t.Fatal("expected no bits set")
	}
	bf.SetBit(10)
	if bf.OnesBefore(20) != 1 {
		t.Fatal("expected 1 bit set")
	}
	bf.SetBit(12)
	if bf.OnesBefore(20) != 2 {
		t.Fatal("expected 2 bit set")
	}
	bf.SetBit(30)
	if bf.OnesBefore(20) != 2 {
		t.Fatal("expected 2 bit set")
	}
	bf.SetBit(100)
	if bf.OnesBefore(20) != 2 {
		t.Fatal("expected 2 bit set")
	}
	bf.UnsetBit(10)
	if bf.OnesBefore(20) != 1 {
		t.Fatal("expected 1 bit set")
	}

	bint := new(big.Int).SetBytes(bf.Bytes())
	for i := 0; i < 128; i++ {
		if bf.Bit(i) != (bint.Bit(i) == 1) {
			t.Fatalf("expected bit %d to be %v", i, bf.Bit(i))
		}
	}
}

var benchmarkSize = 256

func BenchmarkBitfield(t *testing.B) {
	bf := NewBitfield(benchmarkSize)
	for i := 0; i < t.N; i++ {
		if bf.Bit(i % benchmarkSize) {
			t.Fatal("bad")
		}
		bf.SetBit(i % benchmarkSize)
		bf.UnsetBit(i % benchmarkSize)
		bf.SetBit(i % benchmarkSize)
		bf.UnsetBit(i % benchmarkSize)
		bf.SetBit(i % benchmarkSize)
		bf.UnsetBit(i % benchmarkSize)
		bf.SetBit(i % benchmarkSize)
		if !bf.Bit(i % benchmarkSize) {
			t.Fatal("bad")
		}
		bf.UnsetBit(i % benchmarkSize)
		bf.SetBit(i % benchmarkSize)
		bf.UnsetBit(i % benchmarkSize)
		bf.SetBit(i % benchmarkSize)
		bf.UnsetBit(i % benchmarkSize)
		bf.SetBit(i % benchmarkSize)
		bf.UnsetBit(i % benchmarkSize)
		if bf.Bit(i % benchmarkSize) {
			t.Fatal("bad")
		}
	}
}

func BenchmarkBigInt(t *testing.B) {
	bint := new(big.Int).SetBytes(make([]byte, 128/8))
	for i := 0; i < t.N; i++ {
		if bint.Bit(i%benchmarkSize) != 0 {
			t.Fatal("bad")
		}
		bint.SetBit(bint, i%benchmarkSize, 1)
		bint.SetBit(bint, i%benchmarkSize, 0)
		bint.SetBit(bint, i%benchmarkSize, 1)
		bint.SetBit(bint, i%benchmarkSize, 0)
		bint.SetBit(bint, i%benchmarkSize, 1)
		bint.SetBit(bint, i%benchmarkSize, 0)
		bint.SetBit(bint, i%benchmarkSize, 1)
		if bint.Bit(i%benchmarkSize) != 1 {
			t.Fatal("bad")
		}
		bint.SetBit(bint, i%benchmarkSize, 0)
		bint.SetBit(bint, i%benchmarkSize, 1)
		bint.SetBit(bint, i%benchmarkSize, 0)
		bint.SetBit(bint, i%benchmarkSize, 1)
		bint.SetBit(bint, i%benchmarkSize, 0)
		bint.SetBit(bint, i%benchmarkSize, 1)
		bint.SetBit(bint, i%benchmarkSize, 0)
		if bint.Bit(i%benchmarkSize) != 0 {
			t.Fatal("bad")
		}
	}
}
