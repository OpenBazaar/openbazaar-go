package repo_test

import (
	"testing"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestPurchaseRecordIsDisputeable(t *testing.T) {
	subject := factory.NewPurchaseRecord()
	subject.Contract.BuyerOrder.Payment.Method = pb.Order_Payment_DIRECT
	if subject.IsDisputeable() {
		t.Error("Expected direct payment to NOT be disputeable")
	}

	subject = factory.NewPurchaseRecord()
	subject.Contract.BuyerOrder.Payment.Method = pb.Order_Payment_ADDRESS_REQUEST
	if subject.IsDisputeable() {
		t.Error("Expected address requested payment to NOT be disputeable")
	}

	subject = factory.NewPurchaseRecord()
	subject.Contract.BuyerOrder.Payment.Method = pb.Order_Payment_MODERATED
	undisputeableStates := []pb.OrderState{
		pb.OrderState_AWAITING_PAYMENT,
		pb.OrderState_AWAITING_PICKUP,
		pb.OrderState_PARTIALLY_FULFILLED,
		pb.OrderState_COMPLETED,
		pb.OrderState_CANCELED,
		pb.OrderState_DECLINED,
		pb.OrderState_REFUNDED,
		pb.OrderState_DISPUTED,
		pb.OrderState_DECIDED,
		pb.OrderState_RESOLVED,
		pb.OrderState_PAYMENT_FINALIZED,
		pb.OrderState_PROCESSING_ERROR,
	}
	for _, s := range undisputeableStates {
		subject.OrderState = s
		if subject.IsDisputeable() {
			t.Errorf("Expected order in state '%s' to NOT be disputeable", s)
		}
	}
	disputeableStates := []pb.OrderState{
		pb.OrderState_PENDING,
		pb.OrderState_AWAITING_FULFILLMENT,
		pb.OrderState_FULFILLED,
	}
	for _, s := range disputeableStates {
		subject.OrderState = s
		subject.Contract.BuyerOrder.Payment.Method = pb.Order_Payment_DIRECT
		if subject.IsDisputeable() {
			t.Errorf("Expected UNMODERATED order in state '%s' to NOT be disputeable", s)
		}

		subject.OrderState = s
		subject.Contract.BuyerOrder.Payment.Method = pb.Order_Payment_MODERATED
		if !subject.IsDisputeable() {
			t.Errorf("Expected order in state '%s' to BE disputeable", s)
		}
	}
}
