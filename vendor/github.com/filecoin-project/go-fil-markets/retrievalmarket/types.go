package retrievalmarket

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/libp2p/go-libp2p-core/peer"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-fil-markets/shared"
)

//go:generate cbor-gen-for Query QueryResponse DealProposal DealResponse Params QueryParams DealPayment Block ClientDealState ProviderDealState PaymentInfo

// ProtocolID is the protocol for proposing / responding to retrieval deals
const ProtocolID = "/fil/retrieval/0.0.1"

// QueryProtocolID is the protocol for querying information about retrieval
// deal parameters
const QueryProtocolID = "/fil/retrieval/qry/0.0.1"

// Unsubscribe is a function that unsubscribes a subscriber for either the
// client or the provider
type Unsubscribe func()

// PaymentInfo is the payment channel and lane for a deal, once it is setup
type PaymentInfo struct {
	PayCh address.Address
	Lane  uint64
}

// ClientDealState is the current state of a deal from the point of view
// of a retrieval client
type ClientDealState struct {
	DealProposal
	TotalFunds       abi.TokenAmount
	ClientWallet     address.Address
	MinerWallet      address.Address
	PaymentInfo      *PaymentInfo
	Status           DealStatus
	Sender           peer.ID
	TotalReceived    uint64
	Message          string
	BytesPaidFor     uint64
	CurrentInterval  uint64
	PaymentRequested abi.TokenAmount
	FundsSpent       abi.TokenAmount
	WaitMsgCID       *cid.Cid // the CID of any message the client deal is waiting for
}

// ClientEvent is an event that occurs in a deal lifecycle on the client
type ClientEvent uint64

const (
	// ClientEventOpen indicates a deal was initiated
	ClientEventOpen ClientEvent = iota

	// ClientEventPaymentChannelErrored means there was a failure creating a payment channel
	ClientEventPaymentChannelErrored

	// ClientEventAllocateLaneErrored means there was a failure creating a lane in a payment channel
	ClientEventAllocateLaneErrored

	// ClientEventPaymentChannelCreateInitiated means we are waiting for a message to
	// create a payment channel to appear on chain
	ClientEventPaymentChannelCreateInitiated

	// ClientEventPaymentChannelReady means the newly created payment channel is ready for the
	// deal to resume
	ClientEventPaymentChannelReady

	// ClientEventPaymentChannelAddingFunds mean we are waiting for funds to be
	// added to a payment channel
	ClientEventPaymentChannelAddingFunds

	// ClientEventPaymentChannelAddFundsErrored means that adding funds to the payment channel
	// failed
	ClientEventPaymentChannelAddFundsErrored

	// ClientEventWriteDealProposalErrored means a network error writing a deal proposal
	ClientEventWriteDealProposalErrored

	// ClientEventReadDealResponseErrored means a network error reading a deal response
	ClientEventReadDealResponseErrored

	// ClientEventDealRejected means a deal was rejected by the provider
	ClientEventDealRejected

	// ClientEventDealNotFound means a provider could not find a piece for a deal
	ClientEventDealNotFound

	// ClientEventDealAccepted means a provider accepted a deal
	ClientEventDealAccepted

	// ClientEventUnknownResponseReceived means a client received a response it doesn't
	// understand from the provider
	ClientEventUnknownResponseReceived

	// ClientEventFundsExpended indicates a deal has run out of funds in the payment channel
	// forcing the client to add more funds to continue the deal
	ClientEventFundsExpended // when totalFunds is expended

	// ClientEventBadPaymentRequested indicates the provider asked for funds
	// in a way that does not match the terms of the deal
	ClientEventBadPaymentRequested

	// ClientEventCreateVoucherFailed indicates an error happened creating a payment voucher
	ClientEventCreateVoucherFailed

	// ClientEventWriteDealPaymentErrored indicates a network error trying to write a payment
	ClientEventWriteDealPaymentErrored

	// ClientEventPaymentSent indicates a payment was sent to the provider
	ClientEventPaymentSent

	// ClientEventConsumeBlockFailed indicates an error occurred while trying to
	// read a block from the provider
	ClientEventConsumeBlockFailed

	// ClientEventLastPaymentRequested indicates the provider requested a final payment
	ClientEventLastPaymentRequested

	// ClientEventAllBlocksReceived indicates the provider has sent all blocks
	ClientEventAllBlocksReceived

	// ClientEventEarlyTermination indicates the provider completed the deal without sending all blocks
	ClientEventEarlyTermination

	// ClientEventPaymentRequested indicates the provider requested a payment
	ClientEventPaymentRequested

	// ClientEventBlocksReceived indicates the provider has sent blocks
	ClientEventBlocksReceived

	// ClientEventProgress indicates more data was received for a retrieval
	ClientEventProgress

	// ClientEventError indicates an error occurred during a deal
	ClientEventError

	// ClientEventComplete indicates a deal has completed
	ClientEventComplete
)

