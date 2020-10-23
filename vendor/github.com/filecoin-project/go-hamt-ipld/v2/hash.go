package hamt

import (
	"fmt"

	"github.com/spaolacci/murmur3"
)

// hashBits is a helper that allows the reading of the 'next n bits' of a
// digest as an integer. State is retained and calls to `Next` will
// increment the number of consumed bits.
type hashBits struct {
	b        []byte
	consumed int
}

func mkmask(n int) byte {
	return (1 << uint(n)) - 1
}

// Next returns the next 'i' bits of the hashBits value as an integer, or an
// error if there aren't enough bits.
// Not enough bits means that the tree is not large enough to contain the data.
// Where the hash is providing a sufficient enough random distribution this
// means that it is "full", Where the distribution is not sufficiently random
// enough, this means there have been too many collisions. Where a user can
// control keys (that are hashed) and the hash function has some
// predictability, collisions can be forced by producing the same indexes at
// (most) levels.
func (hb *hashBits) Next(i int) (int, error) {
	if hb.consumed+i > len(hb.b)*8 {
		// TODO(rvagg): this msg looks like a UnixFS holdover, it's an overflow
		// and should probably bubble up a proper Err*
		return 0, fmt.Errorf("sharded directory too deep")
	}
	return hb.next(i), nil
}

// where 'i' is not '8', we need to read up to two bytes to extract the bits
// for the index.
func (hb *hashBits) next(i int) int {
	curbi := hb.consumed / 8
	leftb := 8 - (hb.consumed % 8)

	curb := hb.b[curbi]
	if i == leftb {
		out := int(mkmask(i) & curb)
		hb.consumed += i
		return out
	} else if i < leftb {
		a := curb & mkmask(leftb) // mask out the high bits we don't want
		b := a & ^mkmask(leftb-i) // mask out the low bits we don't want
		c := b >> uint(leftb-i)   // shift whats left down
		hb.consumed += i
		return int(c)
	} else {
		out := int(mkmask(leftb) & curb)
		out <<= uint(i - leftb)
		hb.consumed += leftb
		out += hb.next(i - leftb)
		return out
	}
}

func defaultHashFunction(val []byte) []byte {
	h := murmur3.New64()
	h.Write(val)
	return h.Sum(nil)
}
