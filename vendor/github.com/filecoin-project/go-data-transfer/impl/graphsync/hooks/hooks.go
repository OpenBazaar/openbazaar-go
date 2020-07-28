package hooks

import (
	"sync"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-data-transfer/impl/graphsync/extension"
	"github.com/ipfs/go-graphsync"
	ipld "github.com/ipld/go-ipld-prime"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/prometheus/common/log"
)

// Events are semantic data transfer events that happen as a result of graphsync hooks
type Events interface {
	// OnRequestSent is called when we ask the other peer to send us data on the
	// given channel ID
	// return values are:
	// - nil = this request is recognized
	// - error = ignore incoming data for this request
	OnRequestSent(chid datatransfer.ChannelID) error
	// OnDataReceive is called when we receive data for the given channel ID
	// return values are:
	// - nil = continue receiving data
	// - error = cancel this request
	OnDataReceived(chid datatransfer.ChannelID, link ipld.Link, size uint64) error
	// OnDataSent is called when we send data for the given channel ID
	// return values are:
	// - nil = continue sending data
	// - error = cancel this request
	OnDataSent(chid datatransfer.ChannelID, link ipld.Link, size uint64) error
	// OnRequestReceived is called when we receive a new request to send data
	// for the given channel ID
	// return values are:
	// - nil = proceed with sending data
	// - error = cancel this request
	OnRequestReceived(chid datatransfer.ChannelID) error
	// OnResponseCompleted is called when we finish sending data for the given channel ID
	// Error returns are logged but otherwise have not effect
	OnResponseCompleted(chid datatransfer.ChannelID, success bool) error
}

type graphsyncKey struct {
	requestID graphsync.RequestID
	p         peer.ID
}

// Manager manages graphsync hooks for data transfer, translating from
// graphsync hooks to semantic data transfer events
type Manager struct {
	events                Events
	peerID                peer.ID
	graphsyncRequestMapLk sync.RWMutex
	graphsyncRequestMap   map[graphsyncKey]datatransfer.ChannelID
}

// NewManager makes a new hooks manager with the given hook events interface
func NewManager(peerID peer.ID, hookEvents Events) *Manager {
	return &Manager{
		events:              hookEvents,
		peerID:              peerID,
		graphsyncRequestMap: make(map[graphsyncKey]datatransfer.ChannelID),
	}
}

// RegisterHooks registers graphsync hooks for the hooks manager
func (hm *Manager) RegisterHooks(gs graphsync.GraphExchange) {
	gs.RegisterIncomingRequestHook(hm.gsReqRecdHook)
	gs.RegisterCompletedResponseListener(hm.gsCompletedResponseListener)
	gs.RegisterIncomingBlockHook(hm.gsIncomingBlockHook)
	gs.RegisterOutgoingBlockHook(hm.gsOutgoingBlockHook)
	gs.RegisterOutgoingRequestHook(hm.gsOutgoingRequestHook)
}

func (hm *Manager) gsOutgoingRequestHook(p peer.ID, request graphsync.RequestData, hookActions graphsync.OutgoingRequestHookActions) {
	transferData, _ := extension.GetTransferData(request)

	// extension not found; probably not our request.
	if transferData == nil {
		return
	}

	chid := transferData.GetChannelID()
	err := hm.events.OnRequestSent(chid)
	if err != nil {
		return
	}
	// record the outgoing graphsync request to map it to channel ID going forward
	hm.graphsyncRequestMapLk.Lock()
	hm.graphsyncRequestMap[graphsyncKey{request.ID(), hm.peerID}] = chid
	hm.graphsyncRequestMapLk.Unlock()
}

func (hm *Manager) gsIncomingBlockHook(p peer.ID, response graphsync.ResponseData, block graphsync.BlockData, hookActions graphsync.IncomingBlockHookActions) {
	hm.graphsyncRequestMapLk.RLock()
	chid, ok := hm.graphsyncRequestMap[graphsyncKey{response.RequestID(), hm.peerID}]
	hm.graphsyncRequestMapLk.RUnlock()

	if !ok {
		return
	}

	err := hm.events.OnDataReceived(chid, block.Link(), block.BlockSize())
	if err != nil {
		hookActions.TerminateWithError(err)
	}
}

func (hm *Manager) gsOutgoingBlockHook(p peer.ID, request graphsync.RequestData, block graphsync.BlockData, hookActions graphsync.OutgoingBlockHookActions) {
	hm.graphsyncRequestMapLk.RLock()
	chid, ok := hm.graphsyncRequestMap[graphsyncKey{request.ID(), p}]
	hm.graphsyncRequestMapLk.RUnlock()

	if !ok {
		return
	}

	err := hm.events.OnDataSent(chid, block.Link(), block.BlockSize())
	if err != nil {
		hookActions.TerminateWithError(err)
	}
}

// gsReqRecdHook is a graphsync.OnRequestReceivedHook hook
// if an incoming request does not match a previous push request, it returns an error.
func (hm *Manager) gsReqRecdHook(p peer.ID, request graphsync.RequestData, hookActions graphsync.IncomingRequestHookActions) {

	// if this is a push request the sender is us.
	transferData, err := extension.GetTransferData(request)
	if err != nil {
		hookActions.TerminateWithError(err)
		return
	}

	// extension not found; probably not our request.
	if transferData == nil {
		return
	}

	chid := transferData.GetChannelID()

	err = hm.events.OnRequestReceived(chid)
	if err != nil {
		hookActions.TerminateWithError(err)
		return
	}

	hm.graphsyncRequestMapLk.Lock()
	hm.graphsyncRequestMap[graphsyncKey{request.ID(), p}] = chid
	hm.graphsyncRequestMapLk.Unlock()

	raw, _ := request.Extension(extension.ExtensionDataTransfer)
	respData := graphsync.ExtensionData{Name: extension.ExtensionDataTransfer, Data: raw}
	hookActions.ValidateRequest()
	hookActions.SendExtensionData(respData)
}

// gsCompletedResponseListener is a graphsync.OnCompletedResponseListener. We use it learn when the data transfer is complete
// for the side that is responding to a graphsync request
func (hm *Manager) gsCompletedResponseListener(p peer.ID, request graphsync.RequestData, status graphsync.ResponseStatusCode) {
	hm.graphsyncRequestMapLk.RLock()
	chid, ok := hm.graphsyncRequestMap[graphsyncKey{request.ID(), p}]
	hm.graphsyncRequestMapLk.RUnlock()

	if !ok {
		return
	}

	success := status == graphsync.RequestCompletedFull
	err := hm.events.OnResponseCompleted(chid, success)
	if err != nil {
		log.Error(err)
	}
}