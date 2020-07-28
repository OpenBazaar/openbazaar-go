package clientstates_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-statemachine/fsm"
	fsmtest "github.com/filecoin-project/go-statemachine/fsm/testutil"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"golang.org/x/xerrors"

	tut "github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/clientstates"
	smnet "github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/go-fil-markets/storagemarket/testnodes"
)

var clientDealProposal = tut.MakeTestClientDealProposal()

func TestEnsureFunds(t *testing.T) {
	t.Run("immediately succeeds", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealEnsureClientFunds, clientstates.EnsureClientFunds, testCase{
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFundsEnsured, deal.State)
			},
		})
	})
	t.Run("succeeds by sending an AddFunds message", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealEnsureClientFunds, clientstates.EnsureClientFunds, testCase{
			nodeParams: nodeParams{AddFundsCid: tut.GenerateCids(1)[0]},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealClientFunding, deal.State)
			},
		})
	})
	t.Run("EnsureClientFunds fails", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealEnsureClientFunds, clientstates.EnsureClientFunds, testCase{
			nodeParams: nodeParams{
				EnsureFundsError: errors.New("Something went wrong"),
			},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				assert.Equal(t, "adding market funds failed: Something went wrong", deal.Message)
			},
		})
	})
}

func TestWaitForFunding(t *testing.T) {
	t.Run("succeeds", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealClientFunding, clientstates.WaitForFunding, testCase{
			nodeParams: nodeParams{WaitForMessageExitCode: exitcode.Ok},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFundsEnsured, deal.State)
			},
		})
	})
	t.Run("EnsureClientFunds fails", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealClientFunding, clientstates.WaitForFunding, testCase{
			nodeParams: nodeParams{WaitForMessageExitCode: exitcode.ErrInsufficientFunds},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				assert.Equal(t, "adding market funds failed: AddFunds exit code: 19", deal.Message)
			},
		})
	})
}

func TestProposeDeal(t *testing.T) {
	t.Run("succeeds and closes stream", func(t *testing.T) {
		ds := tut.NewTestStorageDealStream(tut.TestStorageDealStreamParams{
			ResponseReader: testResponseReader(t, responseParams{
				state:    storagemarket.StorageDealWaitingForData,
				proposal: clientDealProposal,
			}),
		})
		runAndInspect(t, storagemarket.StorageDealFundsEnsured, clientstates.ProposeDeal, testCase{
			envParams:  envParams{dealStream: ds},
			nodeParams: nodeParams{WaitForMessageExitCode: exitcode.Ok},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealStartDataTransfer, deal.State)
				assert.Equal(t, 1, env.dealStream.CloseCount)
			},
		})
	})
	t.Run("sends a fast retrieval flag", func(t *testing.T) {
		var sentProposal *smnet.Proposal

		ds := tut.NewTestStorageDealStream(tut.TestStorageDealStreamParams{
			ResponseReader: testResponseReader(t, responseParams{
				state:    storagemarket.StorageDealWaitingForData,
				proposal: clientDealProposal,
			}),
			ProposalWriter: func(proposal smnet.Proposal) error {
				sentProposal = &proposal
				return nil
			},
		})

		runAndInspect(t, storagemarket.StorageDealFundsEnsured, clientstates.ProposeDeal, testCase{
			envParams:   envParams{dealStream: ds},
			stateParams: dealStateParams{fastRetrieval: true},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealStartDataTransfer, deal.State)
				assert.Equal(t, true, sentProposal.FastRetrieval)
			},
		})
	})

	t.Run("write proposal fails fails", func(t *testing.T) {
		ds := tut.NewTestStorageDealStream(tut.TestStorageDealStreamParams{
			ProposalWriter: tut.FailStorageProposalWriter,
		})
		runAndInspect(t, storagemarket.StorageDealFundsEnsured, clientstates.ProposeDeal, testCase{
			envParams:  envParams{dealStream: ds},
			nodeParams: nodeParams{WaitForMessageExitCode: exitcode.Ok},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealError, deal.State)
				assert.Equal(t, "sending proposal to storage provider failed: write proposal failed", deal.Message)
			},
		})
	})
	t.Run("read response fails", func(t *testing.T) {
		ds := tut.NewTestStorageDealStream(tut.TestStorageDealStreamParams{
			ResponseReader: tut.FailStorageResponseReader,
		})
		runAndInspect(t, storagemarket.StorageDealFundsEnsured, clientstates.ProposeDeal, testCase{
			envParams: envParams{
				dealStream: ds,
			},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				assert.Equal(t, "error reading Response message: read response failed", deal.Message)
			},
		})
	})
	t.Run("closing the stream fails", func(t *testing.T) {
		ds := tut.NewTestStorageDealStream(tut.TestStorageDealStreamParams{})
		ds.CloseError = xerrors.Errorf("failed to close stream")
		runAndInspect(t, storagemarket.StorageDealFundsEnsured, clientstates.ProposeDeal, testCase{
			envParams: envParams{dealStream: ds},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealError, deal.State)
				assert.Equal(t, "error attempting to close stream: failed to close stream", deal.Message)
			},
		})
	})
	t.Run("getting chain head fails", func(t *testing.T) {
		ds := tut.NewTestStorageDealStream(tut.TestStorageDealStreamParams{})
		runAndInspect(t, storagemarket.StorageDealFundsEnsured, clientstates.ProposeDeal, testCase{
			envParams: envParams{
				dealStream: ds,
			},
			nodeParams: nodeParams{
				GetChainHeadError: xerrors.Errorf("failed getting chain head"),
			},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				assert.Equal(t, "unable to verify signature on deal response", deal.Message)
			},
		})
	})
	t.Run("verify signature fails", func(t *testing.T) {
		ds := tut.NewTestStorageDealStream(tut.TestStorageDealStreamParams{})
		runAndInspect(t, storagemarket.StorageDealFundsEnsured, clientstates.ProposeDeal, testCase{
			envParams: envParams{
				dealStream: ds,
			},
			nodeParams: nodeParams{
				VerifySignatureFails: true,
			},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				assert.Equal(t, "unable to verify signature on deal response", deal.Message)
			},
		})
	})
	t.Run("response contains unexpected state", func(t *testing.T) {
		ds := tut.NewTestStorageDealStream(tut.TestStorageDealStreamParams{
			ResponseReader: testResponseReader(t, responseParams{
				proposal: clientDealProposal,
				state:    storagemarket.StorageDealProposalNotFound,
				message:  "couldn't find deal in store",
			}),
		})
		runAndInspect(t, storagemarket.StorageDealFundsEnsured, clientstates.ProposeDeal, testCase{
			envParams: envParams{
				dealStream: ds,
			},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				assert.Equal(t, "unexpected deal status while waiting for data request: 1 (StorageDealProposalNotFound). Provider message: couldn't find deal in store", deal.Message)
			},
		})
	})
}

