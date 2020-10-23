package datatransfer

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p-core/peer"
)

// RequestValidator is an interface implemented by the client of the
// data transfer module to validate requests
type RequestValidator interface {
	// ValidatePush validates a push request received from the peer that will send data
	ValidatePush(
		sender peer.ID,
		voucher Voucher,
		baseCid cid.Cid,
		selector ipld.Node) (VoucherResult, error)
	// ValidatePull validates a pull request received from the peer that will receive data
	ValidatePull(
		receiver peer.ID,
		voucher Voucher,
		baseCid cid.Cid,
		selector ipld.Node) (VoucherResult, error)
}

// Revalidator is a request validator revalidates in progress requests
// by requesting request additional vouchers, and resuming when it receives them
type Revalidator interface {
	// Revalidate revalidates a request with a new voucher
	Revalidate(channelID ChannelID, voucher Voucher) (VoucherResult, error)
	// OnPullDataSent is called on the responder side when more bytes are sent
	// for a given pull request. The first value indicates whether the request was
	// recognized by this revalidator and should be considered 'handled'. If true,
	// the remaining two values are interpreted. If 'false' the request is passed on
	// to the next revalidators.
	// It should return a VoucherResult + ErrPause to
	// request revalidation or nil to continue uninterrupted,
	// other errors will terminate the request.
	OnPullDataSent(chid ChannelID, additionalBytesSent uint64) (bool, VoucherResult, error)
	// OnPushDataReceived is called on the responder side when more bytes are received
	// for a given push request. The first value indicates whether the request was
	// recognized by this revalidator and should be considered 'handled'. If true,
	// the remaining two values are interpreted. If 'false' the request is passed on
	// to the next revalidators. It should return a VoucherResult + ErrPause to
	// request revalidation or nil to continue uninterrupted,
	// other errors will terminate the request
	OnPushDataReceived(chid ChannelID, additionalBytesReceived uint64) (bool, VoucherResult, error)
	// OnComplete is called to make a final request for revalidation -- often for the
	// purpose of settlement. The first value indicates whether the request was
	// recognized by this revalidator and should be considered 'handled'. If true,
	// the remaining two values are interpreted. If 'false' the request is passed on
	// to the next revalidators.
	// if VoucherResult is non nil, the request will enter a settlement phase awaiting
	// a final update
	OnComplete(chid ChannelID) (bool, VoucherResult, error)
}

// TransportConfigurer provides a mechanism to provide transport specific configuration for a given voucher type
type TransportConfigurer func(chid ChannelID, voucher Voucher, transport Transport)

// ReadyFunc is function that gets called once when the data transfer module is ready
type ReadyFunc func(error)

// Manager is the core interface presented by all implementations of
// of the data transfer sub system
type Manager interface {

	// Start initializes data transfer processing
	Start(ctx context.Context) error

	// OnReady registers a listener for when the data transfer comes on line
	OnReady(ReadyFunc)

	// Stop terminates all data transfers and ends processing
	Stop(ctx context.Context) error

	// RegisterVoucherType registers a validator for the given voucher type
	// will error if voucher type does not implement voucher
	// or if there is a voucher type registered with an identical identifier
	RegisterVoucherType(voucherType Voucher, validator RequestValidator) error

	// RegisterRevalidator registers a revalidator for the given voucher type
	// Note: this is the voucher type used to revalidate. It can share a name
	// with the initial validator type and CAN be the same type, or a different type.
	// The revalidator can simply be the sampe as the original request validator,
	// or a different validator that satisfies the revalidator interface.
	RegisterRevalidator(voucherType Voucher, revalidator Revalidator) error

	// RegisterVoucherResultType allows deserialization of a voucher result,
	// so that a listener can read the metadata
	RegisterVoucherResultType(resultType VoucherResult) error

	// RegisterTransportConfigurer registers the given transport configurer to be run on requests with the given voucher
	// type
	RegisterTransportConfigurer(voucherType Voucher, configurer TransportConfigurer) error

	// open a data transfer that will send data to the recipient peer and
	// transfer parts of the piece that match the selector
	OpenPushDataChannel(ctx context.Context, to peer.ID, voucher Voucher, baseCid cid.Cid, selector ipld.Node) (ChannelID, error)

	// open a data transfer that will request data from the sending peer and
	// transfer parts of the piece that match the selector
	OpenPullDataChannel(ctx context.Context, to peer.ID, voucher Voucher, baseCid cid.Cid, selector ipld.Node) (ChannelID, error)

	// send an intermediate voucher as needed when the receiver sends a request for revalidation
	SendVoucher(ctx context.Context, chid ChannelID, voucher Voucher) error

	// close an open channel (effectively a cancel)
	CloseDataTransferChannel(ctx context.Context, chid ChannelID) error

	// pause a data transfer channel (only allowed if transport supports it)
	PauseDataTransferChannel(ctx context.Context, chid ChannelID) error

	// resume a data transfer channel (only allowed if transport supports it)
	ResumeDataTransferChannel(ctx context.Context, chid ChannelID) error

	// get status of a transfer
	TransferChannelStatus(ctx context.Context, x ChannelID) Status

	// get notified when certain types of events happen
	SubscribeToEvents(subscriber Subscriber) Unsubscribe

	// get all in progress transfers
	InProgressChannels(ctx context.Context) (map[ChannelID]ChannelState, error)

	// RestartDataTransferChannel restarts an existing data transfer channel
	RestartDataTransferChannel(ctx context.Context, chid ChannelID) error
}
