package repo

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"testing"
)

func TestToV5OrderConfirmation(t *testing.T) {
	var (
		orderConfirmation = &pb.OrderConfirmation{
			RequestedAmount: 10000,
		}
		newOrderConfirmation = ToV5OrderConfirmation(orderConfirmation)
		expected             = "10000"
	)

	if newOrderConfirmation.BigRequestedAmount != expected {
		t.Errorf("Expected BigRequestedAmount of %s got %s", expected, orderConfirmation.BigRequestedAmount)
	}

	if newOrderConfirmation.RequestedAmount != 0 {
		t.Errorf("Expected RequestedAmount of 0, got %d", newOrderConfirmation.RequestedAmount)
	}
}
