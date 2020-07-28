package retrievalimpl_test

import (
	"context"
	"errors"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	retrievalimpl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/testnodes"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	tut "github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/go-storedcounter"
)

func TestClient_Query(t *testing.T) {
	ctx := context.Background()

	ds := dss.MutexWrap(datastore.NewMapDatastore())
	storedCounter := storedcounter.New(ds, datastore.NewKey("nextDealID"))
	bs := bstore.NewBlockstore(ds)

	pcid := tut.GenerateCids(1)[0]
	expectedPeer := peer.ID("somevalue")
	rpeer := retrievalmarket.RetrievalPeer{
		Address: address.TestAddress2,
		ID:      expectedPeer,
	}

	expectedQuery := retrievalmarket.Query{
		PayloadCID: pcid,
	}

	expectedQueryResponse := retrievalmarket.QueryResponse{
		Status:                     retrievalmarket.QueryResponseAvailable,
		Size:                       1234,
		PaymentAddress:             address.TestAddress,
		MinPricePerByte:            abi.NewTokenAmount(5678),
		MaxPaymentInterval:         4321,
		MaxPaymentIntervalIncrease: 0,
	}

	t.Run("it works", func(t *testing.T) {
		var qsb tut.QueryStreamBuilder = func(p peer.ID) (rmnet.RetrievalQueryStream, error) {
			return tut.NewTestRetrievalQueryStream(tut.TestQueryStreamParams{
				Writer:     tut.ExpectQueryWriter(t, expectedQuery, "queries should match"),
				RespReader: tut.StubbedQueryResponseReader(expectedQueryResponse),
			}), nil
		}
		net := tut.NewTestRetrievalMarketNetwork(tut.TestNetworkParams{
			QueryStreamBuilder: tut.ExpectPeerOnQueryStreamBuilder(t, expectedPeer, qsb, "Peers should match"),
		})
		c, err := retrievalimpl.NewClient(
			net,
			bs,
			testnodes.NewTestRetrievalClientNode(testnodes.TestRetrievalClientNodeParams{}),
			&tut.TestPeerResolver{},
			ds,
			storedCounter)
		require.NoError(t, err)

		resp, err := c.Query(ctx, rpeer, pcid, retrievalmarket.QueryParams{})
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, expectedQueryResponse, resp)
	})

	t.Run("when the stream returns error, returns error", func(t *testing.T) {
		net := tut.NewTestRetrievalMarketNetwork(tut.TestNetworkParams{
			QueryStreamBuilder: tut.FailNewQueryStream,
		})
		c, err := retrievalimpl.NewClient(
			net,
			bs,
			testnodes.NewTestRetrievalClientNode(testnodes.TestRetrievalClientNodeParams{}),
			&tut.TestPeerResolver{},
			ds,
			storedCounter)
		require.NoError(t, err)

		_, err = c.Query(ctx, rpeer, pcid, retrievalmarket.QueryParams{})
		assert.EqualError(t, err, "new query stream failed")
	})

	t.Run("when WriteDealStatusRequest fails, returns error", func(t *testing.T) {

		qsbuilder := func(p peer.ID) (network.RetrievalQueryStream, error) {
			newStream := tut.NewTestRetrievalQueryStream(tut.TestQueryStreamParams{
				PeerID: p,
				Writer: tut.FailQueryWriter,
			})
			return newStream, nil
		}

		net := tut.NewTestRetrievalMarketNetwork(tut.TestNetworkParams{
			QueryStreamBuilder: qsbuilder,
		})
		c, err := retrievalimpl.NewClient(
			net,
			bs,
			testnodes.NewTestRetrievalClientNode(testnodes.TestRetrievalClientNodeParams{}),
			&tut.TestPeerResolver{},
			ds,
			storedCounter)
		require.NoError(t, err)

		statusCode, err := c.Query(ctx, rpeer, pcid, retrievalmarket.QueryParams{})
		assert.EqualError(t, err, "write query failed")
		assert.Equal(t, retrievalmarket.QueryResponseUndefined, statusCode)
	})

	t.Run("when ReadDealStatusResponse fails, returns error", func(t *testing.T) {
		qsbuilder := func(p peer.ID) (network.RetrievalQueryStream, error) {
			newStream := tut.NewTestRetrievalQueryStream(tut.TestQueryStreamParams{
				PeerID:     p,
				RespReader: tut.FailResponseReader,
			})
			return newStream, nil
		}
		net := tut.NewTestRetrievalMarketNetwork(tut.TestNetworkParams{
			QueryStreamBuilder: qsbuilder,
		})
		c, err := retrievalimpl.NewClient(
			net,
			bs,
			testnodes.NewTestRetrievalClientNode(testnodes.TestRetrievalClientNodeParams{}),
			&tut.TestPeerResolver{},
			ds,
			storedCounter)
		require.NoError(t, err)

		statusCode, err := c.Query(ctx, rpeer, pcid, retrievalmarket.QueryParams{})
		assert.EqualError(t, err, "query response failed")
		assert.Equal(t, retrievalmarket.QueryResponseUndefined, statusCode)
	})
}

func TestClient_FindProviders(t *testing.T) {
	ds := dss.MutexWrap(datastore.NewMapDatastore())
	storedCounter := storedcounter.New(ds, datastore.NewKey("nextDealID"))
	bs := bstore.NewBlockstore(ds)
	expectedPeer := peer.ID("somevalue")

	var qsb tut.QueryStreamBuilder = func(p peer.ID) (rmnet.RetrievalQueryStream, error) {
		return tut.NewTestRetrievalQueryStream(tut.TestQueryStreamParams{
			Writer:     tut.TrivialQueryWriter,
			RespReader: tut.TrivialQueryResponseReader,
		}), nil
	}
	net := tut.NewTestRetrievalMarketNetwork(tut.TestNetworkParams{
		QueryStreamBuilder: tut.ExpectPeerOnQueryStreamBuilder(t, expectedPeer, qsb, "Peers should match"),
	})

	t.Run("when providers are found, returns providers", func(t *testing.T) {
		peers := tut.RequireGenerateRetrievalPeers(t, 3)
		testResolver := tut.TestPeerResolver{Peers: peers}

		c, err := retrievalimpl.NewClient(net, bs, &testnodes.TestRetrievalClientNode{}, &testResolver, ds, storedCounter)
		require.NoError(t, err)

		testCid := tut.GenerateCids(1)[0]
		assert.Len(t, c.FindProviders(testCid), 3)
	})

	t.Run("when there is an error, returns empty provider list", func(t *testing.T) {
		testResolver := tut.TestPeerResolver{Peers: []retrievalmarket.RetrievalPeer{}, ResolverError: errors.New("boom")}
		c, err := retrievalimpl.NewClient(net, bs, &testnodes.TestRetrievalClientNode{}, &testResolver, ds, storedCounter)
		require.NoError(t, err)

		badCid := tut.GenerateCids(1)[0]
		assert.Len(t, c.FindProviders(badCid), 0)
	})

	t.Run("when there are no providers", func(t *testing.T) {
		testResolver := tut.TestPeerResolver{Peers: []retrievalmarket.RetrievalPeer{}}
		c, err := retrievalimpl.NewClient(net, bs, &testnodes.TestRetrievalClientNode{}, &testResolver, ds, storedCounter)
		require.NoError(t, err)

		testCid := tut.GenerateCids(1)[0]
		assert.Len(t, c.FindProviders(testCid), 0)
	})
}
