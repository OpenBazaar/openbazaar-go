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
	CaseID           string
	Timestamp        time.Time
	LastNotifiedAt   time.Time
	BuyerContract    *pb.RicardianContract
	VendorContract   *pb.RicardianContract
	IsBuyerInitiated bool
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
	}
	return NewNotification(notification, createdAt, false)
}
