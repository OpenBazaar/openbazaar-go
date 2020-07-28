package network_test

import (
	"context"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-fil-markets/shared_testutil"
)

type testReceiver struct {
	t                  *testing.T
	dealStreamHandler  func(network.RetrievalDealStream)
	queryStreamHandler func(network.RetrievalQueryStream)
}

func (tr *testReceiver) HandleDealStream(s network.RetrievalDealStream) {
	defer s.Close()
	if tr.dealStreamHandler != nil {
		tr.dealStreamHandler(s)
	}
}
func (tr *testReceiver) HandleQueryStream(s network.RetrievalQueryStream) {
	defer s.Close()
	if tr.queryStreamHandler != nil {
		tr.queryStreamHandler(s)
	}
}

func TestQueryStreamSendReceiveQuery(t *testing.T) {
	ctx := context.Background()
	td := shared_testutil.NewLibp2pTestData(ctx, t)

	fromNetwork := network.NewFromLibp2pHost(td.Host1)
	toNetwork := network.NewFromLibp2pHost(td.Host2)
	toHost := td.Host2.ID()

	// host1 gets no-op receiver
	tr := &testReceiver{t: t}
	require.NoError(t, fromNetwork.SetDelegate(tr))

	// host2 gets receiver
	qchan := make(chan retrievalmarket.Query)
	tr2 := &testReceiver{t: t, queryStreamHandler: func(s network.RetrievalQueryStream) {
		readq, err := s.ReadQuery()
		require.NoError(t, err)
		qchan <- readq
	}}
	require.NoError(t, toNetwork.SetDelegate(tr2))

	// setup query stream host1 --> host 2
	assertQueryReceived(ctx, t, fromNetwork, toHost, qchan)
}

func TestQueryStreamSendReceiveQueryResponse(t *testing.T) {
	ctx := context.Background()
	td := shared_testutil.NewLibp2pTestData(ctx, t)
	fromNetwork := network.NewFromLibp2pHost(td.Host1)
	toNetwork := network.NewFromLibp2pHost(td.Host2)
	toHost := td.Host2.ID()

	// host1 gets no-op receiver
	tr := &testReceiver{t: t}
	require.NoError(t, fromNetwork.SetDelegate(tr))

	// host2 gets receiver
	qchan := make(chan retrievalmarket.QueryResponse)
	tr2 := &testReceiver{t: t, queryStreamHandler: func(s network.RetrievalQueryStream) {
		q, err := s.ReadQueryResponse()
		require.NoError(t, err)
		qchan <- q
	}}
	require.NoError(t, toNetwork.SetDelegate(tr2))

	assertQueryResponseReceived(ctx, t, fromNetwork, toHost, qchan)

}

func TestQueryStreamSendReceiveMultipleSuccessful(t *testing.T) {
	// send query, read in handler, send response back, read response
	ctxBg := context.Background()
	td := shared_testutil.NewLibp2pTestData(ctxBg, t)
	nw1 := network.NewFromLibp2pHost(td.Host1)
	nw2 := network.NewFromLibp2pHost(td.Host2)
	require.NoError(t, td.Host1.Connect(ctxBg, peer.AddrInfo{ID: td.Host2.ID()}))

	// host2 gets a query and sends a response
	qr := shared_testutil.MakeTestQueryResponse()
	done := make(chan bool)
	tr2 := &testReceiver{t: t, queryStreamHandler: func(s network.RetrievalQueryStream) {
		_, err := s.ReadQuery()
		require.NoError(t, err)

		require.NoError(t, s.WriteQueryResponse(qr))
		done <- true
	}}
	require.NoError(t, nw2.SetDelegate(tr2))

	ctx, cancel := context.WithTimeout(ctxBg, 10*time.Second)
	defer cancel()

	qs, err := nw1.NewQueryStream(td.Host2.ID())
	require.NoError(t, err)

	testCid := shared_testutil.GenerateCids(1)[0]

	var resp retrievalmarket.QueryResponse
	go require.NoError(t, qs.WriteQuery(retrievalmarket.Query{PayloadCID: testCid}))
	resp, err = qs.ReadQueryResponse()
	require.NoError(t, err)

	select {
	case <-ctx.Done():
		t.Error("response not received")
	case <-done:
	}

	assert.Equal(t, qr, resp)
}

