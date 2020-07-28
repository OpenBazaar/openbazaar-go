package providerstates_test

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-statemachine/fsm"
	fsmtest "github.com/filecoin-project/go-statemachine/fsm/testutil"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	retrievalimpl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/providerstates"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/testnodes"
	rmtesting "github.com/filecoin-project/go-fil-markets/retrievalmarket/testing"
	testnet "github.com/filecoin-project/go-fil-markets/shared_testutil"
)

func TestReceiveDeal(t *testing.T) {
	ctx := context.Background()
	eventMachine, err := fsm.NewEventProcessor(retrievalmarket.ProviderDealState{}, "Status", providerstates.ProviderEvents)
	require.NoError(t, err)
	runReceiveDeal := func(t *testing.T,
		node *testnodes.TestRetrievalProviderNode,
		params testnet.TestDealStreamParams,
		setupEnv func(e *rmtesting.TestProviderDealEnvironment),
		dealState *retrievalmarket.ProviderDealState) {
		ds := testnet.NewTestRetrievalDealStream(params)
		environment := rmtesting.NewTestProviderDealEnvironment(node, ds, rmtesting.TrivalTestDecider, nil)
		setupEnv(environment)
		fsmCtx := fsmtest.NewTestContext(ctx, eventMachine)
		err := providerstates.ReceiveDeal(fsmCtx, environment, *dealState)
		require.NoError(t, err)
		environment.VerifyExpectations(t)
		node.VerifyExpectations(t)
		fsmCtx.ReplayEvents(t, dealState)
	}

	expectedPiece := testnet.GenerateCids(1)[0]
	proposal := retrievalmarket.DealProposal{
		ID:         retrievalmarket.DealID(10),
		PayloadCID: expectedPiece,
		Params:     retrievalmarket.NewParamsV0(defaultPricePerByte, defaultCurrentInterval, defaultIntervalIncrease),
	}

	blankDealState := func() *retrievalmarket.ProviderDealState {
		return &retrievalmarket.ProviderDealState{
			DealProposal:  proposal,
			Status:        retrievalmarket.DealStatusNew,
			TotalSent:     0,
			FundsReceived: abi.NewTokenAmount(0),
		}
	}

	t.Run("it works", func(t *testing.T) {
		node := testnodes.NewTestRetrievalProviderNode()
		dealState := blankDealState()
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseWriter: testnet.ExpectDealResponseWriter(t, retrievalmarket.DealResponse{
				Status: retrievalmarket.DealStatusAccepted,
				ID:     proposal.ID,
			}),
		}
		setupEnv := func(fe *rmtesting.TestProviderDealEnvironment) {
			fe.ExpectPiece(expectedPiece, 10000)
			fe.ExpectParams(defaultPricePerByte, defaultCurrentInterval, defaultIntervalIncrease, nil)
		}
		runReceiveDeal(t, node, dealStreamParams, setupEnv, dealState)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusAwaitingAcceptance)
		require.Equal(t, dealState.DealProposal, proposal)
		require.Empty(t, dealState.Message)
	})

	t.Run("missing piece", func(t *testing.T) {
		node := testnodes.NewTestRetrievalProviderNode()
		dealState := blankDealState()
		dealStreamParams := testnet.TestDealStreamParams{
			ProposalReader: testnet.StubbedDealProposalReader(proposal),
			ResponseWriter: testnet.ExpectDealResponseWriter(t, retrievalmarket.DealResponse{
				Status:  retrievalmarket.DealStatusDealNotFound,
				ID:      proposal.ID,
				Message: retrievalmarket.ErrNotFound.Error(),
			}),
		}
		setupEnv := func(fe *rmtesting.TestProviderDealEnvironment) {
			fe.ExpectMissingPiece(expectedPiece)
		}
		runReceiveDeal(t, node, dealStreamParams, setupEnv, dealState)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusDealNotFound)
		require.NotEmpty(t, dealState.Message)
	})

	t.Run("deal rejected", func(t *testing.T) {
		node := testnodes.NewTestRetrievalProviderNode()
		dealState := blankDealState()
		message := "Something Terrible Happened"
		dealStreamParams := testnet.TestDealStreamParams{
			ProposalReader: testnet.StubbedDealProposalReader(proposal),
			ResponseWriter: testnet.ExpectDealResponseWriter(t, retrievalmarket.DealResponse{
				Status:  retrievalmarket.DealStatusRejected,
				ID:      proposal.ID,
				Message: message,
			}),
		}
		setupEnv := func(fe *rmtesting.TestProviderDealEnvironment) {
			fe.ExpectPiece(expectedPiece, 10000)
			fe.ExpectParams(defaultPricePerByte, defaultCurrentInterval, defaultIntervalIncrease, errors.New(message))
		}
		runReceiveDeal(t, node, dealStreamParams, setupEnv, dealState)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusRejected)
		require.NotEmpty(t, dealState.Message)
	})

}

