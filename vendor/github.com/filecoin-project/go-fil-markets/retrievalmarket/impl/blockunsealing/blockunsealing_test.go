package blockunsealing_test

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-fil-markets/pieceio/cario"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/blockunsealing"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/testnodes"
	"github.com/filecoin-project/go-fil-markets/shared"
	tut "github.com/filecoin-project/go-fil-markets/shared_testutil"
)

func TestNewLoaderWithUnsealing(t *testing.T) {
	ctx := context.Background()
	cio := cario.NewCarIO()
	testdata := tut.NewTestIPLDTree()
	var carBuffer bytes.Buffer
	err := cio.WriteCar(ctx, testdata, testdata.RootNodeLnk.(cidlink.Link).Cid, shared.AllSelector(), &carBuffer)
	require.NoError(t, err)
	carData := carBuffer.Bytes()

	setupBlockStore := func(t *testing.T) bstore.Blockstore {
		bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
		err = bs.Put(testdata.RootBlock)
		require.NoError(t, err)
		return bs
	}
	deal1 := piecestore.DealInfo{
		DealID:   abi.DealID(rand.Uint64()),
		SectorID: rand.Uint64(),
		Offset:   rand.Uint64(),
		Length:   rand.Uint64(),
	}
	deal2 := piecestore.DealInfo{
		DealID:   abi.DealID(rand.Uint64()),
		SectorID: rand.Uint64(),
		Offset:   rand.Uint64(),
		Length:   rand.Uint64(),
	}
	pieceCID := tut.GenerateCids(1)[0]
	piece := piecestore.PieceInfo{
		PieceCID: pieceCID,
		Deals: []piecestore.DealInfo{
			deal1,
			deal2,
		},
	}
	deal3 := piecestore.DealInfo{
		DealID:   abi.DealID(rand.Uint64()),
		SectorID: rand.Uint64(),
		Offset:   rand.Uint64(),
		Length:   rand.Uint64(),
	}
	pieceCID2 := tut.GenerateCids(1)[0]
	piece2 := piecestore.PieceInfo{
		PieceCID: pieceCID2,
		Deals: []piecestore.DealInfo{
			deal3,
		},
	}
	cidInfo := piecestore.CIDInfo{
		PieceBlockLocations: []piecestore.PieceBlockLocation{
			{
				PieceCID: pieceCID,
			},
			{
				PieceCID: pieceCID2,
			},
		},
	}

	checkSuccessLoad := func(t *testing.T, loaderWithUnsealing blockunsealing.LoaderWithUnsealing, lnk ipld.Link) {
		read, err := loaderWithUnsealing.Load(lnk, ipld.LinkContext{})
		require.NoError(t, err)
		readData, err := ioutil.ReadAll(read)
		require.NoError(t, err)
		c, err := lnk.(cidlink.Link).Prefix().Sum(readData)
		require.NoError(t, err)
		require.Equal(t, c.Bytes(), lnk.(cidlink.Link).Bytes())
	}

	t.Run("when intermediate blockstore has block", func(t *testing.T) {
		bs := setupBlockStore(t)
		unsealer := testnodes.NewTestRetrievalProviderNode()
		pieceStore := tut.NewTestPieceStore()
		loaderWithUnsealing := blockunsealing.NewLoaderWithUnsealing(ctx, bs, pieceStore, cio, unsealer.UnsealSector, nil)
		checkSuccessLoad(t, loaderWithUnsealing, testdata.RootNodeLnk)
		unsealer.VerifyExpectations(t)
	})

	t.Run("when caller has provided a PieceCID", func(t *testing.T) {
		t.Run("succeeds if it can locate the piece", func(t *testing.T) {
			bs := setupBlockStore(t)
			unsealer := testnodes.NewTestRetrievalProviderNode()
			unsealer.ExpectUnseal(deal1.SectorID, deal1.Offset, deal1.Length, carData)
			pieceStore := tut.NewTestPieceStore()
			pieceStore.ExpectCID(testdata.MiddleMapBlock.Cid(), cidInfo)
			pieceStore.ExpectPiece(pieceCID, piece)
			loaderWithUnsealing := blockunsealing.NewLoaderWithUnsealing(ctx, bs, pieceStore, cio, unsealer.UnsealSector, &pieceCID)
			checkSuccessLoad(t, loaderWithUnsealing, testdata.MiddleMapNodeLnk)
			unsealer.VerifyExpectations(t)
		})

		t.Run("fails if it cannot locate the piece", func(t *testing.T) {
			bs := setupBlockStore(t)
			unsealer := testnodes.NewTestRetrievalProviderNode()
			pieceStore := tut.NewTestPieceStoreWithParams(tut.TestPieceStoreParams{GetPieceInfoError: fmt.Errorf("not found")})
			loaderWithUnsealing := blockunsealing.NewLoaderWithUnsealing(ctx, bs, pieceStore, cio, unsealer.UnsealSector, &pieceCID)
			_, err := loaderWithUnsealing.Load(testdata.MiddleMapNodeLnk, ipld.LinkContext{})
			require.Error(t, err)
			unsealer.VerifyExpectations(t)
			pieceStore.VerifyExpectations(t)
		})
	})

	t.Run("when intermediate blockstore does not have block", func(t *testing.T) {
		t.Run("unsealing success on first ref", func(t *testing.T) {
			bs := setupBlockStore(t)
			unsealer := testnodes.NewTestRetrievalProviderNode()
			unsealer.ExpectUnseal(deal1.SectorID, deal1.Offset, deal1.Length, carData)
			pieceStore := tut.NewTestPieceStore()
			pieceStore.ExpectCID(testdata.MiddleMapBlock.Cid(), cidInfo)
			pieceStore.ExpectPiece(pieceCID, piece)
			loaderWithUnsealing := blockunsealing.NewLoaderWithUnsealing(ctx, bs, pieceStore, cio, unsealer.UnsealSector, nil)
			checkSuccessLoad(t, loaderWithUnsealing, testdata.MiddleMapNodeLnk)
			unsealer.VerifyExpectations(t)
			pieceStore.VerifyExpectations(t)
		})

		t.Run("unsealing success on later ref", func(t *testing.T) {
			bs := setupBlockStore(t)
			unsealer := testnodes.NewTestRetrievalProviderNode()
			unsealer.ExpectFailedUnseal(deal1.SectorID, deal1.Offset, deal1.Length)
			unsealer.ExpectUnseal(deal2.SectorID, deal2.Offset, deal2.Length, carData)
			pieceStore := tut.NewTestPieceStore()
			pieceStore.ExpectCID(testdata.MiddleMapBlock.Cid(), cidInfo)
			pieceStore.ExpectPiece(pieceCID, piece)
			loaderWithUnsealing := blockunsealing.NewLoaderWithUnsealing(ctx, bs, pieceStore, cio, unsealer.UnsealSector, nil)
			checkSuccessLoad(t, loaderWithUnsealing, testdata.MiddleMapNodeLnk)
			unsealer.VerifyExpectations(t)
			pieceStore.VerifyExpectations(t)
		})

		t.Run("unsealing success on second piece", func(t *testing.T) {
			bs := setupBlockStore(t)
			unsealer := testnodes.NewTestRetrievalProviderNode()
			unsealer.ExpectFailedUnseal(deal1.SectorID, deal1.Offset, deal1.Length)
			unsealer.ExpectFailedUnseal(deal2.SectorID, deal2.Offset, deal2.Length)
			unsealer.ExpectUnseal(deal3.SectorID, deal3.Offset, deal3.Length, carData)
			pieceStore := tut.NewTestPieceStore()
			pieceStore.ExpectCID(testdata.MiddleMapBlock.Cid(), cidInfo)
			pieceStore.ExpectPiece(pieceCID, piece)
			pieceStore.ExpectPiece(pieceCID2, piece2)
			loaderWithUnsealing := blockunsealing.NewLoaderWithUnsealing(ctx, bs, pieceStore, cio, unsealer.UnsealSector, nil)
			checkSuccessLoad(t, loaderWithUnsealing, testdata.MiddleMapNodeLnk)
			unsealer.VerifyExpectations(t)
			pieceStore.VerifyExpectations(t)
		})

		t.Run("piece lookup success on second piece", func(t *testing.T) {
			bs := setupBlockStore(t)
			unsealer := testnodes.NewTestRetrievalProviderNode()
			unsealer.ExpectUnseal(deal3.SectorID, deal3.Offset, deal3.Length, carData)
			pieceStore := tut.NewTestPieceStore()
			pieceStore.ExpectCID(testdata.MiddleMapBlock.Cid(), cidInfo)
			pieceStore.ExpectMissingPiece(pieceCID)
			pieceStore.ExpectPiece(pieceCID2, piece2)
			loaderWithUnsealing := blockunsealing.NewLoaderWithUnsealing(ctx, bs, pieceStore, cio, unsealer.UnsealSector, nil)
			checkSuccessLoad(t, loaderWithUnsealing, testdata.MiddleMapNodeLnk)
			unsealer.VerifyExpectations(t)
			pieceStore.VerifyExpectations(t)
		})

		t.Run("fails all unsealing", func(t *testing.T) {
			bs := setupBlockStore(t)
			unsealer := testnodes.NewTestRetrievalProviderNode()
			unsealer.ExpectFailedUnseal(deal1.SectorID, deal1.Offset, deal1.Length)
			unsealer.ExpectFailedUnseal(deal2.SectorID, deal2.Offset, deal2.Length)
			unsealer.ExpectFailedUnseal(deal3.SectorID, deal3.Offset, deal3.Length)
			pieceStore := tut.NewTestPieceStore()
			pieceStore.ExpectCID(testdata.MiddleMapBlock.Cid(), cidInfo)
			pieceStore.ExpectPiece(pieceCID, piece)
			pieceStore.ExpectPiece(pieceCID2, piece2)
			loaderWithUnsealing := blockunsealing.NewLoaderWithUnsealing(ctx, bs, pieceStore, cio, unsealer.UnsealSector, nil)
			_, err := loaderWithUnsealing.Load(testdata.MiddleMapNodeLnk, ipld.LinkContext{})
			require.Error(t, err)
			unsealer.VerifyExpectations(t)
			pieceStore.VerifyExpectations(t)
		})

		t.Run("fails looking up cid info", func(t *testing.T) {
			bs := setupBlockStore(t)
			unsealer := testnodes.NewTestRetrievalProviderNode()
			pieceStore := tut.NewTestPieceStore()
			pieceStore.ExpectMissingCID(testdata.MiddleMapBlock.Cid())
			loaderWithUnsealing := blockunsealing.NewLoaderWithUnsealing(ctx, bs, pieceStore, cio, unsealer.UnsealSector, nil)
			_, err := loaderWithUnsealing.Load(testdata.MiddleMapNodeLnk, ipld.LinkContext{})
			require.Error(t, err)
			unsealer.VerifyExpectations(t)
			pieceStore.VerifyExpectations(t)
		})

		t.Run("fails looking up all pieces", func(t *testing.T) {
			bs := setupBlockStore(t)
			unsealer := testnodes.NewTestRetrievalProviderNode()
			pieceStore := tut.NewTestPieceStore()
			pieceStore.ExpectCID(testdata.MiddleMapBlock.Cid(), cidInfo)
			pieceStore.ExpectMissingPiece(pieceCID)
			pieceStore.ExpectMissingPiece(pieceCID2)
			loaderWithUnsealing := blockunsealing.NewLoaderWithUnsealing(ctx, bs, pieceStore, cio, unsealer.UnsealSector, nil)
			_, err := loaderWithUnsealing.Load(testdata.MiddleMapNodeLnk, ipld.LinkContext{})
			require.Error(t, err)
			unsealer.VerifyExpectations(t)
			pieceStore.VerifyExpectations(t)
		})

		t.Run("car io failure", func(t *testing.T) {
			bs := setupBlockStore(t)
			unsealer := testnodes.NewTestRetrievalProviderNode()
			randBytes := make([]byte, 100)
			_, err := rand.Read(randBytes)
			require.NoError(t, err)
			unsealer.ExpectUnseal(deal1.SectorID, deal1.Offset, deal1.Length, randBytes)
			pieceStore := tut.NewTestPieceStore()
			pieceStore.ExpectCID(testdata.MiddleMapBlock.Cid(), cidInfo)
			pieceStore.ExpectPiece(pieceCID, piece)
			loaderWithUnsealing := blockunsealing.NewLoaderWithUnsealing(ctx, bs, pieceStore, cio, unsealer.UnsealSector, nil)
			_, err = loaderWithUnsealing.Load(testdata.MiddleMapNodeLnk, ipld.LinkContext{})
			require.Error(t, err)
			unsealer.VerifyExpectations(t)
		})

	})
}
