package hooks_test

import (
	"bytes"
	"errors"
	"math/rand"
	"testing"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-data-transfer/impl/graphsync/extension"
	"github.com/filecoin-project/go-data-transfer/impl/graphsync/hooks"
	"github.com/filecoin-project/go-data-transfer/testutil"
	"github.com/ipfs/go-graphsync"
	ipld "github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/traversal"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	testCases := map[string]struct {
		makeRequest  func(id graphsync.RequestID, chid datatransfer.ChannelID) graphsync.RequestData
		makeResponse func(id graphsync.RequestID, chid datatransfer.ChannelID) graphsync.ResponseData
		events       fakeEvents
		action       func(gsData *graphsyncTestData)
		check        func(t *testing.T, events *fakeEvents, gsData *graphsyncTestData)
	}{
		"recognized outgoing request will record incoming blocks": {
			action: func(gsData *graphsyncTestData) {
				gsData.outgoingRequestHook()
				gsData.incomingBlockHook()
			},
			check: func(t *testing.T, events *fakeEvents, gsData *graphsyncTestData) {
				require.True(t, events.OnRequestSentCalled)
				require.True(t, events.OnDataReceivedCalled)
				require.NoError(t, gsData.incomingBlockHookActions.TerminationError)
			},
		},
		"non-data-transfer outgoing request will not record incoming blocks": {
			makeRequest: func(id graphsync.RequestID, chid datatransfer.ChannelID) graphsync.RequestData {
				return testutil.NewFakeRequest(id, map[graphsync.ExtensionName][]byte{})
			},
			action: func(gsData *graphsyncTestData) {
				gsData.outgoingRequestHook()
				gsData.incomingBlockHook()
			},
			check: func(t *testing.T, events *fakeEvents, gsData *graphsyncTestData) {
				require.False(t, events.OnRequestSentCalled)
				require.False(t, events.OnDataReceivedCalled)
				require.NoError(t, gsData.incomingBlockHookActions.TerminationError)
			},
		},
		"unrecognized outgoing request will not record incoming blocks": {
			events: fakeEvents{
				OnRequestSentError: errors.New("Not recognized"),
			},
			action: func(gsData *graphsyncTestData) {
				gsData.outgoingRequestHook()
				gsData.incomingBlockHook()
			},
			check: func(t *testing.T, events *fakeEvents, gsData *graphsyncTestData) {
				require.True(t, events.OnRequestSentCalled)
				require.False(t, events.OnDataReceivedCalled)
				require.NoError(t, gsData.incomingBlockHookActions.TerminationError)
			},
		},
		"incoming block error will halt request": {
			events: fakeEvents{
				OnDataReceivedError: errors.New("something went wrong"),
			},
			action: func(gsData *graphsyncTestData) {
				gsData.outgoingRequestHook()
				gsData.incomingBlockHook()
			},
			check: func(t *testing.T, events *fakeEvents, gsData *graphsyncTestData) {
				require.True(t, events.OnRequestSentCalled)
				require.True(t, events.OnDataReceivedCalled)
				require.Error(t, gsData.incomingBlockHookActions.TerminationError)
			},
		},
		"recognized incoming request will validate request": {
			action: func(gsData *graphsyncTestData) {
				gsData.incomingRequestHook()
			},
			check: func(t *testing.T, events *fakeEvents, gsData *graphsyncTestData) {
				require.True(t, events.OnRequestReceivedCalled)
				require.True(t, gsData.incomingRequestHookActions.Validated)
				require.Equal(t, extension.ExtensionDataTransfer, gsData.incomingRequestHookActions.SentExtension.Name)
				require.NoError(t, gsData.incomingRequestHookActions.TerminationError)
			},
		},
		"malformed data transfer extension on incoming request will terminate": {
			makeRequest: func(id graphsync.RequestID, chid datatransfer.ChannelID) graphsync.RequestData {
				return testutil.NewFakeRequest(id, map[graphsync.ExtensionName][]byte{
					extension.ExtensionDataTransfer: testutil.RandomBytes(100),
				})
			},
			action: func(gsData *graphsyncTestData) {
				gsData.incomingRequestHook()
			},
			check: func(t *testing.T, events *fakeEvents, gsData *graphsyncTestData) {
				require.False(t, events.OnRequestReceivedCalled)
				require.False(t, gsData.incomingRequestHookActions.Validated)
				require.Error(t, gsData.incomingRequestHookActions.TerminationError)
			},
		},
		"unrecognized incoming data transfer request will terminate": {
			events: fakeEvents{
				OnRequestReceivedError: errors.New("something went wrong"),
			},
			action: func(gsData *graphsyncTestData) {
				gsData.incomingRequestHook()
			},
			check: func(t *testing.T, events *fakeEvents, gsData *graphsyncTestData) {
				require.True(t, events.OnRequestReceivedCalled)
				require.False(t, gsData.incomingRequestHookActions.Validated)
				require.Error(t, gsData.incomingRequestHookActions.TerminationError)
			},
		},
		"recognized incoming request will record outgoing blocks": {
			action: func(gsData *graphsyncTestData) {
				gsData.incomingRequestHook()
				gsData.outgoingBlockHook()
			},
			check: func(t *testing.T, events *fakeEvents, gsData *graphsyncTestData) {
				require.True(t, events.OnRequestReceivedCalled)
				require.True(t, events.OnDataSentCalled)
				require.NoError(t, gsData.outgoingBlockHookActions.TerminationError)
			},
		},
		"non-data-transfer request will not record outgoing blocks": {
			makeRequest: func(id graphsync.RequestID, chid datatransfer.ChannelID) graphsync.RequestData {
				return testutil.NewFakeRequest(id, map[graphsync.ExtensionName][]byte{})
			},
			action: func(gsData *graphsyncTestData) {
				gsData.incomingRequestHook()
				gsData.outgoingBlockHook()
			},
			check: func(t *testing.T, events *fakeEvents, gsData *graphsyncTestData) {
				require.False(t, events.OnRequestReceivedCalled)
				require.False(t, events.OnDataSentCalled)
			},
		},
		"outgoing data send error will terminate request": {
			events: fakeEvents{
				OnDataSentError: errors.New("something went wrong"),
			},
			action: func(gsData *graphsyncTestData) {
				gsData.incomingRequestHook()
				gsData.outgoingBlockHook()
			},
			check: func(t *testing.T, events *fakeEvents, gsData *graphsyncTestData) {
				require.True(t, events.OnRequestReceivedCalled)
				require.True(t, events.OnDataSentCalled)
				require.Error(t, gsData.outgoingBlockHookActions.TerminationError)
			},
		},
		"recognized incoming request will record successful request completion": {
			makeResponse: func(id graphsync.RequestID, chid datatransfer.ChannelID) graphsync.ResponseData {
				return testutil.NewFakeResponse(id, map[graphsync.ExtensionName][]byte{}, graphsync.RequestCompletedFull)
			},
			action: func(gsData *graphsyncTestData) {
				gsData.incomingRequestHook()
				gsData.responseCompletedListener()
			},
			check: func(t *testing.T, events *fakeEvents, gsData *graphsyncTestData) {
				require.True(t, events.OnRequestReceivedCalled)
				require.True(t, events.OnResponseCompletedCalled)
				require.True(t, events.ResponseSuccess)
			},
		},
		"recognized incoming request will record unsuccessful request completion": {
			makeResponse: func(id graphsync.RequestID, chid datatransfer.ChannelID) graphsync.ResponseData {
				return testutil.NewFakeResponse(id, map[graphsync.ExtensionName][]byte{}, graphsync.RequestCompletedPartial)
			},
			action: func(gsData *graphsyncTestData) {
				gsData.incomingRequestHook()
				gsData.responseCompletedListener()
			},
			check: func(t *testing.T, events *fakeEvents, gsData *graphsyncTestData) {
				require.True(t, events.OnRequestReceivedCalled)
				require.True(t, events.OnResponseCompletedCalled)
				require.False(t, events.ResponseSuccess)
			},
		},
		"non-data-transfer request will not record request completed": {
			makeRequest: func(id graphsync.RequestID, chid datatransfer.ChannelID) graphsync.RequestData {
				return testutil.NewFakeRequest(id, map[graphsync.ExtensionName][]byte{})
			},
			action: func(gsData *graphsyncTestData) {
				gsData.incomingRequestHook()
				gsData.responseCompletedListener()
			},
			check: func(t *testing.T, events *fakeEvents, gsData *graphsyncTestData) {
				require.False(t, events.OnRequestReceivedCalled)
				require.False(t, events.OnResponseCompletedCalled)
			},
		},
	}
	for testCase, data := range testCases {
		t.Run(testCase, func(t *testing.T) {
			peers := testutil.GeneratePeers(2)
			transferID := datatransfer.TransferID(rand.Uint64())
			channelID := datatransfer.ChannelID{Initiator: peers[0], ID: transferID}
			requestID := graphsync.RequestID(rand.Int31())
			var request graphsync.RequestData
			if data.makeRequest != nil {
				request = data.makeRequest(requestID, channelID)
			} else {
				ext := &extension.TransferData{
					TransferID: uint64(transferID),
					Initiator:  peers[0],
					IsPull:     false,
				}
				buf := new(bytes.Buffer)
				err := ext.MarshalCBOR(buf)
				require.NoError(t, err)
				request = testutil.NewFakeRequest(requestID, map[graphsync.ExtensionName][]byte{
					extension.ExtensionDataTransfer: buf.Bytes(),
				})
			}
			var response graphsync.ResponseData
			if data.makeResponse != nil {
				response = data.makeResponse(requestID, channelID)
			} else {
				ext := &extension.TransferData{
					TransferID: uint64(transferID),
					Initiator:  peers[0],
					IsPull:     false,
				}
				buf := new(bytes.Buffer)
				err := ext.MarshalCBOR(buf)
				require.NoError(t, err)
				response = testutil.NewFakeResponse(requestID, map[graphsync.ExtensionName][]byte{
					extension.ExtensionDataTransfer: buf.Bytes(),
				}, graphsync.PartialResponse)
			}
			block := testutil.NewFakeBlockData()
			fgs := testutil.NewFakeGraphSync()
			gsData := &graphsyncTestData{
				fgs:                        fgs,
				p:                          peers[1],
				request:                    request,
				response:                   response,
				block:                      block,
				outgoingRequestHookActions: &fakeOutgoingRequestHookActions{},
				outgoingBlockHookActions:   &fakeOutgoingBlockHookActions{},
				incomingBlockHookActions:   &fakeIncomingBlockHookActions{},
				incomingRequestHookActions: &fakeIncomingRequestHookActions{},
			}
			manager := hooks.NewManager(peers[0], &data.events)
			manager.RegisterHooks(fgs)
			data.action(gsData)
			data.check(t, &data.events, gsData)
		})
	}
}