func TestSendBlocks(t *testing.T) {
	ctx := context.Background()
	node := testnodes.NewTestRetrievalProviderNode()
	eventMachine, err := fsm.NewEventProcessor(retrievalmarket.ProviderDealState{}, "Status", providerstates.ProviderEvents)
	require.NoError(t, err)
	runSendBlocks := func(t *testing.T,
		params testnet.TestDealStreamParams,
		responses []rmtesting.ReadBlockResponse,
		dealState *retrievalmarket.ProviderDealState) {
		ds := testnet.NewTestRetrievalDealStream(params)
		environment := rmtesting.NewTestProviderDealEnvironment(node, ds, rmtesting.TrivalTestDecider, responses)
		fsmCtx := fsmtest.NewTestContext(ctx, eventMachine)
		err := providerstates.SendBlocks(fsmCtx, environment, *dealState)
		require.NoError(t, err)
		fsmCtx.ReplayEvents(t, dealState)
	}

	t.Run("it works", func(t *testing.T) {
		blocks, responses := generateResponses(10, 100, false, false)
		dealState := makeDealState(retrievalmarket.DealStatusAccepted)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseWriter: testnet.ExpectDealResponseWriter(t, retrievalmarket.DealResponse{
				Status:      retrievalmarket.DealStatusFundsNeeded,
				PaymentOwed: defaultPaymentPerInterval,
				Blocks:      blocks,
				ID:          dealState.ID,
			}),
		}
		runSendBlocks(t, dealStreamParams, responses, dealState)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFundsNeeded)
		require.Equal(t, dealState.TotalSent, defaultTotalSent+defaultCurrentInterval)
		require.Empty(t, dealState.Message)
	})

	t.Run("it completes", func(t *testing.T) {
		blocks, responses := generateResponses(10, 100, true, false)
		dealState := makeDealState(retrievalmarket.DealStatusAccepted)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseWriter: testnet.ExpectDealResponseWriter(t, retrievalmarket.DealResponse{
				Status:      retrievalmarket.DealStatusFundsNeededLastPayment,
				PaymentOwed: defaultPaymentPerInterval,
				Blocks:      blocks,
				ID:          dealState.ID,
			}),
		}
		runSendBlocks(t, dealStreamParams, responses, dealState)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFundsNeededLastPayment)
		require.Equal(t, dealState.TotalSent, defaultTotalSent+defaultCurrentInterval)
		require.Empty(t, dealState.Message)
	})

	t.Run("error reading a block", func(t *testing.T) {
		_, responses := generateResponses(10, 100, false, true)
		dealState := makeDealState(retrievalmarket.DealStatusAccepted)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseWriter: testnet.ExpectDealResponseWriter(t, retrievalmarket.DealResponse{
				Status:  retrievalmarket.DealStatusFailed,
				Message: responses[0].Err.Error(),
				ID:      dealState.ID,
			}),
		}
		runSendBlocks(t, dealStreamParams, responses, dealState)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFailed)
		require.NotEmpty(t, dealState.Message)
	})

	t.Run("error writing response", func(t *testing.T) {
		_, responses := generateResponses(10, 100, false, false)
		dealState := makeDealState(retrievalmarket.DealStatusAccepted)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseWriter: testnet.FailDealResponseWriter,
		}
		runSendBlocks(t, dealStreamParams, responses, dealState)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusErrored)
		require.NotEmpty(t, dealState.Message)
	})
}