// ClientEvents is a human readable map of client event name -> event description
var ClientEvents = map[ClientEvent]string{
	ClientEventOpen:                          "ClientEventOpen",
	ClientEventPaymentChannelErrored:         "ClientEventPaymentChannelErrored",
	ClientEventAllocateLaneErrored:           "ClientEventAllocateLaneErrored",
	ClientEventPaymentChannelCreateInitiated: "ClientEventPaymentChannelCreateInitiated",
	ClientEventPaymentChannelReady:           "ClientEventPaymentChannelReady",
	ClientEventPaymentChannelAddingFunds:     "ClientEventPaymentChannelAddingFunds",
	ClientEventPaymentChannelAddFundsErrored: "ClientEventPaymentChannelAddFundsErrored",
	ClientEventWriteDealProposalErrored:      "ClientEventWriteDealProposalErrored",
	ClientEventReadDealResponseErrored:       "ClientEventReadDealResponseErrored",
	ClientEventDealRejected:                  "ClientEventDealRejected",
	ClientEventDealNotFound:                  "ClientEventDealNotFound",
	ClientEventDealAccepted:                  "ClientEventDealAccepted",
	ClientEventUnknownResponseReceived:       "ClientEventUnknownResponseReceived",
	ClientEventFundsExpended:                 "ClientEventFundsExpended",
	ClientEventBadPaymentRequested:           "ClientEventBadPaymentRequested",
	ClientEventCreateVoucherFailed:           "ClientEventCreateVoucherFailed",
	ClientEventWriteDealPaymentErrored:       "ClientEventWriteDealPaymentErrored",
	ClientEventPaymentSent:                   "ClientEventPaymentSent",
	ClientEventConsumeBlockFailed:            "ClientEventConsumeBlockFailed",
	ClientEventLastPaymentRequested:          "ClientEventLastPaymentRequested",
	ClientEventAllBlocksReceived:             "ClientEventAllBlocksReceived",
	ClientEventEarlyTermination:              "ClientEventEarlyTermination",
	ClientEventPaymentRequested:              "ClientEventPaymentRequested",
	ClientEventBlocksReceived:                "ClientEventBlocksReceived",
	ClientEventProgress:                      "ClientEventProgress",
	ClientEventError:                         "ClientEventError",
	ClientEventComplete:                      "ClientEventComplete",
}

// ClientSubscriber is a callback that is registered to listen for retrieval events
type ClientSubscriber func(event ClientEvent, state ClientDealState)