type fakeEvents struct {
	OnRequestSentCalled       bool
	OnRequestSentError        error
	OnDataReceivedCalled      bool
	OnDataReceivedError       error
	OnDataSentCalled          bool
	OnDataSentError           error
	OnRequestReceivedCalled   bool
	OnRequestReceivedError    error
	OnResponseCompletedCalled bool
	OnResponseCompletedErr    error
	ResponseSuccess           bool
}

func (fe *fakeEvents) OnRequestSent(chid datatransfer.ChannelID) error {
	fe.OnRequestSentCalled = true
	return fe.OnRequestSentError
}

func (fe *fakeEvents) OnDataReceived(chid datatransfer.ChannelID, link ipld.Link, size uint64) error {
	fe.OnDataReceivedCalled = true
	return fe.OnDataReceivedError
}

func (fe *fakeEvents) OnDataSent(chid datatransfer.ChannelID, link ipld.Link, size uint64) error {
	fe.OnDataSentCalled = true
	return fe.OnDataSentError
}

func (fe *fakeEvents) OnRequestReceived(chid datatransfer.ChannelID) error {
	fe.OnRequestReceivedCalled = true
	return fe.OnRequestReceivedError
}

func (fe *fakeEvents) OnResponseCompleted(chid datatransfer.ChannelID, success bool) error {
	fe.OnResponseCompletedCalled = true
	fe.ResponseSuccess = success
	return fe.OnResponseCompletedErr
}

