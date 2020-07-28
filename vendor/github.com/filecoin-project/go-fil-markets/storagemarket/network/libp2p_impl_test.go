package network_test

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
)

type testReceiver struct {
	t                       *testing.T
	dealStreamHandler       func(network.StorageDealStream)
	askStreamHandler        func(network.StorageAskStream)
	dealStatusStreamHandler func(stream network.DealStatusStream)
}

var _ network.StorageReceiver = &testReceiver{}

func (tr *testReceiver) HandleDealStream(s network.StorageDealStream) {
	defer s.Close()
	if tr.dealStreamHandler != nil {
		tr.dealStreamHandler(s)
	}
}

func (tr *testReceiver) HandleAskStream(s network.StorageAskStream) {
	defer s.Close()
	if tr.askStreamHandler != nil {
		tr.askStreamHandler(s)
	}
}

func (tr *testReceiver) HandleDealStatusStream(s network.DealStatusStream) {
	defer s.Close()
	if tr.dealStatusStreamHandler != nil {
		tr.dealStatusStreamHandler(s)
	}
}

func TestAskStreamSendReceiveAskRequest(t *testing.T) {
	ctx := context.Background()
	td := shared_testutil.NewLibp2pTestData(ctx, t)

	fromNetwork := network.NewFromLibp2pHost(td.Host1)
	toNetwork := network.NewFromLibp2pHost(td.Host2)
	toHost := td.Host2.ID()

	// host1 gets no-op receiver
	tr := &testReceiver{t: t}
	require.NoError(t, fromNetwork.SetDelegate(tr))

	// host2 gets receiver
	achan := make(chan network.AskRequest)
	tr2 := &testReceiver{t: t, askStreamHandler: func(s network.StorageAskStream) {
		readq, err := s.ReadAskRequest()
		require.NoError(t, err)
		achan <- readq
	}}
	require.NoError(t, toNetwork.SetDelegate(tr2))

	// setup query stream host1 --> host 2
	assertAskRequestReceived(ctx, t, fromNetwork, toHost, achan)
}

func TestAskStreamSendReceiveAskResponse(t *testing.T) {
	ctx := context.Background()
	td := shared_testutil.NewLibp2pTestData(ctx, t)
	fromNetwork := network.NewFromLibp2pHost(td.Host1)
	toNetwork := network.NewFromLibp2pHost(td.Host2)
	toHost := td.Host2.ID()

	// host1 gets no-op receiver
	tr := &testReceiver{t: t}
	require.NoError(t, fromNetwork.SetDelegate(tr))

	// host2 gets receiver
	achan := make(chan network.AskResponse)
	tr2 := &testReceiver{t: t, askStreamHandler: func(s network.StorageAskStream) {
		a, err := s.ReadAskResponse()
		require.NoError(t, err)
		achan <- a
	}}
	require.NoError(t, toNetwork.SetDelegate(tr2))

	assertAskResponseReceived(ctx, t, fromNetwork, toHost, achan)

}