// RetrievalClient is a client interface for making retrieval deals
type RetrievalClient interface {
	// V0

	// Find Providers finds retrieval providers who may be storing a given piece
	FindProviders(payloadCID cid.Cid) []RetrievalPeer

	// Query asks a provider for information about a piece it is storing
	Query(
		ctx context.Context,
		p RetrievalPeer,
		payloadCID cid.Cid,
		params QueryParams,
	) (QueryResponse, error)

	// Retrieve retrieves all or part of a piece with the given retrieval parameters
	Retrieve(
		ctx context.Context,
		payloadCID cid.Cid,
		params Params,
		totalFunds abi.TokenAmount,
		miner peer.ID,
		clientWallet address.Address,
		minerWallet address.Address,
	) (DealID, error)

	// SubscribeToEvents listens for events that happen related to client retrievals
	SubscribeToEvents(subscriber ClientSubscriber) Unsubscribe

	// V1
	AddMoreFunds(id DealID, amount abi.TokenAmount) error
	CancelDeal(id DealID) error
	RetrievalStatus(id DealID)
	ListDeals() map[DealID]ClientDealState
}

// RetrievalClientNode are the node dependencies for a RetrievalClient
type RetrievalClientNode interface {
	GetChainHead(ctx context.Context) (shared.TipSetToken, abi.ChainEpoch, error)

	// GetOrCreatePaymentChannel sets up a new payment channel if one does not exist
	// between a client and a miner and ensures the client has the given amount of funds available in the channel
	GetOrCreatePaymentChannel(ctx context.Context, clientAddress, minerAddress address.Address,
		clientFundsAvailable abi.TokenAmount, tok shared.TipSetToken) (address.Address, cid.Cid, error)

	// Allocate late creates a lane within a payment channel so that calls to
	// CreatePaymentVoucher will automatically make vouchers only for the difference
	// in total
	AllocateLane(paymentChannel address.Address) (uint64, error)

	// CreatePaymentVoucher creates a new payment voucher in the given lane for a
	// given payment channel so that all the payment vouchers in the lane add up
	// to the given amount (so the payment voucher will be for the difference)
	CreatePaymentVoucher(ctx context.Context, paymentChannel address.Address, amount abi.TokenAmount,
		lane uint64, tok shared.TipSetToken) (*paych.SignedVoucher, error)

	// WaitForPaymentChannelAddFunds waits for a message on chain that funds have
	// been sent to a payment channel
	WaitForPaymentChannelAddFunds(messageCID cid.Cid) error

	// WaitForPaymentChannelCreation waits for a message on chain that a
	// payment channel has been created
	WaitForPaymentChannelCreation(messageCID cid.Cid) (address.Address, error)
}

// ProviderDealState is the current state of a deal from the point of view
// of a retrieval provider
type ProviderDealState struct {
	DealProposal
	Status          DealStatus
	Receiver        peer.ID
	TotalSent       uint64
	FundsReceived   abi.TokenAmount
	Message         string
	CurrentInterval uint64
}

// Identifier provides a unique id for this provider deal
func (pds ProviderDealState) Identifier() ProviderDealIdentifier {
	return ProviderDealIdentifier{Receiver: pds.Receiver, DealID: pds.ID}
}

// ProviderDealIdentifier is a value that uniquely identifies a deal
type ProviderDealIdentifier struct {
	Receiver peer.ID
	DealID   DealID
}

func (p ProviderDealIdentifier) String() string {
	return fmt.Sprintf("%v/%v", p.Receiver, p.DealID)
}

// ProviderEvent is an event that occurs in a deal lifecycle on the provider
type ProviderEvent uint64

