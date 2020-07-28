package providerstates_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-statemachine/fsm"
	fsmtest "github.com/filecoin-project/go-statemachine/fsm/testutil"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	"github.com/stretchr/testify/require"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/shared"
	tut "github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/blockrecorder"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/providerstates"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/go-fil-markets/storagemarket/testnodes"
)

func TestValidateDealProposal(t *testing.T) {
	ctx := context.Background()
	eventProcessor, err := fsm.NewEventProcessor(storagemarket.MinerDeal{}, "State", providerstates.ProviderEvents)
	require.NoError(t, err)
	runValidateDealProposal := makeExecutor(ctx, eventProcessor, providerstates.ValidateDealProposal, storagemarket.StorageDealValidating)
	otherAddr, err := address.NewActorAddress([]byte("applesauce"))
	require.NoError(t, err)
	tests := map[string]struct {
		nodeParams        nodeParams
		dealParams        dealParams
		environmentParams environmentParams
		fileStoreParams   tut.TestFileStoreParams
		pieceStoreParams  tut.TestPieceStoreParams
		dealInspector     func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment)
	}{
		"succeeds": {
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealAcceptWait, deal.State)
			},
		},
		"verify signature fails": {
			nodeParams: nodeParams{
				VerifySignatureFails: true,
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealRejecting, deal.State)
				require.Equal(t, "deal rejected: verifying StorageDealProposal: could not verify signature", deal.Message)
			},
		},
		"provider address does not match": {
			environmentParams: environmentParams{
				Address: otherAddr,
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealRejecting, deal.State)
				require.Equal(t, "deal rejected: incorrect provider for deal", deal.Message)
			},
		},
		"MostRecentStateID errors": {
			nodeParams: nodeParams{
				MostRecentStateIDError: errors.New("couldn't get id"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealRejecting, deal.State)
				require.Equal(t, "deal rejected: node error getting most recent state id: couldn't get id", deal.Message)
			},
		},
		"CurrentHeight <= StartEpoch - DealAcceptanceBuffer() succeeds": {
			environmentParams: environmentParams{DealAcceptanceBuffer: 10},
			dealParams:        dealParams{StartEpoch: 200},
			nodeParams:        nodeParams{Height: 190},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealAcceptWait, deal.State)
			},
		},
		"CurrentHeight > StartEpoch - DealAcceptanceBuffer() fails": {
			environmentParams: environmentParams{DealAcceptanceBuffer: 10},
			dealParams:        dealParams{StartEpoch: 200},
			nodeParams:        nodeParams{Height: 191},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealRejecting, deal.State)
				require.Equal(t, "deal rejected: deal start epoch is too soon or deal already expired", deal.Message)
			},
		},
		"PricePerEpoch too low": {
			dealParams: dealParams{
				StoragePricePerEpoch: abi.NewTokenAmount(5000),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealRejecting, deal.State)
				require.Equal(t, "deal rejected: storage price per epoch less than asking price: 5000 < 9765", deal.Message)
			},
		},
		"PieceSize < MinPieceSize": {
			dealParams: dealParams{
				PieceSize: abi.PaddedPieceSize(128),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealRejecting, deal.State)
				require.Equal(t, "deal rejected: piece size less than minimum required size: 128 < 256", deal.Message)
			},
		},
		"Get balance error": {
			nodeParams: nodeParams{
				ClientMarketBalanceError: errors.New("could not get balance"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealRejecting, deal.State)
				require.Equal(t, "deal rejected: node error getting client market balance failed: could not get balance", deal.Message)
			},
		},
		"Not enough funds": {
			nodeParams: nodeParams{
				ClientMarketBalance: abi.NewTokenAmount(150 * 10000),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealRejecting, deal.State)
				require.Equal(t, "deal rejected: clientMarketBalance.Available too small", deal.Message)
			},
		},
		"verified deal succeeds": {
			dealParams: dealParams{
				VerifiedDeal: true,
			},
			nodeParams: nodeParams{
				DataCap: big.NewIntUnsigned(uint64(defaultPieceSize)),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				require.True(t, deal.Proposal.VerifiedDeal)
				tut.AssertDealState(t, storagemarket.StorageDealAcceptWait, deal.State)
			},
		},
		"verified deal fails getting client data cap": {
			dealParams: dealParams{
				VerifiedDeal: true,
			},
			nodeParams: nodeParams{
				GetDataCapError: xerrors.Errorf("failure getting data cap"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				require.True(t, deal.Proposal.VerifiedDeal)
				tut.AssertDealState(t, storagemarket.StorageDealRejecting, deal.State)
				require.Equal(t, "deal rejected: node error fetching verified data cap: failure getting data cap", deal.Message)
			},
		},
		"verified deal fails with insufficient data cap": {
			dealParams: dealParams{
				VerifiedDeal: true,
			},
			nodeParams: nodeParams{
				DataCap: big.NewIntUnsigned(uint64(defaultPieceSize - 1)),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				require.True(t, deal.Proposal.VerifiedDeal)
				tut.AssertDealState(t, storagemarket.StorageDealRejecting, deal.State)
				require.Equal(t, "deal rejected: verified deal DataCap too small for proposed piece size", deal.Message)
			},
		},
	}
	for test, data := range tests {
		t.Run(test, func(t *testing.T) {
			runValidateDealProposal(t, data.nodeParams, data.environmentParams, data.dealParams, data.fileStoreParams, data.pieceStoreParams, data.dealInspector)
		})
	}
}

