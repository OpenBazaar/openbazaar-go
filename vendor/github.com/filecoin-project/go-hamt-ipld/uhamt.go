package hamt

import (
	"math/big"
	"math/bits"
)

// indexForBitPos returns the index within the collapsed array corresponding to
// the given bit in the bitset.  The collapsed array contains only one entry
// per bit set in the bitfield, and this function is used to map the indices.
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