const (
	// ProviderEventOpen indicates a new deal was received from a client
	ProviderEventOpen ProviderEvent = iota

	// ProviderEventDealReceived means the deal has passed initial checks and is
	// in custom decisioning logic
	ProviderEventDealReceived

	// ProviderEventDecisioningError means the Deciding function returned an error
	ProviderEventDecisioningError

	// ProviderEventWriteResponseFailed happens when a network error occurs writing a deal response
	ProviderEventWriteResponseFailed

	// ProviderEventReadPaymentFailed happens when a network error occurs trying to read a
	// payment from the client
	ProviderEventReadPaymentFailed

	// ProviderEventGetPieceSizeErrored happens when the provider encounters an error
	// looking up the requested pieces size
	ProviderEventGetPieceSizeErrored

	// ProviderEventDealNotFound happens when the provider cannot find the piece for the
	// deal proposed by the client
	ProviderEventDealNotFound

	// ProviderEventDealRejected happens when a provider rejects a deal proposed
	// by the client
	ProviderEventDealRejected

	// ProviderEventDealAccepted happens when a provider accepts a deal
	ProviderEventDealAccepted

	// ProviderEventBlockErrored happens when the provider encounters an error
	// trying to read the next block from the piece
	ProviderEventBlockErrored

	// ProviderEventBlocksCompleted happens when the provider reads the last block
	// in the piece
	ProviderEventBlocksCompleted

	// ProviderEventPaymentRequested happens when a provider asks for payment from
	// a client for blocks sent
	ProviderEventPaymentRequested

	// ProviderEventSaveVoucherFailed happens when an attempt to save a payment
	// voucher fails
	ProviderEventSaveVoucherFailed

	// ProviderEventPartialPaymentReceived happens when a provider receives and processes
	// a payment that is less than what was requested to proceed with the deal
	ProviderEventPartialPaymentReceived

	// ProviderEventPaymentReceived happens when a provider receives a payment
	// and resumes processing a deal
	ProviderEventPaymentReceived

	// ProviderEventComplete indicates a retrieval deal was completed for a client
	ProviderEventComplete
)

// ProviderEvents is a human readable map of provider event name -> event description
var ProviderEvents = map[ProviderEvent]string{
	ProviderEventOpen:                   "ProviderEventOpen",
	ProviderEventDealReceived:           "ProviderEventDealReceived",
	ProviderEventDecisioningError:       "ProviderEventDecisioningError",
	ProviderEventWriteResponseFailed:    "ProviderEventWriteResponseFailed",
	ProviderEventReadPaymentFailed:      "ProviderEventReadPaymentFailed",
	ProviderEventGetPieceSizeErrored:    "ProviderEventGetPieceSizeErrored",
	ProviderEventDealNotFound:           "ProviderEventDealNotFound",
	ProviderEventDealRejected:           "ProviderEventDealRejected",
	ProviderEventDealAccepted:           "ProviderEventDealAccepted",
	ProviderEventBlockErrored:           "ProviderEventBlockErrored",
	ProviderEventBlocksCompleted:        "ProviderEventBlocksCompleted",
	ProviderEventPaymentRequested:       "ProviderEventPaymentRequested",
	ProviderEventSaveVoucherFailed:      "ProviderEventSaveVoucherFailed",
	ProviderEventPartialPaymentReceived: "ProviderEventPartialPaymentReceived",
	ProviderEventPaymentReceived:        "ProviderEventPaymentReceived",
	ProviderEventComplete:               "ProviderEventComplete",
}

// ProviderDealID is a unique identifier for a deal on a provider -- it is
// a combination of DealID set by the client and the peer ID of the client
type ProviderDealID struct {
	From peer.ID
	ID   DealID
}

// ProviderSubscriber is a callback that is registered to listen for retrieval events on a provider
type ProviderSubscriber func(event ProviderEvent, state ProviderDealState)

// RetrievalProvider is an interface by which a provider configures their
// retrieval operations and monitors deals received and process
type RetrievalProvider interface {
	// Start begins listening for deals on the given host
	Start() error

	// Stop stops handling incoming requests
	Stop() error

	// V0

	// SetPricePerByte sets the price per byte a miner charges for retrievals
	SetPricePerByte(price abi.TokenAmount)

	// SetPaymentInterval sets the maximum number of bytes a a provider will send before
	// requesting further payment, and the rate at which that value increases
	SetPaymentInterval(paymentInterval uint64, paymentIntervalIncrease uint64)

	// SubscribeToEvents listens for events that happen related to client retrievals
	SubscribeToEvents(subscriber ProviderSubscriber) Unsubscribe

	// V1
	SetPricePerUnseal(price abi.TokenAmount)
	ListDeals() map[ProviderDealID]ProviderDealState
}