func TestDealStreamSendReceiveDealProposal(t *testing.T) {
	// send proposal, read in handler
	ctx := context.Background()
	td := shared_testutil.NewLibp2pTestData(ctx, t)
	fromNetwork := network.NewFromLibp2pHost(td.Host1)
	toNetwork := network.NewFromLibp2pHost(td.Host2)
	toHost := td.Host2.ID()

	tr := &testReceiver{t: t}
	require.NoError(t, fromNetwork.SetDelegate(tr))

	dchan := make(chan retrievalmarket.DealProposal)
	tr2 := &testReceiver{
		t: t,
		dealStreamHandler: func(s network.RetrievalDealStream) {
			readD, err := s.ReadDealProposal()
			require.NoError(t, err)
			dchan <- readD
		},
	}
	require.NoError(t, toNetwork.SetDelegate(tr2))

	assertDealProposalReceived(ctx, t, fromNetwork, toHost, dchan)
}

func TestDealStreamSendReceiveDealResponse(t *testing.T) {
	ctx := context.Background()
	td := shared_testutil.NewLibp2pTestData(ctx, t)
	fromNetwork := network.NewFromLibp2pHost(td.Host1)
	toNetwork := network.NewFromLibp2pHost(td.Host2)
	toPeer := td.Host2.ID()

	tr := &testReceiver{t: t}
	require.NoError(t, fromNetwork.SetDelegate(tr))

	drChan := make(chan retrievalmarket.DealResponse)
	tr2 := &testReceiver{
		t: t,
		dealStreamHandler: func(s network.RetrievalDealStream) {
			readDP, err := s.ReadDealResponse()
			require.NoError(t, err)
			drChan <- readDP
		},
	}
	require.NoError(t, toNetwork.SetDelegate(tr2))
	assertDealResponseReceived(ctx, t, fromNetwork, toPeer, drChan)
}

func TestDealStreamSendReceiveDealPayment(t *testing.T) {
	// send payment, read in handler
	ctx := context.Background()
	td := shared_testutil.NewLibp2pTestData(ctx, t)
	fromNetwork := network.NewFromLibp2pHost(td.Host1)
	toNetwork := network.NewFromLibp2pHost(td.Host2)
	toPeer := td.Host2.ID()

	tr := &testReceiver{t: t}
	require.NoError(t, fromNetwork.SetDelegate(tr))

	dpyChan := make(chan retrievalmarket.DealPayment)
	tr2 := &testReceiver{
		t: t,
		dealStreamHandler: func(s network.RetrievalDealStream) {
			readDpy, err := s.ReadDealPayment()
			require.NoError(t, err)
			dpyChan <- readDpy
		},
	}
	require.NoError(t, toNetwork.SetDelegate(tr2))
	assertDealPaymentReceived(ctx, t, fromNetwork, toPeer, dpyChan)
}

