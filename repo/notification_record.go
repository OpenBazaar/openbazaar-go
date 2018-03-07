package repo

import (
	"encoding/json"
	"strings"
	"time"
)

type NotificationRecord struct {
	Notification Notifier
	CreatedAt    time.Time
}

// GenerateID generates and returns an ID string for persisting and sufficiently unique
// for transmission over the network
func GenerateID() string {
	return NewID()
}

// UnmarshalNotificationRecord accepts fields as persisted in the datastore and returns
// a valid NotificationRecord. An error is returned if a valid record cannot be created
func UnmarshalNotificationRecord(notificationJson string, createdAt int64) (result *NotificationRecord, err error) {
	n := DisputeNotification{}
	if err = json.Unmarshal([]byte(notificationJson), &n); err != nil {
		return
	}
	result = &NotificationRecord{
		CreatedAt:    time.Unix(createdAt, 0),
		Notification: &n,
	}
	return
}

// GetID returns the assigned unique ID string on the Notification
func (r *NotificationRecord) GetID() string { return r.Notification.GetNotificationID() }

// GetType returns string representation of type
func (r *NotificationRecord) GetType() string { return r.Notification.GetNotificationType() }

// GetDowncaseType returns string representation of type suitable for persisting
// in the datastore
func (r *NotificationRecord) GetDowncaseType() string {
	return strings.ToLower(r.Notification.GetNotificationType())
}

// GetSQLTimestamp returns an int representation of when the record was
// created suitable for persisting in the datastore
func (r *NotificationRecord) GetSQLTimestamp() int {
	return int(r.CreatedAt.Unix())
}

// NotificationToMarshalJSON returns a string representation of the Notification suitable
// for persisting in the datastore
func (r *NotificationRecord) MarshalNotificationToJSON() (string, error) {
	s, err := json.Marshal(r.Notification)
	if err != nil {
		return "", err
	}
	return string(s), nil
}
