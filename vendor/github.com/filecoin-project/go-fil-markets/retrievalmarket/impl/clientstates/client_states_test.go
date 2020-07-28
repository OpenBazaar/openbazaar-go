package clientstates_test

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
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/clientstates"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/testnodes"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	testnet "github.com/filecoin-project/go-fil-markets/shared_testutil"
)

type consumeBlockResponse struct {
	size uint64
	done bool
	err  error
}

type fakeEnvironment struct {
	node         retrievalmarket.RetrievalClientNode
	ds           rmnet.RetrievalDealStream
	nextResponse int
	responses    []consumeBlockResponse
}

func (e *fakeEnvironment) Node() retrievalmarket.RetrievalClientNode {
	return e.node
}

func (e *fakeEnvironment) DealStream(id retrievalmarket.DealID) rmnet.RetrievalDealStream {
	return e.ds
}

func (e *fakeEnvironment) ConsumeBlock(context.Context, retrievalmarket.DealID, retrievalmarket.Block) (uint64, bool, error) {
	if e.nextResponse >= len(e.responses) {
		return 0, false, errors.New("ConsumeBlock failed")
	}
	response := e.responses[e.nextResponse]
	e.nextResponse += 1
	return response.size, response.done, response.err
}

func TestSetupPaymentChannel(t *testing.T) {
	ctx := context.Background()
	ds := testnet.NewTestRetrievalDealStream(testnet.TestDealStreamParams{})
	expectedPayCh := address.TestAddress2
	eventMachine, err := fsm.NewEventProcessor(retrievalmarket.ClientDealState{}, "Status", clientstates.ClientEvents)
	require.NoError(t, err)
	runSetupPaymentChannel := func(t *testing.T,
		params testnodes.TestRetrievalClientNodeParams,
		dealState *retrievalmarket.ClientDealState) {
		node := testnodes.NewTestRetrievalClientNode(params)
		environment := &fakeEnvironment{node, ds, 0, nil}
		fsmCtx := fsmtest.NewTestContext(ctx, eventMachine)
		err := clientstates.SetupPaymentChannelStart(fsmCtx, environment, *dealState)
		require.NoError(t, err)
		fsmCtx.ReplayEvents(t, dealState)
	}

	t.Run("payment channel create initiated", func(t *testing.T) {
		envParams := testnodes.TestRetrievalClientNodeParams{
			PayCh:          address.Undef,
			CreatePaychCID: testnet.GenerateCids(1)[0],
		}
		dealState := makeDealState(retrievalmarket.DealStatusAccepted)
		runSetupPaymentChannel(t, envParams, dealState)
		assert.Empty(t, dealState.Message)
		assert.Equal(t, dealState.Status, retrievalmarket.DealStatusPaymentChannelCreating)
	})

	t.Run("payment channel needs funds added", func(t *testing.T) {
		envParams := testnodes.TestRetrievalClientNodeParams{
			AddFundsOnly:   true,
			PayCh:          expectedPayCh,
			CreatePaychCID: testnet.GenerateCids(1)[0],
		}
		dealState := makeDealState(retrievalmarket.DealStatusAccepted)
		runSetupPaymentChannel(t, envParams, dealState)
		require.Empty(t, dealState.Message)
		require.Equal(t, retrievalmarket.DealStatusPaymentChannelAddingFunds, dealState.Status)
		require.Equal(t, expectedPayCh, dealState.PaymentInfo.PayCh)
	})

	t.Run("when create payment channel fails", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusAccepted)
		envParams := testnodes.TestRetrievalClientNodeParams{
			PayCh:    address.Undef,
			PayChErr: errors.New("Something went wrong"),
		}
		runSetupPaymentChannel(t, envParams, dealState)
		require.NotEmpty(t, dealState.Message)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFailed)
	})

}