func TestDecideOnProposal(t *testing.T) {
	ctx := context.Background()
	eventProcessor, err := fsm.NewEventProcessor(storagemarket.MinerDeal{}, "State", providerstates.ProviderEvents)
	require.NoError(t, err)
	runDecideOndeal := makeExecutor(ctx, eventProcessor, providerstates.DecideOnProposal, storagemarket.StorageDealAcceptWait)
	tests := map[string]struct {
		nodeParams        nodeParams
		dealParams        dealParams
		environmentParams environmentParams
		fileStoreParams   tut.TestFileStoreParams
		pieceStoreParams  tut.TestPieceStoreParams
		dealInspector     func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment)
	}{
		"succeeds": {
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealWaitingForData, deal.State)
			},
		},
		"Custom Decision Rejects Deal": {
			environmentParams: environmentParams{
				RejectDeal:   true,
				RejectReason: "I just don't like it",
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealRejecting, deal.State)
				require.Equal(t, "deal rejected: I just don't like it", deal.Message)
			},
		},
		"Custom Decision Errors": {
			environmentParams: environmentParams{
				DecisionError: errors.New("I can't make up my mind"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealRejecting, deal.State)
				require.Equal(t, "deal rejected: custom deal decision logic failed: I can't make up my mind", deal.Message)
			},
		},
		"SendSignedResponse errors": {
			environmentParams: environmentParams{
				SendSignedResponseError: errors.New("could not send"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, "sending response to deal: could not send", deal.Message)
			},
		},
	}
	for test, data := range tests {
		t.Run(test, func(t *testing.T) {
			runDecideOndeal(t, data.nodeParams, data.environmentParams, data.dealParams, data.fileStoreParams, data.pieceStoreParams, data.dealInspector)
		})
	}
}

func TestVerifyData(t *testing.T) {
	ctx := context.Background()
	eventProcessor, err := fsm.NewEventProcessor(storagemarket.MinerDeal{}, "State", providerstates.ProviderEvents)
	require.NoError(t, err)
	expPath := filestore.Path("applesauce.txt")
	expMetaPath := filestore.Path("somemetadata.txt")
	runVerifyData := makeExecutor(ctx, eventProcessor, providerstates.VerifyData, storagemarket.StorageDealVerifyData)
	tests := map[string]struct {
		nodeParams        nodeParams
		dealParams        dealParams
		environmentParams environmentParams
		fileStoreParams   tut.TestFileStoreParams
		pieceStoreParams  tut.TestPieceStoreParams
		dealInspector     func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment)
	}{
		"succeeds": {
			environmentParams: environmentParams{
				Path:         expPath,
				MetadataPath: expMetaPath,
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealEnsureProviderFunds, deal.State)
				require.Equal(t, expPath, deal.PiecePath)
				require.Equal(t, expMetaPath, deal.MetadataPath)
			},
		},
		"generate piece CID fails": {
			environmentParams: environmentParams{
				GenerateCommPError: errors.New("could not generate CommP"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, "deal data verification failed: error generating CommP: could not generate CommP", deal.Message)
			},
		},
		"piece CIDs do not match": {
			environmentParams: environmentParams{
				PieceCid: tut.GenerateCids(1)[0],
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, "deal data verification failed: proposal CommP doesn't match calculated CommP", deal.Message)
			},
		},
	}
	for test, data := range tests {
		t.Run(test, func(t *testing.T) {
			runVerifyData(t, data.nodeParams, data.environmentParams, data.dealParams, data.fileStoreParams, data.pieceStoreParams, data.dealInspector)
		})
	}
}

func TestWaitForFunding(t *testing.T) {
	ctx := context.Background()
	eventProcessor, err := fsm.NewEventProcessor(storagemarket.MinerDeal{}, "State", providerstates.ProviderEvents)
	require.NoError(t, err)
	runWaitForFunding := makeExecutor(ctx, eventProcessor, providerstates.WaitForFunding, storagemarket.StorageDealProviderFunding)
	tests := map[string]struct {
		nodeParams        nodeParams
		dealParams        dealParams
		environmentParams environmentParams
		fileStoreParams   tut.TestFileStoreParams
		pieceStoreParams  tut.TestPieceStoreParams
		dealInspector     func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment)
	}{
		"succeeds": {
			nodeParams: nodeParams{
				WaitForMessageExitCode: exitcode.Ok,
				WaitForMessageRetBytes: []byte{},
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealPublish, deal.State)
			},
		},
		"AddFunds returns non-ok exit code": {
			nodeParams: nodeParams{
				WaitForMessageExitCode: exitcode.ErrInsufficientFunds,
				WaitForMessageRetBytes: []byte{},
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, fmt.Sprintf("error calling node: AddFunds exit code: %s", exitcode.ErrInsufficientFunds), deal.Message)
			},
		},
	}
	for test, data := range tests {
		t.Run(test, func(t *testing.T) {
			runWaitForFunding(t, data.nodeParams, data.environmentParams, data.dealParams, data.fileStoreParams, data.pieceStoreParams, data.dealInspector)
		})
	}
}