func TestProcessPayment(t *testing.T) {
	ctx := context.Background()
	eventMachine, err := fsm.NewEventProcessor(retrievalmarket.ProviderDealState{}, "Status", providerstates.ProviderEvents)
	require.NoError(t, err)
	runProcessPayment := func(t *testing.T, node *testnodes.TestRetrievalProviderNode,
		params testnet.TestDealStreamParams,
		dealState *retrievalmarket.ProviderDealState) {
		ds := testnet.NewTestRetrievalDealStream(params)
		environment := rmtesting.NewTestProviderDealEnvironment(node, ds, rmtesting.TrivalTestDecider, nil)
		fsmCtx := fsmtest.NewTestContext(ctx, eventMachine)
		err = providerstates.ProcessPayment(fsmCtx, environment, *dealState)
		require.NoError(t, err)
		node.VerifyExpectations(t)
		fsmCtx.ReplayEvents(t, dealState)
	}

	payCh := address.TestAddress
	voucher := testnet.MakeTestSignedVoucher()
	voucher.Amount = big.Add(defaultFundsReceived, defaultPaymentPerInterval)
	dealPayment := retrievalmarket.DealPayment{
		ID:             dealID,
		PaymentChannel: payCh,
		PaymentVoucher: voucher,
	}
	t.Run("it works", func(t *testing.T) {
		node := testnodes.NewTestRetrievalProviderNode()
		err := node.ExpectVoucher(payCh, voucher, nil, defaultPaymentPerInterval, defaultPaymentPerInterval, nil)
		require.NoError(t, err)
		dealState := makeDealState(retrievalmarket.DealStatusFundsNeeded)
		dealState.TotalSent = defaultTotalSent + defaultCurrentInterval
		dealStreamParams := testnet.TestDealStreamParams{
			PaymentReader: testnet.StubbedDealPaymentReader(dealPayment),
		}
		runProcessPayment(t, node, dealStreamParams, dealState)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusOngoing)
		require.Equal(t, dealState.FundsReceived, big.Add(defaultFundsReceived, defaultPaymentPerInterval))
		require.Equal(t, dealState.CurrentInterval, defaultCurrentInterval+defaultIntervalIncrease)
		require.Empty(t, dealState.Message)
	})
	t.Run("it completes", func(t *testing.T) {
		node := testnodes.NewTestRetrievalProviderNode()
		err := node.ExpectVoucher(payCh, voucher, nil, defaultPaymentPerInterval, defaultPaymentPerInterval, nil)
		require.NoError(t, err)
		dealState := makeDealState(retrievalmarket.DealStatusFundsNeededLastPayment)
		dealState.TotalSent = defaultTotalSent + defaultCurrentInterval
		dealStreamParams := testnet.TestDealStreamParams{
			PaymentReader: testnet.StubbedDealPaymentReader(dealPayment),
		}
		runProcessPayment(t, node, dealStreamParams, dealState)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFinalizing)
		require.Equal(t, dealState.FundsReceived, big.Add(defaultFundsReceived, defaultPaymentPerInterval))
		require.Equal(t, dealState.CurrentInterval, defaultCurrentInterval+defaultIntervalIncrease)
		require.Empty(t, dealState.Message)
	})

	t.Run("not enough funds sent", func(t *testing.T) {
		node := testnodes.NewTestRetrievalProviderNode()
		smallerPayment := abi.NewTokenAmount(400000)
		err := node.ExpectVoucher(payCh, voucher, nil, defaultPaymentPerInterval, smallerPayment, nil)
		require.NoError(t, err)
		dealState := makeDealState(retrievalmarket.DealStatusFundsNeeded)
		dealState.TotalSent = defaultTotalSent + defaultCurrentInterval
		dealStreamParams := testnet.TestDealStreamParams{
			PaymentReader: testnet.StubbedDealPaymentReader(dealPayment),
			ResponseWriter: testnet.ExpectDealResponseWriter(t, rm.DealResponse{
				ID:          dealState.ID,
				Status:      retrievalmarket.DealStatusFundsNeeded,
				PaymentOwed: big.Sub(defaultPaymentPerInterval, smallerPayment),
			}),
		}
		runProcessPayment(t, node, dealStreamParams, dealState)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFundsNeeded)
		require.Equal(t, dealState.FundsReceived, big.Add(defaultFundsReceived, smallerPayment))
		require.Equal(t, dealState.CurrentInterval, defaultCurrentInterval)
		require.Empty(t, dealState.Message)
	})

	t.Run("voucher already saved", func(t *testing.T) {
		node := testnodes.NewTestRetrievalProviderNode()
		err := node.ExpectVoucher(payCh, voucher, nil, defaultPaymentPerInterval, big.Zero(), nil)
		require.NoError(t, err)
		dealState := makeDealState(retrievalmarket.DealStatusFundsNeeded)
		dealState.TotalSent = defaultTotalSent + defaultCurrentInterval
		dealStreamParams := testnet.TestDealStreamParams{
			PaymentReader: testnet.StubbedDealPaymentReader(dealPayment),
		}
		runProcessPayment(t, node, dealStreamParams, dealState)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusOngoing)
		require.Equal(t, dealState.FundsReceived, big.Add(defaultFundsReceived, defaultPaymentPerInterval))
		require.Equal(t, dealState.CurrentInterval, defaultCurrentInterval+defaultIntervalIncrease)
		require.Empty(t, dealState.Message)
	})

	t.Run("failure processing payment", func(t *testing.T) {
		node := testnodes.NewTestRetrievalProviderNode()
		message := "your money's no good here"
		err := node.ExpectVoucher(payCh, voucher, nil, defaultPaymentPerInterval, abi.NewTokenAmount(0), errors.New(message))
		require.NoError(t, err)
		dealState := makeDealState(retrievalmarket.DealStatusFundsNeeded)
		dealState.TotalSent = defaultTotalSent + defaultCurrentInterval
		dealStreamParams := testnet.TestDealStreamParams{
			PaymentReader: testnet.StubbedDealPaymentReader(dealPayment),
			ResponseWriter: testnet.ExpectDealResponseWriter(t, rm.DealResponse{
				ID:      dealState.ID,
				Status:  retrievalmarket.DealStatusFailed,
				Message: message,
			}),
		}
		runProcessPayment(t, node, dealStreamParams, dealState)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFailed)
		require.NotEmpty(t, dealState.Message)
	})

	t.Run("failure reading payment", func(t *testing.T) {
		node := testnodes.NewTestRetrievalProviderNode()
		dealState := makeDealState(retrievalmarket.DealStatusFundsNeeded)
		dealState.TotalSent = defaultTotalSent + defaultCurrentInterval
		dealStreamParams := testnet.TestDealStreamParams{
			PaymentReader: testnet.FailDealPaymentReader,
		}
		runProcessPayment(t, node, dealStreamParams, dealState)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusErrored)
		require.NotEmpty(t, dealState.Message)
	})
}