func TestWaitForPaymentChannelCreate(t *testing.T) {
	ctx := context.Background()
	ds := testnet.NewTestRetrievalDealStream(testnet.TestDealStreamParams{})
	expectedPayCh := address.TestAddress2
	expectedLane := uint64(10)
	eventMachine, err := fsm.NewEventProcessor(retrievalmarket.ClientDealState{}, "Status", clientstates.ClientEvents)
	require.NoError(t, err)
	runWaitForPaychCreate := func(t *testing.T,
		params testnodes.TestRetrievalClientNodeParams,
		dealState *retrievalmarket.ClientDealState) {
		node := testnodes.NewTestRetrievalClientNode(params)
		environment := &fakeEnvironment{node, ds, 0, nil}
		fsmCtx := fsmtest.NewTestContext(ctx, eventMachine)
		err := clientstates.WaitForPaymentChannelCreate(fsmCtx, environment, *dealState)
		require.NoError(t, err)
		fsmCtx.ReplayEvents(t, dealState)
	}
	msgCID := testnet.GenerateCids(1)[0]

	t.Run("it works", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusPaymentChannelCreating)
		dealState.WaitMsgCID = &msgCID
		params := testnodes.TestRetrievalClientNodeParams{
			PayCh:          expectedPayCh,
			CreatePaychCID: msgCID,
			Lane:           expectedLane,
		}
		runWaitForPaychCreate(t, params, dealState)
		require.Empty(t, dealState.Message)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusPaymentChannelReady)
		require.Equal(t, expectedLane, dealState.PaymentInfo.Lane)
		require.Equal(t, expectedPayCh, dealState.PaymentInfo.PayCh)
	})
	t.Run("if Wait fails", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusPaymentChannelCreating)
		dealState.WaitMsgCID = &msgCID
		params := testnodes.TestRetrievalClientNodeParams{
			PayCh:              expectedPayCh,
			CreatePaychCID:     msgCID,
			WaitForChCreateErr: errors.New("boom"),
		}
		runWaitForPaychCreate(t, params, dealState)
		require.Contains(t, dealState.Message, "boom")
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFailed)
	})

	t.Run("if AllocateLane fails", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusPaymentChannelCreating)
		dealState.WaitMsgCID = &msgCID
		params := testnodes.TestRetrievalClientNodeParams{
			PayCh:          expectedPayCh,
			CreatePaychCID: msgCID,
			LaneError:      errors.New("boom"),
		}
		runWaitForPaychCreate(t, params, dealState)
		require.Contains(t, dealState.Message, "boom")
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFailed)
	})
}