func TestInitiateDataTransfer(t *testing.T) {
	t.Run("succeeds and starts the data transfer", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealStartDataTransfer, clientstates.InitiateDataTransfer, testCase{
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				assert.Len(t, env.startDataTransferCalls, 1)
				assert.Equal(t, env.startDataTransferCalls[0].to, deal.Miner)
				assert.Equal(t, env.startDataTransferCalls[0].baseCid, deal.DataRef.Root)
				tut.AssertDealState(t, storagemarket.StorageDealTransferring, deal.State)
			},
		})
	})
	t.Run("starts polling for acceptance with manual transfers", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealStartDataTransfer, clientstates.InitiateDataTransfer, testCase{
			envParams: envParams{
				manualTransfer: true,
			},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealCheckForAcceptance, deal.State)
				assert.Len(t, env.startDataTransferCalls, 0)
			},
		})
	})
	t.Run("fails if it can't initiate data transfer", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealStartDataTransfer, clientstates.InitiateDataTransfer, testCase{
			envParams: envParams{
				startDataTransferError: xerrors.Errorf("failed to start data transfer"),
			},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
			},
		})
	})
}

func TestCheckForDealAcceptance(t *testing.T) {
	testCids := tut.GenerateCids(4)
	proposalCid := tut.GenerateCid(t, clientDealProposal)

	makeProviderDealState := func(status storagemarket.StorageDealStatus) *storagemarket.ProviderDealState {
		return &storagemarket.ProviderDealState{
			State:       status,
			Message:     "",
			Proposal:    &clientDealProposal.Proposal,
			ProposalCid: &proposalCid,
			AddFundsCid: &testCids[1],
			PublishCid:  &testCids[2],
			DealID:      123,
		}
	}

	t.Run("succeeds when provider indicates a successful deal", func(t *testing.T) {
		successStates := []storagemarket.StorageDealStatus{
			storagemarket.StorageDealActive,
			storagemarket.StorageDealSealing,
			storagemarket.StorageDealStaged,
			storagemarket.StorageDealSlashed,
			storagemarket.StorageDealExpired,
		}

		for _, s := range successStates {
			runAndInspect(t, storagemarket.StorageDealCheckForAcceptance, clientstates.CheckForDealAcceptance, testCase{
				envParams: envParams{
					providerDealState: makeProviderDealState(s),
				},
				inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
					tut.AssertDealState(t, storagemarket.StorageDealProposalAccepted, deal.State)
				},
			})
		}
	})

	t.Run("fails when provider indicates a failed deal", func(t *testing.T) {
		failureStates := []storagemarket.StorageDealStatus{
			storagemarket.StorageDealFailing,
			storagemarket.StorageDealError,
		}

		for _, s := range failureStates {
			runAndInspect(t, storagemarket.StorageDealCheckForAcceptance, clientstates.CheckForDealAcceptance, testCase{
				envParams: envParams{
					providerDealState: makeProviderDealState(s),
				},
				inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
					tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				},
			})
		}
	})

	t.Run("continues polling if there is an error querying provider deal state", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealCheckForAcceptance, clientstates.CheckForDealAcceptance, testCase{
			envParams: envParams{
				getDealStatusErr: xerrors.Errorf("network error"),
			},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealCheckForAcceptance, deal.State)
				assert.Equal(t, uint64(1), deal.PollRetryCount)
				assert.Equal(t, uint64(1), deal.PollErrorCount)
			},
		})
	})

	t.Run("continues polling with an indeterminate deal state", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealCheckForAcceptance, clientstates.CheckForDealAcceptance, testCase{
			envParams: envParams{
				providerDealState: makeProviderDealState(storagemarket.StorageDealVerifyData),
			},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealCheckForAcceptance, deal.State)
			},
		})
	})

	t.Run("fails if the wrong proposal comes back", func(t *testing.T) {
		pds := makeProviderDealState(storagemarket.StorageDealActive)
		pds.ProposalCid = &tut.GenerateCids(1)[0]

		runAndInspect(t, storagemarket.StorageDealCheckForAcceptance, clientstates.CheckForDealAcceptance, testCase{
			envParams: envParams{providerDealState: pds},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealFailing, deal.State)
				assert.Regexp(t, "miner responded to a wrong proposal", deal.Message)
			},
		})
	})
}

