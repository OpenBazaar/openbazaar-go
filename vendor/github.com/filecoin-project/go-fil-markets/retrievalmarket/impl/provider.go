package retrievalimpl

import (
	"context"
	"errors"
	"reflect"
	"sync"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-statemachine/fsm"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-fil-markets/pieceio/cario"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/blockio"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/blockunsealing"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/providerstates"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-fil-markets/shared"
)

// RetrievalProviderOption is a function that configures a retrieval provider
type RetrievalProviderOption func(p *Provider)

// DealDecider is a function that makes a decision about whether to accept a deal
type DealDecider func(ctx context.Context, state retrievalmarket.ProviderDealState) (bool, string, error)

// Provider is the production implementation of the RetrievalProvider interface
type Provider struct {
	bs                      blockstore.Blockstore
	node                    retrievalmarket.RetrievalProviderNode
	network                 rmnet.RetrievalMarketNetwork
	paymentInterval         uint64
	paymentIntervalIncrease uint64
	minerAddress            address.Address
	pieceStore              piecestore.PieceStore
	pricePerByte            abi.TokenAmount
	subscribers             []retrievalmarket.ProviderSubscriber
	subscribersLk           sync.RWMutex
	dealStreams             map[retrievalmarket.ProviderDealIdentifier]rmnet.RetrievalDealStream
	dealStreamsLk           sync.Mutex
	blockReaders            map[retrievalmarket.ProviderDealIdentifier]blockio.BlockReader
	blockReadersLk          sync.Mutex
	stateMachines           fsm.Group
	dealDecider             DealDecider
}

var _ retrievalmarket.RetrievalProvider = new(Provider)
var _ providerstates.ProviderDealEnvironment = new(providerDealEnvironment)

// DefaultPricePerByte is the charge per byte retrieved if the miner does
// not specifically set it
var DefaultPricePerByte = abi.NewTokenAmount(2)

// DefaultPaymentInterval is the baseline interval, set to 1Mb
// if the miner does not explicitly set it otherwise
var DefaultPaymentInterval = uint64(1 << 20)

// DefaultPaymentIntervalIncrease is the amount interval increases on each payment,
// set to to 1Mb if the miner does not explicitly set it otherwise
var DefaultPaymentIntervalIncrease = uint64(1 << 20)

// DealDeciderOpt sets a custom protocol
func DealDeciderOpt(dd DealDecider) RetrievalProviderOption {
	return func(provider *Provider) {
		provider.dealDecider = dd
	}
}

// NewProvider returns a new retrieval Provider
func NewProvider(minerAddress address.Address, node retrievalmarket.RetrievalProviderNode,
	network rmnet.RetrievalMarketNetwork, pieceStore piecestore.PieceStore,
	bs blockstore.Blockstore, ds datastore.Batching, opts ...RetrievalProviderOption,
) (retrievalmarket.RetrievalProvider, error) {

	p := &Provider{
		bs:                      bs,
		node:                    node,
		network:                 network,
		minerAddress:            minerAddress,
		pieceStore:              pieceStore,
		pricePerByte:            DefaultPricePerByte, // TODO: allow setting
		paymentInterval:         DefaultPaymentInterval,
		paymentIntervalIncrease: DefaultPaymentIntervalIncrease,
		dealStreams:             make(map[retrievalmarket.ProviderDealIdentifier]rmnet.RetrievalDealStream),
		blockReaders:            make(map[retrievalmarket.ProviderDealIdentifier]blockio.BlockReader),
	}
	statemachines, err := fsm.New(ds, fsm.Parameters{
		Environment:     &providerDealEnvironment{p},
		StateType:       retrievalmarket.ProviderDealState{},
		StateKeyField:   "Status",
		Events:          providerstates.ProviderEvents,
		StateEntryFuncs: providerstates.ProviderStateEntryFuncs,
		Notifier:        p.notifySubscribers,
	})
	if err != nil {
		return nil, err
	}
	p.Configure(opts...)
	p.stateMachines = statemachines
	return p, nil
}

