package clientstates

import (
	"github.com/filecoin-project/go-statemachine/fsm"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
)

// ClientEvents are the events that can happen in a storage client
var ClientEvents = fsm.Events{
	fsm.Event(storagemarket.ClientEventOpen).
		From(storagemarket.StorageDealUnknown).To(storagemarket.StorageDealEnsureClientFunds),
	fsm.Event(storagemarket.ClientEventFundingInitiated).
		From(storagemarket.StorageDealEnsureClientFunds).To(storagemarket.StorageDealClientFunding).
		Action(func(deal *storagemarket.ClientDeal, mcid cid.Cid) error {
			deal.AddFundsCid = &mcid
			return nil
		}),
	fsm.Event(storagemarket.ClientEventEnsureFundsFailed).
		FromMany(storagemarket.StorageDealClientFunding, storagemarket.StorageDealEnsureClientFunds).To(storagemarket.StorageDealFailing).
		Action(func(deal *storagemarket.ClientDeal, err error) error {
			deal.Message = xerrors.Errorf("adding market funds failed: %w", err).Error()
			return nil
		}),
	fsm.Event(storagemarket.ClientEventFundsEnsured).
		FromMany(storagemarket.StorageDealEnsureClientFunds, storagemarket.StorageDealClientFunding).To(storagemarket.StorageDealFundsEnsured),
	fsm.Event(storagemarket.ClientEventWriteProposalFailed).
		From(storagemarket.StorageDealFundsEnsured).To(storagemarket.StorageDealError).
		Action(func(deal *storagemarket.ClientDeal, err error) error {
			deal.Message = xerrors.Errorf("sending proposal to storage provider failed: %w", err).Error()
			return nil
		}),
	fsm.Event(storagemarket.ClientEventReadResponseFailed).
		From(storagemarket.StorageDealFundsEnsured).To(storagemarket.StorageDealFailing).
		Action(func(deal *storagemarket.ClientDeal, err error) error {
			deal.Message = xerrors.Errorf("error reading Response message: %w", err).Error()
			return nil
		}),
	fsm.Event(storagemarket.ClientEventResponseVerificationFailed).
		From(storagemarket.StorageDealFundsEnsured).To(storagemarket.StorageDealFailing).
		Action(func(deal *storagemarket.ClientDeal) error {
			deal.Message = "unable to verify signature on deal response"
			return nil
		}),
	fsm.Event(storagemarket.ClientEventInitiateDataTransfer).
		From(storagemarket.StorageDealFundsEnsured).To(storagemarket.StorageDealStartDataTransfer),
	fsm.Event(storagemarket.ClientEventUnexpectedDealState).
		From(storagemarket.StorageDealFundsEnsured).To(storagemarket.StorageDealFailing).
		Action(func(deal *storagemarket.ClientDeal, status storagemarket.StorageDealStatus, providerMessage string) error {
			deal.Message = xerrors.Errorf("unexpected deal status while waiting for data request: %d (%s). Provider message: %s", status, storagemarket.DealStates[status], providerMessage).Error()
			return nil
		}),
	fsm.Event(storagemarket.ClientEventDataTransferFailed).
		FromMany(storagemarket.StorageDealStartDataTransfer, storagemarket.StorageDealTransferring).To(storagemarket.StorageDealFailing).
		Action(func(deal *storagemarket.ClientDeal, err error) error {
			deal.Message = xerrors.Errorf("failed to initiate data transfer: %w", err).Error()
			return nil
		}),
	fsm.Event(storagemarket.ClientEventDataTransferInitiated).
		From(storagemarket.StorageDealStartDataTransfer).To(storagemarket.StorageDealTransferring),
	fsm.Event(storagemarket.ClientEventDataTransferComplete).
		FromMany(storagemarket.StorageDealTransferring, storagemarket.StorageDealStartDataTransfer).To(storagemarket.StorageDealCheckForAcceptance),
	fsm.Event(storagemarket.ClientEventWaitForDealState).
		From(storagemarket.StorageDealCheckForAcceptance).ToNoChange().
		Action(func(deal *storagemarket.ClientDeal, pollError bool) error {
			deal.PollRetryCount += 1
			if pollError {
				deal.PollErrorCount += 1
			}
			return nil
		}),
	fsm.Event(storagemarket.ClientEventResponseDealDidNotMatch).
		From(storagemarket.StorageDealCheckForAcceptance).To(storagemarket.StorageDealFailing).
		Action(func(deal *storagemarket.ClientDeal, responseCid cid.Cid, proposalCid cid.Cid) error {
			deal.Message = xerrors.Errorf("miner responded to a wrong proposal: %s != %s", responseCid, proposalCid).Error()
			return nil
		}),
	fsm.Event(storagemarket.ClientEventDealRejected).
		From(storagemarket.StorageDealCheckForAcceptance).To(storagemarket.StorageDealFailing).
		Action(func(deal *storagemarket.ClientDeal, state storagemarket.StorageDealStatus, reason string) error {
			deal.Message = xerrors.Errorf("deal failed: (State=%d) %s", state, reason).Error()
			return nil
		}),
	fsm.Event(storagemarket.ClientEventDealAccepted).
		From(storagemarket.StorageDealCheckForAcceptance).To(storagemarket.StorageDealProposalAccepted).
		Action(func(deal *storagemarket.ClientDeal, publishMessage *cid.Cid) error {
			deal.PublishMessage = publishMessage
			return nil
		}),
	fsm.Event(storagemarket.ClientEventStreamCloseError).
		FromAny().To(storagemarket.StorageDealError).
		Action(func(deal *storagemarket.ClientDeal, err error) error {
			deal.Message = xerrors.Errorf("error attempting to close stream: %w", err).Error()
			return nil
		}),
	fsm.Event(storagemarket.ClientEventDealPublishFailed).
		From(storagemarket.StorageDealProposalAccepted).To(storagemarket.StorageDealError).
		Action(func(deal *storagemarket.ClientDeal, err error) error {
			deal.Message = xerrors.Errorf("error validating deal published: %w", err).Error()
			return nil
		}),
	fsm.Event(storagemarket.ClientEventDealPublished).
		From(storagemarket.StorageDealProposalAccepted).To(storagemarket.StorageDealSealing).
		Action(func(deal *storagemarket.ClientDeal, dealID abi.DealID) error {
			deal.DealID = dealID
			return nil
		}),
	fsm.Event(storagemarket.ClientEventDealActivationFailed).
		From(storagemarket.StorageDealSealing).To(storagemarket.StorageDealError).
		Action(func(deal *storagemarket.ClientDeal, err error) error {
			deal.Message = xerrors.Errorf("error in deal activation: %w", err).Error()
			return nil
		}),
	fsm.Event(storagemarket.ClientEventDealActivated).
		From(storagemarket.StorageDealSealing).To(storagemarket.StorageDealActive),
	fsm.Event(storagemarket.ClientEventDealSlashed).
		From(storagemarket.StorageDealActive).To(storagemarket.StorageDealSlashed).
		Action(func(deal *storagemarket.ClientDeal, slashEpoch abi.ChainEpoch) error {
			deal.SlashEpoch = slashEpoch
			return nil
		}),
	fsm.Event(storagemarket.ClientEventDealExpired).
		From(storagemarket.StorageDealActive).To(storagemarket.StorageDealExpired),
	fsm.Event(storagemarket.ClientEventDealCompletionFailed).
		From(storagemarket.StorageDealActive).To(storagemarket.StorageDealError).
		Action(func(deal *storagemarket.ClientDeal, err error) error {
			deal.Message = xerrors.Errorf("error waiting for deal completion: %w", err).Error()
			return nil
		}),
	fsm.Event(storagemarket.ClientEventFailed).
		From(storagemarket.StorageDealFailing).To(storagemarket.StorageDealError),
	fsm.Event(storagemarket.ClientEventRestart).FromAny().ToNoChange(),
}

// ClientStateEntryFuncs are the handlers for different states in a storage client
var ClientStateEntryFuncs = fsm.StateEntryFuncs{
	storagemarket.StorageDealEnsureClientFunds:  EnsureClientFunds,
	storagemarket.StorageDealClientFunding:      WaitForFunding,
	storagemarket.StorageDealFundsEnsured:       ProposeDeal,
	storagemarket.StorageDealStartDataTransfer:  InitiateDataTransfer,
	storagemarket.StorageDealCheckForAcceptance: CheckForDealAcceptance,
	storagemarket.StorageDealProposalAccepted:   ValidateDealPublished,
	storagemarket.StorageDealSealing:            VerifyDealActivated,
	storagemarket.StorageDealActive:             WaitForDealCompletion,
	storagemarket.StorageDealFailing:            FailDeal,
}

// ClientFinalityStates are the states that terminate deal processing for a deal.
// When a client restarts, it restarts only deals that are not in a finality state.
var ClientFinalityStates = []fsm.StateKey{
	storagemarket.StorageDealSlashed,
	storagemarket.StorageDealExpired,
	storagemarket.StorageDealError,
}
