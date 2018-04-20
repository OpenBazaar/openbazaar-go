package repo

import (
	"time"
)

// SaleRecord represents a one-to-one relationship with records
// in the SQL datastore
type SaleRecord struct {
	OrderID        string
	Timestamp      time.Time
	LastNotifiedAt time.Time
}

// BuildThirtyDayNotification returns a Notification that alerts a SaleRecord
// is more than 45 days old and already expired
func (r *SaleRecord) BuildFourtyFiveDayNotification(createdAt time.Time) *Notification {
	notification := &SaleAgingNotification{
		ID:      NewNotificationID(),
		Type:    NotifierTypeSaleAgedFourtyFiveDays,
		OrderID: r.OrderID,
	}
	return NewNotification(notification, createdAt, false)
}