func TestWaitForPaymentChannelAddFunds(t *testing.T) {
	ctx := context.Background()
	ds := testnet.NewTestRetrievalDealStream(testnet.TestDealStreamParams{})
	expectedPayCh := address.TestAddress2
	expectedLane := uint64(99)
	eventMachine, err := fsm.NewEventProcessor(retrievalmarket.ClientDealState{}, "Status", clientstates.ClientEvents)
	require.NoError(t, err)
	runWaitForPaychAddFunds := func(t *testing.T,
		params testnodes.TestRetrievalClientNodeParams,
		dealState *retrievalmarket.ClientDealState) {
		node := testnodes.NewTestRetrievalClientNode(params)
		environment := &fakeEnvironment{node, ds, 0, nil}
		fsmCtx := fsmtest.NewTestContext(ctx, eventMachine)
		err := clientstates.WaitForPaymentChannelAddFunds(fsmCtx, environment, *dealState)
		require.NoError(t, err)
		fsmCtx.ReplayEvents(t, dealState)
	}
	msgCID := testnet.GenerateCids(1)[0]

	t.Run("it works", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusPaymentChannelAddingFunds)
		dealState.PaymentInfo.PayCh = expectedPayCh
		dealState.WaitMsgCID = &msgCID

		params := testnodes.TestRetrievalClientNodeParams{
			AddFundsOnly: true,
			PayCh:        expectedPayCh,
			AddFundsCID:  msgCID,
			Lane:         expectedLane,
		}
		runWaitForPaychAddFunds(t, params, dealState)
		require.Empty(t, dealState.Message)
		assert.Equal(t, retrievalmarket.DealStatusPaymentChannelReady, dealState.Status)
		assert.Equal(t, expectedLane, dealState.PaymentInfo.Lane)
		assert.Equal(t, expectedPayCh, dealState.PaymentInfo.PayCh)
	})
	t.Run("if Wait fails", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusPaymentChannelAddingFunds)
		dealState.WaitMsgCID = &msgCID
		params := testnodes.TestRetrievalClientNodeParams{
			AddFundsOnly:       true,
			PayCh:              expectedPayCh,
			AddFundsCID:        msgCID,
			WaitForAddFundsErr: errors.New("boom"),
			Lane:               expectedLane,
		}
		runWaitForPaychAddFunds(t, params, dealState)
		assert.Contains(t, dealState.Message, "boom")
		assert.Equal(t, dealState.Status, retrievalmarket.DealStatusFailed)
		assert.Equal(t, uint64(0), dealState.PaymentInfo.Lane)
	})
	t.Run("if AllocateLane fails", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusPaymentChannelAddingFunds)
		dealState.WaitMsgCID = &msgCID
		params := testnodes.TestRetrievalClientNodeParams{
			AddFundsOnly: true,
			PayCh:        expectedPayCh,
			AddFundsCID:  msgCID,
			LaneError:    errors.New("boom"),
			Lane:         expectedLane,
		}
		runWaitForPaychAddFunds(t, params, dealState)
		assert.Contains(t, dealState.Message, "boom")
		assert.Equal(t, dealState.Status, retrievalmarket.DealStatusFailed)
		assert.Equal(t, uint64(0), dealState.PaymentInfo.Lane)
	})
}

func TestProposeDeal(t *testing.T) {
	ctx := context.Background()
	node := testnodes.NewTestRetrievalClientNode(testnodes.TestRetrievalClientNodeParams{})
	eventMachine, err := fsm.NewEventProcessor(retrievalmarket.ClientDealState{}, "Status", clientstates.ClientEvents)
	require.NoError(t, err)
	runProposeDeal := func(t *testing.T, params testnet.TestDealStreamParams, dealState *retrievalmarket.ClientDealState) {
		ds := testnet.NewTestRetrievalDealStream(params)
		environment := &fakeEnvironment{node, ds, 0, nil}
		fsmCtx := fsmtest.NewTestContext(ctx, eventMachine)
		err := clientstates.ProposeDeal(fsmCtx, environment, *dealState)
		require.NoError(t, err)
		fsmCtx.ReplayEvents(t, dealState)
	}

	t.Run("it works", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusNew)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseReader: testnet.StubbedDealResponseReader(retrievalmarket.DealResponse{
				Status: retrievalmarket.DealStatusAccepted,
				ID:     dealState.ID,
			}),
		}
		runProposeDeal(t, dealStreamParams, dealState)
		require.Empty(t, dealState.Message)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusAccepted)
	})

	t.Run("deal rejected", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusNew)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseReader: testnet.StubbedDealResponseReader(retrievalmarket.DealResponse{
				Status:  retrievalmarket.DealStatusRejected,
				ID:      dealState.ID,
				Message: "your deal proposal sucks",
			}),
		}
		runProposeDeal(t, dealStreamParams, dealState)
		require.NotEmpty(t, dealState.Message)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusRejected)
	})

	t.Run("deal not found", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusNew)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseReader: testnet.StubbedDealResponseReader(retrievalmarket.DealResponse{
				Status:  retrievalmarket.DealStatusDealNotFound,
				ID:      dealState.ID,
				Message: "can't find a deal",
			}),
		}
		runProposeDeal(t, dealStreamParams, dealState)
		require.NotEmpty(t, dealState.Message)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusDealNotFound)
	})

	t.Run("unable to send proposal", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusNew)
		dealStreamParams := testnet.TestDealStreamParams{
			ProposalWriter: testnet.FailDealProposalWriter,
		}
		runProposeDeal(t, dealStreamParams, dealState)
		require.NotEmpty(t, dealState.Message)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusErrored)
	})

	t.Run("unable to read response", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusNew)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseReader: testnet.FailDealResponseReader,
		}
		runProposeDeal(t, dealStreamParams, dealState)
		require.NotEmpty(t, dealState.Message)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusErrored)
	})
}

