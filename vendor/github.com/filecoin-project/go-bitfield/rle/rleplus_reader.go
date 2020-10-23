package rlepluslazy

import (
	"golang.org/x/xerrors"
)

func DecodeRLE(buf []byte) (RunIterator, error) {
	if len(buf) > 0 && buf[len(buf)-1] == 0 {
		// trailing zeros bytes not allowed.
		return nil, xerrors.Errorf("not minimally encoded: %w", ErrDecode)
	}

	bv := readBitvec(buf)

	ver := bv.Get(2) // Read version
	if ver != Version {
		return nil, ErrWrongVersion
	}

	it := &rleIterator{bv: bv}

	// next run is previous in relation to prep
	// so we invert the value
	it.nextRun.Val = bv.Get(1) != 1
	if err := it.prep(); err != nil {
		return nil, err
	}
	return it, nil
}

type rleIterator struct {
	bv *rbitvec

	nextRun Run
}

func (it *rleIterator) HasNext() bool {
	return it.nextRun.Valid()
}

func (it *rleIterator) NextRun() (Run, error) {
	ret := it.nextRun
	return ret, it.prep()
}

func (it *rleIterator) prep() error {
	if it.bv.GetBit() {
		it.nextRun.Len = 1
	} else if it.bv.GetBit() {
		it.nextRun.Len = uint64(it.bv.Get(4))
	} else {
		// Modified from the go standard library. Copyright the Go Authors and
		// released under the BSD License.
		var x uint64
		var s uint
		for i := 0; ; i++ {
			if i == 10 {
				return xerrors.Errorf("run too long: %w", ErrDecode)
			}
			b := it.bv.GetByte()
			if b < 0x80 {
				if i > 9 || i == 9 && b > 1 {
					return xerrors.Errorf("run too long: %w", ErrDecode)
				} else if b == 0 && s > 0 {
					return xerrors.Errorf("invalid run: %w", ErrDecode)
				}
				x |= uint64(b) << s
				break
			}
			x |= uint64(b&0x7f) << s
			s += 7
		}
		it.nextRun.Len = x
	}

	it.nextRun.Val = !it.nextRun.Val
	return nil
}
