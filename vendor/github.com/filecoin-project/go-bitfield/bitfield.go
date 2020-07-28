package bitfield

import (
	"errors"
	"fmt"
	"io"

	rlepluslazy "github.com/filecoin-project/go-bitfield/rle"
	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"
)

var (
	ErrBitFieldTooMany = errors.New("to many items in RLE")
	ErrNoBitsSet       = errors.New("bitfield has no set bits")
)

type BitField struct {
	rle rlepluslazy.RLE

	set   map[uint64]struct{}
	unset map[uint64]struct{}
}

// New constructs a new BitField.
func New() BitField {
	bf, err := NewFromBytes([]byte{})
	if err != nil {
		panic(fmt.Sprintf("creating empty rle: %+v", err))
	}
	return bf
}

// NewFromBytes deserializes the encoded bitfield.
func NewFromBytes(rle []byte) (BitField, error) {
	bf := BitField{}
	rlep, err := rlepluslazy.FromBuf(rle)
	if err != nil {
		return BitField{}, xerrors.Errorf("could not decode rle+: %w", err)
	}
	bf.rle = rlep
	bf.set = make(map[uint64]struct{})
	bf.unset = make(map[uint64]struct{})
	return bf, nil

}

func newWithRle(rle rlepluslazy.RLE) *BitField {
	return &BitField{
		set:   make(map[uint64]struct{}),
		unset: make(map[uint64]struct{}),
		rle:   rle,
	}
}

// NewFromSet constructs a bitfield from the given set.
func NewFromSet(setBits []uint64) *BitField {
	res := &BitField{
		set:   make(map[uint64]struct{}, len(setBits)),
		unset: make(map[uint64]struct{}),
	}
	for _, b := range setBits {
		res.set[b] = struct{}{}
	}
	return res
}

// NewFromIter constructs a BitField from the RunIterator.
func NewFromIter(r rlepluslazy.RunIterator) (*BitField, error) {
	buf, err := rlepluslazy.EncodeRuns(r, nil)
	if err != nil {
		return nil, err
	}

	rle, err := rlepluslazy.FromBuf(buf)
	if err != nil {
		return nil, err
	}

	return newWithRle(rle), nil
}

// MergeBitFields returns the union of the two BitFields.
//
// For example, given two BitFields:
//
//     0 1 1 0 1
//     1 1 0 1 0
//
// MergeBitFields would return
//
//     1 1 1 1 1
//
// This operation's runtime is O(number of runs).
func MergeBitFields(a, b *BitField) (*BitField, error) {
	ra, err := a.RunIterator()
	if err != nil {
		return nil, err
	}

	rb, err := b.RunIterator()
	if err != nil {
		return nil, err
	}

	merge, err := rlepluslazy.Or(ra, rb)
	if err != nil {
		return nil, err
	}

	mergebytes, err := rlepluslazy.EncodeRuns(merge, nil)
	if err != nil {
		return nil, err
	}

	rle, err := rlepluslazy.FromBuf(mergebytes)
	if err != nil {
		return nil, err
	}

	return newWithRle(rle), nil
}

// MultiMerge returns the unions of all the passed BitFields.
//
// Calling MultiMerge is identical to calling MergeBitFields repeatedly, just
// more efficient when merging more than two BitFields.
//
// This operation's runtime is O(number of runs * number of bitfields).
func MultiMerge(bfs ...*BitField) (*BitField, error) {
	if len(bfs) == 0 {
		return NewFromSet(nil), nil
	}

	iters := make([]rlepluslazy.RunIterator, 0, len(bfs))
	for _, bf := range bfs {
		iter, err := bf.RunIterator()
		if err != nil {
			return nil, err
		}
		iters = append(iters, iter)
	}

	iter, err := rlepluslazy.Union(iters...)
	if err != nil {
		return nil, err
	}
	return NewFromIter(iter)
}

