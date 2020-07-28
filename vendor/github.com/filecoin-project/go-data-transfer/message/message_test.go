package message_test

import (
	"bytes"
	"math/rand"
	"testing"

	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/traversal/selector/builder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	. "github.com/filecoin-project/go-data-transfer/message"
	"github.com/filecoin-project/go-data-transfer/testutil"
)

func TestNewRequest(t *testing.T) {
	baseCid := testutil.GenerateCids(1)[0]
	selector := builder.NewSelectorSpecBuilder(basicnode.Style.Any).Matcher().Node()
	isPull := true
	id := datatransfer.TransferID(rand.Int31())
	voucher := testutil.NewFakeDTType()
	request, err := NewRequest(id, isPull, voucher.Type(), voucher, baseCid, selector)
	require.NoError(t, err)
	assert.Equal(t, id, request.TransferID())
	assert.False(t, request.IsCancel())
	assert.True(t, request.IsPull())
	assert.True(t, request.IsRequest())
	assert.Equal(t, baseCid.String(), request.BaseCid().String())
	testutil.AssertFakeDTVoucher(t, request, voucher)
	receivedSelector, err := request.Selector()
	require.NoError(t, err)
	require.Equal(t, selector, receivedSelector)
	// Sanity check to make sure we can cast to DataTransferMessage
	msg, ok := request.(DataTransferMessage)
	require.True(t, ok)

	assert.True(t, msg.IsRequest())
	assert.Equal(t, request.TransferID(), msg.TransferID())
}
func TestTransferRequest_MarshalCBOR(t *testing.T) {
	// sanity check MarshalCBOR does its thing w/o error
	req, err := NewTestTransferRequest()
	require.NoError(t, err)
	wbuf := new(bytes.Buffer)
	require.NoError(t, req.MarshalCBOR(wbuf))
	assert.Greater(t, wbuf.Len(), 0)
}
func TestTransferRequest_UnmarshalCBOR(t *testing.T) {
	req, err := NewTestTransferRequest()
	require.NoError(t, err)
	wbuf := new(bytes.Buffer)
	// use ToNet / FromNet
	require.NoError(t, req.ToNet(wbuf))

	desMsg, err := FromNet(wbuf)
	require.NoError(t, err)

	// Verify round-trip
	assert.Equal(t, req.TransferID(), desMsg.TransferID())
	assert.Equal(t, req.IsRequest(), desMsg.IsRequest())

	desReq := desMsg.(DataTransferRequest)
	assert.Equal(t, req.IsPull(), desReq.IsPull())
	assert.Equal(t, req.IsCancel(), desReq.IsCancel())
	assert.Equal(t, req.BaseCid(), desReq.BaseCid())
	testutil.AssertEqualFakeDTVoucher(t, req, desReq)
	testutil.AssertEqualSelector(t, req, desReq)
}

func TestResponses(t *testing.T) {
	id := datatransfer.TransferID(rand.Int31())
	response := NewResponse(id, false) // not accepted
	assert.Equal(t, response.TransferID(), id)
	assert.False(t, response.Accepted())
	assert.False(t, response.IsRequest())

	// Sanity check to make sure we can cast to DataTransferMessage
	msg, ok := response.(DataTransferMessage)
	require.True(t, ok)

	assert.False(t, msg.IsRequest())
	assert.Equal(t, response.TransferID(), msg.TransferID())
}

func TestTransferResponse_MarshalCBOR(t *testing.T) {
	id := datatransfer.TransferID(rand.Int31())
	response := NewResponse(id, true) // accepted

	// sanity check that we can marshal data
	wbuf := new(bytes.Buffer)
	require.NoError(t, response.ToNet(wbuf))
	assert.Greater(t, wbuf.Len(), 0)
}

