package graphsyncimpl

import (
	"context"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p-core/peer"
	xerrors "golang.org/x/xerrors"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-data-transfer/message"
)

type graphsyncReceiver struct {
	impl *graphsyncImpl
}

// ReceiveRequest takes an incoming data transfer request, validates the voucher and
// processes the message.
func (receiver *graphsyncReceiver) ReceiveRequest(
	ctx context.Context,
	initiator peer.ID,
	incoming message.DataTransferRequest) {
	err := receiver.receiveRequest(initiator, incoming)
	if err != nil {
		log.Error(err)
	}
	if err == nil && !incoming.IsPull() {
		stor, _ := incoming.Selector()
		receiver.impl.sendGsRequest(ctx, initiator, incoming.TransferID(), incoming.IsPull(), initiator, cidlink.Link{Cid: incoming.BaseCid()}, stor)
	}
	receiver.impl.sendResponse(ctx, err == nil, initiator, incoming.TransferID())
}

func (receiver *graphsyncReceiver) receiveRequest(
	initiator peer.ID,
	incoming message.DataTransferRequest) error {

	voucher, err := receiver.validateVoucher(initiator, incoming)
	if err != nil {
		return err
	}
	stor, _ := incoming.Selector()

	var dataSender, dataReceiver peer.ID
	if incoming.IsPull() {
		dataSender = receiver.impl.peerID
		dataReceiver = initiator
	} else {
		dataSender = initiator
		dataReceiver = receiver.impl.peerID
	}

	chid, err := receiver.impl.channels.CreateNew(incoming.TransferID(), incoming.BaseCid(), stor, voucher, initiator, dataSender, dataReceiver)
	if err != nil {
		return err
	}
	evt := datatransfer.Event{
		Code:      datatransfer.Open,
		Message:   "Incoming request accepted",
		Timestamp: time.Now(),
	}
	chst, err := receiver.impl.channels.GetByID(chid)
	if err != nil {
		return err
	}
	err = receiver.impl.pubSub.Publish(internalEvent{evt, chst})
	if err != nil {
		log.Warnf("err publishing DT event: %s", err.Error())
	}
	return nil
}

// validateVoucher converts a voucher in an incoming message to its appropriate
// voucher struct, then runs the validator and returns the results.
// returns error if:
//   * reading voucher fails
//   * deserialization of selector fails
//   * validation fails
func (receiver *graphsyncReceiver) validateVoucher(sender peer.ID, incoming message.DataTransferRequest) (datatransfer.Voucher, error) {

	vtypStr := datatransfer.TypeIdentifier(incoming.VoucherType())
	decoder, has := receiver.impl.validatedTypes.Decoder(vtypStr)
	if !has {
		return nil, xerrors.Errorf("unknown voucher type: %s", vtypStr)
	}
	encodable, err := incoming.Voucher(decoder)
	if err != nil {
		return nil, err
	}
	vouch := encodable.(datatransfer.Registerable)

	var validatorFunc func(peer.ID, datatransfer.Voucher, cid.Cid, ipld.Node) error
	processor, _ := receiver.impl.validatedTypes.Processor(vtypStr)
	validator := processor.(datatransfer.RequestValidator)
	if incoming.IsPull() {
		validatorFunc = validator.ValidatePull
	} else {
		validatorFunc = validator.ValidatePush
	}

	stor, err := incoming.Selector()
	if err != nil {
		return vouch, err
	}

	if err = validatorFunc(sender, vouch, incoming.BaseCid(), stor); err != nil {
		return nil, err
	}

	return vouch, nil
}

// ReceiveResponse handles responses to our  Push or Pull data transfer request.
// It schedules a graphsync transfer only if our Pull Request is accepted.
func (receiver *graphsyncReceiver) ReceiveResponse(
	ctx context.Context,
	sender peer.ID,
	incoming message.DataTransferResponse) {
	evt := datatransfer.Event{
		Code:      datatransfer.Error,
		Message:   "",
		Timestamp: time.Now(),
	}
	chid := datatransfer.ChannelID{Initiator: receiver.impl.peerID, ID: incoming.TransferID()}
	chst, err := receiver.impl.channels.GetByID(chid)
	if err != nil {
		log.Warnf("received response from unknown peer %s, transfer ID %d", sender, incoming.TransferID)
		return
	}

	if incoming.Accepted() {
		evt.Code = datatransfer.Progress
		// if we are handling a response to a pull request then they are sending data and the
		// initiator is us
		if chst.Sender() == sender {
			baseCid := chst.BaseCID()
			root := cidlink.Link{Cid: baseCid}
			receiver.impl.sendGsRequest(ctx, receiver.impl.peerID, incoming.TransferID(), true, sender, root, chst.Selector())
		}
	}
	err = receiver.impl.pubSub.Publish(internalEvent{evt, chst})
	if err != nil {
		log.Warnf("err publishing DT event: %s", err.Error())
	}
}

func (receiver *graphsyncReceiver) ReceiveError(err error) {
	log.Errorf("received error message on data transfer: %s", err.Error())
}