func (bf *BitField) RunIterator() (rlepluslazy.RunIterator, error) {
	iter, err := bf.rle.RunIterator()
	if err != nil {
		return nil, err
	}
	if len(bf.set) > 0 {
		slc := make([]uint64, 0, len(bf.set))
		for b := range bf.set {
			slc = append(slc, b)
		}
		set, err := rlepluslazy.RunsFromSlice(slc)
		if err != nil {
			return nil, err
		}
		newIter, err := rlepluslazy.Or(iter, set)
		if err != nil {
			return nil, err
		}
		iter = newIter
	}
	if len(bf.unset) > 0 {
		slc := make([]uint64, 0, len(bf.unset))
		for b := range bf.unset {
			slc = append(slc, b)
		}

		unset, err := rlepluslazy.RunsFromSlice(slc)
		if err != nil {
			return nil, err
		}
		newIter, err := rlepluslazy.Subtract(iter, unset)
		if err != nil {
			return nil, err
		}
		iter = newIter
	}
	return iter, nil
}

// Set sets the given bit in the BitField
//
// This operation's runtime is O(1) up-front. However, it adds an O(bits
// explicitly set) cost to all other operations.
func (bf *BitField) Set(bit uint64) {
	delete(bf.unset, bit)
	bf.set[bit] = struct{}{}
}

// Unset unsets given bit in the BitField
//
// This operation's runtime is O(1). However, it adds an O(bits
// explicitly unset) cost to all other operations.
func (bf *BitField) Unset(bit uint64) {
	delete(bf.set, bit)
	bf.unset[bit] = struct{}{}
}

// Count counts the non-zero bits in the bitfield.
//
// For example, given:
//
//     1 0 1 1
//
// Count() will return 3.
//
// This operation's runtime is O(number of runs).
func (bf *BitField) Count() (uint64, error) {
	s, err := bf.RunIterator()
	if err != nil {
		return 0, err
	}
	return rlepluslazy.Count(s)
}

// All returns a slice of set bits in sorted order.
//
// For example, given:
//
//     1 0 0 1
//
// All will return:
//
//     []uint64{0, 3}
//
// This operation's runtime is O(number of bits).
func (bf *BitField) All(max uint64) ([]uint64, error) {
	c, err := bf.Count()
	if err != nil {
		return nil, xerrors.Errorf("count errror: %w", err)
	}
	if c > max {
		return nil, xerrors.Errorf("expected %d, got %d: %w", max, c, ErrBitFieldTooMany)
	}

	runs, err := bf.RunIterator()
	if err != nil {
		return nil, err
	}

	res, err := rlepluslazy.SliceFromRuns(runs)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// AllMap returns a map of all set bits.
//
// For example, given:
//
//     1 0 0 1
//
// All will return:
//
//     map[uint64]bool{0: true, 3: true}
//
// This operation's runtime is O(number of bits).
func (bf *BitField) AllMap(max uint64) (map[uint64]bool, error) {
	c, err := bf.Count()
	if err != nil {
		return nil, xerrors.Errorf("count errror: %w", err)
	}
	if c > max {
		return nil, xerrors.Errorf("expected %d, got %d: %w", max, c, ErrBitFieldTooMany)
	}

	runs, err := bf.RunIterator()
	if err != nil {
		return nil, err
	}

	res, err := rlepluslazy.SliceFromRuns(runs)
	if err != nil {
		return nil, err
	}

	out := make(map[uint64]bool, len(res))
	for _, i := range res {
		out[i] = true
	}
	return out, nil
}

func (bf *BitField) MarshalCBOR(w io.Writer) error {
	if bf == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	s, err := bf.RunIterator()
	if err != nil {
		return err
	}

	rle, err := rlepluslazy.EncodeRuns(s, []byte{})
	if err != nil {
		return err
	}

	if len(rle) > 8192 {
		return xerrors.Errorf("encoded bitfield was too large (%d)", len(rle))
	}

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajByteString, uint64(len(rle)))); err != nil {
		return err
	}
	if _, err = w.Write(rle); err != nil {
		return xerrors.Errorf("writing rle: %w", err)
	}
	return nil
}

