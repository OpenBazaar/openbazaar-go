package repo_test

import (
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

func TestDisputeCaseRecordIsExpired(t *testing.T) {
	now := time.Now()
	expirationTime := time.Duration(45*24) * time.Hour
	if (&repo.DisputeCaseRecord{Timestamp: now}).IsExpired(now) != false {
		t.Error("Expected recently opened dispute to NOT be expired")
	}
	if (&repo.DisputeCaseRecord{Timestamp: now.Add(-expirationTime)}).IsExpired(now.Add(-time.Duration(1))) != false {
		t.Error("Expected a dispute to NOT be expired just before expected expiration")
	}
	if (&repo.DisputeCaseRecord{Timestamp: now.Add(-expirationTime)}).IsExpired(now) != true {
		t.Error("Expected a dispute to not expired at exactly the expiration time but was NOT")
	}
	if (&repo.DisputeCaseRecord{Timestamp: now.Add(-expirationTime)}).IsExpiredNow() != true {
		t.Error("Expected a dispute to be expired when now is after the expirationTime but was NOT")
	}
}
