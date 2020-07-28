package testnodes

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"io/ioutil"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/shared"
)

type expectedVoucherKey struct {
	paymentChannel string
	voucher        string
	proof          string
	expectedAmount string
}

type sectorKey struct {
	sectorID uint64
	offset   uint64
	length   uint64
}

type voucherResult struct {
	amount abi.TokenAmount
	err    error
}

// TestRetrievalProviderNode is a node adapter for a retrieval provider whose
// responses are mocked
type TestRetrievalProviderNode struct {
	sectorStubs      map[sectorKey][]byte
	expectations     map[sectorKey]struct{}
	received         map[sectorKey]struct{}
	expectedVouchers map[expectedVoucherKey]voucherResult
	receivedVouchers map[expectedVoucherKey]struct{}
}

var _ retrievalmarket.RetrievalProviderNode = &TestRetrievalProviderNode{}

// NewTestRetrievalProviderNode instantiates a new TestRetrievalProviderNode
func NewTestRetrievalProviderNode() *TestRetrievalProviderNode {
	return &TestRetrievalProviderNode{
		sectorStubs:      make(map[sectorKey][]byte),
		expectations:     make(map[sectorKey]struct{}),
		received:         make(map[sectorKey]struct{}),
		expectedVouchers: make(map[expectedVoucherKey]voucherResult),
		receivedVouchers: make(map[expectedVoucherKey]struct{}),
	}
}

// StubUnseal stubs a response to attempting to unseal a sector with the given paramters
func (trpn *TestRetrievalProviderNode) StubUnseal(sectorID uint64, offset uint64, length uint64, data []byte) {
	trpn.sectorStubs[sectorKey{sectorID, offset, length}] = data
}

// ExpectFailedUnseal indicates an expectation that a call will be made to unseal
// a sector with the given params and should fail
func (trpn *TestRetrievalProviderNode) ExpectFailedUnseal(sectorID uint64, offset uint64, length uint64) {
	trpn.expectations[sectorKey{sectorID, offset, length}] = struct{}{}
}

// ExpectUnseal indicates an expectation that a call will be made to unseal
// a sector with the given params and should return the given data
func (trpn *TestRetrievalProviderNode) ExpectUnseal(sectorID uint64, offset uint64, length uint64, data []byte) {
	trpn.expectations[sectorKey{sectorID, offset, length}] = struct{}{}
	trpn.StubUnseal(sectorID, offset, length, data)
}

// UnsealSector simulates unsealing a sector by returning a stubbed response
// or erroring
func (trpn *TestRetrievalProviderNode) UnsealSector(ctx context.Context, sectorID uint64, offset uint64, length uint64) (io.ReadCloser, error) {
	trpn.received[sectorKey{sectorID, offset, length}] = struct{}{}
	data, ok := trpn.sectorStubs[sectorKey{sectorID, offset, length}]
	if !ok {
		return nil, errors.New("Could not unseal")
	}
	return ioutil.NopCloser(bytes.NewReader(data)), nil
}

// VerifyExpectations verifies that all expected calls were made and no other calls
// were made
func (trpn *TestRetrievalProviderNode) VerifyExpectations(t *testing.T) {
	require.Equal(t, len(trpn.expectedVouchers), len(trpn.receivedVouchers))
	require.Equal(t, trpn.expectations, trpn.received)
}

// SavePaymentVoucher simulates saving a payment voucher with a stubbed result
func (trpn *TestRetrievalProviderNode) SavePaymentVoucher(
	ctx context.Context,
	paymentChannel address.Address,
	voucher *paych.SignedVoucher,
	proof []byte,
	expectedAmount abi.TokenAmount,
	tok shared.TipSetToken) (abi.TokenAmount, error) {
	key, err := trpn.toExpectedVoucherKey(paymentChannel, voucher, proof, expectedAmount)
	if err != nil {
		return abi.TokenAmount{}, err
	}
	result, ok := trpn.expectedVouchers[key]
	if ok {
		trpn.receivedVouchers[key] = struct{}{}
		return result.amount, result.err
	}
	return abi.TokenAmount{}, errors.New("SavePaymentVoucher failed")
}

// GetMinerWorkerAddress translates an address
func (trpn *TestRetrievalProviderNode) GetMinerWorkerAddress(ctx context.Context, addr address.Address, tok shared.TipSetToken) (address.Address, error) {
	return addr, nil
}

// GetChainHead returns a mock value for the chain head
func (trpn *TestRetrievalProviderNode) GetChainHead(ctx context.Context) (shared.TipSetToken, abi.ChainEpoch, error) {
	return []byte{42}, 0, nil
}

// --- Non-interface Functions

// to ExpectedVoucherKey creates a lookup key for expected vouchers.
func (trpn *TestRetrievalProviderNode) toExpectedVoucherKey(paymentChannel address.Address, voucher *paych.SignedVoucher, proof []byte, expectedAmount abi.TokenAmount) (expectedVoucherKey, error) {
	pcString := paymentChannel.String()
	buf := new(bytes.Buffer)
	if err := voucher.MarshalCBOR(buf); err != nil {
		return expectedVoucherKey{}, err
	}
	voucherString := base64.RawURLEncoding.EncodeToString(buf.Bytes())
	proofString := string(proof)
	expectedAmountString := expectedAmount.String()
	return expectedVoucherKey{pcString, voucherString, proofString, expectedAmountString}, nil
}

// ExpectVoucher sets a voucher to be expected by SavePaymentVoucher
//     paymentChannel: the address of the payment channel the client creates
//     voucher: the voucher to match
//     proof: the proof to use (can be blank)
// 	   expectedAmount: the expected tokenamount for this voucher
//     actualAmount: the actual amount to use.  use same as expectedAmount unless you want to trigger an error
//     expectedErr:  an error message to expect
func (trpn *TestRetrievalProviderNode) ExpectVoucher(
	paymentChannel address.Address,
	voucher *paych.SignedVoucher,
	proof []byte,
	expectedAmount abi.TokenAmount,
	actualAmount abi.TokenAmount, // the actual amount it should have (same unless you want to trigger an error)
	expectedErr error) error {
	key, err := trpn.toExpectedVoucherKey(paymentChannel, voucher, proof, expectedAmount)
	if err != nil {
		return err
	}
	trpn.expectedVouchers[key] = voucherResult{actualAmount, expectedErr}
	return nil
}