func (bf *BitField) UnmarshalCBOR(r io.Reader) error {
	br := cbg.GetPeeker(r)

	maj, extra, err := cbg.CborReadHeader(br)
	if err != nil {
		return err
	}
	if extra > 8192 {
		return fmt.Errorf("array too large")
	}

	if maj != cbg.MajByteString {
		return fmt.Errorf("expected byte array")
	}

	buf := make([]byte, extra)
	if _, err := io.ReadFull(br, buf); err != nil {
		return err
	}

	rle, err := rlepluslazy.FromBuf(buf)
	if err != nil {
		return xerrors.Errorf("could not decode rle+: %w", err)
	}
	bf.rle = rle
	bf.set = make(map[uint64]struct{})
	bf.unset = make(map[uint64]struct{})

	return nil
}

func (bf *BitField) MarshalJSON() ([]byte, error) {

	c, err := bf.Copy()
	if err != nil {
		return nil, err
	}

	return c.rle.MarshalJSON()
}

func (bf *BitField) UnmarshalJSON(b []byte) error {

	err := bf.rle.UnmarshalJSON(b)
	if err != nil {
		return err
	}
	bf.set = make(map[uint64]struct{})
	bf.unset = make(map[uint64]struct{})
	return nil
}

// ForEach iterates over each set bit.
//
// This operation's runtime is O(bits set).
func (bf *BitField) ForEach(f func(uint64) error) error {
	iter, err := bf.RunIterator()
	if err != nil {
		return err
	}

	var i uint64
	for iter.HasNext() {
		r, err := iter.NextRun()
		if err != nil {
			return err
		}

		if r.Val {
			for j := uint64(0); j < r.Len; j++ {
				if err := f(i); err != nil {
					return err
				}
				i++
			}
		} else {
			i += r.Len
		}
	}
	return nil
}

// IsSet returns true if the given bit is set.
//
// This operation's runtime is O(number of runs).
func (bf *BitField) IsSet(x uint64) (bool, error) {
	if _, ok := bf.set[x]; ok {
		return true, nil
	}

	if _, ok := bf.unset[x]; ok {
		return false, nil
	}

	iter, err := bf.rle.RunIterator()
	if err != nil {
		return false, err
	}

	return rlepluslazy.IsSet(iter, x)
}

// First returns the index of the first set bit. This function returns
// ErrNoBitsSet when no bits have been set.
//
// This operation's runtime is O(1).
func (bf *BitField) First() (uint64, error) {
	iter, err := bf.RunIterator()
	if err != nil {
		return 0, err
	}

	var i uint64
	for iter.HasNext() {
		r, err := iter.NextRun()
		if err != nil {
			return 0, err
		}

		if r.Val {
			return i, nil
		} else {
			i += r.Len
		}
	}
	return 0, ErrNoBitsSet
}

// IsEmpty returns true if the bitset is empty.
//
// This operation's runtime is O(1).
func (bf *BitField) IsEmpty() (bool, error) {
	_, err := bf.First()
	switch err {
	case ErrNoBitsSet:
		return true, nil
	case nil:
		return false, nil
	default:
		return false, err
	}
}