type fakeOutgoingRequestHookActions struct{}

func (fa *fakeOutgoingRequestHookActions) UsePersistenceOption(name string) {}
func (fa *fakeOutgoingRequestHookActions) UseLinkTargetNodeStyleChooser(_ traversal.LinkTargetNodeStyleChooser) {
}

type fakeIncomingBlockHookActions struct {
	TerminationError error
}

func (fa *fakeIncomingBlockHookActions) TerminateWithError(err error) {
	fa.TerminationError = err
}

func (fa *fakeIncomingBlockHookActions) UpdateRequestWithExtensions(_ ...graphsync.ExtensionData) {}

type fakeOutgoingBlockHookActions struct {
	TerminationError error
}

func (fa *fakeOutgoingBlockHookActions) SendExtensionData(_ graphsync.ExtensionData) {}

func (fa *fakeOutgoingBlockHookActions) TerminateWithError(err error) {
	fa.TerminationError = err
}

func (fa *fakeOutgoingBlockHookActions) PauseResponse() {}

type fakeIncomingRequestHookActions struct {
	TerminationError error
	Validated        bool
	SentExtension    graphsync.ExtensionData
}

func (fa *fakeIncomingRequestHookActions) SendExtensionData(ext graphsync.ExtensionData) {
	fa.SentExtension = ext
}

