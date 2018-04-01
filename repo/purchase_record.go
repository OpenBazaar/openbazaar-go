package repo

import (
	"time"
)

// PurchaseRecord represents a one-to-one relationship with records
// ini the SQL datastore
type PurchaseRecord struct {
	OrderID        string
	Timestamp      time.Time
	LastNotifiedAt time.Time
}