// RetrievalProviderNode are the node depedencies for a RetrevalProvider
type RetrievalProviderNode interface {
	GetChainHead(ctx context.Context) (shared.TipSetToken, abi.ChainEpoch, error)

	// returns the worker address associated with a miner
	GetMinerWorkerAddress(ctx context.Context, miner address.Address, tok shared.TipSetToken) (address.Address, error)
	UnsealSector(ctx context.Context, sectorID uint64, offset uint64, length uint64) (io.ReadCloser, error)
	SavePaymentVoucher(ctx context.Context, paymentChannel address.Address, voucher *paych.SignedVoucher, proof []byte, expectedAmount abi.TokenAmount, tok shared.TipSetToken) (abi.TokenAmount, error)
}

// PeerResolver is an interface for looking up providers that may have a piece
type PeerResolver interface {
	GetPeers(payloadCID cid.Cid) ([]RetrievalPeer, error) // TODO: channel
}

// RetrievalPeer is a provider address/peer.ID pair (everything needed to make
// deals for with a miner)
type RetrievalPeer struct {
	Address address.Address
	ID      peer.ID // optional
}

// QueryResponseStatus indicates whether a queried piece is available
type QueryResponseStatus uint64

const (
	// QueryResponseAvailable indicates a provider has a piece and is prepared to
	// return it
	QueryResponseAvailable QueryResponseStatus = iota

	// QueryResponseUnavailable indicates a provider either does not have or cannot
	// serve the queried piece to the client
	QueryResponseUnavailable

	// QueryResponseError indicates something went wrong generating a query response
	QueryResponseError
)

// QueryItemStatus (V1) indicates whether the requested part of a piece (payload or selector)
// is available for retrieval
type QueryItemStatus uint64

const (
	// QueryItemAvailable indicates requested part of the piece is available to be
	// served
	QueryItemAvailable QueryItemStatus = iota

	// QueryItemUnavailable indicates the piece either does not contain the requested
	// item or it cannot be served
	QueryItemUnavailable

	// QueryItemUnknown indicates the provider cannot determine if the given item
	// is part of the requested piece (for example, if the piece is sealed and the
	// miner does not maintain a payload CID index)
	QueryItemUnknown
)

// QueryParams - V1 - indicate what specific information about a piece that a retrieval
// client is interested in, as well as specific parameters the client is seeking
// for the retrieval deal
type QueryParams struct {
	PieceCID *cid.Cid // optional, query if miner has this cid in this piece. some miners may not be able to respond.
	//Selector                   ipld.Node // optional, query if miner has this cid in this piece. some miners may not be able to respond.
	//MaxPricePerByte            abi.TokenAmount    // optional, tell miner uninterested if more expensive than this
	//MinPaymentInterval         uint64    // optional, tell miner uninterested unless payment interval is greater than this
	//MinPaymentIntervalIncrease uint64    // optional, tell miner uninterested unless payment interval increase is greater than this
}

// Query is a query to a given provider to determine information about a piece
// they may have available for retrieval
type Query struct {
	PayloadCID  cid.Cid // V0
	QueryParams         // V1
}

// QueryUndefined is a query with no values
var QueryUndefined = Query{}

// NewQueryV0 creates a V0 query (which only specifies a piece)
func NewQueryV0(payloadCID cid.Cid) Query {
	return Query{PayloadCID: payloadCID}
}

// QueryResponse is a miners response to a given retrieval query
type QueryResponse struct {
	Status        QueryResponseStatus
	PieceCIDFound QueryItemStatus // V1 - if a PieceCID was requested, the result
	//SelectorFound   QueryItemStatus // V1 - if a Selector was requested, the result

	Size uint64 // Total size of piece in bytes
	//ExpectedPayloadSize uint64 // V1 - optional, if PayloadCID + selector are specified and miner knows, can offer an expected size

	PaymentAddress             address.Address // address to send funds to -- may be different than miner addr
	MinPricePerByte            abi.TokenAmount
	MaxPaymentInterval         uint64
	MaxPaymentIntervalIncrease uint64
	Message                    string
}

