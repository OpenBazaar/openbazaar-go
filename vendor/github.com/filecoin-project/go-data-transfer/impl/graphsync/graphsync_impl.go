package graphsyncimpl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	logging "github.com/ipfs/go-log"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/xerrors"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-data-transfer/channels"
	"github.com/filecoin-project/go-data-transfer/impl/graphsync/extension"
	"github.com/filecoin-project/go-data-transfer/impl/graphsync/hooks"
	"github.com/filecoin-project/go-data-transfer/message"
	"github.com/filecoin-project/go-data-transfer/network"
	"github.com/filecoin-project/go-data-transfer/registry"
	"github.com/filecoin-project/go-storedcounter"
	"github.com/hannahhoward/go-pubsub"
)

var log = logging.Logger("graphsync-impl")

type graphsyncImpl struct {
	dataTransferNetwork network.DataTransferNetwork
	validatedTypes      *registry.Registry
	pubSub              *pubsub.PubSub
	channels            *channels.Channels
	gs                  graphsync.GraphExchange
	peerID              peer.ID
	storedCounter       *storedcounter.StoredCounter
}

type internalEvent struct {
	evt   datatransfer.Event
	state datatransfer.ChannelState
}

func dispatcher(evt pubsub.Event, subscriberFn pubsub.SubscriberFn) error {
	ie, ok := evt.(internalEvent)
	if !ok {
		return errors.New("wrong type of event")
	}
	cb, ok := subscriberFn.(datatransfer.Subscriber)
	if !ok {
		return errors.New("wrong type of event")
	}
	cb(ie.evt, ie.state)
	return nil
}

// NewGraphSyncDataTransfer initializes a new graphsync based data transfer manager
func NewGraphSyncDataTransfer(host host.Host, gs graphsync.GraphExchange, storedCounter *storedcounter.StoredCounter) datatransfer.Manager {
	dataTransferNetwork := network.NewFromLibp2pHost(host)
	impl := &graphsyncImpl{
		dataTransferNetwork: dataTransferNetwork,
		validatedTypes:      registry.NewRegistry(),
		pubSub:              pubsub.New(dispatcher),
		channels:            channels.New(),
		gs:                  gs,
		peerID:              host.ID(),
		storedCounter:       storedCounter,
	}

	dtReceiver := &graphsyncReceiver{impl}
	dataTransferNetwork.SetDelegate(dtReceiver)

	hooksManager := hooks.NewManager(host.ID(), impl)
	hooksManager.RegisterHooks(gs)
	return impl
}

func (impl *graphsyncImpl) OnRequestSent(chid datatransfer.ChannelID) error {
	_, err := impl.channels.GetByID(chid)
	return err
}

func (impl *graphsyncImpl) OnDataReceived(chid datatransfer.ChannelID, link ipld.Link, size uint64) error {
	_, err := impl.channels.IncrementReceived(chid, size)
	if err != nil {
		return err
	}
	chst, err := impl.channels.GetByID(chid)
	if err != nil {
		return err
	}
	evt := datatransfer.Event{
		Code:      datatransfer.Progress,
		Message:   fmt.Sprintf("Received %d more bytes", size),
		Timestamp: time.Now(),
	}
	err = impl.pubSub.Publish(internalEvent{evt, chst})
	if err != nil {
		log.Warnf("err publishing DT event: %s", err.Error())
	}
	return nil
}

func (impl *graphsyncImpl) OnDataSent(chid datatransfer.ChannelID, link ipld.Link, size uint64) error {
	_, err := impl.channels.IncrementSent(chid, size)
	if err != nil {
		return err
	}
	chst, err := impl.channels.GetByID(chid)
	if err != nil {
		return err
	}
	evt := datatransfer.Event{
		Code:      datatransfer.Progress,
		Message:   fmt.Sprintf("Sent %d more bytes", size),
		Timestamp: time.Now(),
	}
	err = impl.pubSub.Publish(internalEvent{evt, chst})
	if err != nil {
		log.Warnf("err publishing DT event: %s", err.Error())
	}
	return nil
}

func (impl *graphsyncImpl) OnRequestReceived(chid datatransfer.ChannelID) error {
	_, err := impl.channels.GetByID(chid)
	return err
}

func (impl *graphsyncImpl) OnResponseCompleted(chid datatransfer.ChannelID, success bool) error {
	chst, err := impl.channels.GetByID(chid)
	if err != nil {
		return err
	}

	evt := datatransfer.Event{
		Code:      datatransfer.Error,
		Timestamp: time.Now(),
	}
	if success {
		evt.Code = datatransfer.Complete
	}
	err = impl.pubSub.Publish(internalEvent{evt, chst})
	if err != nil {
		log.Warnf("err publishing DT event: %s", err.Error())
	}
	return nil
}

// RegisterVoucherType registers a validator for the given voucher type
// returns error if:
// * voucher type does not implement voucher
// * there is a voucher type registered with an identical identifier
// * voucherType's Kind is not reflect.Ptr
func (impl *graphsyncImpl) RegisterVoucherType(voucherType datatransfer.Voucher, validator datatransfer.RequestValidator) error {
	err := impl.validatedTypes.Register(voucherType, validator)
	if err != nil {
		return xerrors.Errorf("error registering voucher type: %w", err)
	}
	return nil
}