func TestDealStreamSendReceiveMultipleSuccessful(t *testing.T) {
	// send proposal, read in handler, send response back,
	// read response,
	// send payment, read farther in handler

	bgCtx := context.Background()
	td := shared_testutil.NewLibp2pTestData(bgCtx, t)
	fromNetwork := network.NewFromLibp2pHost(td.Host1)
	toNetwork := network.NewFromLibp2pHost(td.Host2)
	toPeer := td.Host2.ID()

	// set up stream handler, channels, and response
	// dpChan := make(chan retrievalmarket.DealProposal)
	dpyChan := make(chan retrievalmarket.DealPayment)
	dr := shared_testutil.MakeTestDealResponse()

	tr2 := &testReceiver{t: t, dealStreamHandler: func(s network.RetrievalDealStream) {
		_, err := s.ReadDealProposal()
		require.NoError(t, err)

		require.NoError(t, s.WriteDealResponse(dr))

		readDp, err := s.ReadDealPayment()
		require.NoError(t, err)
		dpyChan <- readDp
	}}
	require.NoError(t, toNetwork.SetDelegate(tr2))

	// start sending deal proposal
	ds1, err := fromNetwork.NewDealStream(toPeer)
	require.NoError(t, err)

	dp := shared_testutil.MakeTestDealProposal()

	var receivedPayment retrievalmarket.DealPayment

	ctx, cancel := context.WithTimeout(bgCtx, 10*time.Second)
	defer cancel()

	// write proposal
	require.NoError(t, ds1.WriteDealProposal(dp))

	// read response and verify it's the one we told toNetwork to send
	responseReceived, err := ds1.ReadDealResponse()
	require.NoError(t, err)
	assert.Equal(t, dr.ID, responseReceived.ID)
	assert.Equal(t, dr.Message, responseReceived.Message)
	assert.Equal(t, dr.Status, responseReceived.Status)

	// send payment
	dpy := retrievalmarket.DealPayment{
		ID:             dp.ID,
		PaymentChannel: address.TestAddress,
		PaymentVoucher: shared_testutil.MakeTestSignedVoucher(),
	}
	require.NoError(t, ds1.WriteDealPayment(dpy))

	select {
	case <-ctx.Done():
		t.Errorf("failed to receive messages")
	case receivedPayment = <-dpyChan:
	}

	assert.Equal(t, dpy, receivedPayment)
}

func TestLibp2pRetrievalMarketNetwork_StopHandlingRequests(t *testing.T) {
	bgCtx := context.Background()
	td := shared_testutil.NewLibp2pTestData(bgCtx, t)

	fromNetwork := network.NewFromLibp2pHost(td.Host1)
	toNetwork := network.NewFromLibp2pHost(td.Host2)
	toHost := td.Host2.ID()

	// host1 gets no-op receiver
	tr := &testReceiver{t: t}
	require.NoError(t, fromNetwork.SetDelegate(tr))

	// host2 gets receiver
	qchan := make(chan retrievalmarket.Query)
	tr2 := &testReceiver{t: t, queryStreamHandler: func(s network.RetrievalQueryStream) {
		readq, err := s.ReadQuery()
		require.NoError(t, err)
		qchan <- readq
	}}
	require.NoError(t, toNetwork.SetDelegate(tr2))

	require.NoError(t, toNetwork.StopHandlingRequests())

	_, err := fromNetwork.NewQueryStream(toHost)
	require.Error(t, err, "protocol not supported")
}

// assertDealProposalReceived performs the verification that a deal proposal is received
func assertDealProposalReceived(inCtx context.Context, t *testing.T, fromNetwork network.RetrievalMarketNetwork, toPeer peer.ID, inChan chan retrievalmarket.DealProposal) {
	ctx, cancel := context.WithTimeout(inCtx, 10*time.Second)
	defer cancel()

	qs1, err := fromNetwork.NewDealStream(toPeer)
	require.NoError(t, err)

	// send query to host2
	dp := shared_testutil.MakeTestDealProposal()
	require.NoError(t, qs1.WriteDealProposal(dp))

	var dealReceived retrievalmarket.DealProposal
	select {
	case <-ctx.Done():
		t.Error("deal proposal not received")
	case dealReceived = <-inChan:
	}
	require.NotNil(t, dealReceived)
	assert.Equal(t, dp, dealReceived)
}

