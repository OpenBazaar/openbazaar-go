package bbloom

import (
	"math/rand"
	"testing"
	"time"
)

func TestCollisionRate(t *testing.T) {
	rand.Seed(time.Now().UTC().UnixNano())
	N := 1 << 20
	M := N * 12
	K := 2

	bl, err := New(float64(M), float64(K))
	if err != nil {
		t.Fatal(err)
	}
	var buf [64]byte
	for i := 0; i < N; i++ {
		_, err := rand.Read(buf[:])
		if err != nil {
			t.Fatal(err)
		}

		bl.Add(buf[:])
	}

	Ntest := int(1e6)
	falsePositive := 0

	for i := 0; i < Ntest; i++ {
		_, err := rand.Read(buf[:])
		if err != nil {
			t.Fatal(err)
		}

		if bl.Has(buf[:]) {
			falsePositive++
		}
	}

	t.Logf("false positive ratio: %f", float64(falsePositive)/float64(Ntest))
}