// QueryResponseUndefined is an empty QueryResponse
var QueryResponseUndefined = QueryResponse{}

// PieceRetrievalPrice is the total price to retrieve the piece (size * MinPricePerByte)
func (qr QueryResponse) PieceRetrievalPrice() abi.TokenAmount {
	return big.Mul(qr.MinPricePerByte, abi.NewTokenAmount(int64(qr.Size)))
}

// PayloadRetrievalPrice is the expected price to retrieve just the given payload
// & selector (V1)
//func (qr QueryResponse) PayloadRetrievalPrice() abi.TokenAmount {
//	return types.BigMul(qr.MinPricePerByte, types.NewInt(qr.ExpectedPayloadSize))
//}

// DealStatus is the status of a retrieval deal returned by a provider
// in a DealResponse
type DealStatus uint64

const (
	// DealStatusNew is a deal that nothing has happened with yet
	DealStatusNew DealStatus = iota

	// DealStatusPaymentChannelCreating is the status set while waiting for the
	// payment channel creation to complete
	DealStatusPaymentChannelCreating

	// DealStatusPaymentChannelAddingFunds is the status when we are waiting for funds
	// to finish being sent to the payment channel
	DealStatusPaymentChannelAddingFunds

	// DealStatusPaymentChannelAllocatingLane is the status during lane allocation
	DealStatusPaymentChannelAllocatingLane

	// DealStatusPaymentChannelReady is a deal status that has a payment channel
	// & lane setup
	DealStatusPaymentChannelReady

	// DealStatusAwaitingAcceptance - deal is waiting for the decider function to finish
	DealStatusAwaitingAcceptance

	// DealStatusAccepted means a deal has been accepted by a provider
	// and its is ready to proceed with retrieval
	DealStatusAccepted

	// DealStatusFailed indicates something went wrong during a retrieval
	DealStatusFailed

	// DealStatusRejected indicates the provider rejected a client's deal proposal
	// for some reason
	DealStatusRejected

	// DealStatusFundsNeeded indicates the provider needs a payment voucher to
	// continue processing the deal
	DealStatusFundsNeeded

	// DealStatusOngoing indicates the provider is continuing to process a deal
	DealStatusOngoing

	// DealStatusFundsNeededLastPayment indicates the provider needs a payment voucher
	// in order to complete a deal
	DealStatusFundsNeededLastPayment

	// DealStatusCompleted indicates a deal is complete
	DealStatusCompleted

	// DealStatusDealNotFound indicates an update was received for a deal that could
	// not be identified
	DealStatusDealNotFound

	// DealStatusVerified means a deal has been verified as having the right parameters
	DealStatusVerified

	// DealStatusErrored indicates something went wrong with a deal
	DealStatusErrored

	// DealStatusBlocksComplete indicates that all blocks have been processed for the piece
	DealStatusBlocksComplete

	// DealStatusFinalizing means the last payment has been received and
	// we are just confirming the deal is complete
	DealStatusFinalizing
)

// DealStatuses maps deal status to a human readable representation
var DealStatuses = map[DealStatus]string{
	DealStatusNew:                          "DealStatusNew",
	DealStatusPaymentChannelCreating:       "DealStatusPaymentChannelCreating",
	DealStatusPaymentChannelAddingFunds:    "DealStatusPaymentChannelAddingFunds",
	DealStatusPaymentChannelAllocatingLane: "DealStatusPaymentChannelAllocatingLane",
	DealStatusPaymentChannelReady:          "DealStatusPaymentChannelReady",
	DealStatusAwaitingAcceptance:           "DealStatusAwaitingAcceptance",
	DealStatusAccepted:                     "DealStatusAccepted",
	DealStatusFailed:                       "DealStatusFailed",
	DealStatusRejected:                     "DealStatusRejected",
	DealStatusFundsNeeded:                  "DealStatusFundsNeeded",
	DealStatusOngoing:                      "DealStatusOngoing",
	DealStatusFundsNeededLastPayment:       "DealStatusFundsNeededLastPayment",
	DealStatusCompleted:                    "DealStatusCompleted",
	DealStatusDealNotFound:                 "DealStatusDealNotFound",
	DealStatusVerified:                     "DealStatusVerified",
	DealStatusErrored:                      "DealStatusErrored",
	DealStatusBlocksComplete:               "DealStatusBlocksComplete",
	DealStatusFinalizing:                   "DealStatusFinalizing",
}