func TestAskStreamSendReceiveMultipleSuccessful(t *testing.T) {
	// send query, read in handler, send response back, read response
	ctxBg := context.Background()
	td := shared_testutil.NewLibp2pTestData(ctxBg, t)
	nw1 := network.NewFromLibp2pHost(td.Host1)
	nw2 := network.NewFromLibp2pHost(td.Host2)
	require.NoError(t, td.Host1.Connect(ctxBg, peer.AddrInfo{ID: td.Host2.ID()}))

	// host2 gets a query and sends a response
	ar := shared_testutil.MakeTestStorageAskResponse()
	done := make(chan bool)
	tr2 := &testReceiver{t: t, askStreamHandler: func(s network.StorageAskStream) {
		_, err := s.ReadAskRequest()
		require.NoError(t, err)

		require.NoError(t, s.WriteAskResponse(ar))
		done <- true
	}}
	require.NoError(t, nw2.SetDelegate(tr2))

	ctx, cancel := context.WithTimeout(ctxBg, 10*time.Second)
	defer cancel()

	qs, err := nw1.NewAskStream(ctx, td.Host2.ID())
	require.NoError(t, err)

	var resp network.AskResponse
	go require.NoError(t, qs.WriteAskRequest(shared_testutil.MakeTestStorageAskRequest()))
	resp, err = qs.ReadAskResponse()
	require.NoError(t, err)

	select {
	case <-ctx.Done():
		t.Error("response not received")
	case <-done:
	}

	assert.Equal(t, ar, resp)
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

	dchan := make(chan network.Proposal)
	tr2 := &testReceiver{
		t: t,
		dealStreamHandler: func(s network.StorageDealStream) {
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

	drChan := make(chan network.SignedResponse)
	tr2 := &testReceiver{
		t: t,
		dealStreamHandler: func(s network.StorageDealStream) {
			readDP, err := s.ReadDealResponse()
			require.NoError(t, err)
			drChan <- readDP
		},
	}
	require.NoError(t, toNetwork.SetDelegate(tr2))
	assertDealResponseReceived(ctx, t, fromNetwork, toPeer, drChan)
}

func TestDealStreamSendReceiveMultipleSuccessful(t *testing.T) {
	// send proposal, read in handler, send response back,
	// read response,

	bgCtx := context.Background()
	td := shared_testutil.NewLibp2pTestData(bgCtx, t)
	fromNetwork := network.NewFromLibp2pHost(td.Host1)
	toNetwork := network.NewFromLibp2pHost(td.Host2)
	toPeer := td.Host2.ID()

	// set up stream handler, channels, and response
	dr := shared_testutil.MakeTestStorageNetworkSignedResponse()
	done := make(chan bool)

	tr2 := &testReceiver{t: t, dealStreamHandler: func(s network.StorageDealStream) {
		_, err := s.ReadDealProposal()
		require.NoError(t, err)

		require.NoError(t, s.WriteDealResponse(dr))
		done <- true
	}}
	require.NoError(t, toNetwork.SetDelegate(tr2))

	ctx, cancel := context.WithTimeout(bgCtx, 10*time.Second)
	defer cancel()

	// start sending deal proposal
	ds1, err := fromNetwork.NewDealStream(ctx, toPeer)
	require.NoError(t, err)

	dp := shared_testutil.MakeTestStorageNetworkProposal()

	// write proposal
	require.NoError(t, ds1.WriteDealProposal(dp))

	// read response and verify it's the one we told toNetwork to send
	responseReceived, err := ds1.ReadDealResponse()
	require.NoError(t, err)
	assert.Equal(t, dr, responseReceived)

	select {
	case <-ctx.Done():
		t.Errorf("failed to receive messages")
	case <-done:
	}
}

func TestDealStatusStreamSendReceiveRequest(t *testing.T) {
	ctx := context.Background()
	td := shared_testutil.NewLibp2pTestData(ctx, t)

	fromNetwork := network.NewFromLibp2pHost(td.Host1)
	toNetwork := network.NewFromLibp2pHost(td.Host2)
	toHost := td.Host2.ID()

	// host1 gets no-op receiver
	tr := &testReceiver{t: t}
	require.NoError(t, fromNetwork.SetDelegate(tr))

	// host2 gets receiver
	achan := make(chan network.DealStatusRequest)
	tr2 := &testReceiver{t: t, dealStatusStreamHandler: func(s network.DealStatusStream) {
		readq, err := s.ReadDealStatusRequest()
		require.NoError(t, err)
		achan <- readq
	}}
	require.NoError(t, toNetwork.SetDelegate(tr2))

	// setup query stream host1 --> host 2
	assertDealStatusRequestReceived(ctx, t, fromNetwork, toHost, achan)
}

func TestDealStatusStreamSendReceiveResponse(t *testing.T) {
	ctx := context.Background()
	td := shared_testutil.NewLibp2pTestData(ctx, t)
	fromNetwork := network.NewFromLibp2pHost(td.Host1)
	toNetwork := network.NewFromLibp2pHost(td.Host2)
	toHost := td.Host2.ID()

	// host1 gets no-op receiver
	tr := &testReceiver{t: t}
	require.NoError(t, fromNetwork.SetDelegate(tr))

	// host2 gets receiver
	achan := make(chan network.DealStatusResponse)
	tr2 := &testReceiver{t: t, dealStatusStreamHandler: func(s network.DealStatusStream) {
		a, err := s.ReadDealStatusResponse()
		require.NoError(t, err)
		achan <- a
	}}
	require.NoError(t, toNetwork.SetDelegate(tr2))

	assertDealStatusResponseReceived(ctx, t, fromNetwork, toHost, achan)
}

func TestDealStatusStreamSendReceiveMultipleSuccessful(t *testing.T) {
	// send query, read in handler, send response back, read response
	ctxBg := context.Background()
	td := shared_testutil.NewLibp2pTestData(ctxBg, t)
	nw1 := network.NewFromLibp2pHost(td.Host1)
	nw2 := network.NewFromLibp2pHost(td.Host2)
	require.NoError(t, td.Host1.Connect(ctxBg, peer.AddrInfo{ID: td.Host2.ID()}))

	// host2 gets a query and sends a response
	ar := shared_testutil.MakeTestDealStatusResponse()
	done := make(chan bool)
	tr2 := &testReceiver{t: t, dealStatusStreamHandler: func(s network.DealStatusStream) {
		_, err := s.ReadDealStatusRequest()
		require.NoError(t, err)

		require.NoError(t, s.WriteDealStatusResponse(ar))
		done <- true
	}}
	require.NoError(t, nw2.SetDelegate(tr2))

	ctx, cancel := context.WithTimeout(ctxBg, 10*time.Second)
	defer cancel()

	qs, err := nw1.NewDealStatusStream(ctx, td.Host2.ID())
	require.NoError(t, err)

	var resp network.DealStatusResponse
	go require.NoError(t, qs.WriteDealStatusRequest(shared_testutil.MakeTestDealStatusRequest()))
	resp, err = qs.ReadDealStatusResponse()
	require.NoError(t, err)

	select {
	case <-ctx.Done():
		t.Error("response not received")
	case <-done:
	}

	assert.Equal(t, ar, resp)
}

func TestLibp2pStorageMarketNetwork_StopHandlingRequests(t *testing.T) {
	bgCtx := context.Background()
	td := shared_testutil.NewLibp2pTestData(bgCtx, t)

	fromNetwork := network.NewFromLibp2pHost(td.Host1)
	toNetwork := network.NewFromLibp2pHost(td.Host2)
	toHost := td.Host2.ID()

	// host1 gets no-op receiver
	tr := &testReceiver{t: t}
	require.NoError(t, fromNetwork.SetDelegate(tr))

	// host2 gets receiver
	achan := make(chan network.AskRequest)
	tr2 := &testReceiver{t: t, askStreamHandler: func(s network.StorageAskStream) {
		readar, err := s.ReadAskRequest()
		require.NoError(t, err)
		achan <- readar
	}}
	require.NoError(t, toNetwork.SetDelegate(tr2))

	require.NoError(t, toNetwork.StopHandlingRequests())

	_, err := fromNetwork.NewAskStream(bgCtx, toHost)
	require.Error(t, err, "protocol not supported")
}

// assertDealProposalReceived performs the verification that a deal proposal is received
func assertDealProposalReceived(inCtx context.Context, t *testing.T, fromNetwork network.StorageMarketNetwork, toPeer peer.ID, inChan chan network.Proposal) {
	ctx, cancel := context.WithTimeout(inCtx, 10*time.Second)
	defer cancel()

	qs1, err := fromNetwork.NewDealStream(ctx, toPeer)
	require.NoError(t, err)

	// send query to host2
	dp := shared_testutil.MakeTestStorageNetworkProposal()
	require.NoError(t, qs1.WriteDealProposal(dp))

	var dealReceived network.Proposal
	select {
	case <-ctx.Done():
		t.Error("deal proposal not received")
	case dealReceived = <-inChan:
	}
	require.NotNil(t, dealReceived)
	assert.Equal(t, dp, dealReceived)
}

func assertDealResponseReceived(parentCtx context.Context, t *testing.T, fromNetwork network.StorageMarketNetwork, toPeer peer.ID, inChan chan network.SignedResponse) {
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()

	ds1, err := fromNetwork.NewDealStream(ctx, toPeer)
	require.NoError(t, err)

	dr := shared_testutil.MakeTestStorageNetworkSignedResponse()
	require.NoError(t, ds1.WriteDealResponse(dr))

	var responseReceived network.SignedResponse
	select {
	case <-ctx.Done():
		t.Error("response not received")
	case responseReceived = <-inChan:
	}
	require.NotNil(t, responseReceived)
	assert.Equal(t, dr, responseReceived)
}

// assertAskRequestReceived performs the verification that a AskRequest is received
func assertAskRequestReceived(inCtx context.Context, t *testing.T, fromNetwork network.StorageMarketNetwork, toHost peer.ID, achan chan network.AskRequest) {
	ctx, cancel := context.WithTimeout(inCtx, 10*time.Second)
	defer cancel()

	as1, err := fromNetwork.NewAskStream(ctx, toHost)
	require.NoError(t, err)

	// send query to host2
	a := shared_testutil.MakeTestStorageAskRequest()
	require.NoError(t, as1.WriteAskRequest(a))

	var ina network.AskRequest
	select {
	case <-ctx.Done():
		t.Error("msg not received")
	case ina = <-achan:
	}
	require.NotNil(t, ina)
	assert.Equal(t, a.Miner, ina.Miner)
}

// assertAskResponseReceived performs the verification that a AskResponse is received
func assertAskResponseReceived(inCtx context.Context, t *testing.T,
	fromNetwork network.StorageMarketNetwork,
	toHost peer.ID,
	achan chan network.AskResponse) {
	ctx, cancel := context.WithTimeout(inCtx, 10*time.Second)
	defer cancel()

	// setup query stream host1 --> host 2
	as1, err := fromNetwork.NewAskStream(ctx, toHost)
	require.NoError(t, err)

	// send queryresponse to host2
	ar := shared_testutil.MakeTestStorageAskResponse()
	require.NoError(t, as1.WriteAskResponse(ar))

	// read queryresponse
	var inar network.AskResponse
	select {
	case <-ctx.Done():
		t.Error("msg not received")
	case inar = <-achan:
	}

	require.NotNil(t, inar)
	assert.Equal(t, ar, inar)
}

// assertDealStatusRequestReceived performs the verification that a DealStatusRequest is received
func assertDealStatusRequestReceived(inCtx context.Context, t *testing.T, fromNetwork network.StorageMarketNetwork, toHost peer.ID, achan chan network.DealStatusRequest) {
	ctx, cancel := context.WithTimeout(inCtx, 10*time.Second)
	defer cancel()

	as1, err := fromNetwork.NewDealStatusStream(ctx, toHost)
	require.NoError(t, err)

	// send query to host2
	a := shared_testutil.MakeTestDealStatusRequest()
	require.NoError(t, as1.WriteDealStatusRequest(a))

	var ina network.DealStatusRequest
	select {
	case <-ctx.Done():
		t.Error("msg not received")
	case ina = <-achan:
	}
	require.NotNil(t, ina)
	assert.Equal(t, a, ina)
}

// assertDealStatusResponseReceived performs the verification that a QueryResponse is received
func assertDealStatusResponseReceived(inCtx context.Context, t *testing.T,
	fromNetwork network.StorageMarketNetwork,
	toHost peer.ID,
	achan chan network.DealStatusResponse) {
	ctx, cancel := context.WithTimeout(inCtx, 10*time.Second)
	defer cancel()

	// setup query stream host1 --> host 2
	as1, err := fromNetwork.NewDealStatusStream(ctx, toHost)
	require.NoError(t, err)

	// send queryresponse to host2
	ar := shared_testutil.MakeTestDealStatusResponse()
	require.NoError(t, as1.WriteDealStatusResponse(ar))

	// read queryresponse
	var inar network.DealStatusResponse
	select {
	case <-ctx.Done():
		t.Error("msg not received")
	case inar = <-achan:
	}

	require.NotNil(t, inar)
	assert.Equal(t, ar, inar)
}