func TestValidateDealPublished(t *testing.T) {
	t.Run("succeeds", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealProposalAccepted, clientstates.ValidateDealPublished, testCase{
			nodeParams: nodeParams{ValidatePublishedDealID: abi.DealID(5)},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealSealing, deal.State)
				assert.Equal(t, abi.DealID(5), deal.DealID)
			},
		})
	})
	t.Run("fails", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealProposalAccepted, clientstates.ValidateDealPublished, testCase{
			nodeParams: nodeParams{
				ValidatePublishedDealID: abi.DealID(5),
				ValidatePublishedError:  errors.New("Something went wrong"),
			},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealError, deal.State)
				assert.Equal(t, "error validating deal published: Something went wrong", deal.Message)
			},
		})
	})
}

func TestVerifyDealActivated(t *testing.T) {
	t.Run("succeeds", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealSealing, clientstates.VerifyDealActivated, testCase{
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealActive, deal.State)
			},
		})
	})
	t.Run("fails synchronously", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealSealing, clientstates.VerifyDealActivated, testCase{
			nodeParams: nodeParams{DealCommittedSyncError: errors.New("Something went wrong")},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealError, deal.State)
				assert.Equal(t, "error in deal activation: Something went wrong", deal.Message)
			},
		})
	})
	t.Run("fails asynchronously", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealSealing, clientstates.VerifyDealActivated, testCase{
			nodeParams: nodeParams{DealCommittedAsyncError: errors.New("Something went wrong later")},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealError, deal.State)
				assert.Equal(t, "error in deal activation: Something went wrong later", deal.Message)
			},
		})
	})
}

