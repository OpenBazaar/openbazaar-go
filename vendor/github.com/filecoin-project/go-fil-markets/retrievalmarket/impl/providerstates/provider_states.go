package providerstates

import (
	"context"
	"errors"

	"github.com/filecoin-project/go-statemachine/fsm"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	rm "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
)

// ProviderDealEnvironment is a bridge to the environment a provider deal is executing in
// It provides access to relevant functionality on the retrieval provider
type ProviderDealEnvironment interface {
	// Node returns the node interface for this deal
	Node() rm.RetrievalProviderNode
	// GetPieceSize returns the size of the piece for a given payload CID,
	// looking only in the specified PieceCID if given
	GetPieceSize(c cid.Cid, pieceCID *cid.Cid) (uint64, error)
	// DealStream returns the relevant libp2p interface for this deal
	DealStream(id rm.ProviderDealIdentifier) rmnet.RetrievalDealStream
	// NextBlock returns the next block for the given payload, unsealing if neccesary
	NextBlock(context.Context, rm.ProviderDealIdentifier) (rm.Block, bool, error)
	// CheckDealParams verifies the given deal params are acceptable
	CheckDealParams(pricePerByte abi.TokenAmount, paymentInterval uint64, paymentIntervalIncrease uint64) error
	// RunDealDecisioningLogic runs custom deal decision logic to decide if a deal is accepted, if present
	RunDealDecisioningLogic(ctx context.Context, state rm.ProviderDealState) (bool, string, error)
}

// ReceiveDeal receives and evaluates a deal proposal
func ReceiveDeal(ctx fsm.Context, environment ProviderDealEnvironment, deal rm.ProviderDealState) error {
	dealProposal := deal.DealProposal

	// verify we have the piece
	_, err := environment.GetPieceSize(dealProposal.PayloadCID, dealProposal.PieceCID)
	if err != nil {
		if err == rm.ErrNotFound {
			return ctx.Trigger(rm.ProviderEventDealNotFound)
		}
		return ctx.Trigger(rm.ProviderEventGetPieceSizeErrored, err)
	}

	// check that the deal parameters match our required parameters or
	// reject outright
	err = environment.CheckDealParams(dealProposal.PricePerByte,
		dealProposal.PaymentInterval,
		dealProposal.PaymentIntervalIncrease)
	if err != nil {
		return ctx.Trigger(rm.ProviderEventDealRejected, err)
	}
	return ctx.Trigger(rm.ProviderEventDealReceived)
}

// DecideOnDeal runs any custom deal decider and if it passes, tell client
// it's accepted, and move to the next state
func DecideOnDeal(ctx fsm.Context, env ProviderDealEnvironment, state rm.ProviderDealState) error {
	accepted, reason, err := env.RunDealDecisioningLogic(ctx.Context(), state)
	if err != nil {
		return ctx.Trigger(rm.ProviderEventDecisioningError, err)
	}
	if !accepted {
		return ctx.Trigger(rm.ProviderEventDealRejected, errors.New(reason))
	}
	err = env.DealStream(state.Identifier()).WriteDealResponse(rm.DealResponse{
		Status: rm.DealStatusAccepted,
		ID:     state.ID,
	})
	if err != nil {
		return ctx.Trigger(rm.ProviderEventWriteResponseFailed, err)
	}

	return ctx.Trigger(rm.ProviderEventDealAccepted, state.DealProposal)
}

