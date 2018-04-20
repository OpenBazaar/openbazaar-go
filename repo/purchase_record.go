package repo

import (
	"time"
)

// PurchaseRecord represents a one-to-one relationship with records
// in the SQL datastore
type PurchaseRecord struct {
	OrderID        string
	Timestamp      time.Time
	LastNotifiedAt time.Time
}

// BuildZeroDayNotification returns a Notification for a new PurchaseRecord
// which was just opened
func (r *PurchaseRecord) BuildZeroDayNotification(createdAt time.Time) *Notification {
	return r.buildPurchaseAgingNotification(NotifierTypePurchaseAgedZeroDays, createdAt)
}

// BuildFifteenDayNotification returns a Notification that alerts a PurchaseRecord
// is more than 15 days old
func (r *PurchaseRecord) BuildFifteenDayNotification(createdAt time.Time) *Notification {
	return r.buildPurchaseAgingNotification(NotifierTypePurchaseAgedFifteenDays, createdAt)
}

// BuildFourtyDayNotification returns a Notification that alerts a PurchaseRecord
// is more than 40 days old
func (r *PurchaseRecord) BuildFourtyDayNotification(createdAt time.Time) *Notification {
	return r.buildPurchaseAgingNotification(NotifierTypePurchaseAgedFourtyDays, createdAt)
}

// BuildFourtyFiveDayNotification returns a Notification that alerts a PurchaseRecord
// is more than 44 days old and about to expire
func (r *PurchaseRecord) BuildFourtyFourDayNotification(createdAt time.Time) *Notification {
	return r.buildPurchaseAgingNotification(NotifierTypePurchaseAgedFourtyFourDays, createdAt)
}

// BuildThirtyDayNotification returns a Notification that alerts a PurchaseRecord
// is more than 45 days old and already expired
func (r *PurchaseRecord) BuildFourtyFiveDayNotification(createdAt time.Time) *Notification {
	return r.buildPurchaseAgingNotification(NotifierTypePurchaseAgedFourtyFiveDays, createdAt)
}

func (r *PurchaseRecord) buildPurchaseAgingNotification(nType NotificationType, createdAt time.Time) *Notification {
	notification := &PurchaseAgingNotification{
		ID:      NewNotificationID(),
		Type:    nType,
		OrderID: r.OrderID,
	}
	return NewNotification(notification, createdAt, false)
}