func TestEnsureProviderFunds(t *testing.T) {
	ctx := context.Background()
	eventProcessor, err := fsm.NewEventProcessor(storagemarket.MinerDeal{}, "State", providerstates.ProviderEvents)
	require.NoError(t, err)
	runEnsureProviderFunds := makeExecutor(ctx, eventProcessor, providerstates.EnsureProviderFunds, storagemarket.StorageDealEnsureProviderFunds)
	cids := tut.GenerateCids(1)
	tests := map[string]struct {
		nodeParams        nodeParams
		dealParams        dealParams
		environmentParams environmentParams
		fileStoreParams   tut.TestFileStoreParams
		pieceStoreParams  tut.TestPieceStoreParams
		dealInspector     func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment)
	}{
		"succeeds immediately": {
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealPublish, deal.State)
			},
		},
		"succeeds by sending an AddBalance message": {
			dealParams: dealParams{
				ProviderCollateral: abi.NewTokenAmount(1),
			},
			nodeParams: nodeParams{
				AddFundsCid: cids[0],
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealProviderFunding, deal.State)
				require.Equal(t, &cids[0], deal.AddFundsCid)
			},
		},
		"get miner worker fails": {
			nodeParams: nodeParams{
				MinerWorkerError: errors.New("could not get worker"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, "error calling node: looking up miner worker: could not get worker", deal.Message)
			},
		},
		"ensureFunds errors": {
			nodeParams: nodeParams{
				EnsureFundsError: errors.New("not enough funds"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, "error calling node: ensuring funds: not enough funds", deal.Message)
			},
		},
	}
	for test, data := range tests {
		t.Run(test, func(t *testing.T) {
			runEnsureProviderFunds(t, data.nodeParams, data.environmentParams, data.dealParams, data.fileStoreParams, data.pieceStoreParams, data.dealInspector)
		})
	}
}

func TestPublishDeal(t *testing.T) {
	ctx := context.Background()
	eventProcessor, err := fsm.NewEventProcessor(storagemarket.MinerDeal{}, "State", providerstates.ProviderEvents)
	require.NoError(t, err)
	runPublishDeal := makeExecutor(ctx, eventProcessor, providerstates.PublishDeal, storagemarket.StorageDealPublish)
	tests := map[string]struct {
		nodeParams        nodeParams
		dealParams        dealParams
		environmentParams environmentParams
		fileStoreParams   tut.TestFileStoreParams
		pieceStoreParams  tut.TestPieceStoreParams
		dealInspector     func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment)
	}{
		"succeeds": {
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealPublishing, deal.State)
			},
		},
		"PublishDealsErrors errors": {
			nodeParams: nodeParams{
				PublishDealsError: errors.New("could not post to chain"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, "error calling node: publishing deal: could not post to chain", deal.Message)
			},
		},
	}
	for test, data := range tests {
		t.Run(test, func(t *testing.T) {
			runPublishDeal(t, data.nodeParams, data.environmentParams, data.dealParams, data.fileStoreParams, data.pieceStoreParams, data.dealInspector)
		})
	}
}

func TestWaitForPublish(t *testing.T) {
	ctx := context.Background()
	eventProcessor, err := fsm.NewEventProcessor(storagemarket.MinerDeal{}, "State", providerstates.ProviderEvents)
	require.NoError(t, err)
	runWaitForPublish := makeExecutor(ctx, eventProcessor, providerstates.WaitForPublish, storagemarket.StorageDealPublishing)
	expDealID, psdReturnBytes := generatePublishDealsReturn(t)

	tests := map[string]struct {
		nodeParams        nodeParams
		dealParams        dealParams
		environmentParams environmentParams
		fileStoreParams   tut.TestFileStoreParams
		pieceStoreParams  tut.TestPieceStoreParams
		dealInspector     func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment)
	}{
		"succeeds": {
			nodeParams: nodeParams{
				WaitForMessageRetBytes: psdReturnBytes,
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealStaged, deal.State)
				require.Equal(t, expDealID, deal.DealID)
			},
		},
		"PublishStorageDeal errors": {
			nodeParams: nodeParams{
				WaitForMessageExitCode: exitcode.SysErrForbidden,
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, "PublishStorageDeal error: PublishStorageDeals exit code: SysErrForbidden(8)", deal.Message)
			},
		},
	}
	for test, data := range tests {
		t.Run(test, func(t *testing.T) {
			runWaitForPublish(t, data.nodeParams, data.environmentParams, data.dealParams, data.fileStoreParams, data.pieceStoreParams, data.dealInspector)
		})
	}
}

