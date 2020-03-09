package repo

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"testing"
)

func TestToV5OrderRefund(t *testing.T) {
	var (
		refund = &pb.Refund{
			RefundTransaction: &pb.Refund_TransactionInfo{
				Value: 10000,
			},
		}
		newRefund = ToV5Refund(refund)
		expected  = "10000"
	)

	if newRefund.RefundTransaction.BigValue != expected {
		t.Errorf("Expected BigValue of %s got %s", expected, newRefund.RefundTransaction.BigValue)
	}

	if newRefund.RefundTransaction.Value != 0 {
		t.Errorf("Expected Value of 0, got %d", newRefund.RefundTransaction.Value)
	}
}
