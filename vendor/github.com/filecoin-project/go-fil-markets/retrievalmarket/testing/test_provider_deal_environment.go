// Package testing provides test implementations of retieval market interfaces
package testing

import (
	"context"
	"errors"
	"testing"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	retrievalimpl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
)

// TestProviderDealEnvironment is a test implementation of ProviderDealEnvironment used
// by the provider state machine.
type TestProviderDealEnvironment struct {
	decider              retrievalimpl.DealDecider
	node                 rm.RetrievalProviderNode
	ds                   rmnet.RetrievalDealStream
	nextResponse         int
	responses            []ReadBlockResponse
	expectedParams       map[dealParamsKey]error
	receivedParams       map[dealParamsKey]bool
	expectedCIDs         map[cid.Cid]uint64
	expectedMissingCIDs  map[cid.Cid]struct{}
	receivedCIDs         map[cid.Cid]struct{}
	receivedMissingCIDs  map[cid.Cid]struct{}
	expectedDeciderCalls map[string]struct{}
	receivedDeciderCalls map[string]struct{}
}

// NewTestProviderDealEnvironment returns a new TestProviderDealEnvironment instance
func NewTestProviderDealEnvironment(node rm.RetrievalProviderNode,
	ds rmnet.RetrievalDealStream,
	decider retrievalimpl.DealDecider,
	responses []ReadBlockResponse) *TestProviderDealEnvironment {
	return &TestProviderDealEnvironment{
		node:                 node,
		ds:                   ds,
		nextResponse:         0,
		responses:            responses,
		expectedParams:       make(map[dealParamsKey]error),
		receivedParams:       make(map[dealParamsKey]bool),
		expectedCIDs:         make(map[cid.Cid]uint64),
		expectedMissingCIDs:  make(map[cid.Cid]struct{}),
		receivedCIDs:         make(map[cid.Cid]struct{}),
		receivedMissingCIDs:  make(map[cid.Cid]struct{}),
		expectedDeciderCalls: make(map[string]struct{}),
		receivedDeciderCalls: make(map[string]struct{}),
		decider:              decider,
	}
}

// ExpectPiece records a piece being expected to be queried and return the given piece info
func (te *TestProviderDealEnvironment) ExpectPiece(c cid.Cid, size uint64) {
	te.expectedCIDs[c] = size
}

// ExpectMissingPiece records a piece being expected to be queried and should fail
func (te *TestProviderDealEnvironment) ExpectMissingPiece(c cid.Cid) {
	te.expectedMissingCIDs[c] = struct{}{}
}

// ExpectParams expects a given call for CheckDealParams and stubbs a response
func (te *TestProviderDealEnvironment) ExpectParams(pricePerByte abi.TokenAmount,
	paymentInterval uint64,
	paymentIntervalIncrease uint64,
	response error) {
	te.expectedParams[dealParamsKey{pricePerByte.String(), paymentInterval, paymentIntervalIncrease}] = response
}

// ExpectDeciderCalledWith expects that the deal decision logic will be run on the given deal ID
func (te *TestProviderDealEnvironment) ExpectDeciderCalledWith(dealid rm.DealID) {
	te.expectedDeciderCalls[dealid.String()] = struct{}{}
}

// VerifyExpectations checks that the expected calls were made on the TestProviderDealEnvironment
func (te *TestProviderDealEnvironment) VerifyExpectations(t *testing.T) {
	require.Equal(t, len(te.expectedParams), len(te.receivedParams))
	require.Equal(t, len(te.expectedCIDs), len(te.receivedCIDs))
	require.Equal(t, len(te.expectedMissingCIDs), len(te.receivedMissingCIDs))
	require.Equal(t, len(te.expectedDeciderCalls), len(te.receivedDeciderCalls))
}

// Node returns a provider node instance
func (te *TestProviderDealEnvironment) Node() rm.RetrievalProviderNode {
	return te.node
}

// DealStream returns a provided RetrievalDealStream instance
func (te *TestProviderDealEnvironment) DealStream(_ rm.ProviderDealIdentifier) rmnet.RetrievalDealStream {
	return te.ds
}

// GetPieceSize returns a stubbed response for a piece
func (te *TestProviderDealEnvironment) GetPieceSize(c cid.Cid, pieceCID *cid.Cid) (uint64, error) {
	pio, ok := te.expectedCIDs[c]
	if ok {
		te.receivedCIDs[c] = struct{}{}
		return pio, nil
	}
	_, ok = te.expectedMissingCIDs[c]
	if ok {
		te.receivedMissingCIDs[c] = struct{}{}
		return 0, rm.ErrNotFound
	}
	return 0, errors.New("GetPieceSize failed")
}

// CheckDealParams returns a stubbed response for the given parameters
func (te *TestProviderDealEnvironment) CheckDealParams(pricePerByte abi.TokenAmount, paymentInterval uint64, paymentIntervalIncrease uint64) error {
	key := dealParamsKey{pricePerByte.String(), paymentInterval, paymentIntervalIncrease}
	err, ok := te.expectedParams[key]
	if !ok {
		return errors.New("CheckDealParamsFailed")
	}
	te.receivedParams[key] = true
	return err
}

// NextBlock returns a series of stubbed responses
func (te *TestProviderDealEnvironment) NextBlock(_ context.Context, _ rm.ProviderDealIdentifier) (rm.Block, bool, error) {
	if te.nextResponse >= len(te.responses) {
		return rm.EmptyBlock, false, errors.New("Something went wrong")
	}
	response := te.responses[te.nextResponse]
	te.nextResponse++
	return response.Block, response.Done, response.Err
}

// RunDealDecisioningLogic simulates running deal decision logic
func (te *TestProviderDealEnvironment) RunDealDecisioningLogic(ctx context.Context, state rm.ProviderDealState) (bool, string, error) {
	te.receivedDeciderCalls[state.ID.String()] = struct{}{}
	if te.decider == nil {
		return TrivalTestDecider(ctx, state)
	}
	return te.decider(ctx, state)
}

// TrivalTestDecider is a shortest possible DealDecider that accepts all deals
var TrivalTestDecider retrievalimpl.DealDecider = func(_ context.Context, _ rm.ProviderDealState) (bool, string, error) {
	return true, "", nil
}

type dealParamsKey struct {
	pricePerByte            string
	paymentInterval         uint64
	paymentIntervalIncrease uint64
}

// ReadBlockResponse is a stubbed response to calling NextBlock
type ReadBlockResponse struct {
	Block rm.Block
	Done  bool
	Err   error
}
