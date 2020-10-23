package util

import (
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-bitfield/rle"
)

type BitField = bitfield.BitField

func isEmpty(iter rlepluslazy.RunIterator) (bool, error) {
	// Look for the first non-zero bit.
	for iter.HasNext() {
		r, err := iter.NextRun()
		if err != nil {
			return false, err
		}
		if r.Val {
			return false, nil
		}
	}
	return true, nil
}

// Checks whether bitfield `a` contains any bit that is set in bitfield `b`.
func BitFieldContainsAny(a, b BitField) (bool, error) {
	aruns, err := a.RunIterator()
	if err != nil {
		return false, err
	}

	bruns, err := b.RunIterator()
	if err != nil {
		return false, err
	}

	// Take the intersection of the two bitfields.
	combined, err := rlepluslazy.And(aruns, bruns)
	if err != nil {
		return false, err
	}

	// Look for the first non-zero bit.
	empty, err := isEmpty(combined)
	if err != nil {
		return false, err
	}
	return !empty, nil
}

// Checks whether bitfield `a` contains all bits set in bitfield `b`.
func BitFieldContainsAll(a, b BitField) (bool, error) {
	aruns, err := a.RunIterator()
	if err != nil {
		return false, err
	}

	bruns, err := b.RunIterator()
	if err != nil {
		return false, err
	}

	// Remove any elements in a from b. If b contains bits not in a, some
	// bits will remain.
	combined, err := rlepluslazy.Subtract(bruns, aruns)
	if err != nil {
		return false, err
	}
	return isEmpty(combined)
}