func TestDecideOnDeal(t *testing.T) {
	ctx := context.Background()
	eventMachine, err := fsm.NewEventProcessor(retrievalmarket.ProviderDealState{}, "Status", providerstates.ProviderEvents)
	require.NoError(t, err)
	runDecideDeal := func(t *testing.T,
		node *testnodes.TestRetrievalProviderNode,
		params testnet.TestDealStreamParams,
		setupEnv func(e *rmtesting.TestProviderDealEnvironment),
		decider retrievalimpl.DealDecider,
		dealState *retrievalmarket.ProviderDealState) {
		ds := testnet.NewTestRetrievalDealStream(params)
		environment := rmtesting.NewTestProviderDealEnvironment(node, ds, decider, nil)
		setupEnv(environment)
		fsmCtx := fsmtest.NewTestContext(ctx, eventMachine)
		err := providerstates.DecideOnDeal(fsmCtx, environment, *dealState)
		require.NoError(t, err)
		environment.VerifyExpectations(t)
		node.VerifyExpectations(t)
		fsmCtx.ReplayEvents(t, dealState)
	}

	proposal := retrievalmarket.DealProposal{
		ID:         retrievalmarket.DealID(10),
		PayloadCID: testnet.GenerateCids(1)[0],
		Params:     retrievalmarket.NewParamsV0(defaultPricePerByte, defaultCurrentInterval, defaultIntervalIncrease),
	}

	startingDealState := func() *retrievalmarket.ProviderDealState {
		return &retrievalmarket.ProviderDealState{
			DealProposal:  proposal,
			Status:        retrievalmarket.DealStatusAwaitingAcceptance,
			FundsReceived: abi.NewTokenAmount(0),
		}
	}
	acceptedDsParams := testnet.TestDealStreamParams{
		ResponseWriter: testnet.ExpectDealResponseWriter(t, retrievalmarket.DealResponse{
			Status: retrievalmarket.DealStatusAccepted,
			ID:     proposal.ID,
		})}

	type testCases map[string]struct {
		dsParams testnet.TestDealStreamParams
		decider  retrievalimpl.DealDecider
		setupEnv func(*rmtesting.TestProviderDealEnvironment)
		verify   func(*testing.T, *rm.ProviderDealState)
	}
	tcs := testCases{
		"qapla'": {
			dsParams: acceptedDsParams,
			setupEnv: func(te *rmtesting.TestProviderDealEnvironment) {
				te.ExpectDeciderCalledWith(proposal.ID)
			},
			verify: func(t *testing.T, state *rm.ProviderDealState) {
				assert.Equal(t, state.Status, retrievalmarket.DealStatusAccepted)
				assert.Empty(t, state.Message)
				assert.Equal(t, defaultCurrentInterval, state.CurrentInterval)
			},
		},
		"if decider fails, deal errors": {
			dsParams: acceptedDsParams,
			decider: func(ctx context.Context, state rm.ProviderDealState) (bool, string, error) {
				return false, "", errors.New("boom")
			},
			setupEnv: func(te *rmtesting.TestProviderDealEnvironment) {
				te.ExpectDeciderCalledWith(proposal.ID)
			},
			verify: func(t *testing.T, state *rm.ProviderDealState) {
				assert.Equal(t, retrievalmarket.DealStatusErrored, state.Status)
				assert.Equal(t, "boom", state.Message)
			},
		},
		"if decider rejects, deal is rejected": {
			dsParams: acceptedDsParams,
			decider: func(ctx context.Context, state rm.ProviderDealState) (bool, string, error) {
				return false, "Thursday, I don't care about you", nil
			},
			setupEnv: func(te *rmtesting.TestProviderDealEnvironment) {
				te.ExpectDeciderCalledWith(proposal.ID)
			},
			verify: func(t *testing.T, state *rm.ProviderDealState) {
				assert.Equal(t, retrievalmarket.DealStatusRejected, state.Status)
				assert.Equal(t, "Thursday, I don't care about you", state.Message)
			},
		},
		"if response write error, deal errors": {
			dsParams: testnet.TestDealStreamParams{
				ProposalReader: testnet.StubbedDealProposalReader(proposal),
				ResponseWriter: testnet.FailDealResponseWriter,
			},
			setupEnv: func(te *rmtesting.TestProviderDealEnvironment) {
				te.ExpectDeciderCalledWith(proposal.ID)
			},
			verify: func(t *testing.T, state *rm.ProviderDealState) {
				assert.Equal(t, retrievalmarket.DealStatusErrored, state.Status)
				assert.NotEmpty(t, state.Message)
			},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			node := testnodes.NewTestRetrievalProviderNode()
			dealState := startingDealState()
			runDecideDeal(t, node, tc.dsParams, tc.setupEnv, tc.decider, dealState)
			assert.Equal(t, proposal, dealState.DealProposal)
			tc.verify(t, dealState)
		})
	}
}

