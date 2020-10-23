package graphsync

import (
	"context"
	"errors"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/traversal"
	"github.com/libp2p/go-libp2p-core/peer"
)

// RequestID is a unique identifier for a GraphSync request.
type RequestID int32

// Priority a priority for a GraphSync request.
type Priority int32

// ResponseStatusCode is a status returned for a GraphSync Request.
type ResponseStatusCode int32

// ExtensionName is a name for a GraphSync extension
type ExtensionName string

// ExtensionData is a name/data pair for a graphsync extension
type ExtensionData struct {
	Name ExtensionName
	Data []byte
}

const (

	// Known Graphsync Extensions

	// ExtensionMetadata provides response metadata for a Graphsync request and is
	// documented at
	// https://github.com/ipld/specs/blob/master/block-layer/graphsync/known_extensions.md
	ExtensionMetadata = ExtensionName("graphsync/response-metadata")

	// ExtensionDoNotSendCIDs tells the responding peer not to send certain blocks if they
	// are encountered in a traversal and is documented at
	// https://github.com/ipld/specs/blob/master/block-layer/graphsync/known_extensions.md
	ExtensionDoNotSendCIDs = ExtensionName("graphsync/do-not-send-cids")

	// ExtensionDeDupByKey tells the responding peer to only deduplicate block sending
	// for requests that have the same key. The data for the extension is a string key
	ExtensionDeDupByKey = ExtensionName("graphsync/dedup-by-key")

	// GraphSync Response Status Codes

	// Informational Response Codes (partial)

	// RequestAcknowledged means the request was received and is being worked on.
	RequestAcknowledged = ResponseStatusCode(10)
	// AdditionalPeers means additional peers were found that may be able
	// to satisfy the request and contained in the extra block of the response.
	AdditionalPeers = ResponseStatusCode(11)
	// NotEnoughGas means fulfilling this request requires payment.
	NotEnoughGas = ResponseStatusCode(12)
	// OtherProtocol means a different type of response than GraphSync is
	// contained in extra.
	OtherProtocol = ResponseStatusCode(13)
	// PartialResponse may include blocks and metadata about the in progress response
	// in extra.
	PartialResponse = ResponseStatusCode(14)
	// RequestPaused indicates a request is paused and will not send any more data
	// until unpaused
	RequestPaused = ResponseStatusCode(15)

	// Success Response Codes (request terminated)

	// RequestCompletedFull means the entire fulfillment of the GraphSync request
	// was sent back.
	RequestCompletedFull = ResponseStatusCode(20)
	// RequestCompletedPartial means the response is completed, and part of the
	// GraphSync request was sent back, but not the complete request.
	RequestCompletedPartial = ResponseStatusCode(21)

	// Error Response Codes (request terminated)

	// RequestRejected means the node did not accept the incoming request.
	RequestRejected = ResponseStatusCode(30)
	// RequestFailedBusy means the node is too busy, try again later. Backoff may
	// be contained in extra.
	RequestFailedBusy = ResponseStatusCode(31)
	// RequestFailedUnknown means the request failed for an unspecified reason. May
	// contain data about why in extra.
	RequestFailedUnknown = ResponseStatusCode(32)
	// RequestFailedLegal means the request failed for legal reasons.
	RequestFailedLegal = ResponseStatusCode(33)
	// RequestFailedContentNotFound means the respondent does not have the content.
	RequestFailedContentNotFound = ResponseStatusCode(34)
	// RequestCancelled means the responder was processing the request but decided to top, for whatever reason
	RequestCancelled = ResponseStatusCode(35)
)

// RequestContextCancelledErr is an error message received on the error channel when the request context given by the user is cancelled/times out
type RequestContextCancelledErr struct{}

func (e RequestContextCancelledErr) Error() string {
	return "Request Context Cancelled"
}

// RequestFailedBusyErr is an error message received on the error channel when the peer is busy
type RequestFailedBusyErr struct{}

func (e RequestFailedBusyErr) Error() string {
	return "Request Failed - Peer Is Busy"
}

// RequestFailedContentNotFoundErr is an error message received on the error channel when the content is not found
type RequestFailedContentNotFoundErr struct{}

func (e RequestFailedContentNotFoundErr) Error() string {
	return "Request Failed - Content Not Found"
}

// RequestFailedLegalErr is an error message received on the error channel when the request fails for legal reasons
type RequestFailedLegalErr struct{}

func (e RequestFailedLegalErr) Error() string {
	return "Request Failed - For Legal Reasons"
}

// RequestFailedUnknownErr is an error message received on the error channel when the request fails for unknown reasons
type RequestFailedUnknownErr struct{}

func (e RequestFailedUnknownErr) Error() string {
	return "Request Failed - Unknown Reason"
}

