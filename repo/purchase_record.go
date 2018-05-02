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
)

// PurchaseRecord represents a one-to-one relationship with records
// in the SQL datastore
type PurchaseRecord struct {
	Contract       *pb.RicardianContract
	OrderID        string
	Timestamp      time.Time
	LastNotifiedAt time.Time
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