// Slice treats the BitField as an ordered set of set bits, then slices this set.
//
// That is, it skips start set bits, then returns the next count set bits.
//
// For example, given:
//
//    1 0 1 1 0 1 1
//
// bf.Slice(2, 2) would return:
//
//    0 0 0 1 0 1 0
//
// This operation's runtime is O(number of runs).
func (bf *BitField) Slice(start, count uint64) (*BitField, error) {
	iter, err := bf.RunIterator()
	if err != nil {
		return nil, err
	}

	valsUntilStart := start

	var sliceRuns []rlepluslazy.Run
	var i, outcount uint64
	for iter.HasNext() && valsUntilStart > 0 {
		r, err := iter.NextRun()
		if err != nil {
			return nil, err
		}

		if r.Val {
			if r.Len <= valsUntilStart {
				valsUntilStart -= r.Len
				i += r.Len
			} else {
				i += valsUntilStart

				rem := r.Len - valsUntilStart
				if rem > count {
					rem = count
				}

				sliceRuns = append(sliceRuns,
					rlepluslazy.Run{Val: false, Len: i},
					rlepluslazy.Run{Val: true, Len: rem},
				)
				outcount += rem
				valsUntilStart = 0
			}
		} else {
			i += r.Len
		}
	}

	for iter.HasNext() && outcount < count {
		r, err := iter.NextRun()
		if err != nil {
			return nil, err
		}

		if r.Val {
			if r.Len <= count-outcount {
				sliceRuns = append(sliceRuns, r)
				outcount += r.Len
			} else {
				sliceRuns = append(sliceRuns, rlepluslazy.Run{Val: true, Len: count - outcount})
				outcount = count
			}
		} else {
			if len(sliceRuns) == 0 {
				r.Len += i
			}
			sliceRuns = append(sliceRuns, r)
		}
	}
	if outcount < count {
		return nil, fmt.Errorf("not enough bits set in field to satisfy slice count")
	}

	buf, err := rlepluslazy.EncodeRuns(&rlepluslazy.RunSliceIterator{Runs: sliceRuns}, nil)
	if err != nil {
		return nil, err
	}

	rle, err := rlepluslazy.FromBuf(buf)
	if err != nil {
		return nil, err
	}

	return &BitField{rle: rle}, nil
}

// IntersectBitField returns the intersection of the two BitFields.
//
// For example, given two BitFields:
//
//     0 1 1 0 1
//     1 1 0 1 0
//
// IntersectBitField would return
//
//     0 1 0 0 0
//
// This operation's runtime is O(number of runs).
func IntersectBitField(a, b *BitField) (*BitField, error) {
	ar, err := a.RunIterator()
	if err != nil {
		return nil, err
	}

	br, err := b.RunIterator()
	if err != nil {
		return nil, err
	}

	andIter, err := rlepluslazy.And(ar, br)
	if err != nil {
		return nil, err
	}

	buf, err := rlepluslazy.EncodeRuns(andIter, nil)
	if err != nil {
		return nil, err
	}

	rle, err := rlepluslazy.FromBuf(buf)
	if err != nil {
		return nil, err
	}

	return newWithRle(rle), nil
}

// SubtractBitField returns the difference between the two BitFields. That is,
// it returns a bitfield of all bits set in a but not set in b.
//
// For example, given two BitFields:
//
//     0 1 1 0 1 // a
//     1 1 0 1 0 // b
//
// SubtractBitFields would return
//
//     0 0 1 0 1
//
// This operation's runtime is O(number of runs).
func SubtractBitField(a, b *BitField) (*BitField, error) {
	ar, err := a.RunIterator()
	if err != nil {
		return nil, err
	}

	br, err := b.RunIterator()
	if err != nil {
		return nil, err
	}

	andIter, err := rlepluslazy.Subtract(ar, br)
	if err != nil {
		return nil, err
	}

	buf, err := rlepluslazy.EncodeRuns(andIter, nil)
	if err != nil {
		return nil, err
	}

	rle, err := rlepluslazy.FromBuf(buf)
	if err != nil {
		return nil, err
	}

	return newWithRle(rle), nil
}

// Copy flushes the bitfield and returns a copy that can be mutated
// without changing the original values
func (bf *BitField) Copy() (*BitField, error) {
	r, err := bf.RunIterator()
	if err != nil {
		return nil, err
	}

	buf, err := rlepluslazy.EncodeRuns(r, nil)
	if err != nil {
		return nil, err
	}

	rle, err := rlepluslazy.FromBuf(buf)
	if err != nil {
		return nil, err
	}

	return newWithRle(rle), nil
}

// BitIterator iterates over the bits in the bitmap
func (bf *BitField) BitIterator() (rlepluslazy.BitIterator, error) {
	r, err := bf.RunIterator()
	if err != nil {
		return nil, err
	}
	return rlepluslazy.BitsFromRuns(r)
}
