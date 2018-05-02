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
		Type:      NotifierTypeVendorDisputeTimeout,
		OrderID:   r.OrderID,
		ExpiresIn: uint(0),
	}
	return NewNotification(notification, createdAt, false)
}