// RequestCancelledErr is an error message received on the error channel that indicates the responder cancelled a request
type RequestCancelledErr struct{}

func (e RequestCancelledErr) Error() string {
	return "Request Failed - Responder Cancelled"
}

var (
	// ErrExtensionAlreadyRegistered means a user extension can be registered only once
	ErrExtensionAlreadyRegistered = errors.New("extension already registered")
)

// ResponseProgress is the fundamental unit of responses making progress in Graphsync.
type ResponseProgress struct {
	Node      ipld.Node // a node which matched the graphsync query
	Path      ipld.Path // the path of that node relative to the traversal start
	LastBlock struct {  // LastBlock stores the Path and Link of the last block edge we had to load.
		Path ipld.Path
		Link ipld.Link
	}
}

// RequestData describes a received graphsync request.
type RequestData interface {
	// ID Returns the request ID for this Request
	ID() RequestID

	// Root returns the CID to the root block of this request
	Root() cid.Cid

	// Selector returns the byte representation of the selector for this request
	Selector() ipld.Node

	// Priority returns the priority of this request
	Priority() Priority

	// Extension returns the content for an extension on a response, or errors
	// if extension is not present
	Extension(name ExtensionName) ([]byte, bool)

	// IsCancel returns true if this particular request is being cancelled
	IsCancel() bool
}

// ResponseData describes a received Graphsync response
type ResponseData interface {
	// RequestID returns the request ID for this response
	RequestID() RequestID

	// Status returns the status for a response
	Status() ResponseStatusCode

	// Extension returns the content for an extension on a response, or errors
	// if extension is not present
	Extension(name ExtensionName) ([]byte, bool)
}

// BlockData gives information about a block included in a graphsync response
type BlockData interface {
	// Link is the link/cid for the block
	Link() ipld.Link

	// BlockSize specifies the size of the block
	BlockSize() uint64

	// BlockSize specifies the amount of data actually transmitted over the network
	BlockSizeOnWire() uint64
}

// IncomingRequestHookActions are actions that a request hook can take to change
// behavior for the response
type IncomingRequestHookActions interface {
	SendExtensionData(ExtensionData)
	UsePersistenceOption(name string)
	UseLinkTargetNodePrototypeChooser(traversal.LinkTargetNodePrototypeChooser)
	TerminateWithError(error)
	ValidateRequest()
	PauseResponse()
}

// OutgoingBlockHookActions are actions that an outgoing block hook can take to
// change the execution of a request
type OutgoingBlockHookActions interface {
	SendExtensionData(ExtensionData)
	TerminateWithError(error)
	PauseResponse()
}

// OutgoingRequestHookActions are actions that an outgoing request hook can take
// to change the execution of a request
type OutgoingRequestHookActions interface {
	UsePersistenceOption(name string)
	UseLinkTargetNodePrototypeChooser(traversal.LinkTargetNodePrototypeChooser)
}

// IncomingResponseHookActions are actions that incoming response hook can take
// to change the execution of a request
type IncomingResponseHookActions interface {
	TerminateWithError(error)
	UpdateRequestWithExtensions(...ExtensionData)
}

// IncomingBlockHookActions are actions that incoming block hook can take
// to change the execution of a request
type IncomingBlockHookActions interface {
	TerminateWithError(error)
	UpdateRequestWithExtensions(...ExtensionData)
	PauseRequest()
}

// RequestUpdatedHookActions are actions that can be taken in a request updated hook to
// change execution of the response
type RequestUpdatedHookActions interface {
	TerminateWithError(error)
	SendExtensionData(ExtensionData)
	UnpauseResponse()
}

// OnIncomingRequestHook is a hook that runs each time a new request is received.
// It receives the peer that sent the request and all data about the request.
// It receives an interface for customizing the response to this request
type OnIncomingRequestHook func(p peer.ID, request RequestData, hookActions IncomingRequestHookActions)

// OnIncomingResponseHook is a hook that runs each time a new response is received.
// It receives the peer that sent the response and all data about the response.
// It receives an interface for customizing how we handle the ongoing execution of the request
type OnIncomingResponseHook func(p peer.ID, responseData ResponseData, hookActions IncomingResponseHookActions)

// OnIncomingBlockHook is a hook that runs each time a new block is validated as
// part of the response, regardless of whether it came locally or over the network
// It receives that sent the response, the most recent response, a link for the block received,
// and the size of the block received
// The difference between BlockSize & BlockSizeOnWire can be used to determine
// where the block came from (Local vs remote)
// It receives an interface for customizing how we handle the ongoing execution of the request
type OnIncomingBlockHook func(p peer.ID, responseData ResponseData, blockData BlockData, hookActions IncomingBlockHookActions)

// OnOutgoingRequestHook is a hook that runs immediately prior to sending a request
// It receives the peer we're sending a request to and all the data aobut the request
// It receives an interface for customizing how we handle executing this request
type OnOutgoingRequestHook func(p peer.ID, request RequestData, hookActions OutgoingRequestHookActions)

