package repo_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

func TestNotificationMarshalling(t *testing.T) {
	var exampleNotifications = []repo.Notifier{
		repo.CompletionNotification{
			ID:      "orderCompletionID",
			Type:    repo.NotifierTypeCompletionNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.DisputeAcceptedNotification{
			ID:      "disputeAcceptedID",
			Type:    repo.NotifierTypeDisputeAcceptedNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.DisputeCloseNotification{
			ID:      "disputeCloseID",
			Type:    repo.NotifierTypeDisputeCloseNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.DisputeOpenNotification{
			ID:      "disputeOpenID",
			Type:    repo.NotifierTypeDisputeOpenNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.DisputeUpdateNotification{
			ID:      "disputeUpdateID",
			Type:    repo.NotifierTypeDisputeUpdateNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.FollowNotification{
			ID:     "followID",
			Type:   repo.NotifierTypeFollowNotification,
			PeerId: repo.NewNotificationID(),
		},
		repo.FulfillmentNotification{
			ID:      "fulfillmentID",
			Type:    repo.NotifierTypeFulfillmentNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.ModeratorAddNotification{
			ID:     "moderatorAddID",
			Type:   repo.NotifierTypeModeratorAddNotification,
			PeerId: repo.NewNotificationID(),
		},
		repo.ModeratorDisputeExpiry{
			ID:     "disputeNotificationID",
			Type:   repo.NotifierTypeModeratorDisputeExpiry,
			CaseID: repo.NewNotificationID(),
		},
		repo.ModeratorRemoveNotification{
			ID:     "moderatorRemoveID",
			Type:   repo.NotifierTypeModeratorRemoveNotification,
			PeerId: repo.NewNotificationID(),
		},
		repo.OrderCancelNotification{
			ID:      "orderCancelID",
			Type:    repo.NotifierTypeOrderCancelNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.OrderConfirmationNotification{
			ID:      "orderConfirmID",
			Type:    repo.NotifierTypeOrderConfirmationNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.OrderDeclinedNotification{
			ID:      "orderDeclinedID",
			Type:    repo.NotifierTypeOrderDeclinedNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.OrderNotification{
			ID:      "orderNotificationID",
			Type:    repo.NotifierTypeOrderNewNotification,
			BuyerID: repo.NewNotificationID(),
		},
		repo.PaymentNotification{
			ID:      "paymentID",
			Type:    repo.NotifierTypePaymentNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.ProcessingErrorNotification{
			ID:      "processingErrorID",
			Type:    repo.NotifierTypeProcessingErrorNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.BuyerDisputeTimeout{
			ID:      "buyerDisputeID",
			Type:    repo.NotifierTypeBuyerDisputeTimeout,
			OrderID: repo.NewNotificationID(),
		},
		repo.RefundNotification{
			ID:      "refundID",
			Type:    repo.NotifierTypeRefundNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.SaleAgingNotification{
			ID:      "saleAgingID",
			Type:    repo.NotifierTypeSaleAgedFourtyFiveDays,
			OrderID: repo.NewNotificationID(),
		},
		repo.UnfollowNotification{
			ID:     "unfollowID",
			Type:   repo.NotifierTypeUnfollowNotification,
			PeerId: repo.NewNotificationID(),
		},
	}

	for _, n := range exampleNotifications {
		var (
			expected = repo.NewNotification(n, time.Now(), false)
			actual   = &repo.Notification{}
		)
		data, err := json.Marshal(expected)
		if err != nil {
			t.Errorf("failed marshaling '%s': %s\n", expected.GetType(), err)
			continue
		}
		if err := json.Unmarshal(data, actual); err != nil {
			t.Errorf("failed unmarshaling '%s': %s\n", expected.GetType(), err)
			continue
		}

		if actual.GetType() != expected.GetType() {
			t.Error("Expected notification to match types, but did not")
			t.Errorf("Expected: %s\n", expected.GetType())
			t.Errorf("Actual: %s\n", actual.GetType())
		}
		if reflect.DeepEqual(actual.NotifierData, expected.NotifierData) != true {
			t.Error("Expected notifier data to match, but did not")
			t.Errorf("Expected: %+v\n", expected.NotifierData)
			t.Errorf("Actual: %+v\n", actual.NotifierData)
		}
	}
}

func TestLegacyNotificationMarshalling(t *testing.T) {
	var exampleNotifications = []repo.Notifier{
		repo.CompletionNotification{
			ID:      "orderCompletionID",
			Type:    repo.NotifierTypeCompletionNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.DisputeAcceptedNotification{
			ID:      "disputeAcceptedID",
			Type:    repo.NotifierTypeDisputeAcceptedNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.DisputeCloseNotification{
			ID:      "disputeCloseID",
			Type:    repo.NotifierTypeDisputeCloseNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.DisputeOpenNotification{
			ID:      "disputeOpenID",
			Type:    repo.NotifierTypeDisputeOpenNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.DisputeUpdateNotification{
			ID:      "disputeUpdateID",
			Type:    repo.NotifierTypeDisputeUpdateNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.FollowNotification{
			ID:     "followID",
			Type:   repo.NotifierTypeFollowNotification,
			PeerId: repo.NewNotificationID(),
		},
		repo.FulfillmentNotification{
			ID:      "fulfillmentID",
			Type:    repo.NotifierTypeFulfillmentNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.ModeratorAddNotification{
			ID:     "moderatorAddID",
			Type:   repo.NotifierTypeModeratorAddNotification,
			PeerId: repo.NewNotificationID(),
		},
		repo.ModeratorRemoveNotification{
			ID:     "moderatorRemoveID",
			Type:   repo.NotifierTypeModeratorRemoveNotification,
			PeerId: repo.NewNotificationID(),
		},
		repo.OrderCancelNotification{
			ID:      "orderCancelID",
			Type:    repo.NotifierTypeOrderCancelNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.OrderConfirmationNotification{
			ID:      "orderConfirmID",
			Type:    repo.NotifierTypeOrderConfirmationNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.OrderDeclinedNotification{
			ID:      "orderDeclinedID",
			Type:    repo.NotifierTypeOrderDeclinedNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.OrderNotification{
			ID:      "orderNotificationID",
			Type:    repo.NotifierTypeOrderNewNotification,
			BuyerID: repo.NewNotificationID(),
		},
		repo.PaymentNotification{
			ID:      "paymentID",
			Type:    repo.NotifierTypePaymentNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.ProcessingErrorNotification{
			ID:      "processingErrorID",
			Type:    repo.NotifierTypeProcessingErrorNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.RefundNotification{
			ID:      "refundID",
			Type:    repo.NotifierTypeRefundNotification,
			OrderId: repo.NewNotificationID(),
		},
		repo.UnfollowNotification{
			ID:     "unfollowID",
			Type:   repo.NotifierTypeUnfollowNotification,
			PeerId: repo.NewNotificationID(),
		},
	}

	for _, n := range exampleNotifications {
		var (
			actual = &repo.Notification{}
		)
		data, err := json.Marshal(n)
		if err != nil {
			t.Errorf("failed marshaling '%s': %s\n", n.GetType(), err)
			continue
		}
		if err := json.Unmarshal(data, actual); err != nil {
			t.Errorf("failed unmarshaling '%s': %s\n", n.GetType(), err)
			continue
		}

		if actual.GetID() != n.GetID() {
			t.Error("Expected notification to match ID, but did not")
			t.Errorf("Expected: %s\n", n.GetID())
			t.Errorf("Actual: %s\n", actual.GetID())
		}
		if actual.GetType() != n.GetType() {
			t.Error("Expected notification to match types, but did not")
			t.Errorf("Expected: %s\n", n.GetType())
			t.Errorf("Actual: %s\n", actual.GetType())
		}
		if reflect.DeepEqual(actual.NotifierData, n) != true {
			t.Error("Expected notifier data to match, but did not")
			t.Errorf("Expected: %+v\n", n)
			t.Errorf("Actual: %+v\n", actual.NotifierData)
		}
	}
}

func TestNotificationSatisfiesNotifierInterface(t *testing.T) {
	notifier := repo.SaleAgingNotification{
		ID:      "saleAgingID",
		Type:    repo.NotifierTypeSaleAgedFourtyFiveDays,
		OrderID: repo.NewNotificationID(),
	}
	var _ repo.Notifier = repo.NewNotification(notifier, time.Now(), false)
}