// IsTerminalError returns true if this status indicates processing of this deal
// is complete with an error
func IsTerminalError(status DealStatus) bool {
	return status == DealStatusDealNotFound ||
		status == DealStatusFailed ||
		status == DealStatusRejected
}

// IsTerminalSuccess returns true if this status indicates processing of this deal
// is complete with a success
func IsTerminalSuccess(status DealStatus) bool {
	return status == DealStatusCompleted
}

// IsTerminalStatus returns true if this status indicates processing of a deal is
// complete (either success or error)
func IsTerminalStatus(status DealStatus) bool {
	return IsTerminalError(status) || IsTerminalSuccess(status)
}

// Params are the parameters requested for a retrieval deal proposal
type Params struct {
	Selector                *cbg.Deferred // V1
	PieceCID                *cid.Cid
	PricePerByte            abi.TokenAmount
	PaymentInterval         uint64 // when to request payment
	PaymentIntervalIncrease uint64 //
}

// NewParamsV0 generates parameters for a retrieval deal, which is always a whole piece deal
func NewParamsV0(pricePerByte abi.TokenAmount, paymentInterval uint64, paymentIntervalIncrease uint64) Params {
	return Params{
		PricePerByte:            pricePerByte,
		PaymentInterval:         paymentInterval,
		PaymentIntervalIncrease: paymentIntervalIncrease,
	}
}

// NewParamsV1 generates parameters for a retrieval deal, including a selector
func NewParamsV1(pricePerByte abi.TokenAmount, paymentInterval uint64, paymentIntervalIncrease uint64, sel ipld.Node, pieceCid *cid.Cid) Params {
	var buffer bytes.Buffer
	err := dagcbor.Encoder(sel, &buffer)
	if err != nil {
		return Params{}
	}

	return Params{
		Selector:                &cbg.Deferred{Raw: buffer.Bytes()},
		PieceCID:                pieceCid,
		PricePerByte:            pricePerByte,
		PaymentInterval:         paymentInterval,
		PaymentIntervalIncrease: paymentIntervalIncrease,
	}
}

// DealID is an identifier for a retrieval deal (unique to a client)
type DealID uint64

func (d DealID) String() string {
	return fmt.Sprintf("%d", d)
}

// DealProposal is a proposal for a new retrieval deal
type DealProposal struct {
	PayloadCID cid.Cid
	ID         DealID
	Params
}

// DealProposalUndefined is an undefined deal proposal
var DealProposalUndefined = DealProposal{}

// Block is an IPLD block in bitswap format
type Block struct {
	Prefix []byte
	Data   []byte
}

// EmptyBlock is just a block with no content
var EmptyBlock = Block{}

// DealResponse is a response to a retrieval deal proposal
type DealResponse struct {
	Status DealStatus
	ID     DealID

	// payment required to proceed
	PaymentOwed abi.TokenAmount

	Message string
	Blocks  []Block // V0 only
}

// DealResponseUndefined is an undefined deal response
var DealResponseUndefined = DealResponse{}

// DealPayment is a payment for an in progress retrieval deal
type DealPayment struct {
	ID             DealID
	PaymentChannel address.Address
	PaymentVoucher *paych.SignedVoucher
}

// DealPaymentUndefined is an undefined deal payment
var DealPaymentUndefined = DealPayment{}

var (
	// ErrNotFound means a piece was not found during retrieval
	ErrNotFound = errors.New("not found")

	// ErrVerification means a retrieval contained a block response that did not verify
	ErrVerification = errors.New("Error when verify data")
)
