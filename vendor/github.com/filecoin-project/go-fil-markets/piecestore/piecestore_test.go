package piecestore_test

import (
	"math/rand"
	"testing"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/shared_testutil"
)

func TestStorePieceInfo(t *testing.T) {

	pieceCid := shared_testutil.GenerateCids(1)[0]
	initializePieceStore := func(t *testing.T) piecestore.PieceStore {
		ps := piecestore.NewPieceStore(datastore.NewMapDatastore())
		_, err := ps.GetPieceInfo(pieceCid)
		assert.Error(t, err)
		return ps
	}

	// Add a deal info
	t.Run("can add deals", func(t *testing.T) {
		ps := initializePieceStore(t)
		dealInfo := piecestore.DealInfo{
			DealID:   abi.DealID(rand.Uint64()),
			SectorID: rand.Uint64(),
			Offset:   rand.Uint64(),
			Length:   rand.Uint64(),
		}
		err := ps.AddDealForPiece(pieceCid, dealInfo)
		assert.NoError(t, err)

		pi, err := ps.GetPieceInfo(pieceCid)
		assert.NoError(t, err)
		assert.Len(t, pi.Deals, 1)
		assert.Equal(t, pi.Deals[0], dealInfo)
	})

	t.Run("adding same deal twice does not dup", func(t *testing.T) {
		ps := initializePieceStore(t)
		dealInfo := piecestore.DealInfo{
			DealID:   abi.DealID(rand.Uint64()),
			SectorID: rand.Uint64(),
			Offset:   rand.Uint64(),
			Length:   rand.Uint64(),
		}
		err := ps.AddDealForPiece(pieceCid, dealInfo)
		assert.NoError(t, err)

		pi, err := ps.GetPieceInfo(pieceCid)
		assert.NoError(t, err)
		assert.Len(t, pi.Deals, 1)
		assert.Equal(t, pi.Deals[0], dealInfo)

		err = ps.AddDealForPiece(pieceCid, dealInfo)
		assert.NoError(t, err)

		pi, err = ps.GetPieceInfo(pieceCid)
		assert.NoError(t, err)
		assert.Len(t, pi.Deals, 1)
		assert.Equal(t, pi.Deals[0], dealInfo)
	})
}

func TestStoreCIDInfo(t *testing.T) {
	pieceCids := shared_testutil.GenerateCids(2)
	pieceCid1 := pieceCids[0]
	pieceCid2 := pieceCids[1]
	testCIDs := shared_testutil.GenerateCids(3)
	blockLocations := make([]piecestore.BlockLocation, 0, 3)
	for i := 0; i < 3; i++ {
		blockLocations = append(blockLocations, piecestore.BlockLocation{
			RelOffset: rand.Uint64(),
			BlockSize: rand.Uint64(),
		})
	}

	initializePieceStore := func(t *testing.T) piecestore.PieceStore {
		ps := piecestore.NewPieceStore(datastore.NewMapDatastore())
		_, err := ps.GetCIDInfo(testCIDs[0])
		assert.Error(t, err)
		return ps
	}

	t.Run("can add piece block locations", func(t *testing.T) {
		ps := initializePieceStore(t)
		err := ps.AddPieceBlockLocations(pieceCid1, map[cid.Cid]piecestore.BlockLocation{
			testCIDs[0]: blockLocations[0],
			testCIDs[1]: blockLocations[1],
			testCIDs[2]: blockLocations[2],
		})
		assert.NoError(t, err)

		ci, err := ps.GetCIDInfo(testCIDs[0])
		assert.NoError(t, err)
		assert.Len(t, ci.PieceBlockLocations, 1)
		assert.Equal(t, ci.PieceBlockLocations[0], piecestore.PieceBlockLocation{blockLocations[0], pieceCid1})

		ci, err = ps.GetCIDInfo(testCIDs[1])
		assert.NoError(t, err)
		assert.Len(t, ci.PieceBlockLocations, 1)
		assert.Equal(t, ci.PieceBlockLocations[0], piecestore.PieceBlockLocation{blockLocations[1], pieceCid1})

		ci, err = ps.GetCIDInfo(testCIDs[2])
		assert.NoError(t, err)
		assert.Len(t, ci.PieceBlockLocations, 1)
		assert.Equal(t, ci.PieceBlockLocations[0], piecestore.PieceBlockLocation{blockLocations[2], pieceCid1})
	})

	t.Run("overlapping adds", func(t *testing.T) {
		ps := initializePieceStore(t)
		err := ps.AddPieceBlockLocations(pieceCid1, map[cid.Cid]piecestore.BlockLocation{
			testCIDs[0]: blockLocations[0],
			testCIDs[1]: blockLocations[2],
		})
		assert.NoError(t, err)
		err = ps.AddPieceBlockLocations(pieceCid2, map[cid.Cid]piecestore.BlockLocation{
			testCIDs[1]: blockLocations[1],
			testCIDs[2]: blockLocations[2],
		})
		assert.NoError(t, err)

		ci, err := ps.GetCIDInfo(testCIDs[0])
		assert.NoError(t, err)
		assert.Len(t, ci.PieceBlockLocations, 1)
		assert.Equal(t, ci.PieceBlockLocations[0], piecestore.PieceBlockLocation{blockLocations[0], pieceCid1})

		ci, err = ps.GetCIDInfo(testCIDs[1])
		assert.NoError(t, err)
		assert.Len(t, ci.PieceBlockLocations, 2)
		assert.Equal(t, ci.PieceBlockLocations[0], piecestore.PieceBlockLocation{blockLocations[2], pieceCid1})
		assert.Equal(t, ci.PieceBlockLocations[1], piecestore.PieceBlockLocation{blockLocations[1], pieceCid2})

		ci, err = ps.GetCIDInfo(testCIDs[2])
		assert.NoError(t, err)
		assert.Len(t, ci.PieceBlockLocations, 1)
		assert.Equal(t, ci.PieceBlockLocations[0], piecestore.PieceBlockLocation{blockLocations[2], pieceCid2})
	})

	t.Run("duplicate adds", func(t *testing.T) {
		ps := initializePieceStore(t)
		err := ps.AddPieceBlockLocations(pieceCid1, map[cid.Cid]piecestore.BlockLocation{
			testCIDs[0]: blockLocations[0],
			testCIDs[1]: blockLocations[1],
		})
		assert.NoError(t, err)
		err = ps.AddPieceBlockLocations(pieceCid1, map[cid.Cid]piecestore.BlockLocation{
			testCIDs[1]: blockLocations[1],
			testCIDs[2]: blockLocations[2],
		})
		assert.NoError(t, err)

		ci, err := ps.GetCIDInfo(testCIDs[0])
		assert.NoError(t, err)
		assert.Len(t, ci.PieceBlockLocations, 1)
		assert.Equal(t, ci.PieceBlockLocations[0], piecestore.PieceBlockLocation{blockLocations[0], pieceCid1})

		ci, err = ps.GetCIDInfo(testCIDs[1])
		assert.NoError(t, err)
		assert.Len(t, ci.PieceBlockLocations, 1)
		assert.Equal(t, ci.PieceBlockLocations[0], piecestore.PieceBlockLocation{blockLocations[1], pieceCid1})

		ci, err = ps.GetCIDInfo(testCIDs[2])
		assert.NoError(t, err)
		assert.Len(t, ci.PieceBlockLocations, 1)
		assert.Equal(t, ci.PieceBlockLocations[0], piecestore.PieceBlockLocation{blockLocations[2], pieceCid1})
	})
}
