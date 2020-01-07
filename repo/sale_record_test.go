package repo_test

import (
	"testing"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestSaleRecordIsDisputeable(t *testing.T) {
	subject := factory.NewSaleRecord()
	subject.Contract.BuyerOrder.Payment.Method = pb.Order_Payment_DIRECT
	if subject.IsDisputeable() {
		t.Error("Expected direct payment to NOT be disputeable")
	}

	subject = factory.NewSaleRecord()
	subject.Contract.BuyerOrder.Payment.Method = pb.Order_Payment_ADDRESS_REQUEST
	if subject.IsDisputeable() {
		t.Error("Expected address requested payment to NOT be disputeable")
	}

	subject = factory.NewSaleRecord()
	subject.Contract.BuyerOrder.Payment.Method = pb.Order_Payment_MODERATED
	undisputeableStates := []pb.OrderState{
		pb.OrderState_AWAITING_FULFILLMENT,
		pb.OrderState_AWAITING_PAYMENT,
		pb.OrderState_AWAITING_PICKUP,
		pb.OrderState_COMPLETED,
		pb.OrderState_CANCELED,
		pb.OrderState_DECLINED,
		pb.OrderState_REFUNDED,
		pb.OrderState_DISPUTED,
		pb.OrderState_DECIDED,
		pb.OrderState_RESOLVED,
		pb.OrderState_PAYMENT_FINALIZED,
		pb.OrderState_PENDING,
		pb.OrderState_PROCESSING_ERROR,
	}
	for _, s := range undisputeableStates {
		subject.OrderState = s
		if subject.IsDisputeable() {
			t.Errorf("Expected order in state '%s' to NOT be disputeable", s)
		}
	}
	disputeableStates := []pb.OrderState{
		pb.OrderState_PARTIALLY_FULFILLED,
		pb.OrderState_FULFILLED,
	}
	for _, s := range disputeableStates {
		subject.OrderState = s
		subject.Contract.BuyerOrder.Payment.Method = pb.Order_Payment_DIRECT
		if subject.IsDisputeable() {
			t.Errorf("Expected UNMODERATED order in state '%s' to NOT be disputeable", s)
		}

		subject.Contract.BuyerOrder.Payment.Method = pb.Order_Payment_MODERATED
		if !subject.IsDisputeable() {
			t.Errorf("Expected order in state '%s' to BE disputeable", s)
		}
	}
}

func TestSaleRecord_SupportsTimedEscrowRelease(t *testing.T) {
	tests := []struct {
		currency              string
		supportsEscrowRelease bool
	}{
		{
			"BTC",
			true,
		},
		{
			"TBTC",
			true,
		},
		{
			"BCH",
			true,
		},
		{
			"TBCH",
			true,
		},
		{
			"LTC",
			true,
		},
		{
			"TLTC",
			true,
		},
		{
			"ZEC",
			false,
		},
		{
			"TZEC",
			false,
		},
	}
	subject := factory.NewSaleRecord()
	for _, test := range tests {
		subject.Contract.BuyerOrder.Payment.AmountCurrency = &pb.CurrencyDefinition{Code: test.currency, Divisibility: 8}
		supportsEscrowRelease := subject.SupportsTimedEscrowRelease()
		if supportsEscrowRelease != test.supportsEscrowRelease {
			t.Errorf("SupportsEscrowRelease test failed for %s."+
				" Expected %t, got %t", test.currency, test.supportsEscrowRelease, supportsEscrowRelease)
		}
	}
}
