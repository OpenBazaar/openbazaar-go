package clientstates

import (
	"context"
	"time"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-statemachine/fsm"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/clientutils"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/requestvalidation"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
)

var log = logging.Logger("storagemarket_impl")

// ClientDealEnvironment is an abstraction for interacting with
// dependencies from the storage client environment
type ClientDealEnvironment interface {
	Node() storagemarket.StorageClientNode
	NewDealStream(ctx context.Context, p peer.ID) (network.StorageDealStream, error)
	StartDataTransfer(ctx context.Context, to peer.ID, voucher datatransfer.Voucher, baseCid cid.Cid, selector ipld.Node) error
	GetProviderDealState(ctx context.Context, proposalCid cid.Cid) (*storagemarket.ProviderDealState, error)
	PollingInterval() time.Duration
}

// ClientStateEntryFunc is the type for all state entry functions on a storage client
type ClientStateEntryFunc func(ctx fsm.Context, environment ClientDealEnvironment, deal storagemarket.ClientDeal) error

// EnsureClientFunds attempts to ensure the client has enough funds for the deal being proposed
func EnsureClientFunds(ctx fsm.Context, environment ClientDealEnvironment, deal storagemarket.ClientDeal) error {
	node := environment.Node()

	tok, _, err := node.GetChainHead(ctx.Context())
	if err != nil {
		return ctx.Trigger(storagemarket.ClientEventEnsureFundsFailed, xerrors.Errorf("acquiring chain head: %w", err))
	}

	mcid, err := node.EnsureFunds(ctx.Context(), deal.Proposal.Client, deal.Proposal.Client, deal.Proposal.ClientBalanceRequirement(), tok)

	if err != nil {
		return ctx.Trigger(storagemarket.ClientEventEnsureFundsFailed, err)
	}

	// if no message was sent, and there was no error, funds were already available
	if mcid == cid.Undef {
		return ctx.Trigger(storagemarket.ClientEventFundsEnsured)
	}

	return ctx.Trigger(storagemarket.ClientEventFundingInitiated, mcid)
}

// WaitForFunding waits for an AddFunds message to appear on the chain
func WaitForFunding(ctx fsm.Context, environment ClientDealEnvironment, deal storagemarket.ClientDeal) error {
	node := environment.Node()

	return node.WaitForMessage(ctx.Context(), *deal.AddFundsCid, func(code exitcode.ExitCode, bytes []byte, err error) error {
		if err != nil {
			return ctx.Trigger(storagemarket.ClientEventEnsureFundsFailed, xerrors.Errorf("AddFunds err: %w", err))
		}
		if code != exitcode.Ok {
			return ctx.Trigger(storagemarket.ClientEventEnsureFundsFailed, xerrors.Errorf("AddFunds exit code: %s", code.String()))
		}
		return ctx.Trigger(storagemarket.ClientEventFundsEnsured)

	})
}

// ProposeDeal sends the deal proposal to the provider
func ProposeDeal(ctx fsm.Context, environment ClientDealEnvironment, deal storagemarket.ClientDeal) error {
	proposal := network.Proposal{
		DealProposal:  &deal.ClientDealProposal,
		Piece:         deal.DataRef,
		FastRetrieval: deal.FastRetrieval,
	}

	s, err := environment.NewDealStream(ctx.Context(), deal.Miner)
	if err != nil {
		return ctx.Trigger(storagemarket.ClientEventWriteProposalFailed, err)
	}

	if err := s.WriteDealProposal(proposal); err != nil {
		return ctx.Trigger(storagemarket.ClientEventWriteProposalFailed, err)
	}

	resp, err := s.ReadDealResponse()
	if err != nil {
		return ctx.Trigger(storagemarket.ClientEventReadResponseFailed, err)
	}

	err = s.Close()
	if err != nil {
		return ctx.Trigger(storagemarket.ClientEventStreamCloseError, err)
	}

	tok, _, err := environment.Node().GetChainHead(ctx.Context())
	if err != nil {
		return ctx.Trigger(storagemarket.ClientEventResponseVerificationFailed)
	}

	if err := clientutils.VerifyResponse(ctx.Context(), resp, deal.MinerWorker, tok, environment.Node().VerifySignature); err != nil {
		return ctx.Trigger(storagemarket.ClientEventResponseVerificationFailed)
	}

	if resp.Response.State != storagemarket.StorageDealWaitingForData {
		return ctx.Trigger(storagemarket.ClientEventUnexpectedDealState, resp.Response.State, resp.Response.Message)
	}

	return ctx.Trigger(storagemarket.ClientEventInitiateDataTransfer)
}

// InitiateDataTransfer initiates data transfer to the provider
func InitiateDataTransfer(ctx fsm.Context, environment ClientDealEnvironment, deal storagemarket.ClientDeal) error {
	if deal.DataRef.TransferType == storagemarket.TTManual {
		log.Infof("manual data transfer for deal %s", deal.ProposalCid)
		return ctx.Trigger(storagemarket.ClientEventDataTransferComplete)
	}

	log.Infof("sending data for a deal %s", deal.ProposalCid)

	// initiate a push data transfer. This will complete asynchronously and the
	// completion of the data transfer will trigger a change in deal state
	err := environment.StartDataTransfer(ctx.Context(),
		deal.Miner,
		&requestvalidation.StorageDataTransferVoucher{Proposal: deal.ProposalCid},
		deal.DataRef.Root,
		shared.AllSelector(),
	)

	if err != nil {
		return ctx.Trigger(storagemarket.ClientEventDataTransferFailed, xerrors.Errorf("failed to open push data channel: %w", err))
	}

	return ctx.Trigger(storagemarket.ClientEventDataTransferInitiated)
}

