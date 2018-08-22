package repo_test

import (
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
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

func TestResolutionPaymentOutpoints(t *testing.T) {
	subject := factory.NewDisputeCaseRecord()
	buyerOutpoints := []*pb.Outpoint{{Hash: "buyeroutpoint"}}
	vendorOutpoints := []*pb.Outpoint{{Hash: "vendoroutpoint"}}
	subject.BuyerOutpoints = buyerOutpoints
	subject.VendorOutpoints = vendorOutpoints

	if subject.ResolutionPaymentOutpoints(repo.PayoutRatio{Buyer: 100, Vendor: 0})[0].Hash != buyerOutpoints[0].Hash {
		t.Error("expected outpoints to return buyer set with buyer having majority payout")
	}
	if subject.ResolutionPaymentOutpoints(repo.PayoutRatio{Buyer: 0, Vendor: 100})[0].Hash != vendorOutpoints[0].Hash {
		t.Error("expected outpoints to return vendor set with vendor having majority payout")
	}

	subject.BuyerOutpoints = nil
	if subject.ResolutionPaymentOutpoints(repo.PayoutRatio{Buyer: 100, Vendor: 0})[0].Hash != vendorOutpoints[0].Hash {
		t.Error("expected outpoints to substitude vendor set when buyer set is missing")
	}

	subject.BuyerOutpoints = buyerOutpoints
	subject.VendorOutpoints = nil
	if subject.ResolutionPaymentOutpoints(repo.PayoutRatio{Buyer: 0, Vendor: 100})[0].Hash != buyerOutpoints[0].Hash {
		t.Error("expected outpoints to substitude vendor set when buyer set is missing")
	}
}