func TestProcessPaymentRequested(t *testing.T) {
	ctx := context.Background()
	eventMachine, err := fsm.NewEventProcessor(retrievalmarket.ClientDealState{}, "Status", clientstates.ClientEvents)
	require.NoError(t, err)
	runProcessPaymentRequested := func(t *testing.T,
		netParams testnet.TestDealStreamParams,
		nodeParams testnodes.TestRetrievalClientNodeParams,
		dealState *retrievalmarket.ClientDealState) {
		ds := testnet.NewTestRetrievalDealStream(netParams)
		node := testnodes.NewTestRetrievalClientNode(nodeParams)
		environment := &fakeEnvironment{node, ds, 0, nil}
		fsmCtx := fsmtest.NewTestContext(ctx, eventMachine)
		err := clientstates.ProcessPaymentRequested(fsmCtx, environment, *dealState)
		require.NoError(t, err)
		fsmCtx.ReplayEvents(t, dealState)
	}

	testVoucher := &paych.SignedVoucher{}

	t.Run("it works", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusFundsNeeded)
		dealStreamParams := testnet.TestDealStreamParams{}
		nodeParams := testnodes.TestRetrievalClientNodeParams{
			Voucher: testVoucher,
		}
		runProcessPaymentRequested(t, dealStreamParams, nodeParams, dealState)
		require.Empty(t, dealState.Message)
		require.Equal(t, dealState.PaymentRequested, abi.NewTokenAmount(0))
		require.Equal(t, dealState.FundsSpent, big.Add(defaultFundsSpent, defaultPaymentRequested))
		require.Equal(t, dealState.BytesPaidFor, defaultTotalReceived)
		require.Equal(t, dealState.CurrentInterval, defaultCurrentInterval+defaultIntervalIncrease)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusOngoing)
	})

	t.Run("last payment", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusFundsNeededLastPayment)
		dealStreamParams := testnet.TestDealStreamParams{}
		nodeParams := testnodes.TestRetrievalClientNodeParams{
			Voucher: testVoucher,
		}
		runProcessPaymentRequested(t, dealStreamParams, nodeParams, dealState)
		require.Empty(t, dealState.Message)
		require.Equal(t, dealState.PaymentRequested, abi.NewTokenAmount(0))
		require.Equal(t, dealState.FundsSpent, big.Add(defaultFundsSpent, defaultPaymentRequested))
		require.Equal(t, dealState.BytesPaidFor, defaultTotalReceived)
		require.Equal(t, dealState.CurrentInterval, defaultCurrentInterval+defaultIntervalIncrease)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFinalizing)
	})

	t.Run("not enough funds left", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusFundsNeeded)
		dealState.FundsSpent = defaultTotalFunds
		dealStreamParams := testnet.TestDealStreamParams{}
		nodeParams := testnodes.TestRetrievalClientNodeParams{
			Voucher: testVoucher,
		}
		runProcessPaymentRequested(t, dealStreamParams, nodeParams, dealState)
		require.NotEmpty(t, dealState.Message)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFailed)
	})

	t.Run("not enough bytes since last payment", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusFundsNeeded)
		dealState.BytesPaidFor = defaultBytesPaidFor + 500
		dealStreamParams := testnet.TestDealStreamParams{}
		nodeParams := testnodes.TestRetrievalClientNodeParams{
			Voucher: testVoucher,
		}
		runProcessPaymentRequested(t, dealStreamParams, nodeParams, dealState)
		require.NotEmpty(t, dealState.Message)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFailed)
	})

	t.Run("more bytes since last payment than interval works, can charge more", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusFundsNeeded)
		dealState.BytesPaidFor = defaultBytesPaidFor - 500
		largerPaymentRequested := abi.NewTokenAmount(750000)
		dealState.PaymentRequested = largerPaymentRequested
		dealStreamParams := testnet.TestDealStreamParams{}
		nodeParams := testnodes.TestRetrievalClientNodeParams{
			Voucher: testVoucher,
		}
		runProcessPaymentRequested(t, dealStreamParams, nodeParams, dealState)
		require.Empty(t, dealState.Message)
		require.Equal(t, dealState.PaymentRequested, abi.NewTokenAmount(0))
		require.Equal(t, dealState.FundsSpent, big.Add(defaultFundsSpent, largerPaymentRequested))
		require.Equal(t, dealState.BytesPaidFor, defaultTotalReceived)
		require.Equal(t, dealState.CurrentInterval, defaultCurrentInterval+defaultIntervalIncrease)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusOngoing)
	})

	t.Run("too much payment requested", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusFundsNeeded)
		dealState.PaymentRequested = abi.NewTokenAmount(750000)
		dealStreamParams := testnet.TestDealStreamParams{}
		nodeParams := testnodes.TestRetrievalClientNodeParams{
			Voucher: testVoucher,
		}
		runProcessPaymentRequested(t, dealStreamParams, nodeParams, dealState)
		require.NotEmpty(t, dealState.Message)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFailed)
	})

	t.Run("too little payment requested works but records correctly", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusFundsNeeded)
		smallerPaymentRequested := abi.NewTokenAmount(250000)
		dealState.PaymentRequested = smallerPaymentRequested
		dealStreamParams := testnet.TestDealStreamParams{}
		nodeParams := testnodes.TestRetrievalClientNodeParams{
			Voucher: testVoucher,
		}
		runProcessPaymentRequested(t, dealStreamParams, nodeParams, dealState)
		require.Empty(t, dealState.Message)
		require.Equal(t, dealState.PaymentRequested, abi.NewTokenAmount(0))
		require.Equal(t, dealState.FundsSpent, big.Add(defaultFundsSpent, smallerPaymentRequested))
		// only records change for those bytes paid for
		require.Equal(t, dealState.BytesPaidFor, defaultBytesPaidFor+500)
		// no interval increase
		require.Equal(t, dealState.CurrentInterval, defaultCurrentInterval)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusOngoing)
	})

	t.Run("voucher create fails", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusFundsNeeded)
		dealStreamParams := testnet.TestDealStreamParams{}
		nodeParams := testnodes.TestRetrievalClientNodeParams{
			VoucherError: errors.New("Something Went Wrong"),
		}
		runProcessPaymentRequested(t, dealStreamParams, nodeParams, dealState)
		require.NotEmpty(t, dealState.Message)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFailed)
	})

	t.Run("unable to send payment", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusFundsNeeded)
		dealStreamParams := testnet.TestDealStreamParams{
			PaymentWriter: testnet.FailDealPaymentWriter,
		}
		nodeParams := testnodes.TestRetrievalClientNodeParams{
			Voucher: testVoucher,
		}
		runProcessPaymentRequested(t, dealStreamParams, nodeParams, dealState)
		require.NotEmpty(t, dealState.Message)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusErrored)
	})
}