func TestHandoffDeal(t *testing.T) {
	ctx := context.Background()
	eventProcessor, err := fsm.NewEventProcessor(storagemarket.MinerDeal{}, "State", providerstates.ProviderEvents)
	require.NoError(t, err)
	runHandoffDeal := makeExecutor(ctx, eventProcessor, providerstates.HandoffDeal, storagemarket.StorageDealStaged)
	tests := map[string]struct {
		nodeParams        nodeParams
		dealParams        dealParams
		environmentParams environmentParams
		fileStoreParams   tut.TestFileStoreParams
		pieceStoreParams  tut.TestPieceStoreParams
		dealInspector     func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment)
	}{
		"succeeds": {
			dealParams: dealParams{
				PiecePath:     defaultPath,
				FastRetrieval: true,
			},
			fileStoreParams: tut.TestFileStoreParams{
				Files:         []filestore.File{defaultDataFile},
				ExpectedOpens: []filestore.Path{defaultPath},
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealSealing, deal.State)
				require.Len(t, env.node.OnDealCompleteCalls, 1)
				require.True(t, env.node.OnDealCompleteCalls[0].FastRetrieval)
			},
		},
		"opening file errors": {
			dealParams: dealParams{
				PiecePath: filestore.Path("missing.txt"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, fmt.Sprintf("accessing file store: reading piece at path missing.txt: %s", tut.TestErrNotFound.Error()), deal.Message)
			},
		},
		"OnDealComplete errors": {
			dealParams: dealParams{
				PiecePath: defaultPath,
			},
			fileStoreParams: tut.TestFileStoreParams{
				Files:         []filestore.File{defaultDataFile},
				ExpectedOpens: []filestore.Path{defaultPath},
			},
			nodeParams: nodeParams{
				OnDealCompleteError: errors.New("failed building sector"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, "handing off deal to node: failed building sector", deal.Message)
			},
		},
	}
	for test, data := range tests {
		t.Run(test, func(t *testing.T) {
			runHandoffDeal(t, data.nodeParams, data.environmentParams, data.dealParams, data.fileStoreParams, data.pieceStoreParams, data.dealInspector)
		})
	}
}

func TestVerifyDealActivated(t *testing.T) {
	ctx := context.Background()
	eventProcessor, err := fsm.NewEventProcessor(storagemarket.MinerDeal{}, "State", providerstates.ProviderEvents)
	require.NoError(t, err)
	runVerifyDealActivated := makeExecutor(ctx, eventProcessor, providerstates.VerifyDealActivated, storagemarket.StorageDealSealing)
	tests := map[string]struct {
		nodeParams        nodeParams
		dealParams        dealParams
		environmentParams environmentParams
		fileStoreParams   tut.TestFileStoreParams
		pieceStoreParams  tut.TestPieceStoreParams
		dealInspector     func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment)
	}{
		"succeeds": {
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealRecordPiece, deal.State)
			},
		},
		"sync error": {
			nodeParams: nodeParams{
				DealCommittedSyncError: errors.New("couldn't check deal commitment"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, "error activating deal: couldn't check deal commitment", deal.Message)
			},
		},
		"async error": {
			nodeParams: nodeParams{
				DealCommittedAsyncError: errors.New("deal did not appear on chain"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, "error activating deal: deal did not appear on chain", deal.Message)
			},
		},
	}
	for test, data := range tests {
		t.Run(test, func(t *testing.T) {
			runVerifyDealActivated(t, data.nodeParams, data.environmentParams, data.dealParams, data.fileStoreParams, data.pieceStoreParams, data.dealInspector)
		})
	}
}

func TestRecordPieceInfo(t *testing.T) {
	ctx := context.Background()
	eventProcessor, err := fsm.NewEventProcessor(storagemarket.MinerDeal{}, "State", providerstates.ProviderEvents)
	require.NoError(t, err)
	runRecordPieceInfo := makeExecutor(ctx, eventProcessor, providerstates.RecordPieceInfo, storagemarket.StorageDealRecordPiece)
	tests := map[string]struct {
		nodeParams        nodeParams
		dealParams        dealParams
		environmentParams environmentParams
		fileStoreParams   tut.TestFileStoreParams
		pieceStoreParams  tut.TestPieceStoreParams
		dealInspector     func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment)
	}{
		"succeeds": {
			dealParams: dealParams{
				PiecePath: defaultPath,
			},
			fileStoreParams: tut.TestFileStoreParams{
				Files:             []filestore.File{defaultDataFile},
				ExpectedDeletions: []filestore.Path{defaultPath},
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealActive, deal.State)
			},
		},
		"succeeds w metadata": {
			dealParams: dealParams{
				PiecePath:    defaultPath,
				MetadataPath: defaultMetadataPath,
			},
			fileStoreParams: tut.TestFileStoreParams{
				Files:             []filestore.File{defaultDataFile, defaultMetadataFile},
				ExpectedOpens:     []filestore.Path{defaultMetadataPath},
				ExpectedDeletions: []filestore.Path{defaultMetadataPath, defaultPath},
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealActive, deal.State)
			},
		},
		"locate piece fails": {
			dealParams: dealParams{
				DealID: abi.DealID(1234),
			},
			nodeParams: nodeParams{
				LocatePieceForDealWithinSectorError: errors.New("could not find piece"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, "locating piece for deal ID 1234 in sector: could not find piece", deal.Message)
			},
		},
		"reading metadata fails": {
			dealParams: dealParams{
				MetadataPath: filestore.Path("Missing.txt"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, fmt.Sprintf("error reading piece metadata: %s", tut.TestErrNotFound.Error()), deal.Message)
			},
		},
		"add piece block locations errors": {
			pieceStoreParams: tut.TestPieceStoreParams{
				AddPieceBlockLocationsError: errors.New("could not add block locations"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, "accessing piece store: adding piece block locations: could not add block locations", deal.Message)
			},
		},
		"add deal for piece errors": {
			pieceStoreParams: tut.TestPieceStoreParams{
				AddDealForPieceError: errors.New("could not add deal info"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, "accessing piece store: adding deal info for piece: could not add deal info", deal.Message)
			},
		},
	}
	for test, data := range tests {
		t.Run(test, func(t *testing.T) {
			runRecordPieceInfo(t, data.nodeParams, data.environmentParams, data.dealParams, data.fileStoreParams, data.pieceStoreParams, data.dealInspector)
		})
	}
}

