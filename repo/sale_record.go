package repo

import (
	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

var (
	VendorDisputeTimeout_lastInterval = time.Duration(45*24) * time.Hour
)

// SaleRecord represents a one-to-one relationship with records
// in the SQL datastore
type SaleRecord struct {
	Contract       *pb.RicardianContract
	OrderID        string
	Timestamp      time.Time
	LastNotifiedAt time.Time
}

// BuildVendorDisputeTimeoutLastNotification returns a Notification that alerts a SaleRecord
// is more than 45 days old and already expired
func (r *SaleRecord) BuildVendorDisputeTimeoutLastNotification(createdAt time.Time) *Notification {
	notification := &VendorDisputeTimeout{
		ID:        NewNotificationID(),
		ExpiresIn: uint(0),
		OrderID:   r.OrderID,
		Thumbnail: Thumbnail{},
		Type:      NotifierTypeVendorDisputeTimeout,
	}
	if len(r.Contract.VendorListings) > 0 && len(r.Contract.VendorListings[0].Item.Images) > 0 {
		notification.Thumbnail = Thumbnail{
			Tiny:  r.Contract.VendorListings[0].Item.Images[0].Tiny,
			Small: r.Contract.VendorListings[0].Item.Images[0].Small,
		}
	}
	return NewNotification(notification, createdAt, false)
}
