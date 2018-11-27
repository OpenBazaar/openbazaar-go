package cid

import (
	"encoding/binary"
	"testing"
)

func TestUvarintRoundTrip(t *testing.T) {
	testCases := []uint64{0, 1, 2, 127, 128, 129, 255, 256, 257, 1<<63 - 1}
	for _, tc := range testCases {
		buf := make([]byte, 16)
		binary.PutUvarint(buf, tc)
		v, l1 := uvarint(string(buf))
		_, l2 := binary.Uvarint(buf)
		if tc != v {
			t.Errorf("roundtrip failed expected %d but got %d", tc, v)
		}
		if l1 != l2 {
			t.Errorf("length incorrect expected %d but got %d", l2, l1)
		}
	}
}
