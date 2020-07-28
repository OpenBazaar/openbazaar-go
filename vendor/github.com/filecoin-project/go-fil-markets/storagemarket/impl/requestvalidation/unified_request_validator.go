package requestvalidation

import (
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-statestore"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p-core/peer"
)

// UnifiedRequestValidator is a data transfer request validator that validates
// StorageDataTransferVoucher from the given state store
// It can be made to only accept push requests (Provider) or pull requests (Client)
// by passing nil for the statestore value for pushes or pulls
type UnifiedRequestValidator struct {
	pushDeals *statestore.StateStore
	pullDeals *statestore.StateStore
}

// NewUnifiedRequestValidator returns a new instance of UnifiedRequestValidator
func NewUnifiedRequestValidator(pushDeals *statestore.StateStore, pullDeals *statestore.StateStore) *UnifiedRequestValidator {
	return &UnifiedRequestValidator{
		pushDeals: pushDeals,
		pullDeals: pullDeals,
	}
}

// SetPushDeals sets the store to look up push deals with
func (v *UnifiedRequestValidator) SetPushDeals(pushDeals *statestore.StateStore) {
	v.pushDeals = pushDeals
}

// SetPullDeals sets the store to look up pull deals with
func (v *UnifiedRequestValidator) SetPullDeals(pullDeals *statestore.StateStore) {
	v.pullDeals = pullDeals
}

// ValidatePush implements the ValidatePush method of a data transfer request validator.
// If no pushStore exists, it rejects the request
// Otherwise, it calls the ValidatePush function to validate the deal
func (v *UnifiedRequestValidator) ValidatePush(sender peer.ID, voucher datatransfer.Voucher, baseCid cid.Cid, selector ipld.Node) error {
	if v.pushDeals == nil {
		return ErrNoPushAccepted
	}

	return ValidatePush(v.pushDeals, sender, voucher, baseCid, selector)
}

// ValidatePull implements the ValidatePull method of a data transfer request validator.
// If no pullStore exists, it rejects the request
// Otherwise, it calls the ValidatePull function to validate the deal
func (v *UnifiedRequestValidator) ValidatePull(receiver peer.ID, voucher datatransfer.Voucher, baseCid cid.Cid, selector ipld.Node) error {
	if v.pullDeals == nil {
		return ErrNoPullAccepted
	}

	return ValidatePull(v.pullDeals, receiver, voucher, baseCid, selector)
}

var _ datatransfer.RequestValidator = &UnifiedRequestValidator{}