// Stop stops handling incoming requests.
func (p *Provider) Stop() error {
	return p.network.StopHandlingRequests()
}

// Start begins listening for deals on the given host.
// Start must be called in order to accept incoming deals.
func (p *Provider) Start() error {
	return p.network.SetDelegate(p)
}

// V0

// SetPricePerByte sets the price per byte a miner charges for retrievals
func (p *Provider) SetPricePerByte(price abi.TokenAmount) {
	p.pricePerByte = price
}

// SetPaymentInterval sets the maximum number of bytes a a Provider will send before
// requesting further payment, and the rate at which that value increases
func (p *Provider) SetPaymentInterval(paymentInterval uint64, paymentIntervalIncrease uint64) {
	p.paymentInterval = paymentInterval
	p.paymentIntervalIncrease = paymentIntervalIncrease
}

// unsubscribeAt returns a function that removes an item from the subscribers list by comparing
// their reflect.ValueOf before pulling the item out of the slice.  Does not preserve order.
// Subsequent, repeated calls to the func with the same Subscriber are a no-op.
func (p *Provider) unsubscribeAt(sub retrievalmarket.ProviderSubscriber) retrievalmarket.Unsubscribe {
	return func() {
		p.subscribersLk.Lock()
		defer p.subscribersLk.Unlock()
		curLen := len(p.subscribers)
		for i, el := range p.subscribers {
			if reflect.ValueOf(sub) == reflect.ValueOf(el) {
				p.subscribers[i] = p.subscribers[curLen-1]
				p.subscribers = p.subscribers[:curLen-1]
				return
			}
		}
	}
}

func (p *Provider) notifySubscribers(eventName fsm.EventName, state fsm.StateType) {
	p.subscribersLk.RLock()
	defer p.subscribersLk.RUnlock()
	evt := eventName.(retrievalmarket.ProviderEvent)
	ds := state.(retrievalmarket.ProviderDealState)
	for _, cb := range p.subscribers {
		cb(evt, ds)
	}
}

// SubscribeToEvents listens for events that happen related to client retrievals
func (p *Provider) SubscribeToEvents(subscriber retrievalmarket.ProviderSubscriber) retrievalmarket.Unsubscribe {
	p.subscribersLk.Lock()
	p.subscribers = append(p.subscribers, subscriber)
	p.subscribersLk.Unlock()

	return p.unsubscribeAt(subscriber)
}

// V1

func (p *Provider) SetPricePerUnseal(price abi.TokenAmount) {
	panic("not implemented")
}

// ListDeals lists in all known retrieval deals
func (p *Provider) ListDeals() map[retrievalmarket.ProviderDealID]retrievalmarket.ProviderDealState {
	var deals []retrievalmarket.ProviderDealState
	_ = p.stateMachines.List(&deals)
	dealMap := make(map[retrievalmarket.ProviderDealID]retrievalmarket.ProviderDealState)
	for _, deal := range deals {
		dealMap[retrievalmarket.ProviderDealID{From: deal.Receiver, ID: deal.ID}] = deal
	}
	return dealMap
}

