package retrievalimpl

import (
	"context"
	"reflect"
	"sync"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-statemachine/fsm"
	"github.com/filecoin-project/specs-actors/actors/abi"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log/v2"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/blockio"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/clientstates"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-fil-markets/shared"

	"github.com/filecoin-project/go-storedcounter"
)

var log = logging.Logger("retrieval")

// Client is the production implementation of the RetrievalClient interface
type Client struct {
	network       rmnet.RetrievalMarketNetwork
	bs            blockstore.Blockstore
	node          retrievalmarket.RetrievalClientNode
	storedCounter *storedcounter.StoredCounter

	subscribersLk  sync.RWMutex
	subscribers    []retrievalmarket.ClientSubscriber
	resolver       retrievalmarket.PeerResolver
	blockVerifiers map[retrievalmarket.DealID]blockio.BlockVerifier
	dealStreams    map[retrievalmarket.DealID]rmnet.RetrievalDealStream
	stateMachines  fsm.Group
}

var _ retrievalmarket.RetrievalClient = &Client{}

// NewClient creates a new retrieval client
func NewClient(
	network rmnet.RetrievalMarketNetwork,
	bs blockstore.Blockstore,
	node retrievalmarket.RetrievalClientNode,
	resolver retrievalmarket.PeerResolver,
	ds datastore.Batching,
	storedCounter *storedcounter.StoredCounter,
) (retrievalmarket.RetrievalClient, error) {
	c := &Client{
		network:        network,
		bs:             bs,
		node:           node,
		resolver:       resolver,
		storedCounter:  storedCounter,
		dealStreams:    make(map[retrievalmarket.DealID]rmnet.RetrievalDealStream),
		blockVerifiers: make(map[retrievalmarket.DealID]blockio.BlockVerifier),
	}
	stateMachines, err := fsm.New(ds, fsm.Parameters{
		Environment:     &clientDealEnvironment{c},
		StateType:       retrievalmarket.ClientDealState{},
		StateKeyField:   "Status",
		Events:          clientstates.ClientEvents,
		StateEntryFuncs: clientstates.ClientStateEntryFuncs,
		Notifier:        c.notifySubscribers,
	})
	if err != nil {
		return nil, err
	}
	c.stateMachines = stateMachines
	return c, nil
}

// V0

// FindProviders uses PeerResolver interface to locate a list of providers who may have a given payload CID.
func (c *Client) FindProviders(payloadCID cid.Cid) []retrievalmarket.RetrievalPeer {
	peers, err := c.resolver.GetPeers(payloadCID)
	if err != nil {
		log.Errorf("failed to get peers: %s", err)
		return []retrievalmarket.RetrievalPeer{}
	}
	return peers
}

/*
Query sends a retrieval query to a specific retrieval provider, to determine
if the provider can serve a retrieval request and what its specific parameters for
the request are.

The client a new `RetrievalQueryStream` for the chosen peer ID,
and calls WriteQuery on it, which constructs a data-transfer message and writes it to the Query stream.
*/
func (c *Client) Query(_ context.Context, p retrievalmarket.RetrievalPeer, payloadCID cid.Cid, params retrievalmarket.QueryParams) (retrievalmarket.QueryResponse, error) {
	s, err := c.network.NewQueryStream(p.ID)
	if err != nil {
		log.Warn(err)
		return retrievalmarket.QueryResponseUndefined, err
	}
	defer s.Close()

	err = s.WriteQuery(retrievalmarket.Query{
		PayloadCID:  payloadCID,
		QueryParams: params,
	})
	if err != nil {
		log.Warn(err)
		return retrievalmarket.QueryResponseUndefined, err
	}

	return s.ReadQueryResponse()
}

/*
Retrieve initiates the retrieval deal flow, which involves multiple requests and responses

To start this processes, the client creates a new `RetrievalDealStream`.  Currently, this connection is
kept open through the entire deal until completion or failure.  Make deals pauseable as well as surviving
a restart is a planned future feature.

Retrieve should be called after using FindProviders and Query are used to identify an appropriate provider to
retrieve the deal from. The parameters identified in Query should be passed to Retrieve to ensure the
greatest likelihood the provider will accept the deal

When called, the client takes the following actions:

1. Creates a deal ID using the next value from its storedcounter.

2. Constructs a `DealProposal` with deal terms

3. Tells its statemachine to begin tracking this deal state by dealID.

4. Constructs a `blockio.SelectorVerifier` and adds it to its dealID-keyed map of block verifiers.

5. Triggers a `ClientEventOpen` event on its statemachine.

From then on, the statemachine controls the deal flow in the client. Other components may listen for events in this flow by calling
`SubscribeToEvents` on the Client. The Client handles consuming blocks it receives from the provider, via `ConsumeBlocks` function

Documentation of the client state machine can be found at https://godoc.org/github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/clientstates
*/
func (c *Client) Retrieve(ctx context.Context, payloadCID cid.Cid, params retrievalmarket.Params, totalFunds abi.TokenAmount, miner peer.ID, clientWallet address.Address, minerWallet address.Address) (retrievalmarket.DealID, error) {
	var err error
	next, err := c.storedCounter.Next()
	if err != nil {
		return 0, err
	}
	dealID := retrievalmarket.DealID(next)

	dealState := retrievalmarket.ClientDealState{
		DealProposal: retrievalmarket.DealProposal{
			PayloadCID: payloadCID,
			ID:         dealID,
			Params:     params,
		},
		TotalFunds:       totalFunds,
		ClientWallet:     clientWallet,
		MinerWallet:      minerWallet,
		TotalReceived:    0,
		CurrentInterval:  params.PaymentInterval,
		BytesPaidFor:     0,
		PaymentRequested: abi.NewTokenAmount(0),
		FundsSpent:       abi.NewTokenAmount(0),
		Status:           retrievalmarket.DealStatusNew,
		Sender:           miner,
	}

	// start the deal processing
	err = c.stateMachines.Begin(dealState.ID, &dealState)
	if err != nil {
		return 0, err
	}

	// open stream
	s, err := c.network.NewDealStream(dealState.Sender)
	if err != nil {
		return 0, err
	}

	c.dealStreams[dealID] = s

	sel := shared.AllSelector()
	if params.Selector != nil {
		sel, err = retrievalmarket.DecodeNode(params.Selector)
		if err != nil {
			return 0, xerrors.Errorf("selector is invalid: %w", err)
		}
	}

	c.blockVerifiers[dealID] = blockio.NewSelectorVerifier(cidlink.Link{Cid: dealState.DealProposal.PayloadCID}, sel)

	err = c.stateMachines.Send(dealState.ID, retrievalmarket.ClientEventOpen)
	if err != nil {
		s.Close()
		return 0, err
	}

	return dealID, nil
}