func TestProcessNextResponse(t *testing.T) {
	ctx := context.Background()
	node := testnodes.NewTestRetrievalClientNode(testnodes.TestRetrievalClientNodeParams{})
	eventMachine, err := fsm.NewEventProcessor(retrievalmarket.ClientDealState{}, "Status", clientstates.ClientEvents)
	require.NoError(t, err)
	runProcessNextResponse := func(t *testing.T,
		netParams testnet.TestDealStreamParams,
		responses []consumeBlockResponse,
		dealState *retrievalmarket.ClientDealState) {
		ds := testnet.NewTestRetrievalDealStream(netParams)
		environment := &fakeEnvironment{node, ds, 0, responses}
		fsmCtx := fsmtest.NewTestContext(ctx, eventMachine)
		err := clientstates.ProcessNextResponse(fsmCtx, environment, *dealState)
		require.NoError(t, err)
		fsmCtx.ReplayEvents(t, dealState)
	}
	paymentOwed := abi.NewTokenAmount(1000)
	t.Run("it works", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusOngoing)
		blocks, consumeBlockResponses := generateBlocks(10, 100, false, false)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseReader: testnet.StubbedDealResponseReader(retrievalmarket.DealResponse{
				Status: retrievalmarket.DealStatusOngoing,
				ID:     dealState.ID,
				Blocks: blocks,
			}),
		}
		runProcessNextResponse(t, dealStreamParams, consumeBlockResponses, dealState)
		require.Empty(t, dealState.Message)
		require.Equal(t, dealState.TotalReceived, defaultTotalReceived+1000)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusOngoing)
	})

	t.Run("completes", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusOngoing)
		blocks, consumeBlockResponses := generateBlocks(10, 100, true, false)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseReader: testnet.StubbedDealResponseReader(retrievalmarket.DealResponse{
				Status: retrievalmarket.DealStatusCompleted,
				ID:     dealState.ID,
				Blocks: blocks,
			}),
		}
		runProcessNextResponse(t, dealStreamParams, consumeBlockResponses, dealState)
		require.Empty(t, dealState.Message)
		require.Equal(t, dealState.TotalReceived, defaultTotalReceived+1000)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusCompleted)
	})

	t.Run("completes last payment", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusOngoing)
		blocks, consumeBlockResponses := generateBlocks(10, 100, true, false)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseReader: testnet.StubbedDealResponseReader(retrievalmarket.DealResponse{
				Status:      retrievalmarket.DealStatusFundsNeededLastPayment,
				ID:          dealState.ID,
				PaymentOwed: paymentOwed,
				Blocks:      blocks,
			}),
		}
		runProcessNextResponse(t, dealStreamParams, consumeBlockResponses, dealState)
		require.Empty(t, dealState.Message)
		require.Equal(t, dealState.TotalReceived, defaultTotalReceived+1000)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFundsNeededLastPayment)
		require.Equal(t, dealState.PaymentRequested, paymentOwed)
	})

	t.Run("receive complete status but deal is not complete errors", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusOngoing)
		blocks, consumeBlockResponses := generateBlocks(10, 100, false, false)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseReader: testnet.StubbedDealResponseReader(retrievalmarket.DealResponse{
				Status: retrievalmarket.DealStatusCompleted,
				ID:     dealState.ID,
				Blocks: blocks,
			}),
		}
		runProcessNextResponse(t, dealStreamParams, consumeBlockResponses, dealState)
		require.NotEmpty(t, dealState.Message)
		require.Equal(t, dealState.TotalReceived, defaultTotalReceived)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFailed)
	})
	t.Run("payment requested", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusOngoing)
		blocks, consumeBlockResponses := generateBlocks(10, 100, false, false)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseReader: testnet.StubbedDealResponseReader(retrievalmarket.DealResponse{
				Status:      retrievalmarket.DealStatusFundsNeeded,
				ID:          dealState.ID,
				PaymentOwed: paymentOwed,
				Blocks:      blocks,
			}),
		}
		runProcessNextResponse(t, dealStreamParams, consumeBlockResponses, dealState)
		require.Empty(t, dealState.Message)
		require.Equal(t, dealState.TotalReceived, defaultTotalReceived+1000)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFundsNeeded)
		require.Equal(t, dealState.PaymentRequested, paymentOwed)
	})

	t.Run("unexpected status errors", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusOngoing)
		blocks, consumeBlockResponses := generateBlocks(10, 100, false, false)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseReader: testnet.StubbedDealResponseReader(retrievalmarket.DealResponse{
				Status: retrievalmarket.DealStatusNew,
				ID:     dealState.ID,
				Blocks: blocks,
			}),
		}
		runProcessNextResponse(t, dealStreamParams, consumeBlockResponses, dealState)
		require.NotEmpty(t, dealState.Message)
		require.Equal(t, dealState.TotalReceived, defaultTotalReceived)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFailed)
	})

	t.Run("consume block errors", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusOngoing)
		blocks, consumeBlockResponses := generateBlocks(10, 100, false, true)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseReader: testnet.StubbedDealResponseReader(retrievalmarket.DealResponse{
				Status: retrievalmarket.DealStatusOngoing,
				ID:     dealState.ID,
				Blocks: blocks,
			}),
		}
		runProcessNextResponse(t, dealStreamParams, consumeBlockResponses, dealState)
		require.NotEmpty(t, dealState.Message)
		require.Equal(t, dealState.TotalReceived, defaultTotalReceived)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusFailed)
	})

	t.Run("read response errors", func(t *testing.T) {
		dealState := makeDealState(retrievalmarket.DealStatusOngoing)
		dealStreamParams := testnet.TestDealStreamParams{
			ResponseReader: testnet.FailDealResponseReader,
		}
		runProcessNextResponse(t, dealStreamParams, nil, dealState)
		require.NotEmpty(t, dealState.Message)
		require.Equal(t, dealState.TotalReceived, defaultTotalReceived)
		require.Equal(t, dealState.Status, retrievalmarket.DealStatusErrored)
	})
}

