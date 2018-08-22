package repo

import (
	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

var (
	ModeratorDisputeExpiry_firstInterval  = time.Duration(15*24) * time.Hour
	ModeratorDisputeExpiry_secondInterval = time.Duration(30*24) * time.Hour
	ModeratorDisputeExpiry_thirdInterval  = time.Duration(44*24) * time.Hour
	ModeratorDisputeExpiry_lastInterval   = time.Duration(45*24) * time.Hour
)

// DisputeCaseRecord is a one-to-one relationship with records in the
// SQL datastore
type DisputeCaseRecord struct {
	CaseID                      string
	Claim                       string
	OrderState                  pb.OrderState
	Timestamp                   time.Time
	LastDisputeExpiryNotifiedAt time.Time
	BuyerContract               *pb.RicardianContract
	BuyerOutpoints              []*pb.Outpoint
	BuyerPayoutAddress          string
	VendorContract              *pb.RicardianContract
	VendorOutpoints             []*pb.Outpoint
	VendorPayoutAddress         string
	IsBuyerInitiated            bool
	CoinType                    string
	PaymentCoin                 string
}

// BuildModeratorDisputeExpiryFirstNotification returns a Notification with ExpiresIn set for the First Interval
func (r *DisputeCaseRecord) BuildModeratorDisputeExpiryFirstNotification(createdAt time.Time) *Notification {
	return r.buildModeratorDisputeExpiry(ModeratorDisputeExpiry_firstInterval, createdAt)
}

// BuildModeratorDisputeExpirySecondNotification returns a Notification with ExpiresIn set for the Second Interval
func (r *DisputeCaseRecord) BuildModeratorDisputeExpirySecondNotification(createdAt time.Time) *Notification {
	return r.buildModeratorDisputeExpiry(ModeratorDisputeExpiry_secondInterval, createdAt)
}

// BuildModeratorDisputeExpiryThirdNotification returns a Notification with ExpiresIn set for the Third Interval
func (r *DisputeCaseRecord) BuildModeratorDisputeExpiryThirdNotification(createdAt time.Time) *Notification {
	return r.buildModeratorDisputeExpiry(ModeratorDisputeExpiry_thirdInterval, createdAt)
}

// BuildModeratorDisputeExpiryLastNotification returns a Notification with ExpiresIn set for the Last Interval
func (r *DisputeCaseRecord) BuildModeratorDisputeExpiryLastNotification(createdAt time.Time) *Notification {
	return r.buildModeratorDisputeExpiry(ModeratorDisputeExpiry_lastInterval, createdAt)
}

func (r *DisputeCaseRecord) buildModeratorDisputeExpiry(interval time.Duration, createdAt time.Time) *Notification {
	timeRemaining := ModeratorDisputeExpiry_lastInterval - interval
	notification := ModeratorDisputeExpiry{
		ID:        NewNotificationID(),
		Type:      NotifierTypeModeratorDisputeExpiry,
		CaseID:    r.CaseID,
		ExpiresIn: uint(timeRemaining.Seconds()),
		Thumbnail: Thumbnail{},
	}
	if r.IsBuyerInitiated {
		if len(r.BuyerContract.VendorListings) > 0 && len(r.BuyerContract.VendorListings[0].Item.Images) > 0 {
			notification.Thumbnail = Thumbnail{
				Tiny:  r.BuyerContract.VendorListings[0].Item.Images[0].Tiny,
				Small: r.BuyerContract.VendorListings[0].Item.Images[0].Small,
			}
		}
	} else {
		if len(r.VendorContract.VendorListings) > 0 && len(r.VendorContract.VendorListings[0].Item.Images) > 0 {
			notification.Thumbnail = Thumbnail{
				Tiny:  r.VendorContract.VendorListings[0].Item.Images[0].Tiny,
				Small: r.VendorContract.VendorListings[0].Item.Images[0].Small,
			}
		}
	}
	return NewNotification(notification, createdAt, false)
}

// IsExpired returns a bool indicating whether the case is still open right now
func (r *DisputeCaseRecord) IsExpiredNow() bool {
	return r.IsExpired(time.Now())
}

// IsExpired returns a bool indicating whether the case is still open
func (r *DisputeCaseRecord) IsExpired(when time.Time) bool {
	expiresAt := r.Timestamp.Add(ModeratorDisputeExpiry_lastInterval)
	return when.Equal(expiresAt) || when.After(expiresAt)
}

// Contract returns the contract from the dispute if one has been supplied by
// either the buyer or vendor
func (r *DisputeCaseRecord) Contract() *pb.RicardianContract {
	contract := r.BuyerContract
	if contract == nil {
		contract = r.VendorContract
	}
	return contract
}

// ResolutionPaymentFeePerByte returns the preferred outpoints to be used when resolving
// a pending DisputeCaseResolution based on the provided PayoutRatio
func (r *DisputeCaseRecord) ResolutionPaymentFeePerByte(ratio PayoutRatio, defaultFee uint64) uint64 {
	switch {
	case ratio.BuyerMajority(), ratio.EvenMajority():
		return r.BuyerContract.BuyerOrder.RefundFee
	case ratio.VendorMajority():
		if len(r.VendorContract.VendorOrderFulfillment) > 0 && r.VendorContract.VendorOrderFulfillment[0].Payout != nil {
			return r.VendorContract.VendorOrderFulfillment[0].Payout.PayoutFeePerByte
		}
	}
	return defaultFee
}

// ResolutionPaymentOutpoints returns the preferred outpoints to be used when resolving
// a pending DisputeCaseResolution based on the provided PayoutRatio
func (r *DisputeCaseRecord) ResolutionPaymentOutpoints(ratio PayoutRatio) (outpoints []*pb.Outpoint) {
	if ratio.VendorMajority() {
		outpoints = r.VendorOutpoints
		if outpoints == nil {
			outpoints = r.BuyerOutpoints
		}
	} else {
		outpoints = r.BuyerOutpoints
		if outpoints == nil {
			outpoints = r.VendorOutpoints
		}
	}
	return
}

// ResolutionPaymentContract returns the preferred contract to be used when resolving
// a pending DisputeCaseRecord based on the provided PayoutRatio
func (r *DisputeCaseRecord) ResolutionPaymentContract(ratio PayoutRatio) (contract *pb.RicardianContract) {
	if ratio.VendorAny() {
		return r.VendorContract
	}
	return r.BuyerContract
}
