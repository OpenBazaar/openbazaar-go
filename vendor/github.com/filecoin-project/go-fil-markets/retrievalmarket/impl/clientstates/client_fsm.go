package clientstates

import (
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-statemachine/fsm"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
)

func recordPaymentOwed(deal *rm.ClientDealState, totalProcessed uint64, paymentOwed abi.TokenAmount) error {
	deal.TotalReceived += totalProcessed
	deal.PaymentRequested = paymentOwed
	return nil
}

func recordProcessed(deal *rm.ClientDealState, totalProcessed uint64) error {
	deal.TotalReceived += totalProcessed
	return nil
}

// ClientEvents are the events that can happen in a retrieval client
var ClientEvents = fsm.Events{
	fsm.Event(rm.ClientEventOpen).
		From(rm.DealStatusNew).ToNoChange(),
	fsm.Event(rm.ClientEventPaymentChannelErrored).
		FromMany(rm.DealStatusAccepted, rm.DealStatusPaymentChannelCreating).To(rm.DealStatusFailed).
		Action(func(deal *rm.ClientDealState, err error) error {
			deal.Message = xerrors.Errorf("get or create payment channel: %w", err).Error()
			return nil
		}),
	fsm.Event(rm.ClientEventPaymentChannelCreateInitiated).
		From(rm.DealStatusAccepted).To(rm.DealStatusPaymentChannelCreating).
		Action(func(deal *rm.ClientDealState, msgCID cid.Cid) error {
			deal.WaitMsgCID = &msgCID
			return nil
		}),
	fsm.Event(rm.ClientEventPaymentChannelAddingFunds).
		FromMany(rm.DealStatusAccepted).To(rm.DealStatusPaymentChannelAddingFunds).
		Action(func(deal *rm.ClientDealState, msgCID cid.Cid, payCh address.Address) error {
			deal.WaitMsgCID = &msgCID
			deal.PaymentInfo = &rm.PaymentInfo{
				PayCh: payCh,
			}
			return nil
		}),
	fsm.Event(rm.ClientEventPaymentChannelReady).
		FromMany(rm.DealStatusPaymentChannelCreating, rm.DealStatusPaymentChannelAddingFunds).
		To(rm.DealStatusPaymentChannelReady).
		Action(func(deal *rm.ClientDealState, payCh address.Address, lane uint64) error {
			deal.PaymentInfo = &rm.PaymentInfo{
				PayCh: payCh,
				Lane:  lane,
			}
			return nil
		}),
	fsm.Event(rm.ClientEventAllocateLaneErrored).
		FromMany(rm.DealStatusPaymentChannelCreating, rm.DealStatusPaymentChannelAddingFunds).
		To(rm.DealStatusFailed).
		Action(func(deal *rm.ClientDealState, err error) error {
			deal.Message = xerrors.Errorf("allocating payment lane: %w", err).Error()
			return nil
		}),
	fsm.Event(rm.ClientEventPaymentChannelAddFundsErrored).
		From(rm.DealStatusPaymentChannelAddingFunds).To(rm.DealStatusFailed).
		Action(func(deal *rm.ClientDealState, err error) error {
			deal.Message = xerrors.Errorf("wait for add funds: %w", err).Error()
			return nil
		}),
	fsm.Event(rm.ClientEventWriteDealProposalErrored).
		FromAny().To(rm.DealStatusErrored).
		Action(func(deal *rm.ClientDealState, err error) error {
			deal.Message = xerrors.Errorf("proposing deal: %w", err).Error()
			return nil
		}),
	fsm.Event(rm.ClientEventReadDealResponseErrored).
		FromAny().To(rm.DealStatusErrored).
		Action(func(deal *rm.ClientDealState, err error) error {
			deal.Message = xerrors.Errorf("reading deal response: %w", err).Error()
			return nil
		}),
	fsm.Event(rm.ClientEventDealRejected).
		From(rm.DealStatusNew).To(rm.DealStatusRejected).
		Action(func(deal *rm.ClientDealState, message string) error {
			deal.Message = fmt.Sprintf("deal rejected: %s", message)
			return nil
		}),
	fsm.Event(rm.ClientEventDealNotFound).
		From(rm.DealStatusNew).To(rm.DealStatusDealNotFound).
		Action(func(deal *rm.ClientDealState, message string) error {
			deal.Message = fmt.Sprintf("deal not found: %s", message)
			return nil
		}),
	fsm.Event(rm.ClientEventDealAccepted).
		From(rm.DealStatusNew).To(rm.DealStatusAccepted),
	fsm.Event(rm.ClientEventUnknownResponseReceived).
		FromAny().To(rm.DealStatusFailed).
		Action(func(deal *rm.ClientDealState) error {
			deal.Message = "Unexpected deal response status"
			return nil
		}),
	fsm.Event(rm.ClientEventFundsExpended).
		FromMany(rm.DealStatusFundsNeeded, rm.DealStatusFundsNeededLastPayment).To(rm.DealStatusFailed).
		Action(func(deal *rm.ClientDealState, expectedTotal string, actualTotal string) error {
			deal.Message = fmt.Sprintf("not enough funds left: expected amt = %s, actual amt = %s", expectedTotal, actualTotal)
			return nil
		}),
	fsm.Event(rm.ClientEventBadPaymentRequested).
		FromMany(rm.DealStatusFundsNeeded, rm.DealStatusFundsNeededLastPayment).To(rm.DealStatusFailed).
		Action(func(deal *rm.ClientDealState, message string) error {
			deal.Message = message
			return nil
		}),
	fsm.Event(rm.ClientEventCreateVoucherFailed).
		FromMany(rm.DealStatusFundsNeeded, rm.DealStatusFundsNeededLastPayment).To(rm.DealStatusFailed).
		Action(func(deal *rm.ClientDealState, err error) error {
			deal.Message = xerrors.Errorf("creating payment voucher: %w", err).Error()
			return nil
		}),
	fsm.Event(rm.ClientEventWriteDealPaymentErrored).
		FromAny().To(rm.DealStatusErrored).
		Action(func(deal *rm.ClientDealState, err error) error {
			deal.Message = xerrors.Errorf("writing deal payment: %w", err).Error()
			return nil
		}),
	fsm.Event(rm.ClientEventPaymentSent).
		From(rm.DealStatusFundsNeeded).To(rm.DealStatusOngoing).
		From(rm.DealStatusFundsNeededLastPayment).To(rm.DealStatusFinalizing).
		Action(func(deal *rm.ClientDealState) error {
			// paymentRequested = 0
			// fundsSpent = fundsSpent + paymentRequested
			// if paymentRequested / pricePerByte >= currentInterval
			// currentInterval = currentInterval + proposal.intervalIncrease
			// bytesPaidFor = bytesPaidFor + (paymentRequested / pricePerByte)
			deal.FundsSpent = big.Add(deal.FundsSpent, deal.PaymentRequested)
			bytesPaidFor := big.Div(deal.PaymentRequested, deal.PricePerByte).Uint64()
			if bytesPaidFor >= deal.CurrentInterval {
				deal.CurrentInterval += deal.DealProposal.PaymentIntervalIncrease
			}
			deal.BytesPaidFor += bytesPaidFor
			deal.PaymentRequested = abi.NewTokenAmount(0)
			return nil
		}),
	fsm.Event(rm.ClientEventConsumeBlockFailed).
		FromMany(rm.DealStatusPaymentChannelReady, rm.DealStatusOngoing).To(rm.DealStatusFailed).
		Action(func(deal *rm.ClientDealState, err error) error {
			deal.Message = xerrors.Errorf("consuming block: %w", err).Error()
			return nil
		}),
	fsm.Event(rm.ClientEventLastPaymentRequested).
		FromMany(rm.DealStatusPaymentChannelReady,
			rm.DealStatusOngoing,
			rm.DealStatusBlocksComplete).To(rm.DealStatusFundsNeededLastPayment).
		Action(recordPaymentOwed),
	fsm.Event(rm.ClientEventAllBlocksReceived).
		FromMany(rm.DealStatusPaymentChannelReady,
			rm.DealStatusOngoing,
			rm.DealStatusBlocksComplete).To(rm.DealStatusBlocksComplete).
		Action(recordProcessed),
	fsm.Event(rm.ClientEventComplete).
		FromMany(rm.DealStatusPaymentChannelReady,
			rm.DealStatusOngoing,
			rm.DealStatusBlocksComplete,
			rm.DealStatusFinalizing).To(rm.DealStatusCompleted).
		Action(recordProcessed),
	fsm.Event(rm.ClientEventEarlyTermination).
		FromMany(rm.DealStatusPaymentChannelReady, rm.DealStatusOngoing).To(rm.DealStatusFailed).
		Action(func(deal *rm.ClientDealState) error {
			deal.Message = "received complete status before all blocks received"
			return nil
		}),
	fsm.Event(rm.ClientEventPaymentRequested).
		FromMany(rm.DealStatusPaymentChannelReady, rm.DealStatusOngoing).To(rm.DealStatusFundsNeeded).
		Action(recordPaymentOwed),
	fsm.Event(rm.ClientEventBlocksReceived).
		From(rm.DealStatusPaymentChannelReady).To(rm.DealStatusOngoing).
		From(rm.DealStatusOngoing).ToNoChange().
		Action(recordProcessed),
}

// ClientStateEntryFuncs are the handlers for different states in a retrieval client
var ClientStateEntryFuncs = fsm.StateEntryFuncs{
	rm.DealStatusNew:                       ProposeDeal,
	rm.DealStatusAccepted:                  SetupPaymentChannelStart,
	rm.DealStatusPaymentChannelCreating:    WaitForPaymentChannelCreate,
	rm.DealStatusPaymentChannelAddingFunds: WaitForPaymentChannelAddFunds,
	rm.DealStatusPaymentChannelReady:       ProcessNextResponse,
	rm.DealStatusOngoing:                   ProcessNextResponse,
	rm.DealStatusBlocksComplete:            ProcessNextResponse,
	rm.DealStatusFundsNeeded:               ProcessPaymentRequested,
	rm.DealStatusFundsNeededLastPayment:    ProcessPaymentRequested,
	rm.DealStatusFinalizing:                Finalize,
}