// SendBlocks sends blocks to the client until funds are needed
func SendBlocks(ctx fsm.Context, environment ProviderDealEnvironment, deal rm.ProviderDealState) error {
	totalSent := deal.TotalSent
	totalPaidFor := big.Div(deal.FundsReceived, deal.PricePerByte).Uint64()
	var blocks []rm.Block

	// read blocks until we reach current interval
	responseStatus := rm.DealStatusFundsNeeded
	for totalSent-totalPaidFor < deal.CurrentInterval {
		block, done, err := environment.NextBlock(ctx.Context(), deal.Identifier())
		if err != nil {
			return ctx.Trigger(rm.ProviderEventBlockErrored, err)
		}
		blocks = append(blocks, block)
		totalSent += uint64(len(block.Data))
		if done {
			err := ctx.Trigger(rm.ProviderEventBlocksCompleted)
			if err != nil {
				return err
			}
			responseStatus = rm.DealStatusFundsNeededLastPayment
			break
		}
	}

	// send back response of blocks plus payment owed
	paymentOwed := big.Mul(abi.NewTokenAmount(int64(totalSent-totalPaidFor)), deal.PricePerByte)

	err := environment.DealStream(deal.Identifier()).WriteDealResponse(rm.DealResponse{
		ID:          deal.ID,
		Status:      responseStatus,
		PaymentOwed: paymentOwed,
		Blocks:      blocks,
	})

	if err != nil {
		return ctx.Trigger(rm.ProviderEventWriteResponseFailed, err)
	}

	return ctx.Trigger(rm.ProviderEventPaymentRequested, totalSent)
}

// ProcessPayment processes a payment from the client and resumes the deal if successful
func ProcessPayment(ctx fsm.Context, environment ProviderDealEnvironment, deal rm.ProviderDealState) error {
	// read payment, or fail
	payment, err := environment.DealStream(deal.Identifier()).ReadDealPayment()
	if err != nil {
		return ctx.Trigger(rm.ProviderEventReadPaymentFailed, xerrors.Errorf("reading payment: %w", err))
	}

	tok, _, err := environment.Node().GetChainHead(ctx.Context())
	if err != nil {
		return ctx.Trigger(rm.ProviderEventSaveVoucherFailed, err)
	}

	// attempt to redeem voucher
	// (totalSent * pricePerbyte) - fundsReceived
	paymentOwed := big.Sub(big.Mul(abi.NewTokenAmount(int64(deal.TotalSent)), deal.PricePerByte), deal.FundsReceived)
	received, err := environment.Node().SavePaymentVoucher(ctx.Context(), payment.PaymentChannel, payment.PaymentVoucher, nil, paymentOwed, tok)
	if err != nil {
		return ctx.Trigger(rm.ProviderEventSaveVoucherFailed, err)
	}

	// received = 0 / err = nil indicates that the voucher was already saved, but this may be ok
	// if we are making a deal with ourself - in this case, we'll instead calculate received
	// but subtracting from fund sent
	if big.Cmp(received, big.Zero()) == 0 {
		received = big.Sub(payment.PaymentVoucher.Amount, deal.FundsReceived)
	}

	// check if all payments are received to continue the deal, or send updated required payment
	if received.LessThan(paymentOwed) {
		err := environment.DealStream(deal.Identifier()).WriteDealResponse(rm.DealResponse{
			ID:          deal.ID,
			Status:      deal.Status,
			PaymentOwed: big.Sub(paymentOwed, received),
		})
		if err != nil {
			return ctx.Trigger(rm.ProviderEventWriteResponseFailed, err)
		}
		return ctx.Trigger(rm.ProviderEventPartialPaymentReceived, received)
	}

	// resume deal
	return ctx.Trigger(rm.ProviderEventPaymentReceived, received)
}

// SendFailResponse sends a failure response before closing the deal
func SendFailResponse(ctx fsm.Context, environment ProviderDealEnvironment, deal rm.ProviderDealState) error {
	stream := environment.DealStream(deal.Identifier())
	err := stream.WriteDealResponse(rm.DealResponse{
		Status:  deal.Status,
		Message: deal.Message,
		ID:      deal.ID,
	})
	if err != nil {
		return ctx.Trigger(rm.ProviderEventWriteResponseFailed, err)
	}
	return nil
}

// Finalize completes a deal
func Finalize(ctx fsm.Context, environment ProviderDealEnvironment, deal rm.ProviderDealState) error {
	err := environment.DealStream(deal.Identifier()).WriteDealResponse(rm.DealResponse{
		Status: rm.DealStatusCompleted,
		ID:     deal.ID,
	})
	if err != nil {
		return ctx.Trigger(rm.ProviderEventWriteResponseFailed, err)
	}

	return ctx.Trigger(rm.ProviderEventComplete)
}
