// Package dtutils provides event listeners for the client and provider to
// listen for events on the data transfer module and dispatch FSM events based on them
package dtutils

import (
	"errors"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-statemachine/fsm"
	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/requestvalidation"
)

var log = logging.Logger("storagemarket_impl")

var (
	// ErrDataTransferFailed means a data transfer for a deal failed
	ErrDataTransferFailed = errors.New("deal data transfer failed")
)

// EventReceiver is any thing that can receive FSM events
type EventReceiver interface {
	Send(id interface{}, name fsm.EventName, args ...interface{}) (err error)
}

// ProviderDataTransferSubscriber is the function called when an event occurs in a data
// transfer received by a provider -- it reads the voucher to verify this event occurred
// in a storage market deal, then, based on the data transfer event that occurred, it generates
// and update message for the deal -- either moving to staged for a completion
// event or moving to error if a data transfer error occurs
func ProviderDataTransferSubscriber(deals EventReceiver) datatransfer.Subscriber {
	return func(event datatransfer.Event, channelState datatransfer.ChannelState) {
		voucher, ok := channelState.Voucher().(*requestvalidation.StorageDataTransferVoucher)
		// if this event is for a transfer not related to storage, ignore
		if !ok {
			return
		}

		// data transfer events for progress do not affect deal state
		switch event.Code {
		case datatransfer.Open:
			err := deals.Send(voucher.Proposal, storagemarket.ProviderEventDataTransferInitiated)
			if err != nil {
				log.Errorf("processing dt event: %w", err)
			}
		case datatransfer.Complete:
			err := deals.Send(voucher.Proposal, storagemarket.ProviderEventDataTransferCompleted)
			if err != nil {
				log.Errorf("processing dt event: %w", err)
			}
		case datatransfer.Error:
			err := deals.Send(voucher.Proposal, storagemarket.ProviderEventDataTransferFailed, ErrDataTransferFailed)
			if err != nil {
				log.Errorf("processing dt event: %w", err)
			}
		default:
		}
	}
}

// ClientDataTransferSubscriber is the function called when an event occurs in a data
// transfer initiated on the client -- it reads the voucher to verify this even occurred
// in a storage market deal, then, based on the data transfer event that occurred, it dispatches
// an event to the appropriate state machine
func ClientDataTransferSubscriber(deals EventReceiver) datatransfer.Subscriber {
	return func(event datatransfer.Event, channelState datatransfer.ChannelState) {
		voucher, ok := channelState.Voucher().(*requestvalidation.StorageDataTransferVoucher)
		// if this event is for a transfer not related to storage, ignore
		if !ok {
			return
		}

		// data transfer events for progress do not affect deal state
		switch event.Code {
		case datatransfer.Complete:
			err := deals.Send(voucher.Proposal, storagemarket.ClientEventDataTransferComplete)
			if err != nil {
				log.Errorf("processing dt event: %w", err)
			}
		case datatransfer.Error:
			err := deals.Send(voucher.Proposal, storagemarket.ClientEventDataTransferFailed, ErrDataTransferFailed)
			if err != nil {
				log.Errorf("processing dt event: %w", err)
			}
		default:
		}
	}
}
