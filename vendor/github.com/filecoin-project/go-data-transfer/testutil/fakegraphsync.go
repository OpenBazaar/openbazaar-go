package testutil

import (
	"context"
	"math/rand"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"
)

// ReceivedGraphSyncRequest contains data about a received graphsync request
type ReceivedGraphSyncRequest struct {
	P          peer.ID
	Root       ipld.Link
	Selector   ipld.Node
	Extensions []graphsync.ExtensionData
}

// FakeGraphSync implements a GraphExchange but does nothing
type FakeGraphSync struct {
	requests                  chan ReceivedGraphSyncRequest // records calls to fakeGraphSync.Request
	OutgoingRequestHook       graphsync.OnOutgoingRequestHook
	IncomingBlockHook         graphsync.OnIncomingBlockHook
	OutgoingBlockHook         graphsync.OnOutgoingBlockHook
	IncomingRequestHook       graphsync.OnIncomingRequestHook
	ResponseCompletedListener graphsync.OnResponseCompletedListener
}

// NewFakeGraphSync returns a new fake graphsync implementation
func NewFakeGraphSync() *FakeGraphSync {
	return &FakeGraphSync{
		requests: make(chan ReceivedGraphSyncRequest, 1),
	}
}

// AssertNoRequestReceived asserts that no requests should ahve been received by this graphsync implementation
func (fgs *FakeGraphSync) AssertNoRequestReceived(t *testing.T) {
	require.Empty(t, fgs.requests, "should not receive request")
}

// AssertRequestReceived asserts a request should be received before the context closes (and returns said request)
func (fgs *FakeGraphSync) AssertRequestReceived(ctx context.Context, t *testing.T) ReceivedGraphSyncRequest {
	var requestReceived ReceivedGraphSyncRequest
	select {
	case <-ctx.Done():
		t.Fatal("did not receive message sent")
	case requestReceived = <-fgs.requests:
	}
	return requestReceived
}

// Request initiates a new GraphSync request to the given peer using the given selector spec.
func (fgs *FakeGraphSync) Request(ctx context.Context, p peer.ID, root ipld.Link, selector ipld.Node, extensions ...graphsync.ExtensionData) (<-chan graphsync.ResponseProgress, <-chan error) {

	fgs.requests <- ReceivedGraphSyncRequest{p, root, selector, extensions}
	responses := make(chan graphsync.ResponseProgress)
	errors := make(chan error)
	close(responses)
	close(errors)
	return responses, errors
}

// RegisterPersistenceOption registers an alternate loader/storer combo that can be substituted for the default
func (fgs *FakeGraphSync) RegisterPersistenceOption(name string, loader ipld.Loader, storer ipld.Storer) error {
	return nil
}

// RegisterIncomingRequestHook adds a hook that runs when a request is received
func (fgs *FakeGraphSync) RegisterIncomingRequestHook(hook graphsync.OnIncomingRequestHook) graphsync.UnregisterHookFunc {
	fgs.IncomingRequestHook = hook
	return nil
}

// RegisterIncomingResponseHook adds a hook that runs when a response is received
func (fgs *FakeGraphSync) RegisterIncomingResponseHook(_ graphsync.OnIncomingResponseHook) graphsync.UnregisterHookFunc {
	return nil
}

// RegisterOutgoingRequestHook adds a hook that runs immediately prior to sending a new request
func (fgs *FakeGraphSync) RegisterOutgoingRequestHook(hook graphsync.OnOutgoingRequestHook) graphsync.UnregisterHookFunc {
	fgs.OutgoingRequestHook = hook
	return nil
}

// RegisterOutgoingBlockHook adds a hook that runs every time a block is sent from a responder
func (fgs *FakeGraphSync) RegisterOutgoingBlockHook(hook graphsync.OnOutgoingBlockHook) graphsync.UnregisterHookFunc {
	fgs.OutgoingBlockHook = hook
	return nil
}

// RegisterIncomingBlockHook adds a hook that runs every time a block is received by the requestor
func (fgs *FakeGraphSync) RegisterIncomingBlockHook(hook graphsync.OnIncomingBlockHook) graphsync.UnregisterHookFunc {
	fgs.IncomingBlockHook = hook
	return nil
}