// unsubscribeAt returns a function that removes an item from the subscribers list by comparing
// their reflect.ValueOf before pulling the item out of the slice.  Does not preserve order.
// Subsequent, repeated calls to the func with the same Subscriber are a no-op.
func (c *Client) unsubscribeAt(sub retrievalmarket.ClientSubscriber) retrievalmarket.Unsubscribe {
	return func() {
		c.subscribersLk.Lock()
		defer c.subscribersLk.Unlock()
		curLen := len(c.subscribers)
		for i, el := range c.subscribers {
			if reflect.ValueOf(sub) == reflect.ValueOf(el) {
				c.subscribers[i] = c.subscribers[curLen-1]
				c.subscribers = c.subscribers[:curLen-1]
				return
			}
		}
	}
}

func (c *Client) notifySubscribers(eventName fsm.EventName, state fsm.StateType) {
	c.subscribersLk.RLock()
	defer c.subscribersLk.RUnlock()
	evt := eventName.(retrievalmarket.ClientEvent)
	ds := state.(retrievalmarket.ClientDealState)
	for _, cb := range c.subscribers {
		cb(evt, ds)
	}
}

// SubscribeToEvents allows another component to listen for events on the RetrievalClient
// in order to track deals as they progress through the deal flow
func (c *Client) SubscribeToEvents(subscriber retrievalmarket.ClientSubscriber) retrievalmarket.Unsubscribe {
	c.subscribersLk.Lock()
	c.subscribers = append(c.subscribers, subscriber)
	c.subscribersLk.Unlock()

	return c.unsubscribeAt(subscriber)
}

// V1
func (c *Client) AddMoreFunds(retrievalmarket.DealID, abi.TokenAmount) error {
	panic("not implemented")
}

func (c *Client) CancelDeal(retrievalmarket.DealID) error {
	panic("not implemented")
}

func (c *Client) RetrievalStatus(retrievalmarket.DealID) {
	panic("not implemented")
}

// ListDeals lists in all known retrieval deals
func (c *Client) ListDeals() map[retrievalmarket.DealID]retrievalmarket.ClientDealState {
	var deals []retrievalmarket.ClientDealState
	_ = c.stateMachines.List(&deals)
	dealMap := make(map[retrievalmarket.DealID]retrievalmarket.ClientDealState)
	for _, deal := range deals {
		dealMap[deal.ID] = deal
	}
	return dealMap
}

type clientDealEnvironment struct {
	c *Client
}

func (c *clientDealEnvironment) Node() retrievalmarket.RetrievalClientNode {
	return c.c.node
}

func (c *clientDealEnvironment) DealStream(dealID retrievalmarket.DealID) rmnet.RetrievalDealStream {
	return c.c.dealStreams[dealID]
}

func (c *clientDealEnvironment) ConsumeBlock(ctx context.Context, dealID retrievalmarket.DealID, block retrievalmarket.Block) (uint64, bool, error) {
	prefix, err := cid.PrefixFromBytes(block.Prefix)
	if err != nil {
		return 0, false, err
	}

	scid, err := prefix.Sum(block.Data)
	if err != nil {
		return 0, false, err
	}

	blk, err := blocks.NewBlockWithCid(block.Data, scid)
	if err != nil {
		return 0, false, err
	}

	verifier, ok := c.c.blockVerifiers[dealID]
	if !ok {
		return 0, false, xerrors.New("no block verifier found")
	}

	done, err := verifier.Verify(ctx, blk)
	if err != nil {
		log.Warnf("block verify failed: %s", err)
		return 0, false, err
	}

	// TODO: Smarter out, maybe add to filestore automagically
	//  (Also, persist intermediate nodes)
	err = c.c.bs.Put(blk)
	if err != nil {
		log.Warnf("block write failed: %s", err)
		return 0, false, err
	}

	return uint64(len(block.Data)), done, nil
}

// ClientFSMParameterSpec is a valid set of parameters for a client deal FSM - used in doc generation
var ClientFSMParameterSpec = fsm.Parameters{
	Environment:     &clientDealEnvironment{},
	StateType:       retrievalmarket.ClientDealState{},
	StateKeyField:   "Status",
	Events:          clientstates.ClientEvents,
	StateEntryFuncs: clientstates.ClientStateEntryFuncs,
}