func TestTransferResponse_UnmarshalCBOR(t *testing.T) {
	id := datatransfer.TransferID(rand.Int31())
	response := NewResponse(id, true) // accepted

	wbuf := new(bytes.Buffer)
	require.NoError(t, response.ToNet(wbuf))

	// verify round trip
	desMsg, err := FromNet(wbuf)
	require.NoError(t, err)
	assert.False(t, desMsg.IsRequest())
	assert.Equal(t, id, desMsg.TransferID())

	desResp, ok := desMsg.(DataTransferResponse)
	require.True(t, ok)
	assert.True(t, desResp.Accepted())
}

func TestRequestCancel(t *testing.T) {
	id := datatransfer.TransferID(rand.Int31())
	req := CancelRequest(id)
	require.Equal(t, req.TransferID(), id)
	require.True(t, req.IsRequest())
	require.True(t, req.IsCancel())

	wbuf := new(bytes.Buffer)
	require.NoError(t, req.ToNet(wbuf))

	deserialized, err := FromNet(wbuf)
	require.NoError(t, err)

	deserializedRequest, ok := deserialized.(DataTransferRequest)
	require.True(t, ok)
	require.Equal(t, deserializedRequest.TransferID(), req.TransferID())
	require.Equal(t, deserializedRequest.IsCancel(), req.IsCancel())
	require.Equal(t, deserializedRequest.IsRequest(), req.IsRequest())
}

func TestToNetFromNetEquivalency(t *testing.T) {
	baseCid := testutil.GenerateCids(1)[0]
	selector := builder.NewSelectorSpecBuilder(basicnode.Style.Any).Matcher().Node()
	isPull := false
	id := datatransfer.TransferID(rand.Int31())
	accepted := false
	voucher := testutil.NewFakeDTType()
	request, err := NewRequest(id, isPull, voucher.Type(), voucher, baseCid, selector)
	require.NoError(t, err)
	buf := new(bytes.Buffer)
	err = request.ToNet(buf)
	require.NoError(t, err)
	require.Greater(t, buf.Len(), 0)
	deserialized, err := FromNet(buf)
	require.NoError(t, err)

	deserializedRequest, ok := deserialized.(DataTransferRequest)
	require.True(t, ok)

	require.Equal(t, deserializedRequest.TransferID(), request.TransferID())
	require.Equal(t, deserializedRequest.IsCancel(), request.IsCancel())
	require.Equal(t, deserializedRequest.IsPull(), request.IsPull())
	require.Equal(t, deserializedRequest.IsRequest(), request.IsRequest())
	require.Equal(t, deserializedRequest.BaseCid(), request.BaseCid())
	testutil.AssertEqualFakeDTVoucher(t, request, deserializedRequest)
	testutil.AssertEqualSelector(t, request, deserializedRequest)

	response := NewResponse(id, accepted)
	err = response.ToNet(buf)
	require.NoError(t, err)
	deserialized, err = FromNet(buf)
	require.NoError(t, err)

	deserializedResponse, ok := deserialized.(DataTransferResponse)
	require.True(t, ok)

	require.Equal(t, deserializedResponse.TransferID(), response.TransferID())
	require.Equal(t, deserializedResponse.Accepted(), response.Accepted())
	require.Equal(t, deserializedResponse.IsRequest(), response.IsRequest())

	request = CancelRequest(id)
	err = request.ToNet(buf)
	require.NoError(t, err)
	deserialized, err = FromNet(buf)
	require.NoError(t, err)

	deserializedRequest, ok = deserialized.(DataTransferRequest)
	require.True(t, ok)

	require.Equal(t, deserializedRequest.TransferID(), request.TransferID())
	require.Equal(t, deserializedRequest.IsCancel(), request.IsCancel())
	require.Equal(t, deserializedRequest.IsRequest(), request.IsRequest())
}

func NewTestTransferRequest() (DataTransferRequest, error) {
	bcid := testutil.GenerateCids(1)[0]
	selector := builder.NewSelectorSpecBuilder(basicnode.Style.Any).Matcher().Node()
	isPull := false
	id := datatransfer.TransferID(rand.Int31())
	voucher := testutil.NewFakeDTType()
	return NewRequest(id, isPull, voucher.Type(), voucher, bcid, selector)
}