func TestWaitForDealCompletion(t *testing.T) {
	ctx := context.Background()
	eventProcessor, err := fsm.NewEventProcessor(storagemarket.MinerDeal{}, "State", providerstates.ProviderEvents)
	require.NoError(t, err)
	runWaitForDealCompletion := makeExecutor(ctx, eventProcessor, providerstates.WaitForDealCompletion, storagemarket.StorageDealActive)
	tests := map[string]struct {
		nodeParams        nodeParams
		dealParams        dealParams
		environmentParams environmentParams
		fileStoreParams   tut.TestFileStoreParams
		pieceStoreParams  tut.TestPieceStoreParams
		dealInspector     func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment)
	}{
		"slashing succeeds": {
			nodeParams: nodeParams{OnDealSlashedEpoch: abi.ChainEpoch(5)},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealSlashed, deal.State)
				require.Equal(t, abi.ChainEpoch(5), deal.SlashEpoch)
			},
		},
		"expiration succeeds": {
			// OnDealSlashedEpoch of zero signals to test node to call onDealExpired()
			nodeParams: nodeParams{OnDealSlashedEpoch: abi.ChainEpoch(0)},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealExpired, deal.State)
			},
		},
		"slashing fails": {
			nodeParams: nodeParams{OnDealSlashedError: errors.New("an err")},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealError, deal.State)
				require.Equal(t, "error waiting for deal completion: deal slashing err: an err", deal.Message)
			},
		},
		"expiration fails": {
			nodeParams: nodeParams{OnDealExpiredError: errors.New("an err")},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealError, deal.State)
				require.Equal(t, "error waiting for deal completion: deal expiration err: an err", deal.Message)
			},
		},
		"fails synchronously": {
			nodeParams: nodeParams{WaitForDealCompletionError: errors.New("an err")},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealError, deal.State)
				require.Equal(t, "error waiting for deal completion: an err", deal.Message)
			},
		},
	}

	for test, data := range tests {
		t.Run(test, func(t *testing.T) {
			runWaitForDealCompletion(t, data.nodeParams, data.environmentParams, data.dealParams, data.fileStoreParams, data.pieceStoreParams, data.dealInspector)
		})
	}
}

func TestRejectDeal(t *testing.T) {
	ctx := context.Background()
	eventProcessor, err := fsm.NewEventProcessor(storagemarket.MinerDeal{}, "State", providerstates.ProviderEvents)
	require.NoError(t, err)
	runRejectDeal := makeExecutor(ctx, eventProcessor, providerstates.RejectDeal, storagemarket.StorageDealRejecting)
	tests := map[string]struct {
		nodeParams        nodeParams
		dealParams        dealParams
		environmentParams environmentParams
		fileStoreParams   tut.TestFileStoreParams
		pieceStoreParams  tut.TestPieceStoreParams
		dealInspector     func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment)
	}{
		"succeeds": {
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, 1, env.disconnectCalls)
			},
		},
		"fails if it cannot send a response": {
			environmentParams: environmentParams{
				SendSignedResponseError: xerrors.New("error sending response"),
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				require.Equal(t, deal.Message, "sending response to deal: error sending response")
			},
		},
	}
	for test, data := range tests {
		t.Run(test, func(t *testing.T) {
			runRejectDeal(t, data.nodeParams, data.environmentParams, data.dealParams, data.fileStoreParams, data.pieceStoreParams, data.dealInspector)
		})
	}
}

