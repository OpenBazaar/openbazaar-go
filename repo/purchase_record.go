package repo

import (
	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

var (
	BuyerDisputeTimeout_firstInterval  = time.Duration(15*24) * time.Hour
	BuyerDisputeTimeout_secondInterval = time.Duration(40*24) * time.Hour
	BuyerDisputeTimeout_thirdInterval  = time.Duration(44*24) * time.Hour
	BuyerDisputeTimeout_lastInterval   = time.Duration(45*24) * time.Hour

	BuyerDisputeExpiry_firstInterval  = time.Duration(15*24) * time.Hour
	BuyerDisputeExpiry_secondInterval = time.Duration(40*24) * time.Hour
	BuyerDisputeExpiry_lastInterval   = time.Duration(44*24) * time.Hour

	BuyerDisputeExpiry_totalDuration = time.Duration(45*24) * time.Hour
)

// PurchaseRecord represents a one-to-one relationship with records
// in the SQL datastore
type PurchaseRecord struct {
	Contract                     *pb.RicardianContract
	DisputedAt                   time.Time
	OrderID                      string
	OrderState                   pb.OrderState
	Timestamp                    time.Time
	LastDisputeTimeoutNotifiedAt time.Time
	LastDisputeExpiryNotifiedAt  time.Time
}

// IsDisputeable returns whether the Purchase is in a state that it can be disputed with a
// third-party moderator
func (r *PurchaseRecord) IsDisputeable() bool {
	if r.IsModeratedContract() {
		switch r.OrderState {
		case pb.OrderState_PENDING, pb.OrderState_AWAITING_FULFILLMENT, pb.OrderState_FULFILLED:
			return true
		}
	}
	return false
}

// IsModeratedContract returns whether the contract includes a third-party moderator
func (r *PurchaseRecord) IsModeratedContract() bool {
	return r.Contract != nil && r.Contract.BuyerOrder != nil && r.Contract.BuyerOrder.Payment != nil && r.Contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED
}

// BuildBuyerDisputeTimeoutFirstNotification returns a Notification with ExpiresIn set for the First Interval
func (r *PurchaseRecord) BuildBuyerDisputeTimeoutFirstNotification(createdAt time.Time) *Notification {
	return r.buildBuyerDisputeTimeout(BuyerDisputeTimeout_firstInterval, createdAt)
}

// BuildBuyerDisputeTimeoutSecondNotification returns a Notification with ExpiresIn set for the Second Interval
func (r *PurchaseRecord) BuildBuyerDisputeTimeoutSecondNotification(createdAt time.Time) *Notification {
	return r.buildBuyerDisputeTimeout(BuyerDisputeTimeout_secondInterval, createdAt)
}

// BuildBuyerDisputeTimeoutThirdNotification returns a Notification with ExpiresIn set for the Third Interval
func (r *PurchaseRecord) BuildBuyerDisputeTimeoutThirdNotification(createdAt time.Time) *Notification {
	return r.buildBuyerDisputeTimeout(BuyerDisputeTimeout_thirdInterval, createdAt)
}

// BuildBuyerDisputeTimeoutLastNotification returns a Notification with ExpiresIn set for the Last Interval
func (r *PurchaseRecord) BuildBuyerDisputeTimeoutLastNotification(createdAt time.Time) *Notification {
	return r.buildBuyerDisputeTimeout(BuyerDisputeTimeout_lastInterval, createdAt)
}

func (r *PurchaseRecord) buildBuyerDisputeTimeout(interval time.Duration, createdAt time.Time) *Notification {
	timeRemaining := BuyerDisputeTimeout_lastInterval - interval
	notification := &BuyerDisputeTimeout{
		ID:        NewNotificationID(),
		ExpiresIn: uint(timeRemaining.Seconds()),
		OrderID:   r.OrderID,
		Thumbnail: Thumbnail{},
		Type:      NotifierTypeBuyerDisputeTimeout,
	}
	if len(r.Contract.VendorListings) > 0 && len(r.Contract.VendorListings[0].Item.Images) > 0 {
		notification.Thumbnail = Thumbnail{
			Tiny:  r.Contract.VendorListings[0].Item.Images[0].Tiny,
			Small: r.Contract.VendorListings[0].Item.Images[0].Small,
		}
	}
	return NewNotification(notification, createdAt, false)
}

// BuildBuyerDisputeExpiryFirstNotification returns a Notification with ExpiresIn set for the First Interval
func (r *PurchaseRecord) BuildBuyerDisputeExpiryFirstNotification(createdAt time.Time) *Notification {
	return r.buildBuyerDisputeExpiry(BuyerDisputeExpiry_firstInterval, createdAt)
}

// BuildBuyerDisputeExpirySecondNotification returns a Notification with ExpiresIn set for the Second Interval
func (r *PurchaseRecord) BuildBuyerDisputeExpirySecondNotification(createdAt time.Time) *Notification {
	return r.buildBuyerDisputeExpiry(BuyerDisputeExpiry_secondInterval, createdAt)
}

// BuildBuyerDisputeExpiryLastNotification returns a Notification with ExpiresIn set for the Last Interval
func (r *PurchaseRecord) BuildBuyerDisputeExpiryLastNotification(createdAt time.Time) *Notification {
	return r.buildBuyerDisputeExpiry(BuyerDisputeExpiry_lastInterval, createdAt)
}

func (r *PurchaseRecord) buildBuyerDisputeExpiry(interval time.Duration, createdAt time.Time) *Notification {
	timeRemaining := BuyerDisputeExpiry_lastInterval + (time.Duration(24) * time.Hour) - interval
	notification := &BuyerDisputeExpiry{
		ID:        NewNotificationID(),
		ExpiresIn: uint(timeRemaining.Seconds()),
		OrderID:   r.OrderID,
		Thumbnail: Thumbnail{},
		Type:      NotifierTypeBuyerDisputeExpiry,
	}
	if len(r.Contract.VendorListings) > 0 && len(r.Contract.VendorListings[0].Item.Images) > 0 {
		notification.Thumbnail = Thumbnail{
			Tiny:  r.Contract.VendorListings[0].Item.Images[0].Tiny,
			Small: r.Contract.VendorListings[0].Item.Images[0].Small,
		}
	}
	return NewNotification(notification, createdAt, false)
}