// OpenPushDataChannel opens a data transfer that will send data to the recipient peer and
// transfer parts of the piece that match the selector
func (impl *graphsyncImpl) OpenPushDataChannel(ctx context.Context, requestTo peer.ID, voucher datatransfer.Voucher, baseCid cid.Cid, selector ipld.Node) (datatransfer.ChannelID, error) {
	tid, err := impl.sendDtRequest(ctx, selector, false, voucher, baseCid, requestTo)
	if err != nil {
		return datatransfer.ChannelID{}, err
	}

	chid, err := impl.channels.CreateNew(tid, baseCid, selector, voucher,
		impl.peerID, impl.peerID, requestTo) // initiator = us, sender = us, receiver = them
	if err != nil {
		return chid, err
	}
	evt := datatransfer.Event{
		Code:      datatransfer.Open,
		Message:   "New Request Initiated",
		Timestamp: time.Now(),
	}
	chst, err := impl.channels.GetByID(chid)
	if err != nil {
		return chid, err
	}
	err = impl.pubSub.Publish(internalEvent{evt, chst})
	if err != nil {
		log.Warnf("err publishing DT event: %s", err.Error())
	}
	return chid, nil
}

// OpenPullDataChannel opens a data transfer that will request data from the sending peer and
// transfer parts of the piece that match the selector
func (impl *graphsyncImpl) OpenPullDataChannel(ctx context.Context, requestTo peer.ID, voucher datatransfer.Voucher, baseCid cid.Cid, selector ipld.Node) (datatransfer.ChannelID, error) {

	tid, err := impl.sendDtRequest(ctx, selector, true, voucher, baseCid, requestTo)
	if err != nil {
		return datatransfer.ChannelID{}, err
	}
	// initiator = us, sender = them, receiver = us
	chid, err := impl.channels.CreateNew(tid, baseCid, selector, voucher,
		impl.peerID, requestTo, impl.peerID)
	if err != nil {
		return chid, err
	}
	evt := datatransfer.Event{
		Code:      datatransfer.Open,
		Message:   "New Request Initiated",
		Timestamp: time.Now(),
	}
	chst, err := impl.channels.GetByID(chid)
	if err != nil {
		return chid, err
	}
	err = impl.pubSub.Publish(internalEvent{evt, chst})
	if err != nil {
		log.Warnf("err publishing DT event: %s", err.Error())
	}
	return chid, nil
}

// sendDtRequest encapsulates message creation and posting to the data transfer network with the provided parameters
func (impl *graphsyncImpl) sendDtRequest(ctx context.Context, selector ipld.Node, isPull bool, voucher datatransfer.Voucher, baseCid cid.Cid, to peer.ID) (datatransfer.TransferID, error) {
	next, err := impl.storedCounter.Next()
	if err != nil {
		return 0, err
	}
	tid := datatransfer.TransferID(next)
	req, err := message.NewRequest(tid, isPull, voucher.Type(), voucher, baseCid, selector)
	if err != nil {
		return 0, err
	}
	if err := impl.dataTransferNetwork.SendMessage(ctx, to, req); err != nil {
		return 0, err
	}
	return tid, nil
}

func (impl *graphsyncImpl) sendResponse(ctx context.Context, isAccepted bool, to peer.ID, tid datatransfer.TransferID) {
	resp := message.NewResponse(tid, isAccepted)
	if err := impl.dataTransferNetwork.SendMessage(ctx, to, resp); err != nil {
		log.Error(err)
	}
}

// close an open channel (effectively a cancel)
func (impl *graphsyncImpl) CloseDataTransferChannel(x datatransfer.ChannelID) {}

// get status of a transfer
func (impl *graphsyncImpl) TransferChannelStatus(x datatransfer.ChannelID) datatransfer.Status {
	return datatransfer.ChannelNotFoundError
}

// get notified when certain types of events happen
func (impl *graphsyncImpl) SubscribeToEvents(subscriber datatransfer.Subscriber) datatransfer.Unsubscribe {
	return datatransfer.Unsubscribe(impl.pubSub.Subscribe(subscriber))
}

// get all in progress transfers
func (impl *graphsyncImpl) InProgressChannels() map[datatransfer.ChannelID]datatransfer.ChannelState {
	return impl.channels.InProgress()
}

// sendGsRequest assembles a graphsync request and determines if the transfer was completed/successful.
// notifies subscribers of final request status.
func (impl *graphsyncImpl) sendGsRequest(ctx context.Context, initiator peer.ID, transferID datatransfer.TransferID, isPull bool, dataSender peer.ID, root cidlink.Link, stor ipld.Node) {
	extDtData := extension.NewTransferData(transferID, initiator, isPull)
	var buf bytes.Buffer
	if err := extDtData.MarshalCBOR(&buf); err != nil {
		log.Error(err)
	}
	extData := buf.Bytes()
	_, errChan := impl.gs.Request(ctx, dataSender, root, stor,
		graphsync.ExtensionData{
			Name: extension.ExtensionDataTransfer,
			Data: extData,
		})
	go func() {
		var lastError error
		for err := range errChan {
			lastError = err
		}
		evt := datatransfer.Event{
			Code:      datatransfer.Error,
			Timestamp: time.Now(),
		}
		chid := datatransfer.ChannelID{Initiator: initiator, ID: transferID}
		chst, err := impl.channels.GetByID(chid)
		if err != nil {
			msg := "cannot find a matching channel for this request"
			evt.Message = msg
		} else {
			if lastError == nil {
				evt.Code = datatransfer.Complete
			} else {
				evt.Message = lastError.Error()
			}
		}
		err = impl.pubSub.Publish(internalEvent{evt, chst})
		if err != nil {
			log.Warnf("err publishing DT event: %s", err.Error())
		}
	}()
}