func TestFailDeal(t *testing.T) {
	ctx := context.Background()
	eventProcessor, err := fsm.NewEventProcessor(storagemarket.MinerDeal{}, "State", providerstates.ProviderEvents)
	require.NoError(t, err)
	runFailDeal := makeExecutor(ctx, eventProcessor, providerstates.FailDeal, storagemarket.StorageDealFailing)
	tests := map[string]struct {
		nodeParams        nodeParams
		dealParams        dealParams
		environmentParams environmentParams
		fileStoreParams   tut.TestFileStoreParams
		pieceStoreParams  tut.TestPieceStoreParams
		dealInspector     func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment)
	}{
		"succeeds": {
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealError, deal.State)
			},
		},
		"succeeds, file deletions": {
			dealParams: dealParams{
				PiecePath:    defaultPath,
				MetadataPath: defaultMetadataPath,
			},
			fileStoreParams: tut.TestFileStoreParams{
				Files:             []filestore.File{defaultDataFile, defaultMetadataFile},
				ExpectedDeletions: []filestore.Path{defaultPath, defaultMetadataPath},
			},
			dealInspector: func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealError, deal.State)
			},
		},
	}
	for test, data := range tests {
		t.Run(test, func(t *testing.T) {
			runFailDeal(t, data.nodeParams, data.environmentParams, data.dealParams, data.fileStoreParams, data.pieceStoreParams, data.dealInspector)
		})
	}
}

// all of these default parameters are setup to allow a deal to complete each handler with no errors
var defaultHeight = abi.ChainEpoch(50)
var defaultTipSetToken = []byte{1, 2, 3}
var defaultStoragePricePerEpoch = abi.NewTokenAmount(10000)
var defaultPieceSize = abi.PaddedPieceSize(1048576)
var defaultStartEpoch = abi.ChainEpoch(200)
var defaultEndEpoch = abi.ChainEpoch(400)
var defaultPieceCid = tut.GenerateCids(1)[0]
var defaultPath = filestore.Path("file.txt")
var defaultMetadataPath = filestore.Path("metadataPath.txt")
var defaultClientAddress = address.TestAddress
var defaultProviderAddress = address.TestAddress2
var defaultMinerAddr, _ = address.NewActorAddress([]byte("miner"))
var defaultClientCollateral = abi.NewTokenAmount(0)
var defaultProviderCollateral = abi.NewTokenAmount(0)
var defaultDataRef = storagemarket.DataRef{
	Root:         tut.GenerateCids(1)[0],
	TransferType: storagemarket.TTGraphsync,
}
var defaultClientMarketBalance = abi.NewTokenAmount(200 * 10000)

var defaultAsk = storagemarket.StorageAsk{
	Price:        abi.NewTokenAmount(10000000),
	MinPieceSize: abi.PaddedPieceSize(256),
	MaxPieceSize: 1 << 20,
}

var testData = tut.NewTestIPLDTree()
var dataBuf = new(bytes.Buffer)
var blockLocationBuf = new(bytes.Buffer)
var _ error = testData.DumpToCar(dataBuf, blockrecorder.RecordEachBlockTo(blockLocationBuf))
var defaultDataFile = tut.NewTestFile(tut.TestFileParams{
	Buffer: dataBuf,
	Path:   defaultPath,
	Size:   400,
})
var defaultMetadataFile = tut.NewTestFile(tut.TestFileParams{
	Buffer: blockLocationBuf,
	Path:   defaultMetadataPath,
	Size:   400,
})

func generatePublishDealsReturn(t *testing.T) (abi.DealID, []byte) {
	dealId := abi.DealID(rand.Uint64())

	psdReturn := market.PublishStorageDealsReturn{IDs: []abi.DealID{dealId}}
	psdReturnBytes := bytes.NewBuffer([]byte{})
	err := psdReturn.MarshalCBOR(psdReturnBytes)
	require.NoError(t, err)

	return dealId, psdReturnBytes.Bytes()
}

type nodeParams struct {
	MinerAddr                           address.Address
	MinerWorkerError                    error
	EnsureFundsError                    error
	Height                              abi.ChainEpoch
	TipSetToken                         shared.TipSetToken
	ClientMarketBalance                 abi.TokenAmount
	ClientMarketBalanceError            error
	AddFundsCid                         cid.Cid
	VerifySignatureFails                bool
	MostRecentStateIDError              error
	PieceLength                         uint64
	PieceSectorID                       uint64
	PublishDealsError                   error
	OnDealCompleteError                 error
	LocatePieceForDealWithinSectorError error
	DealCommittedSyncError              error
	DealCommittedAsyncError             error
	WaitForMessageBlocks                bool
	WaitForMessageError                 error
	WaitForMessageExitCode              exitcode.ExitCode
	WaitForMessageRetBytes              []byte
	WaitForDealCompletionError          error
	OnDealExpiredError                  error
	OnDealSlashedError                  error
	OnDealSlashedEpoch                  abi.ChainEpoch
	DataCap                             verifreg.DataCap
	GetDataCapError                     error
}

type dealParams struct {
	PiecePath            filestore.Path
	MetadataPath         filestore.Path
	DealID               abi.DealID
	DataRef              *storagemarket.DataRef
	StoragePricePerEpoch abi.TokenAmount
	ProviderCollateral   abi.TokenAmount
	PieceSize            abi.PaddedPieceSize
	StartEpoch           abi.ChainEpoch
	EndEpoch             abi.ChainEpoch
	FastRetrieval        bool
	VerifiedDeal         bool
}

type environmentParams struct {
	Address                 address.Address
	Ask                     storagemarket.StorageAsk
	DataTransferError       error
	PieceCid                cid.Cid
	Path                    filestore.Path
	MetadataPath            filestore.Path
	GenerateCommPError      error
	SendSignedResponseError error
	DisconnectError         error
	DealAcceptanceBuffer    int64
	TagsProposal            bool
	RejectDeal              bool
	RejectReason            string
	DecisionError           error
}

