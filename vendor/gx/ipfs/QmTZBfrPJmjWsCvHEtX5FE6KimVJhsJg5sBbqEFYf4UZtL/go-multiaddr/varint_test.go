package multiaddr

import (
	"encoding/binary"
	"testing"
)

func checkVarint(t *testing.T, x int) {
	buf := make([]byte, binary.MaxVarintLen64)
	expected := binary.PutUvarint(buf, uint64(x))

	size := VarintSize(x)
	if size != expected {
		t.Fatalf("expected varintsize of %d to be %d, got %d", x, expected, size)
	}
}

func TestVarintSize(t *testing.T) {
	max := 1 << 16
	for x := 0; x < max; x++ {
		checkVarint(t, x)
	}
}