// OnOutgoingBlockHook is a hook that runs immediately after a requestor sends a new block
// on a response
// It receives the peer we're sending a request to, all the data aobut the request, a link for the block sent,
// and the size of the block sent
// It receives an interface for taking further action on the response
type OnOutgoingBlockHook func(p peer.ID, request RequestData, block BlockData, hookActions OutgoingBlockHookActions)

// OnRequestUpdatedHook is a hook that runs when an update to a request is received
// It receives the peer we're sending to, the original request, the request update
// It receives an interface to taking further action on the response
type OnRequestUpdatedHook func(p peer.ID, request RequestData, updateRequest RequestData, hookActions RequestUpdatedHookActions)

// OnBlockSentListener runs when a block is sent over the wire
type OnBlockSentListener func(p peer.ID, request RequestData, block BlockData)

// OnNetworkErrorListener runs when queued data is not able to be sent
type OnNetworkErrorListener func(p peer.ID, request RequestData, err error)

// OnResponseCompletedListener provides a way to listen for when responder has finished serving a response
type OnResponseCompletedListener func(p peer.ID, request RequestData, status ResponseStatusCode)

// OnRequestorCancelledListener provides a way to listen for responses the requestor canncels
type OnRequestorCancelledListener func(p peer.ID, request RequestData)

// UnregisterHookFunc is a function call to unregister a hook that was previously registered
type UnregisterHookFunc func()

// GraphExchange is a protocol that can exchange IPLD graphs based on a selector
type GraphExchange interface {
	// Request initiates a new GraphSync request to the given peer using the given selector spec.
	Request(ctx context.Context, p peer.ID, root ipld.Link, selector ipld.Node, extensions ...ExtensionData) (<-chan ResponseProgress, <-chan error)

	// RegisterPersistenceOption registers an alternate loader/storer combo that can be substituted for the default
	RegisterPersistenceOption(name string, loader ipld.Loader, storer ipld.Storer) error

	// UnregisterPersistenceOption unregisters an alternate loader/storer combo
	UnregisterPersistenceOption(name string) error

	// RegisterIncomingRequestHook adds a hook that runs when a request is received
	RegisterIncomingRequestHook(hook OnIncomingRequestHook) UnregisterHookFunc

	// RegisterIncomingResponseHook adds a hook that runs when a response is received
	RegisterIncomingResponseHook(OnIncomingResponseHook) UnregisterHookFunc

	// RegisterIncomingBlockHook adds a hook that runs when a block is received and validated (put in block store)
	RegisterIncomingBlockHook(OnIncomingBlockHook) UnregisterHookFunc

	// RegisterOutgoingRequestHook adds a hook that runs immediately prior to sending a new request
	RegisterOutgoingRequestHook(hook OnOutgoingRequestHook) UnregisterHookFunc

	// RegisterOutgoingBlockHook adds a hook that runs every time a block is sent from a responder
	RegisterOutgoingBlockHook(hook OnOutgoingBlockHook) UnregisterHookFunc

	// RegisterRequestUpdatedHook adds a hook that runs every time an update to a request is received
	RegisterRequestUpdatedHook(hook OnRequestUpdatedHook) UnregisterHookFunc

	// RegisterCompletedResponseListener adds a listener on the responder for completed responses
	RegisterCompletedResponseListener(listener OnResponseCompletedListener) UnregisterHookFunc

	// RegisterRequestorCancelledListener adds a listener on the responder for
	// responses cancelled by the requestor
	RegisterRequestorCancelledListener(listener OnRequestorCancelledListener) UnregisterHookFunc

	// RegisterBlockSentListener adds a listener for when blocks are actually sent over the wire
	RegisterBlockSentListener(listener OnBlockSentListener) UnregisterHookFunc

	// RegisterNetworkErrorListener adds a listener for when errors occur sending data over the wire
	RegisterNetworkErrorListener(listener OnNetworkErrorListener) UnregisterHookFunc

	// UnpauseRequest unpauses a request that was paused in a block hook based request ID
	// Can also send extensions with unpause
	UnpauseRequest(RequestID, ...ExtensionData) error

	// PauseRequest pauses an in progress request (may take 1 or more blocks to process)
	PauseRequest(RequestID) error

	// UnpauseResponse unpauses a response that was paused in a block hook based on peer ID and request ID
	// Can also send extensions with unpause
	UnpauseResponse(peer.ID, RequestID, ...ExtensionData) error

	// PauseResponse pauses an in progress response (may take 1 or more blocks to process)
	PauseResponse(peer.ID, RequestID) error

	// CancelResponse cancels an in progress response
	CancelResponse(peer.ID, RequestID) error
}