type executor func(t *testing.T,
	node nodeParams,
	params environmentParams,
	dealParams dealParams,
	fileStoreParams tut.TestFileStoreParams,
	pieceStoreParams tut.TestPieceStoreParams,
	dealInspector func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment))

func makeExecutor(ctx context.Context,
	eventProcessor fsm.EventProcessor,
	stateEntryFunc providerstates.ProviderStateEntryFunc,
	initialState storagemarket.StorageDealStatus) executor {
	return func(t *testing.T,
		nodeParams nodeParams,
		params environmentParams,
		dealParams dealParams,
		fileStoreParams tut.TestFileStoreParams,
		pieceStoreParams tut.TestPieceStoreParams,
		dealInspector func(t *testing.T, deal storagemarket.MinerDeal, env *fakeEnvironment)) {

		smstate := testnodes.NewStorageMarketState()
		if nodeParams.Height != abi.ChainEpoch(0) {
			smstate.Epoch = nodeParams.Height
			smstate.TipSetToken = nodeParams.TipSetToken
		} else {
			smstate.Epoch = defaultHeight
			smstate.TipSetToken = defaultTipSetToken
		}
		if !nodeParams.ClientMarketBalance.Nil() {
			smstate.AddFunds(defaultClientAddress, nodeParams.ClientMarketBalance)
		} else {
			smstate.AddFunds(defaultClientAddress, defaultClientMarketBalance)
		}

		common := testnodes.FakeCommonNode{
			SMState:                    smstate,
			GetChainHeadError:          nodeParams.MostRecentStateIDError,
			GetBalanceError:            nodeParams.ClientMarketBalanceError,
			VerifySignatureFails:       nodeParams.VerifySignatureFails,
			EnsureFundsError:           nodeParams.EnsureFundsError,
			DealCommittedSyncError:     nodeParams.DealCommittedSyncError,
			DealCommittedAsyncError:    nodeParams.DealCommittedAsyncError,
			AddFundsCid:                nodeParams.AddFundsCid,
			WaitForMessageBlocks:       nodeParams.WaitForMessageBlocks,
			WaitForMessageError:        nodeParams.WaitForMessageError,
			WaitForMessageExitCode:     nodeParams.WaitForMessageExitCode,
			WaitForMessageRetBytes:     nodeParams.WaitForMessageRetBytes,
			WaitForDealCompletionError: nodeParams.WaitForDealCompletionError,
			OnDealExpiredError:         nodeParams.OnDealExpiredError,
			OnDealSlashedError:         nodeParams.OnDealSlashedError,
			OnDealSlashedEpoch:         nodeParams.OnDealSlashedEpoch,
		}

		node := &testnodes.FakeProviderNode{
			FakeCommonNode:                      common,
			MinerAddr:                           nodeParams.MinerAddr,
			MinerWorkerError:                    nodeParams.MinerWorkerError,
			PieceLength:                         nodeParams.PieceLength,
			PieceSectorID:                       nodeParams.PieceSectorID,
			PublishDealsError:                   nodeParams.PublishDealsError,
			OnDealCompleteError:                 nodeParams.OnDealCompleteError,
			LocatePieceForDealWithinSectorError: nodeParams.LocatePieceForDealWithinSectorError,
			DataCap:                             nodeParams.DataCap,
			GetDataCapErr:                       nodeParams.GetDataCapError,
		}

		if nodeParams.MinerAddr == address.Undef {
			node.MinerAddr = defaultMinerAddr
		}

		proposal := market.DealProposal{
			PieceCID:             defaultPieceCid,
			PieceSize:            defaultPieceSize,
			Client:               defaultClientAddress,
			Provider:             defaultProviderAddress,
			StartEpoch:           defaultStartEpoch,
			EndEpoch:             defaultEndEpoch,
			StoragePricePerEpoch: defaultStoragePricePerEpoch,
			ProviderCollateral:   defaultProviderCollateral,
			ClientCollateral:     defaultClientCollateral,
		}
		if !dealParams.StoragePricePerEpoch.Nil() {
			proposal.StoragePricePerEpoch = dealParams.StoragePricePerEpoch
		}
		if !dealParams.ProviderCollateral.Nil() {
			proposal.ProviderCollateral = dealParams.ProviderCollateral
		}
		if dealParams.StartEpoch != abi.ChainEpoch(0) {
			proposal.StartEpoch = dealParams.StartEpoch
		}
		if dealParams.EndEpoch != abi.ChainEpoch(0) {
			proposal.EndEpoch = dealParams.EndEpoch
		}
		if dealParams.PieceSize != abi.PaddedPieceSize(0) {
			proposal.PieceSize = dealParams.PieceSize
		}
		proposal.VerifiedDeal = dealParams.VerifiedDeal
		signedProposal := &market.ClientDealProposal{
			Proposal:        proposal,
			ClientSignature: *tut.MakeTestSignature(),
		}
		dataRef := &defaultDataRef
		if dealParams.DataRef != nil {
			dataRef = dealParams.DataRef
		}
		dealState, err := tut.MakeTestMinerDeal(initialState,
			signedProposal, dataRef)
		require.NoError(t, err)
		dealState.AddFundsCid = &tut.GenerateCids(1)[0]
		dealState.PublishCid = &tut.GenerateCids(1)[0]
		if dealParams.PiecePath != filestore.Path("") {
			dealState.PiecePath = dealParams.PiecePath
		}
		if dealParams.MetadataPath != filestore.Path("") {
			dealState.MetadataPath = dealParams.MetadataPath
		}
		if dealParams.DealID != abi.DealID(0) {
			dealState.DealID = dealParams.DealID
		}
		dealState.FastRetrieval = dealParams.FastRetrieval
		fs := tut.NewTestFileStore(fileStoreParams)
		pieceStore := tut.NewTestPieceStoreWithParams(pieceStoreParams)
		expectedTags := make(map[string]struct{})
		if params.TagsProposal {
			expectedTags[dealState.ProposalCid.String()] = struct{}{}
		}
		environment := &fakeEnvironment{
			expectedTags:            expectedTags,
			receivedTags:            make(map[string]struct{}),
			address:                 params.Address,
			node:                    node,
			ask:                     params.Ask,
			dataTransferError:       params.DataTransferError,
			pieceCid:                params.PieceCid,
			path:                    params.Path,
			metadataPath:            params.MetadataPath,
			generateCommPError:      params.GenerateCommPError,
			sendSignedResponseError: params.SendSignedResponseError,
			disconnectError:         params.DisconnectError,
			rejectDeal:              params.RejectDeal,
			rejectReason:            params.RejectReason,
			decisionError:           params.DecisionError,
			dealAcceptanceBuffer:    abi.ChainEpoch(params.DealAcceptanceBuffer),
			fs:                      fs,
			pieceStore:              pieceStore,
		}
		if environment.pieceCid == cid.Undef {
			environment.pieceCid = defaultPieceCid
		}
		if environment.path == filestore.Path("") {
			environment.path = defaultPath
		}
		if environment.metadataPath == filestore.Path("") {
			environment.metadataPath = defaultMetadataPath
		}
		if environment.address == address.Undef {
			environment.address = defaultProviderAddress
		}
		if environment.ask == storagemarket.StorageAskUndefined {
			environment.ask = defaultAsk
		}

		fsmCtx := fsmtest.NewTestContext(ctx, eventProcessor)
		err = stateEntryFunc(fsmCtx, environment, *dealState)
		require.NoError(t, err)
		fsmCtx.ReplayEvents(t, dealState)
		dealInspector(t, *dealState, environment)

		fs.VerifyExpectations(t)
		pieceStore.VerifyExpectations(t)
		environment.VerifyExpectations(t)
	}
}

