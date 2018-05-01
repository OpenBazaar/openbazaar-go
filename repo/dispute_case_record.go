package repo

import (
	"time"
)

// DisputeCaseRecord is a one-to-one relationship with records in the
// SQL datastore
type DisputeCaseRecord struct {
	CaseID         string
	Timestamp      time.Time
	LastNotifiedAt time.Time
}

// BuildZeroDayNotification returns a Notification for a new DisputeCaseRecord
// which was just opened
func (r *DisputeCaseRecord) BuildZeroDayNotification(createdAt time.Time) *Notification {
	return r.buildDisputeAgingNotification(NotifierTypeDisputeAgedZeroDays, createdAt)
}

// BuildFifteenDayNotification returns a Notification that alerts a DisputeCaseRecord
// is more than 15 days old
func (r *DisputeCaseRecord) BuildFifteenDayNotification(createdAt time.Time) *Notification {
	return r.buildDisputeAgingNotification(NotifierTypeDisputeAgedFifteenDays, createdAt)
}

// BuildFourtyDayNotification returns a Notification that alerts a isputeCaseRecord
// is more than 40 days old
func (r *DisputeCaseRecord) BuildThirtyDayNotification(createdAt time.Time) *Notification {
	return r.buildDisputeAgingNotification(NotifierTypeDisputeAgedFourtyDays, createdAt)
}

// BuildFourtyFiveDayNotification returns a Notification that alerts a DisputeCaseRecord
// is more than 44 days old and about to expire
func (r *DisputeCaseRecord) BuildFourtyFourDayNotification(createdAt time.Time) *Notification {
	return r.buildDisputeAgingNotification(NotifierTypeDisputeAgedFourtyFourDays, createdAt)
}

// BuildThirtyDayNotification returns a Notification that alerts a DisputeCaseRecord
// is more than 45 days old and already expired
func (r *DisputeCaseRecord) BuildFourtyFiveDayNotification(createdAt time.Time) *Notification {
	return r.buildDisputeAgingNotification(NotifierTypeDisputeAgedFourtyFiveDays, createdAt)
}

func (r *DisputeCaseRecord) buildDisputeAgingNotification(nType NotificationType, createdAt time.Time) *Notification {
	notification := DisputeAgingNotification{
		ID:     NewNotificationID(),
		Type:   nType,
		CaseID: r.CaseID,
	}
	return NewNotification(notification, createdAt, false)
}
