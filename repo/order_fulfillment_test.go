package repo

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"testing"
)

func TestToV5OrderFulfillment(t *testing.T) {
	var (
		orderFulfillment = &pb.OrderFulfillment{
			Payout: &pb.OrderFulfillment_Payout{
				PayoutFeePerByte: 10,
			},
		}
		newOrderFulfillment = ToV5OrderFulfillment(orderFulfillment)
		expected            = "10"
	)

	if newOrderFulfillment.Payout.BigPayoutFeePerByte != expected {
		t.Errorf("Expected BigPayoutFeePerByte of %s got %s", expected, newOrderFulfillment.Payout.BigPayoutFeePerByte)
	}

	if newOrderFulfillment.Payout.PayoutFeePerByte != 0 {
		t.Errorf("Expected PayoutFeePerByte of 0, got %d", newOrderFulfillment.Payout.PayoutFeePerByte)
	}
}
