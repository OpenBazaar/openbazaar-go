package storageimpl

import (
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-statemachine/fsm"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/hannahhoward/go-pubsub"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-fil-markets/pieceio"
	"github.com/filecoin-project/go-fil-markets/pieceio/cario"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/discovery"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/clientstates"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/clientutils"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/dtutils"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
)

var log = logging.Logger("storagemarket_impl")

const DefaultPollingInterval = 30 * time.Second

var _ storagemarket.StorageClient = &Client{}

// Client is the production implementation of the StorageClient interface
type Client struct {
	net network.StorageMarketNetwork

	dataTransfer datatransfer.Manager
	bs           blockstore.Blockstore
	pio          pieceio.PieceIO
	discovery    *discovery.Local

	node            storagemarket.StorageClientNode
	pubSub          *pubsub.PubSub
	statemachines   fsm.Group
	pollingInterval time.Duration
}

// StorageClientOption allows custom configuration of a storage client
type StorageClientOption func(c *Client)

// DealPollingInterval sets the interval at which this client will query the Provider for deal state while
// waiting for deal acceptance
func DealPollingInterval(t time.Duration) StorageClientOption {
	return func(c *Client) {
		c.pollingInterval = t
	}
}

// NewClient creates a new storage client
func NewClient(
	net network.StorageMarketNetwork,
	bs blockstore.Blockstore,
	dataTransfer datatransfer.Manager,
	discovery *discovery.Local,
	ds datastore.Batching,
	scn storagemarket.StorageClientNode,
	options ...StorageClientOption,
) (*Client, error) {
	carIO := cario.NewCarIO()
	pio := pieceio.NewPieceIO(carIO, bs)

	c := &Client{
		net:             net,
		dataTransfer:    dataTransfer,
		bs:              bs,
		pio:             pio,
		discovery:       discovery,
		node:            scn,
		pubSub:          pubsub.New(clientDispatcher),
		pollingInterval: DefaultPollingInterval,
	}

	statemachines, err := newClientStateMachine(
		ds,
		&clientDealEnvironment{c},
		c.dispatch,
	)
	if err != nil {
		return nil, err
	}
	c.statemachines = statemachines

	c.Configure(options...)

	// register a data transfer event handler -- this will send events to the state machines based on DT events
	dataTransfer.SubscribeToEvents(dtutils.ClientDataTransferSubscriber(statemachines))

	return c, nil
}

// Start initializes deal processing on a StorageClient and restarts
// in progress deals
func (c *Client) Start(ctx context.Context) error {
	go func() {
		err := c.restartDeals()
		if err != nil {
			log.Errorf("Failed to restart deals: %s", err.Error())
		}
	}()
	return nil
}

// Stop ends deal processing on a StorageClient
func (c *Client) Stop() error {
	return c.statemachines.Stop(context.TODO())
}