var dealID = retrievalmarket.DealID(10)
var defaultCurrentInterval = uint64(1000)
var defaultIntervalIncrease = uint64(500)
var defaultPricePerByte = abi.NewTokenAmount(500)
var defaultPaymentPerInterval = big.Mul(defaultPricePerByte, abi.NewTokenAmount(int64(defaultCurrentInterval)))
var defaultTotalSent = uint64(5000)
var defaultFundsReceived = abi.NewTokenAmount(2500000)

func makeDealState(status retrievalmarket.DealStatus) *retrievalmarket.ProviderDealState {
	return &retrievalmarket.ProviderDealState{
		Status:          status,
		TotalSent:       defaultTotalSent,
		CurrentInterval: defaultCurrentInterval,
		FundsReceived:   defaultFundsReceived,
		DealProposal: retrievalmarket.DealProposal{
			ID:     dealID,
			Params: retrievalmarket.NewParamsV0(defaultPricePerByte, defaultCurrentInterval, defaultIntervalIncrease),
		},
	}
}

func generateResponses(count uint64, blockSize uint64, completeOnLast bool,
	errorOnFirst bool) ([]retrievalmarket.Block, []rmtesting.ReadBlockResponse) {
	responses := make([]rmtesting.ReadBlockResponse, count)
	blocks := make([]retrievalmarket.Block, count)
	var i uint64 = 0
	for ; i < count; i++ {
		data := make([]byte, blockSize)
		var err error
		_, err = rand.Read(data)
		complete := false
		if i == 0 && errorOnFirst {
			err = errors.New("something went wrong")
		}

		if i == count-1 && completeOnLast {
			complete = true
		}
		block := retrievalmarket.Block{
			Prefix: cid.NewPrefixV1(cid.Raw, mh.SHA2_256).Bytes(),
			Data:   data,
		}
		blocks[i] = block
		responses[i] = rmtesting.ReadBlockResponse{
			Block: block, Done: complete, Err: err}
	}
	return blocks, responses
}