func TestWaitForDealCompletion(t *testing.T) {
	t.Run("slashing succeeds", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealActive, clientstates.WaitForDealCompletion, testCase{
			nodeParams: nodeParams{OnDealSlashedEpoch: abi.ChainEpoch(5)},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealSlashed, deal.State)
				assert.Equal(t, abi.ChainEpoch(5), deal.SlashEpoch)
			},
		})
	})
	t.Run("expiration succeeds", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealActive, clientstates.WaitForDealCompletion, testCase{
			// OnDealSlashedEpoch of zero signals to test node to call onDealExpired()
			nodeParams: nodeParams{OnDealSlashedEpoch: abi.ChainEpoch(0)},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealExpired, deal.State)
			},
		})
	})
	t.Run("slashing fails", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealActive, clientstates.WaitForDealCompletion, testCase{
			nodeParams: nodeParams{OnDealSlashedError: errors.New("an err")},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealError, deal.State)
				assert.Equal(t, "error waiting for deal completion: deal slashing err: an err", deal.Message)
			},
		})
	})
	t.Run("expiration fails", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealActive, clientstates.WaitForDealCompletion, testCase{
			nodeParams: nodeParams{OnDealExpiredError: errors.New("an err")},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealError, deal.State)
				assert.Equal(t, "error waiting for deal completion: deal expiration err: an err", deal.Message)
			},
		})
	})
	t.Run("fails synchronously", func(t *testing.T) {
		runAndInspect(t, storagemarket.StorageDealActive, clientstates.WaitForDealCompletion, testCase{
			nodeParams: nodeParams{WaitForDealCompletionError: errors.New("an err")},
			inspector: func(deal storagemarket.ClientDeal, env *fakeEnvironment) {
				tut.AssertDealState(t, storagemarket.StorageDealError, deal.State)
				assert.Equal(t, "error waiting for deal completion: an err", deal.Message)
			},
		})
	})
}

type envParams struct {
	dealStream             *tut.TestStorageDealStream
	startDataTransferError error
	manualTransfer         bool
	providerDealState      *storagemarket.ProviderDealState
	getDealStatusErr       error
	pollingInterval        time.Duration
}

type dealStateParams struct {
	addFundsCid   *cid.Cid
	fastRetrieval bool
}

type executor func(t *testing.T,
	nodeParams nodeParams,
	envParams envParams,
	dealInspector func(deal storagemarket.ClientDeal, env *fakeEnvironment))

func makeExecutor(ctx context.Context,
	eventProcessor fsm.EventProcessor,
	initialState storagemarket.StorageDealStatus,
	stateEntryFunc clientstates.ClientStateEntryFunc,
	dealParams dealStateParams,
	clientDealProposal *market.ClientDealProposal) executor {
	return func(t *testing.T,
		nodeParams nodeParams,
		envParams envParams,
		dealInspector func(deal storagemarket.ClientDeal, env *fakeEnvironment)) {
		node := makeNode(nodeParams)
		dealState, err := tut.MakeTestClientDeal(initialState, clientDealProposal, envParams.manualTransfer)
		assert.NoError(t, err)
		dealState.AddFundsCid = &tut.GenerateCids(1)[0]
		dealState.FastRetrieval = dealParams.fastRetrieval

		if dealParams.addFundsCid != nil {
			dealState.AddFundsCid = dealParams.addFundsCid
		}

		environment := &fakeEnvironment{
			node:                   node,
			dealStream:             envParams.dealStream,
			startDataTransferError: envParams.startDataTransferError,
			providerDealState:      envParams.providerDealState,
			getDealStatusErr:       envParams.getDealStatusErr,
			pollingInterval:        envParams.pollingInterval,
		}

		if environment.pollingInterval == 0 {
			environment.pollingInterval = 0
		}

		fsmCtx := fsmtest.NewTestContext(ctx, eventProcessor)
		err = stateEntryFunc(fsmCtx, environment, *dealState)
		assert.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
		fsmCtx.ReplayEvents(t, dealState)
		dealInspector(*dealState, environment)
	}
}

type nodeParams struct {
	AddFundsCid                cid.Cid
	EnsureFundsError           error
	VerifySignatureFails       bool
	GetBalanceError            error
	GetChainHeadError          error
	WaitForMessageBlocks       bool
	WaitForMessageError        error
	WaitForMessageExitCode     exitcode.ExitCode
	WaitForMessageRetBytes     []byte
	ClientAddr                 address.Address
	ValidationError            error
	ValidatePublishedDealID    abi.DealID
	ValidatePublishedError     error
	DealCommittedSyncError     error
	DealCommittedAsyncError    error
	WaitForDealCompletionError error
	OnDealExpiredError         error
	OnDealSlashedError         error
	OnDealSlashedEpoch         abi.ChainEpoch
}