func assertDealResponseReceived(parentCtx context.Context, t *testing.T, fromNetwork network.RetrievalMarketNetwork, toPeer peer.ID, inChan chan retrievalmarket.DealResponse) {
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()

	ds1, err := fromNetwork.NewDealStream(toPeer)
	require.NoError(t, err)

	fakeBlk := retrievalmarket.Block{
		Prefix: []byte("prefix"),
		Data:   []byte("data"),
	}

	dr := retrievalmarket.DealResponse{
		Status:      retrievalmarket.DealStatusCompleted,
		ID:          retrievalmarket.DealID(rand.Uint64()),
		PaymentOwed: abi.TokenAmount{Int: big.NewInt(rand.Int63())},
		Message:     "some message",
		Blocks:      []retrievalmarket.Block{fakeBlk},
	}
	require.NoError(t, ds1.WriteDealResponse(dr))

	var responseReceived retrievalmarket.DealResponse
	select {
	case <-ctx.Done():
		t.Error("response not received")
	case responseReceived = <-inChan:
	}
	require.NotNil(t, responseReceived)
	assert.Equal(t, dr, responseReceived)
}

func assertDealPaymentReceived(parentCtx context.Context, t *testing.T, fromNetwork network.RetrievalMarketNetwork, toPeer peer.ID, inChan chan retrievalmarket.DealPayment) {
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()

	ds1, err := fromNetwork.NewDealStream(toPeer)
	require.NoError(t, err)

	dp := retrievalmarket.DealPayment{
		ID:             retrievalmarket.DealID(rand.Uint64()),
		PaymentChannel: address.TestAddress,
		PaymentVoucher: shared_testutil.MakeTestSignedVoucher(),
	}
	require.NoError(t, ds1.WriteDealPayment(dp))

	var responseReceived retrievalmarket.DealPayment
	select {
	case <-ctx.Done():
		t.Error("response not received")
	case responseReceived = <-inChan:
	}
	require.NotNil(t, responseReceived)
	assert.Equal(t, dp.ID, responseReceived.ID)
	assert.Equal(t, dp.PaymentChannel, responseReceived.PaymentChannel)
	assert.Equal(t, *dp.PaymentVoucher, *responseReceived.PaymentVoucher)
}

// assertQueryReceived performs the verification that a DealStatusRequest is received
func assertQueryReceived(inCtx context.Context, t *testing.T, fromNetwork network.RetrievalMarketNetwork, toHost peer.ID, qchan chan retrievalmarket.Query) {
	ctx, cancel := context.WithTimeout(inCtx, 10*time.Second)
	defer cancel()

	qs1, err := fromNetwork.NewQueryStream(toHost)
	require.NoError(t, err)

	// send query to host2
	cid := shared_testutil.GenerateCids(1)[0]
	q := retrievalmarket.NewQueryV0(cid)
	require.NoError(t, qs1.WriteQuery(q))

	var inq retrievalmarket.Query
	select {
	case <-ctx.Done():
		t.Error("msg not received")
	case inq = <-qchan:
	}
	require.NotNil(t, inq)
	assert.Equal(t, q.PayloadCID, inq.PayloadCID)
}

// assertQueryResponseReceived performs the verification that a DealStatusResponse is received
func assertQueryResponseReceived(inCtx context.Context, t *testing.T,
	fromNetwork network.RetrievalMarketNetwork,
	toHost peer.ID,
	qchan chan retrievalmarket.QueryResponse) {
	ctx, cancel := context.WithTimeout(inCtx, 10*time.Second)
	defer cancel()

	// setup query stream host1 --> host 2
	qs1, err := fromNetwork.NewQueryStream(toHost)
	require.NoError(t, err)

	// send queryresponse to host2
	qr := shared_testutil.MakeTestQueryResponse()
	require.NoError(t, qs1.WriteQueryResponse(qr))

	// read queryresponse
	var inqr retrievalmarket.QueryResponse
	select {
	case <-ctx.Done():
		t.Error("msg not received")
	case inqr = <-qchan:
	}

	require.NotNil(t, inqr)
	assert.Equal(t, qr, inqr)
}