/*
HandleQueryStream is called by the network implementation whenever a new message is received on the query protocol

A Provider handling a retrieval `Query` does the following:

1. Get the node's chain head in order to get its miner worker address.

2. Look in its piece store for determine if it can serve the given payload CID.

3. Combine these results with its existing parameters for retrieval deals to construct a `retrievalmarket.QueryResponse` struct.

4.0 Writes this response to the `Query` stream.

The connection is kept open only as long as the query-response exchange.
*/
func (p *Provider) HandleQueryStream(stream rmnet.RetrievalQueryStream) {
	defer stream.Close()
	query, err := stream.ReadQuery()
	if err != nil {
		return
	}

	answer := retrievalmarket.QueryResponse{
		Status:                     retrievalmarket.QueryResponseUnavailable,
		PieceCIDFound:              retrievalmarket.QueryItemUnavailable,
		MinPricePerByte:            p.pricePerByte,
		MaxPaymentInterval:         p.paymentInterval,
		MaxPaymentIntervalIncrease: p.paymentIntervalIncrease,
	}

	ctx := context.TODO()

	tok, _, err := p.node.GetChainHead(ctx)
	if err != nil {
		log.Errorf("Retrieval query: GetChainHead: %s", err)
		return
	}

	paymentAddress, err := p.node.GetMinerWorkerAddress(ctx, p.minerAddress, tok)
	if err != nil {
		log.Errorf("Retrieval query: Lookup Payment Address: %s", err)
		answer.Status = retrievalmarket.QueryResponseError
		answer.Message = err.Error()
	} else {
		answer.PaymentAddress = paymentAddress

		pieceCID := cid.Undef
		if query.PieceCID != nil {
			pieceCID = *query.PieceCID
		}
		pieceInfo, err := getPieceInfoFromCid(p.pieceStore, query.PayloadCID, pieceCID)

		if err == nil && len(pieceInfo.Deals) > 0 {
			answer.Status = retrievalmarket.QueryResponseAvailable
			// TODO: get price, look for already unsealed ref to reduce work
			answer.Size = uint64(pieceInfo.Deals[0].Length) // TODO: verify on intermediate
			answer.PieceCIDFound = retrievalmarket.QueryItemAvailable
		}

		if err != nil && !xerrors.Is(err, retrievalmarket.ErrNotFound) {
			log.Errorf("Retrieval query: GetRefs: %s", err)
			answer.Status = retrievalmarket.QueryResponseError
			answer.Message = err.Error()
		}

	}
	if err := stream.WriteQueryResponse(answer); err != nil {
		log.Errorf("Retrieval query: WriteCborRPC: %s", err)
		return
	}
}

/*
HandleDealStream is called by the network implementation whenever a new message is received on the deal protocol

When a provider receives a DealProposal of the deal protocol, it takes the following steps:

1. Tells its statemachine to begin tracking this deal state by dealID.

2. Constructs a `blockunsealing.LoaderWithUnsealing` that abstracts the process of unsealing pieces as needed when loading blocks

3. Constructs a `blockio.BlockReader` and adds it to its dealID-keyed map of block readers.

4. Triggers a `ProviderEventOpen` event on its statemachine.

From then on, the statemachine controls the deal flow in the client. Other components may listen for events in this flow by calling
`SubscribeToEvents` on the Provider. The Provider handles loading the next block to send to the client.*/
func (p *Provider) HandleDealStream(stream rmnet.RetrievalDealStream) {
	// read deal proposal (or fail)
	err := p.newProviderDeal(stream)
	if err != nil {
		log.Error(err)
		stream.Close()
	}
}

// Configure reconfigures a provider after initialization
func (p *Provider) Configure(opts ...RetrievalProviderOption) {
	for _, opt := range opts {
		opt(p)
	}
}

func (p *Provider) newProviderDeal(stream rmnet.RetrievalDealStream) error {
	dealProposal, err := stream.ReadDealProposal()
	if err != nil {
		return err
	}

	pds := retrievalmarket.ProviderDealState{
		DealProposal: dealProposal,
		Receiver:     stream.Receiver(),
	}

	p.dealStreamsLk.Lock()
	p.dealStreams[pds.Identifier()] = stream
	p.dealStreamsLk.Unlock()

	loaderWithUnsealing := blockunsealing.NewLoaderWithUnsealing(context.TODO(), p.bs, p.pieceStore, cario.NewCarIO(), p.node.UnsealSector, dealProposal.PieceCID)

	// validate the selector, if provided
	var sel ipld.Node
	if dealProposal.Params.Selector != nil {
		sel, err = retrievalmarket.DecodeNode(dealProposal.Params.Selector)
		if err != nil {
			return xerrors.Errorf("selector is invalid: %w", err)
		}
	} else {
		sel = shared.AllSelector()
	}

	br := blockio.NewSelectorBlockReader(cidlink.Link{Cid: dealProposal.PayloadCID}, sel, loaderWithUnsealing.Load)
	p.blockReadersLk.Lock()
	p.blockReaders[pds.Identifier()] = br
	p.blockReadersLk.Unlock()

	// start the deal processing, synchronously so we can log the error and close the stream if it doesn't start
	err = p.stateMachines.Begin(pds.Identifier(), &pds)
	if err != nil {
		return err
	}

	err = p.stateMachines.Send(pds.Identifier(), retrievalmarket.ProviderEventOpen)
	if err != nil {
		return err
	}

	return nil
}