func makeNode(params nodeParams) storagemarket.StorageClientNode {
	var out testnodes.FakeClientNode
	out.SMState = testnodes.NewStorageMarketState()
	out.AddFundsCid = params.AddFundsCid
	out.EnsureFundsError = params.EnsureFundsError
	out.VerifySignatureFails = params.VerifySignatureFails
	out.GetBalanceError = params.GetBalanceError
	out.GetChainHeadError = params.GetChainHeadError
	out.WaitForMessageBlocks = params.WaitForMessageBlocks
	out.WaitForMessageError = params.WaitForMessageError
	out.WaitForMessageExitCode = params.WaitForMessageExitCode
	out.WaitForMessageRetBytes = params.WaitForMessageRetBytes
	out.ClientAddr = params.ClientAddr
	out.ValidationError = params.ValidationError
	out.ValidatePublishedDealID = params.ValidatePublishedDealID
	out.ValidatePublishedError = params.ValidatePublishedError
	out.DealCommittedSyncError = params.DealCommittedSyncError
	out.DealCommittedAsyncError = params.DealCommittedAsyncError
	out.WaitForDealCompletionError = params.WaitForDealCompletionError
	out.OnDealExpiredError = params.OnDealExpiredError
	out.OnDealSlashedError = params.OnDealSlashedError
	out.OnDealSlashedEpoch = params.OnDealSlashedEpoch
	return &out
}

type fakeEnvironment struct {
	node                   storagemarket.StorageClientNode
	dealStream             *tut.TestStorageDealStream
	startDataTransferError error
	startDataTransferCalls []dataTransferParams
	providerDealState      *storagemarket.ProviderDealState
	getDealStatusErr       error
	pollingInterval        time.Duration
}

type dataTransferParams struct {
	to       peer.ID
	voucher  datatransfer.Voucher
	baseCid  cid.Cid
	selector ipld.Node
}

func (fe *fakeEnvironment) StartDataTransfer(_ context.Context, to peer.ID, voucher datatransfer.Voucher, baseCid cid.Cid, selector ipld.Node) error {
	fe.startDataTransferCalls = append(fe.startDataTransferCalls, dataTransferParams{
		to:       to,
		voucher:  voucher,
		baseCid:  baseCid,
		selector: selector,
	})
	return fe.startDataTransferError
}

func (fe *fakeEnvironment) Node() storagemarket.StorageClientNode {
	return fe.node
}

func (fe *fakeEnvironment) WriteDealProposal(_ peer.ID, _ cid.Cid, proposal smnet.Proposal) error {
	return fe.dealStream.WriteDealProposal(proposal)
}

func (fe *fakeEnvironment) NewDealStream(_ context.Context, _ peer.ID) (smnet.StorageDealStream, error) {
	return fe.dealStream, nil
}

func (fe *fakeEnvironment) GetProviderDealState(_ context.Context, _ cid.Cid) (*storagemarket.ProviderDealState, error) {
	if fe.getDealStatusErr != nil {
		return nil, fe.getDealStatusErr
	}
	return fe.providerDealState, nil
}

func (fe *fakeEnvironment) PollingInterval() time.Duration {
	return fe.pollingInterval
}

var _ clientstates.ClientDealEnvironment = &fakeEnvironment{}

type responseParams struct {
	proposal       *market.ClientDealProposal
	state          storagemarket.StorageDealStatus
	message        string
	publishMessage *cid.Cid
	proposalCid    cid.Cid
}

func testResponseReader(t *testing.T, params responseParams) tut.StorageDealResponseReader {
	response := smnet.Response{
		State:          params.state,
		Proposal:       params.proposalCid,
		Message:        params.message,
		PublishMessage: params.publishMessage,
	}

	if response.Proposal == cid.Undef {
		proposalNd, err := cborutil.AsIpld(params.proposal)
		assert.NoError(t, err)
		response.Proposal = proposalNd.Cid()
	}

	return tut.StubbedStorageResponseReader(smnet.SignedResponse{
		Response:  response,
		Signature: tut.MakeTestSignature(),
	})
}

type testCase struct {
	envParams   envParams
	nodeParams  nodeParams
	stateParams dealStateParams
	inspector   func(deal storagemarket.ClientDeal, env *fakeEnvironment)
}

func runAndInspect(t *testing.T, initialState storagemarket.StorageDealStatus, stateFunc clientstates.ClientStateEntryFunc, tc testCase) {
	ctx := context.Background()
	eventProcessor, err := fsm.NewEventProcessor(storagemarket.ClientDeal{}, "State", clientstates.ClientEvents)
	assert.NoError(t, err)
	executor := makeExecutor(ctx, eventProcessor, initialState, stateFunc, tc.stateParams, clientDealProposal)
	executor(t, tc.nodeParams, tc.envParams, tc.inspector)
}