// ListProviders queries chain state and returns active storage providers
func (c *Client) ListProviders(ctx context.Context) (<-chan storagemarket.StorageProviderInfo, error) {
	tok, _, err := c.node.GetChainHead(ctx)
	if err != nil {
		return nil, err
	}

	providers, err := c.node.ListStorageProviders(ctx, tok)
	if err != nil {
		return nil, err
	}

	out := make(chan storagemarket.StorageProviderInfo)

	go func() {
		defer close(out)
		for _, p := range providers {
			select {
			case out <- *p:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

// ListDeals lists on-chain deals associated with this storage client
func (c *Client) ListDeals(ctx context.Context, addr address.Address) ([]storagemarket.StorageDeal, error) {
	tok, _, err := c.node.GetChainHead(ctx)
	if err != nil {
		return nil, err
	}

	return c.node.ListClientDeals(ctx, addr, tok)
}

// ListLocalDeals lists deals initiated by this storage client
func (c *Client) ListLocalDeals(ctx context.Context) ([]storagemarket.ClientDeal, error) {
	var out []storagemarket.ClientDeal
	if err := c.statemachines.List(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetLocalDeal lists deals that are in progress or rejected
func (c *Client) GetLocalDeal(ctx context.Context, cid cid.Cid) (storagemarket.ClientDeal, error) {
	var out storagemarket.ClientDeal
	if err := c.statemachines.Get(cid).Get(&out); err != nil {
		return storagemarket.ClientDeal{}, err
	}
	return out, nil
}

// GetAsk queries a provider for its current storage ask
//
// The client creates a new `StorageAskStream` for the chosen peer ID,
// and calls WriteAskRequest on it, which constructs a message and writes it to the Ask stream.
// When it receives a response, it verifies the signature and returns the validated
// StorageAsk if successful
func (c *Client) GetAsk(ctx context.Context, info storagemarket.StorageProviderInfo) (*storagemarket.SignedStorageAsk, error) {
	if len(info.Addrs) > 0 {
		c.net.AddAddrs(info.PeerID, info.Addrs)
	}
	s, err := c.net.NewAskStream(ctx, info.PeerID)
	if err != nil {
		return nil, xerrors.Errorf("failed to open stream to miner: %w", err)
	}

	request := network.AskRequest{Miner: info.Address}
	if err := s.WriteAskRequest(request); err != nil {
		return nil, xerrors.Errorf("failed to send ask request: %w", err)
	}

	out, err := s.ReadAskResponse()
	if err != nil {
		return nil, xerrors.Errorf("failed to read ask response: %w", err)
	}

	if out.Ask == nil {
		return nil, xerrors.Errorf("got no ask back")
	}

	if out.Ask.Ask.Miner != info.Address {
		return nil, xerrors.Errorf("got back ask for wrong miner")
	}

	tok, _, err := c.node.GetChainHead(ctx)
	if err != nil {
		return nil, err
	}

	isValid, err := c.node.ValidateAskSignature(ctx, out.Ask, tok)
	if err != nil {
		return nil, err
	}

	if !isValid {
		return nil, xerrors.Errorf("ask was not properly signed")
	}

	return out.Ask, nil
}

// GetProviderDealState queries a provider for the current state of a client's deal
func (c *Client) GetProviderDealState(ctx context.Context, proposalCid cid.Cid) (*storagemarket.ProviderDealState, error) {
	var deal storagemarket.ClientDeal
	err := c.statemachines.Get(proposalCid).Get(&deal)
	if err != nil {
		return nil, xerrors.Errorf("could not get client deal state: %w", err)
	}

	s, err := c.net.NewDealStatusStream(ctx, deal.Miner)
	if err != nil {
		return nil, xerrors.Errorf("failed to open stream to miner: %w", err)
	}

	buf, err := cborutil.Dump(&deal.ProposalCid)
	if err != nil {
		return nil, xerrors.Errorf("failed serialize deal status request: %w", err)
	}

	addr, err := c.node.GetDefaultWalletAddress(ctx)
	if err != nil {
		return nil, xerrors.Errorf("failed to get client address: %w", err)
	}

	signature, err := c.node.SignBytes(ctx, addr, buf)
	if err != nil {
		return nil, xerrors.Errorf("failed to sign deal status request: %w", err)
	}

	if err := s.WriteDealStatusRequest(network.DealStatusRequest{Proposal: proposalCid, Signature: *signature}); err != nil {
		return nil, xerrors.Errorf("failed to send deal status request: %w", err)
	}

	resp, err := s.ReadDealStatusResponse()
	if err != nil {
		return nil, xerrors.Errorf("failed to read deal status response: %w", err)
	}

	valid, err := c.verifyStatusResponseSignature(ctx, deal.MinerWorker, resp)
	if err != nil {
		return nil, err
	}

	if !valid {
		return nil, xerrors.Errorf("invalid deal status response signature")
	}

	return &resp.DealState, nil
}

/*
ProposeStorageDeal initiates the retrieval deal flow, which involves multiple requests and responses.

This function is called after using ListProviders and QueryAs are used to identify an appropriate provider
to store data. The parameters passed to ProposeStorageDeal should matched those returned by the miner from
QueryAsk to ensure the greatest likelihood the provider will accept the deal.

When called, the client takes the following actions:

1. Calculates the PieceCID for this deal from the given PayloadCID. (by writing the payload to a CAR file then calculating
a merkle root for the resulting data)

2. Constructs a `DealProposal` (spec-actors type) with deal terms

3. Signs the `DealProposal` to make a ClientDealProposal

4. Gets the CID for the ClientDealProposal

5. Construct a ClientDeal to track the state of this deal.

6. Tells its statemachine to begin tracking the deal state by the CID of the ClientDealProposal

7. Triggers a `ClientEventOpen` event on its statemachine.

8. Records the Provider as a possible peer for retrieving this data in the future

From then on, the statemachine controls the deal flow in the client. Other components may listen for events in this flow by calling
`SubscribeToEvents` on the Client. The Client also provides access to the node and network and other functionality through
its implementation of the Client FSM's ClientDealEnvironment.

Documentation of the client state machine can be found at https://godoc.org/github.com/filecoin-project/go-fil-markets/storagemarket/impl/clientstates
*/
func (c *Client) ProposeStorageDeal(ctx context.Context, addr address.Address, info *storagemarket.StorageProviderInfo, data *storagemarket.DataRef, startEpoch abi.ChainEpoch, endEpoch abi.ChainEpoch, price abi.TokenAmount, collateral abi.TokenAmount, rt abi.RegisteredSealProof, fastRetrieval bool, verifiedDeal bool) (*storagemarket.ProposeStorageDealResult, error) {
	commP, pieceSize, err := clientutils.CommP(ctx, c.pio, rt, data)
	if err != nil {
		return nil, xerrors.Errorf("computing commP failed: %w", err)
	}

	if uint64(pieceSize.Padded()) > info.SectorSize {
		return nil, fmt.Errorf("cannot propose a deal whose piece size (%d) is greater than sector size (%d)", pieceSize.Padded(), info.SectorSize)
	}

	dealProposal := market.DealProposal{
		PieceCID:             commP,
		PieceSize:            pieceSize.Padded(),
		Client:               addr,
		Provider:             info.Address,
		StartEpoch:           startEpoch,
		EndEpoch:             endEpoch,
		StoragePricePerEpoch: price,
		ProviderCollateral:   abi.NewTokenAmount(int64(pieceSize)), // TODO: real calc
		ClientCollateral:     big.Zero(),
		VerifiedDeal:         verifiedDeal,
	}

	clientDealProposal, err := c.node.SignProposal(ctx, addr, dealProposal)
	if err != nil {
		return nil, xerrors.Errorf("signing deal proposal failed: %w", err)
	}

	proposalNd, err := cborutil.AsIpld(clientDealProposal)
	if err != nil {
		return nil, xerrors.Errorf("getting proposal node failed: %w", err)
	}

	deal := &storagemarket.ClientDeal{
		ProposalCid:        proposalNd.Cid(),
		ClientDealProposal: *clientDealProposal,
		State:              storagemarket.StorageDealUnknown,
		Miner:              info.PeerID,
		MinerWorker:        info.Worker,
		DataRef:            data,
		FastRetrieval:      fastRetrieval,
	}

	err = c.statemachines.Begin(proposalNd.Cid(), deal)
	if err != nil {
		return nil, xerrors.Errorf("setting up deal tracking: %w", err)
	}

	err = c.statemachines.Send(deal.ProposalCid, storagemarket.ClientEventOpen)
	if err != nil {
		return nil, xerrors.Errorf("initializing state machine: %w", err)
	}

	return &storagemarket.ProposeStorageDealResult{
			ProposalCid: deal.ProposalCid,
		}, c.discovery.AddPeer(data.Root, retrievalmarket.RetrievalPeer{
			Address: dealProposal.Provider,
			ID:      deal.Miner,
		})
}

// GetPaymentEscrow returns the current funds available for deal payment
func (c *Client) GetPaymentEscrow(ctx context.Context, addr address.Address) (storagemarket.Balance, error) {
	tok, _, err := c.node.GetChainHead(ctx)
	if err != nil {
		return storagemarket.Balance{}, err
	}

	return c.node.GetBalance(ctx, addr, tok)
}

// AddPaymentEscrow adds funds for storage deals
func (c *Client) AddPaymentEscrow(ctx context.Context, addr address.Address, amount abi.TokenAmount) error {
	done := make(chan error, 1)

	mcid, err := c.node.AddFunds(ctx, addr, amount)
	if err != nil {
		return err
	}

	err = c.node.WaitForMessage(ctx, mcid, func(code exitcode.ExitCode, bytes []byte, err error) error {
		if err != nil {
			done <- xerrors.Errorf("AddFunds errored: %w", err)
		} else if code != exitcode.Ok {
			done <- xerrors.Errorf("AddFunds error, exit code: %s", code.String())
		} else {
			done <- nil
		}
		return nil
	})

	if err != nil {
		return err
	}

	return <-done
}

// SubscribeToEvents allows another component to listen for events on the StorageClient
// in order to track deals as they progress through the deal flow
func (c *Client) SubscribeToEvents(subscriber storagemarket.ClientSubscriber) shared.Unsubscribe {
	return shared.Unsubscribe(c.pubSub.Subscribe(subscriber))
}

// PollingInterval is a getter for the polling interval option
func (c *Client) PollingInterval() time.Duration {
	return c.pollingInterval
}

// Configure applies the given list of StorageClientOptions after a StorageClient
// is initialized
func (c *Client) Configure(options ...StorageClientOption) {
	for _, option := range options {
		option(c)
	}
}

func (c *Client) restartDeals() error {
	var deals []storagemarket.ClientDeal
	err := c.statemachines.List(&deals)
	if err != nil {
		return err
	}

	for _, deal := range deals {
		err = c.statemachines.Send(deal.ProposalCid, storagemarket.ClientEventRestart)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) dispatch(eventName fsm.EventName, deal fsm.StateType) {
	evt, ok := eventName.(storagemarket.ClientEvent)
	if !ok {
		log.Errorf("dropped bad event %s", eventName)
	}
	realDeal, ok := deal.(storagemarket.ClientDeal)
	if !ok {
		log.Errorf("not a ClientDeal %v", deal)
	}
	pubSubEvt := internalClientEvent{evt, realDeal}

	if err := c.pubSub.Publish(pubSubEvt); err != nil {
		log.Errorf("failed to publish event %d", evt)
	}
}

func (c *Client) verifyStatusResponseSignature(ctx context.Context, miner address.Address, response network.DealStatusResponse) (bool, error) {
	tok, _, err := c.node.GetChainHead(ctx)
	if err != nil {
		return false, xerrors.Errorf("getting chain head: %w", err)
	}

	buf, err := cborutil.Dump(&response.DealState)
	if err != nil {
		return false, xerrors.Errorf("serializing: %w", err)
	}

	valid, err := c.node.VerifySignature(ctx, response.Signature, miner, buf, tok)
	if err != nil {
		return false, xerrors.Errorf("validating signature: %w", err)
	}

	return valid, nil
}

func newClientStateMachine(ds datastore.Datastore, env fsm.Environment, notifier fsm.Notifier) (fsm.Group, error) {
	return fsm.New(ds, fsm.Parameters{
		Environment:     env,
		StateType:       storagemarket.ClientDeal{},
		StateKeyField:   "State",
		Events:          clientstates.ClientEvents,
		StateEntryFuncs: clientstates.ClientStateEntryFuncs,
		FinalityStates:  clientstates.ClientFinalityStates,
		Notifier:        notifier,
	})
}

type internalClientEvent struct {
	evt  storagemarket.ClientEvent
	deal storagemarket.ClientDeal
}

func clientDispatcher(evt pubsub.Event, fn pubsub.SubscriberFn) error {
	ie, ok := evt.(internalClientEvent)
	if !ok {
		return xerrors.New("wrong type of event")
	}
	cb, ok := fn.(storagemarket.ClientSubscriber)
	if !ok {
		return xerrors.New("wrong type of event")
	}
	cb(ie.evt, ie.deal)
	return nil
}

// -------
// clientDealEnvironment
// -------

type clientDealEnvironment struct {
	c *Client
}

func (c *clientDealEnvironment) NewDealStream(ctx context.Context, p peer.ID) (network.StorageDealStream, error) {
	return c.c.net.NewDealStream(ctx, p)
}

func (c *clientDealEnvironment) Node() storagemarket.StorageClientNode {
	return c.c.node
}

func (c *clientDealEnvironment) StartDataTransfer(ctx context.Context, to peer.ID, voucher datatransfer.Voucher, baseCid cid.Cid, selector ipld.Node) error {
	_, err := c.c.dataTransfer.OpenPushDataChannel(ctx, to, voucher, baseCid, selector)
	return err
}

func (c *clientDealEnvironment) GetProviderDealState(ctx context.Context, proposalCid cid.Cid) (*storagemarket.ProviderDealState, error) {
	return c.c.GetProviderDealState(ctx, proposalCid)
}

func (c *clientDealEnvironment) PollingInterval() time.Duration {
	return c.c.pollingInterval
}

// ClientFSMParameterSpec is a valid set of parameters for a client deal FSM - used in doc generation
var ClientFSMParameterSpec = fsm.Parameters{
	Environment:     &clientDealEnvironment{},
	StateType:       storagemarket.ClientDeal{},
	StateKeyField:   "State",
	Events:          clientstates.ClientEvents,
	StateEntryFuncs: clientstates.ClientStateEntryFuncs,
	FinalityStates:  clientstates.ClientFinalityStates,
}

var _ clientstates.ClientDealEnvironment = &clientDealEnvironment{}