var defaultTotalFunds = abi.NewTokenAmount(4000000)
var defaultCurrentInterval = uint64(1000)
var defaultIntervalIncrease = uint64(500)
var defaultPricePerByte = abi.NewTokenAmount(500)
var defaultTotalReceived = uint64(6000)
var defaultBytesPaidFor = uint64(5000)
var defaultFundsSpent = abi.NewTokenAmount(2500000)
var defaultPaymentRequested = abi.NewTokenAmount(500000)

func makeDealState(status retrievalmarket.DealStatus) *retrievalmarket.ClientDealState {
	return &retrievalmarket.ClientDealState{
		TotalFunds:       defaultTotalFunds,
		MinerWallet:      address.TestAddress,
		ClientWallet:     address.TestAddress2,
		PaymentInfo:      &retrievalmarket.PaymentInfo{},
		Status:           status,
		BytesPaidFor:     defaultBytesPaidFor,
		TotalReceived:    defaultTotalReceived,
		CurrentInterval:  defaultCurrentInterval,
		FundsSpent:       defaultFundsSpent,
		PaymentRequested: defaultPaymentRequested,
		DealProposal: retrievalmarket.DealProposal{
			ID:     retrievalmarket.DealID(10),
			Params: retrievalmarket.NewParamsV0(defaultPricePerByte, 0, defaultIntervalIncrease),
		},
	}
}

func generateBlocks(count uint64, blockSize uint64, completeOnLast bool, errorOnFirst bool) ([]retrievalmarket.Block, []consumeBlockResponse) {
	blocks := make([]retrievalmarket.Block, count)
	responses := make([]consumeBlockResponse, count)
	var i uint64 = 0
	for ; i < count; i++ {
		data := make([]byte, blockSize)
		var err error
		_, err = rand.Read(data)
		blocks[i] = retrievalmarket.Block{
			Prefix: cid.NewPrefixV1(cid.Raw, mh.SHA2_256).Bytes(),
			Data:   data,
		}
		complete := false
		if i == 0 && errorOnFirst {
			err = errors.New("something went wrong")
		}

		if i == count-1 && completeOnLast {
			complete = true
		}
		responses[i] = consumeBlockResponse{blockSize, complete, err}
	}
	return blocks, responses
}