// CheckForDealAcceptance is run until the deal is sealed and published by the provider, or errors
func CheckForDealAcceptance(ctx fsm.Context, environment ClientDealEnvironment, deal storagemarket.ClientDeal) error {
	dealState, err := environment.GetProviderDealState(ctx.Context(), deal.ProposalCid)
	if err != nil {
		log.Warnf("error when querying provider deal state: %w", err) // TODO: at what point do we fail the deal?
		return waitAgain(ctx, environment, true)
	}

	if isFailed(dealState.State) {
		return ctx.Trigger(storagemarket.ClientEventDealRejected, dealState.State, dealState.Message)
	}

	if isAccepted(dealState.State) {
		if *dealState.ProposalCid != deal.ProposalCid {
			return ctx.Trigger(storagemarket.ClientEventResponseDealDidNotMatch, *dealState.ProposalCid, deal.ProposalCid)
		}

		return ctx.Trigger(storagemarket.ClientEventDealAccepted, dealState.PublishCid)
	}

	return waitAgain(ctx, environment, false)
}

func waitAgain(ctx fsm.Context, environment ClientDealEnvironment, pollError bool) error {
	t := time.NewTimer(environment.PollingInterval())

	go func() {
		select {
		case <-t.C:
			_ = ctx.Trigger(storagemarket.ClientEventWaitForDealState, pollError)
		case <-ctx.Context().Done():
			t.Stop()
			return
		}
	}()

	return nil
}

// ValidateDealPublished confirms with the chain that a deal was published
func ValidateDealPublished(ctx fsm.Context, environment ClientDealEnvironment, deal storagemarket.ClientDeal) error {

	dealID, err := environment.Node().ValidatePublishedDeal(ctx.Context(), deal)
	if err != nil {
		return ctx.Trigger(storagemarket.ClientEventDealPublishFailed, err)
	}

	return ctx.Trigger(storagemarket.ClientEventDealPublished, dealID)
}

// VerifyDealActivated confirms that a deal was successfully committed to a sector and is active
func VerifyDealActivated(ctx fsm.Context, environment ClientDealEnvironment, deal storagemarket.ClientDeal) error {
	cb := func(err error) {
		if err != nil {
			_ = ctx.Trigger(storagemarket.ClientEventDealActivationFailed, err)
		} else {
			_ = ctx.Trigger(storagemarket.ClientEventDealActivated)
		}
	}

	if err := environment.Node().OnDealSectorCommitted(ctx.Context(), deal.Proposal.Provider, deal.DealID, cb); err != nil {
		return ctx.Trigger(storagemarket.ClientEventDealActivationFailed, err)
	}

	return nil
}

// WaitForDealCompletion waits for the deal to be slashed or to expire
func WaitForDealCompletion(ctx fsm.Context, environment ClientDealEnvironment, deal storagemarket.ClientDeal) error {
	node := environment.Node()

	// Called when the deal expires
	expiredCb := func(err error) {
		if err != nil {
			_ = ctx.Trigger(storagemarket.ClientEventDealCompletionFailed, xerrors.Errorf("deal expiration err: %w", err))
		} else {
			_ = ctx.Trigger(storagemarket.ClientEventDealExpired)
		}
	}

	// Called when the deal is slashed
	slashedCb := func(slashEpoch abi.ChainEpoch, err error) {
		if err != nil {
			_ = ctx.Trigger(storagemarket.ClientEventDealCompletionFailed, xerrors.Errorf("deal slashing err: %w", err))
		} else {
			_ = ctx.Trigger(storagemarket.ClientEventDealSlashed, slashEpoch)
		}
	}

	if err := node.OnDealExpiredOrSlashed(ctx.Context(), deal.DealID, expiredCb, slashedCb); err != nil {
		return ctx.Trigger(storagemarket.ClientEventDealCompletionFailed, err)
	}

	return nil
}

// FailDeal cleans up a failing deal
func FailDeal(ctx fsm.Context, environment ClientDealEnvironment, deal storagemarket.ClientDeal) error {
	// TODO: store in some sort of audit log
	log.Errorf("deal %s failed: %s", deal.ProposalCid, deal.Message)

	return ctx.Trigger(storagemarket.ClientEventFailed)
}

func isAccepted(status storagemarket.StorageDealStatus) bool {
	return status == storagemarket.StorageDealStaged ||
		status == storagemarket.StorageDealSealing ||
		status == storagemarket.StorageDealActive ||
		status == storagemarket.StorageDealExpired ||
		status == storagemarket.StorageDealSlashed
}

func isFailed(status storagemarket.StorageDealStatus) bool {
	return status == storagemarket.StorageDealFailing ||
		status == storagemarket.StorageDealError
}