type fakeEnvironment struct {
	address                 address.Address
	node                    *testnodes.FakeProviderNode
	ask                     storagemarket.StorageAsk
	dataTransferError       error
	pieceCid                cid.Cid
	path                    filestore.Path
	metadataPath            filestore.Path
	generateCommPError      error
	sendSignedResponseError error
	disconnectCalls         int
	disconnectError         error
	rejectDeal              bool
	rejectReason            string
	decisionError           error
	fs                      filestore.FileStore
	pieceStore              piecestore.PieceStore
	dealAcceptanceBuffer    abi.ChainEpoch
	expectedTags            map[string]struct{}
	receivedTags            map[string]struct{}
}

func (fe *fakeEnvironment) Address() address.Address {
	return fe.address
}

func (fe *fakeEnvironment) Node() storagemarket.StorageProviderNode {
	return fe.node
}

func (fe *fakeEnvironment) Ask() storagemarket.StorageAsk {
	return fe.ask
}

func (fe *fakeEnvironment) GeneratePieceCommitmentToFile(payloadCid cid.Cid, selector ipld.Node) (cid.Cid, filestore.Path, filestore.Path, error) {
	return fe.pieceCid, fe.path, fe.metadataPath, fe.generateCommPError
}

func (fe *fakeEnvironment) SendSignedResponse(ctx context.Context, response *network.Response) error {
	return fe.sendSignedResponseError
}

func (fe *fakeEnvironment) VerifyExpectations(t *testing.T) {
	require.Equal(t, fe.expectedTags, fe.receivedTags)
}

func (fe *fakeEnvironment) Disconnect(proposalCid cid.Cid) error {
	fe.disconnectCalls += 1
	return fe.disconnectError
}

func (fe *fakeEnvironment) FileStore() filestore.FileStore {
	return fe.fs
}

func (fe *fakeEnvironment) PieceStore() piecestore.PieceStore {
	return fe.pieceStore
}

func (fe *fakeEnvironment) DealAcceptanceBuffer() abi.ChainEpoch {
	return fe.dealAcceptanceBuffer
}

func (fe *fakeEnvironment) RunCustomDecisionLogic(context.Context, storagemarket.MinerDeal) (bool, string, error) {
	return !fe.rejectDeal, fe.rejectReason, fe.decisionError
}