// RegisterRequestUpdatedHook adds a hook that runs every time an update to a request is received
func (fgs *FakeGraphSync) RegisterRequestUpdatedHook(hook graphsync.OnRequestUpdatedHook) graphsync.UnregisterHookFunc {
	return nil
}

// RegisterCompletedResponseListener adds a listener on the responder for completed responses
func (fgs *FakeGraphSync) RegisterCompletedResponseListener(listener graphsync.OnResponseCompletedListener) graphsync.UnregisterHookFunc {
	fgs.ResponseCompletedListener = listener
	return nil
}

// UnpauseResponse unpauses a response that was paused in a block hook based on peer ID and request ID
func (fgs *FakeGraphSync) UnpauseResponse(_ peer.ID, _ graphsync.RequestID) error {
	return nil
}

var _ graphsync.GraphExchange = &FakeGraphSync{}

type fakeBlkData struct {
	link ipld.Link
	size uint64
}

func (fbd fakeBlkData) Link() ipld.Link {
	return fbd.link
}

func (fbd fakeBlkData) BlockSize() uint64 {
	return fbd.size
}

func (fbd fakeBlkData) BlockSizeOnWire() uint64 {
	return fbd.size
}

// NewFakeBlockData returns a fake block that matches the block data interface
func NewFakeBlockData() graphsync.BlockData {
	return &fakeBlkData{
		link: cidlink.Link{Cid: GenerateCids(1)[0]},
		size: rand.Uint64(),
	}
}

type fakeRequest struct {
	id         graphsync.RequestID
	root       cid.Cid
	selector   ipld.Node
	priority   graphsync.Priority
	isCancel   bool
	extensions map[graphsync.ExtensionName][]byte
}

// ID Returns the request ID for this Request
func (fr *fakeRequest) ID() graphsync.RequestID {
	return fr.id
}

// Root returns the CID to the root block of this request
func (fr *fakeRequest) Root() cid.Cid {
	return fr.root
}

// Selector returns the byte representation of the selector for this request
func (fr *fakeRequest) Selector() ipld.Node {
	return fr.selector
}

// Priority returns the priority of this request
func (fr *fakeRequest) Priority() graphsync.Priority {
	return fr.priority
}

// Extension returns the content for an extension on a response, or errors
// if extension is not present
func (fr *fakeRequest) Extension(name graphsync.ExtensionName) ([]byte, bool) {
	data, has := fr.extensions[name]
	return data, has
}

// IsCancel returns true if this particular request is being cancelled
func (fr *fakeRequest) IsCancel() bool {
	return fr.isCancel
}

// NewFakeRequest returns a fake request that matches the request data interface
func NewFakeRequest(id graphsync.RequestID, extensions map[graphsync.ExtensionName][]byte) graphsync.RequestData {
	return &fakeRequest{
		id:         id,
		root:       GenerateCids(1)[0],
		selector:   allSelector,
		priority:   graphsync.Priority(rand.Int()),
		isCancel:   false,
		extensions: extensions,
	}
}

type fakeResponse struct {
	id         graphsync.RequestID
	status     graphsync.ResponseStatusCode
	extensions map[graphsync.ExtensionName][]byte
}

// RequestID returns the request ID for this response
func (fr *fakeResponse) RequestID() graphsync.RequestID {
	return fr.id
}

// Status returns the status for a response
func (fr *fakeResponse) Status() graphsync.ResponseStatusCode {
	return fr.status
}

// Extension returns the content for an extension on a response, or errors
// if extension is not present
func (fr *fakeResponse) Extension(name graphsync.ExtensionName) ([]byte, bool) {
	data, has := fr.extensions[name]
	return data, has
}

// NewFakeResponse returns a fake response that matches the response data interface
func NewFakeResponse(id graphsync.RequestID, extensions map[graphsync.ExtensionName][]byte, status graphsync.ResponseStatusCode) graphsync.ResponseData {
	return &fakeResponse{
		id:         id,
		status:     status,
		extensions: extensions,
	}
}