func (fa *fakeIncomingRequestHookActions) UsePersistenceOption(name string) {}

func (fa *fakeIncomingRequestHookActions) UseLinkTargetNodeStyleChooser(_ traversal.LinkTargetNodeStyleChooser) {
}

func (fa *fakeIncomingRequestHookActions) TerminateWithError(err error) {
	fa.TerminationError = err
}

func (fa *fakeIncomingRequestHookActions) ValidateRequest() {
	fa.Validated = true
}

type graphsyncTestData struct {
	fgs                        *testutil.FakeGraphSync
	p                          peer.ID
	block                      graphsync.BlockData
	request                    graphsync.RequestData
	response                   graphsync.ResponseData
	outgoingRequestHookActions *fakeOutgoingRequestHookActions
	incomingBlockHookActions   *fakeIncomingBlockHookActions
	outgoingBlockHookActions   *fakeOutgoingBlockHookActions
	incomingRequestHookActions *fakeIncomingRequestHookActions
}

func (gs *graphsyncTestData) outgoingRequestHook() {
	gs.fgs.OutgoingRequestHook(gs.p, gs.request, gs.outgoingRequestHookActions)
}
func (gs *graphsyncTestData) incomingBlockHook() {
	gs.fgs.IncomingBlockHook(gs.p, gs.response, gs.block, gs.incomingBlockHookActions)
}
func (gs *graphsyncTestData) outgoingBlockHook() {
	gs.fgs.OutgoingBlockHook(gs.p, gs.request, gs.block, gs.outgoingBlockHookActions)
}
func (gs *graphsyncTestData) incomingRequestHook() {
	gs.fgs.IncomingRequestHook(gs.p, gs.request, gs.incomingRequestHookActions)
}

func (gs *graphsyncTestData) responseCompletedListener() {
	gs.fgs.ResponseCompletedListener(gs.p, gs.request, gs.response.Status())
}
