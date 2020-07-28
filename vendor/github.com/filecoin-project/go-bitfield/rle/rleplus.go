package rlepluslazy

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"golang.org/x/xerrors"
)

const Version = 0

var (
	ErrWrongVersion = errors.New("invalid RLE+ version")
	ErrDecode       = fmt.Errorf("invalid encoding for RLE+ version %d", Version)
)

type RLE struct {
	buf  []byte
	runs []Run
}

func FromBuf(buf []byte) (RLE, error) {
	rle := RLE{buf: buf}

	if len(buf) > 0 && buf[0]&3 != Version {
		return RLE{}, xerrors.Errorf("could not create RLE+ for a buffer: %w", ErrWrongVersion)
	}

	return rle, nil
}

func (rle *RLE) RunIterator() (RunIterator, error) {
	if rle.runs == nil {
		source, err := DecodeRLE(rle.buf)
		if err != nil {
			return nil, xerrors.Errorf("decoding RLE: %w", err)
		}
		var length uint64
		var runs []Run
		for source.HasNext() {
			r, err := source.NextRun()
			if err != nil {
				return nil, xerrors.Errorf("reading run: %w", err)
			}
			if math.MaxUint64-r.Len < length {
				return nil, xerrors.New("RLE+ overflows")
			}
			length += r.Len
			runs = append(runs, r)
		}
		rle.runs = runs
	}

	return &RunSliceIterator{Runs: rle.runs}, nil
}

func (rle *RLE) Count() (uint64, error) {
	it, err := rle.RunIterator()
	if err != nil {
		return 0, err
	}
	return Count(it)
}

// Encoded as an array of run-lengths, always starting with zeroes (absent values)
// E.g.: The set {0, 1, 2, 8, 9} is the bitfield 1110000011, and would be marshalled as [0, 3, 5, 2]
func (rle *RLE) MarshalJSON() ([]byte, error) {
	r, err := rle.RunIterator()
	if err != nil {
		return nil, err
	}

	var ret []uint64
	if r.HasNext() {
		first, err := r.NextRun()
		if err != nil {
			return nil, err
		}
		if first.Val {
			ret = append(ret, 0)
		}
		ret = append(ret, first.Len)

		for r.HasNext() {
			next, err := r.NextRun()
			if err != nil {
				return nil, err
			}

			ret = append(ret, next.Len)
		}
	} else {
		ret = []uint64{0}
	}

	return json.Marshal(ret)
}

func (rle *RLE) UnmarshalJSON(b []byte) error {
	var buf []uint64

	if err := json.Unmarshal(b, &buf); err != nil {
		return err
	}

	rle.runs = []Run{}
	val := false
	for i, v := range buf {
		if v == 0 {
			if i != 0 {
				return xerrors.New("Cannot have a zero-length run except at start")
			}
		} else {
			rle.runs = append(rle.runs, Run{
				Val: val,
				Len: v,
			})
		}
		val = !val
	}

	return nil
}
