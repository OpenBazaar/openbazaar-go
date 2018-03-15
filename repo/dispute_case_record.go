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

// BuildZeroDayNotification returns a NotificationRecord for a new DisputeCaseRecord
// which was just opened
func (r *DisputeCaseRecord) BuildZeroDayNotification(createdAt time.Time) *NotificationRecord {
	notification := &DisputeNotification{
		ID:     GenerateID(),
		Type:   NotifierTypeDisputeAgedZeroDaysOld,
		CaseID: r.CaseID,
	}
	return &NotificationRecord{Notification: notification, CreatedAt: createdAt}
}

// BuildFifteenDayNotification returns a NotificationRecord that alerts a DisputeCaseRecord
// is more than 15 days old
func (r *DisputeCaseRecord) BuildFifteenDayNotification(createdAt time.Time) *NotificationRecord {
	notification := &DisputeNotification{
		ID:     GenerateID(),
		Type:   NotifierTypeDisputeAgedFifteenDaysOld,
		CaseID: r.CaseID,
	}
	return &NotificationRecord{Notification: notification, CreatedAt: createdAt}
}

// BuildThirtyDayNotification returns a NotificationRecord that alerts a isputeCaseRecord
// is more than 30 days old
func (r *DisputeCaseRecord) BuildThirtyDayNotification(createdAt time.Time) *NotificationRecord {
	notification := &DisputeNotification{
		ID:     GenerateID(),
		Type:   NotifierTypeDisputeAgedThirtyDaysOld,
		CaseID: r.CaseID,
	}
	return &NotificationRecord{Notification: notification, CreatedAt: createdAt}
}

// BuildFourtyFiveDayNotification returns a NotificationRecord that alerts a DisputeCaseRecord
// is more than 44 days old and about to expire
func (r *DisputeCaseRecord) BuildFourtyFourDayNotification(createdAt time.Time) *NotificationRecord {
	notification := &DisputeNotification{
		ID:     GenerateID(),
		Type:   NotifierTypeDisputeAgedFourtyFourDaysOld,
		CaseID: r.CaseID,
	}
	return &NotificationRecord{Notification: notification, CreatedAt: createdAt}
}

// BuildThirtyDayNotification returns a NotificationRecord that alerts a DisputeCaseRecord
// is more than 45 days old and already expired
func (r *DisputeCaseRecord) BuildFourtyFiveDayNotification(createdAt time.Time) *NotificationRecord {
	notification := &DisputeNotification{
		ID:     GenerateID(),
		Type:   NotifierTypeDisputeAgedFourtyFiveDaysOld,
		CaseID: r.CaseID,
	}
	return &NotificationRecord{Notification: notification, CreatedAt: createdAt}
}
