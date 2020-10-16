package abi

import (
	"math/bits"

	cid "github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
)

// UnpaddedPieceSize is the size of a piece, in bytes
type UnpaddedPieceSize uint64
type PaddedPieceSize uint64

func (s UnpaddedPieceSize) Padded() PaddedPieceSize {
	return PaddedPieceSize(s + (s / 127))
}

func (s UnpaddedPieceSize) Validate() error {
	if s < 127 {
		return xerrors.New("minimum piece size is 127 bytes")
	}

	// is 127 * 2^n
	if uint64(s)>>bits.TrailingZeros64(uint64(s)) != 127 {
		return xerrors.New("unpadded piece size must be a power of 2 multiple of 127")
	}

	return nil
}

func (s PaddedPieceSize) Unpadded() UnpaddedPieceSize {
	return UnpaddedPieceSize(s - (s / 128))
}

func (s PaddedPieceSize) Validate() error {
	if s < 128 {
		return xerrors.New("minimum padded piece size is 128 bytes")
	}

	if bits.OnesCount64(uint64(s)) != 1 {
		return xerrors.New("padded piece size must be a power of 2")
	}

	return nil
}

type PieceInfo struct {
	Size     PaddedPieceSize // Size in nodes. For BLS12-381 (capacity 254 bits), must be >= 16. (16 * 8 = 128)
	PieceCID cid.Cid
}
