package core_test

import (
	"strings"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/golang/protobuf/ptypes"
)

func TestReleaseFundsAfterTimeoutErrors(t *testing.T) {
	sale := factory.NewSaleRecord()
	sale.Contract = factory.NewDisputedContract()

	// Fresh dispute timestamp test
	disputeStart, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		t.Fatal(err)
	} else {
		sale.Contract.Dispute.Timestamp = disputeStart
	}
	node := &core.OpenBazaarNode{}

	err = node.ReleaseFundsAfterTimeout(sale.Contract, []*wallet.TransactionRecord{})
	if err == nil {
		t.Fatal("Expected sale which just now opened a dispute to return an error")
	}
	if !strings.Contains(err.Error(), core.ErrPrematureReleaseOfTimedoutEscrowFunds.Error()) {
		t.Error("Expected error to indicate the problem to be due to premature release of escrow funds")
	}

	// Expiring dispute timestamp test
	disputeStart, err = ptypes.TimestampProto(time.Now().Add(time.Duration(repo.DisputeTotalDurationHours) * time.Hour).Add(time.Duration(-1) * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	sale.Contract.Dispute.Timestamp = disputeStart
	err = node.ReleaseFundsAfterTimeout(sale.Contract, []*wallet.TransactionRecord{})
	if err == nil {
		t.Fatal("Expected sale whose dispute funds are one minute prior to timing out to return an error")
	}
	if !strings.Contains(err.Error(), core.ErrPrematureReleaseOfTimedoutEscrowFunds.Error()) {
		t.Error("Expected error to indicate the problem to be due to premature release of escrow funds")
	}
}
