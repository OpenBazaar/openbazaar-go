package hamt

import (
	"math/big"
	"math/bits"
)

// indexForBitPos returns the index within the collapsed array corresponding to
// the given bit in the bitset.  The collapsed array contains only one entry
// per bit set in the bitfield, and this function is used to map the indices.
// This is similar to a popcount() operation but is limited to a certain index.
// e.g. a Bitfield of `10010110000` shows that we have a 4 elements in the
// associated array. Indexes `[1]` and `[2]` are not present, but index `[3]`
// is at the second position of our Pointers array.
func (n *Node) indexForBitPos(bp int) int {
	return indexForBitPos(bp, n.Bitfield)
}

func indexForBitPos(bp int, bitfield *big.Int) int {
	var x uint
	var count, i int
	w := bitfield.Bits()
	for x = uint(bp); x > bits.UintSize && i < len(w); x -= bits.UintSize {
		count += bits.OnesCount(uint(w[i]))
		i++
	}
	if i == len(w) {
		return count
	}
	return count + bits.OnesCount(uint(w[i])&((1<<x)-1))
}

// How many elements does the bitfield say we should have? Count the ones.
func (n *Node) bitsSetCount() int {
	w := n.Bitfield.Bits()
	count := 0
	for _, b := range w {
		count += bits.OnesCount(uint(b))
	}
	return count
}