type providerDealEnvironment struct {
	p *Provider
}

func (p *providerDealEnvironment) Node() retrievalmarket.RetrievalProviderNode {
	return p.p.node
}

func (p *providerDealEnvironment) DealStream(id retrievalmarket.ProviderDealIdentifier) rmnet.RetrievalDealStream {
	p.p.dealStreamsLk.Lock()
	defer p.p.dealStreamsLk.Unlock()
	return p.p.dealStreams[id]
}

func (p *providerDealEnvironment) CheckDealParams(pricePerByte abi.TokenAmount, paymentInterval uint64, paymentIntervalIncrease uint64) error {
	if pricePerByte.LessThan(p.p.pricePerByte) {
		return errors.New("Price per byte too low")
	}
	if paymentInterval > p.p.paymentInterval {
		return errors.New("Payment interval too large")
	}
	if paymentIntervalIncrease > p.p.paymentIntervalIncrease {
		return errors.New("Payment interval increase too large")
	}
	return nil
}

func (p *providerDealEnvironment) NextBlock(ctx context.Context, id retrievalmarket.ProviderDealIdentifier) (retrievalmarket.Block, bool, error) {
	p.p.blockReadersLk.Lock()
	br, ok := p.p.blockReaders[id]
	p.p.blockReadersLk.Unlock()
	if !ok {
		return retrievalmarket.Block{}, false, errors.New("Could not read block")
	}
	return br.ReadBlock(ctx)
}

func (p *providerDealEnvironment) GetPieceSize(c cid.Cid, pieceCID *cid.Cid) (uint64, error) {
	inPieceCid := cid.Undef
	if pieceCID != nil {
		inPieceCid = *pieceCID
	}
	pieceInfo, err := getPieceInfoFromCid(p.p.pieceStore, c, inPieceCid)
	if err != nil {
		return 0, err
	}
	if len(pieceInfo.Deals) == 0 {
		return 0, errors.New("Not enough piece info")
	}
	return pieceInfo.Deals[0].Length, nil
}

func (p *providerDealEnvironment) RunDealDecisioningLogic(ctx context.Context, state retrievalmarket.ProviderDealState) (bool, string, error) {
	if p.p.dealDecider == nil {
		return true, "", nil
	}
	return p.p.dealDecider(ctx, state)
}

func getPieceInfoFromCid(pieceStore piecestore.PieceStore, payloadCID, pieceCID cid.Cid) (piecestore.PieceInfo, error) {
	cidInfo, err := pieceStore.GetCIDInfo(payloadCID)
	if err != nil {
		return piecestore.PieceInfoUndefined, xerrors.Errorf("get cid info: %w", err)
	}
	var lastErr error
	for _, pieceBlockLocation := range cidInfo.PieceBlockLocations {
		pieceInfo, err := pieceStore.GetPieceInfo(pieceBlockLocation.PieceCID)
		if err == nil {
			if pieceCID.Equals(cid.Undef) || pieceInfo.PieceCID.Equals(pieceCID) {
				return pieceInfo, nil
			}
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = xerrors.Errorf("unknown pieceCID %s", pieceCID.String())
	}
	return piecestore.PieceInfoUndefined, xerrors.Errorf("could not locate piece: %w", lastErr)
}

// ProviderFSMParameterSpec is a valid set of parameters for a provider FSM - used in doc generation
var ProviderFSMParameterSpec = fsm.Parameters{
	Environment:     &providerDealEnvironment{},
	StateType:       retrievalmarket.ProviderDealState{},
	StateKeyField:   "Status",
	Events:          providerstates.ProviderEvents,
	StateEntryFuncs: providerstates.ProviderStateEntryFuncs,
}
