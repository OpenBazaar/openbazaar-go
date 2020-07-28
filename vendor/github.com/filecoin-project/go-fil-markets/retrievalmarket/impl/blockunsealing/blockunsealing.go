/*
Package blockunsealing contains the logic needed to unseal sealed blocks for retrieval
*/
package blockunsealing

import (
	"bytes"
	"context"
	"io"

	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-fil-markets/pieceio"
	"github.com/filecoin-project/go-fil-markets/piecestore"
)

// LoaderWithUnsealing is an ipld.Loader function that will also unseal pieces as needed
type LoaderWithUnsealing interface {
	Load(lnk ipld.Link, lnkCtx ipld.LinkContext) (io.Reader, error)
}

type loaderWithUnsealing struct {
	ctx        context.Context
	bs         blockstore.Blockstore
	pieceStore piecestore.PieceStore
	carIO      pieceio.CarIO
	unsealer   UnsealingFunc
	pieceCid   *cid.Cid
}

// UnsealingFunc is a function that unseals sectors at a given offset and length
type UnsealingFunc func(ctx context.Context, sectorId uint64, offset uint64, length uint64) (io.ReadCloser, error)

// NewLoaderWithUnsealing creates a loader that will attempt to read blocks from the blockstore but unseal the piece
// as needed using the passed unsealing function
func NewLoaderWithUnsealing(ctx context.Context, bs blockstore.Blockstore, pieceStore piecestore.PieceStore, carIO pieceio.CarIO, unsealer UnsealingFunc, pieceCid *cid.Cid) LoaderWithUnsealing {
	return &loaderWithUnsealing{ctx, bs, pieceStore, carIO, unsealer, pieceCid}
}

func (lu *loaderWithUnsealing) Load(lnk ipld.Link, lnkCtx ipld.LinkContext) (io.Reader, error) {
	cl, ok := lnk.(cidlink.Link)
	if !ok {
		return nil, xerrors.New("Unsupported link type")
	}
	c := cl.Cid
	// check if intermediate blockstore has cid
	has, err := lu.bs.Has(c)
	if err != nil {
		return nil, xerrors.Errorf("attempting to load cid from blockstore: %w", err)
	}

	// attempt unseal if block is not in blockstore
	if !has {
		err = lu.attemptUnseal(c)
		if err != nil {
			return nil, err
		}
	}

	blk, err := lu.bs.Get(c)
	if err != nil {
		return nil, xerrors.Errorf("attempting to load cid from blockstore: %w", err)
	}

	return bytes.NewReader(blk.RawData()), nil
}

func (lu *loaderWithUnsealing) attemptUnseal(c cid.Cid) error {
	var err error
	var reader io.Reader
	var cidInfo piecestore.CIDInfo

	// if the deal proposal specified a Piece CID, only check that piece
	if lu.pieceCid != nil {
		reader, err = lu.firstSuccessfulUnsealByPieceCID(*lu.pieceCid)
	} else {
		cidInfo, err = lu.pieceStore.GetCIDInfo(c)
		if err != nil {
			return xerrors.Errorf("error looking up information on CID: %w", err)
		}

		reader, err = lu.firstSuccessfulUnseal(cidInfo)
	}
	// no successful unseal
	if err != nil {
		return xerrors.Errorf("Unable to unseal piece: %w", err)
	}

	// attempt to load data as a car file into the block store
	_, err = lu.carIO.LoadCar(lu.bs, reader)
	if err != nil {
		return xerrors.Errorf("attempting to read Car file: %w", err)
	}

	return nil
}

func (lu *loaderWithUnsealing) firstSuccessfulUnseal(payloadCidInfo piecestore.CIDInfo) (io.ReadCloser, error) {
	var lastErr error
	for _, pieceBlockLocation := range payloadCidInfo.PieceBlockLocations {
		reader, err := lu.firstSuccessfulUnsealByPieceCID(pieceBlockLocation.PieceCID)
		if err == nil {
			return reader, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (lu *loaderWithUnsealing) firstSuccessfulUnsealByPieceCID(pieceCID cid.Cid) (io.ReadCloser, error) {
	pieceInfo, err := lu.pieceStore.GetPieceInfo(pieceCID)
	if err != nil {
		return nil, err
	}

	// try to unseal data from all pieces
	lastErr := xerrors.New("no sectors found to unseal from")
	for _, deal := range pieceInfo.Deals {
		reader, err := lu.unsealer(lu.ctx, deal.SectorID, deal.Offset, deal.Length)
		if err == nil {
			return reader, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
