package repo

import (
	"crypto/rand"
	"encoding/json"
	//"errors"
	"fmt"
	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	"time"
)

// Notifier is an interface which is used to send data to the frontend
type Notifier interface {
	// GetID returns the unique string identifier for the Notifier and is used to
	// uniquely persist the Notifier in the DB. Some Notifiers are not persisted.
	// Until we can represent this as part of the interface, the Notifiers which
	// do not get persisted can safely return an empty string. Notifiers which are
	// persisted and return a non-unique GetID() string will eventually fail the DB's
	// uniqueness contraints during runtime.
	GetID() string

	// GetType returns the type as a NotificationType
	GetType() NotificationType

	// GetSMTPTitleAndBody returns the title and body strings to be used
	// in any notification content. The bool can return false to bypass the
	// SMTP notification for this Notifier.
	GetSMTPTitleAndBody() (string, string, bool)

	// Data returns the marhsalled []byte suitable for transmission to the client
	// over the HTTP connection
	Data() ([]byte, error)

	// WebsocketData returns the marhsalled []byte suitable for transmission to the client
	// over the websocket connection
	WebsocketData() ([]byte, error)
}

// NewNotification is a helper that returns a properly instantiated *Notification
func NewNotification(n Notifier, createdAt time.Time, isRead bool) *Notification {
	return &Notification{
		ID:           n.GetID(),
		CreatedAt:    createdAt.UTC(),
		IsRead:       isRead,
		NotifierData: n,
		NotifierType: n.GetType(),
	}
}

// Notification represents both a record from the Notifications Datastore as well
// as an unmarshalling envelope for the Notifier interface field NotifierData.
// NOTE: Only ID, NotifierData and NotifierType fields are valid in both contexts. This is
// because (*Notification).MarshalJSON only wraps the NotifierData field. NotifierData
// describes ID and NotifierType and will also be valid when unmarshalled.
// TODO: Ecapsulate the whole Notification struct inside of MarshalJSON and update persisted
// serializations to match in the Notifications Datastore
type Notification struct {
	ID           string           `json:"-"`
	CreatedAt    time.Time        `json:"timestamp"`
	IsRead       bool             `json:"read"`
	NotifierData Notifier         `json:"notification"`
	NotifierType NotificationType `json:"-"`
}

func (n *Notification) GetID() string         { return n.ID }
func (n *Notification) GetTypeString() string { return string(n.GetType()) }
func (n *Notification) GetType() NotificationType {
	if string(n.NotifierType) == "" {
		n.NotifierType = n.NotifierData.GetType()
	}
	return n.NotifierType
}
func (n *Notification) GetUnixCreatedAt() int { return int(n.CreatedAt.Unix()) }
func (n *Notification) GetSMTPTitleAndBody() (string, string, bool) {
	return n.NotifierData.GetSMTPTitleAndBody()
}
func (n *Notification) Data() ([]byte, error)          { return json.MarshalIndent(n, "", "    ") }
func (n *Notification) WebsocketData() ([]byte, error) { return n.Data() }

type notificationTransporter struct {
	CreatedAt    time.Time        `json:"timestamp"`
	IsRead       bool             `json:"read"`
	NotifierData json.RawMessage  `json:"notification"`
	NotifierType NotificationType `json:"type"`
}

func (n *Notification) MarshalJSON() ([]byte, error) {
	notifierData, err := json.Marshal(n.NotifierData)
	if err != nil {
		return nil, err
	}
	payload := notificationTransporter{
		CreatedAt:    n.CreatedAt,
		IsRead:       n.IsRead,
		NotifierData: notifierData,
		NotifierType: n.GetType(),
	}
	return json.Marshal(payload)
}

func (n *Notification) UnmarshalJSON(data []byte) error {
	// First check if we have a legacy notification to unravel
	if legacyType, ok := extractLegacyNotificationType(data); ok {
		switch legacyType {
		case NotifierTypeCompletionNotification:
			var notifier = CompletionNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypeDisputeAcceptedNotification:
			var notifier = DisputeAcceptedNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypeDisputeCloseNotification:
			var notifier = DisputeCloseNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypeDisputeOpenNotification:
			var notifier = DisputeOpenNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypeDisputeUpdateNotification:
			var notifier = DisputeUpdateNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypeFollowNotification:
			var notifier = FollowNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypeFulfillmentNotification:
			var notifier = FulfillmentNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypeModeratorAddNotification:
			var notifier = ModeratorAddNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypeModeratorRemoveNotification:
			var notifier = ModeratorRemoveNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypeOrderCancelNotification:
			var notifier = OrderCancelNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypeOrderConfirmationNotification:
			var notifier = OrderConfirmationNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypeOrderDeclinedNotification:
			var notifier = OrderDeclinedNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypeOrderNewNotification:
			var notifier = OrderNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypePaymentNotification:
			var notifier = PaymentNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypeProcessingErrorNotification:
			var notifier = ProcessingErrorNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypeRefundNotification:
			var notifier = RefundNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		case NotifierTypeUnfollowNotification:
			var notifier = UnfollowNotification{}
			if err := json.Unmarshal(data, &notifier); err != nil {
				return err
			}
			n.NotifierData = notifier
		}
	}

	if n.NotifierData != nil {
		n.NotifierType = n.NotifierData.GetType()
		n.ID = n.NotifierData.GetID()
		return nil
	}

	// Assume we didn't find a legacy Notification. Let's process it as a
	// properly wrapped payload
	var payload notificationTransporter
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("unmarshal notification: %s", err.Error())
	}

	switch payload.NotifierType {
	case NotifierTypeBuyerDisputeTimeout:
		var notifier = BuyerDisputeTimeout{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeBuyerDisputeExpiry:
		var notifier = BuyerDisputeExpiry{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeCompletionNotification:
		var notifier = CompletionNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeDisputeAcceptedNotification:
		var notifier = DisputeAcceptedNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeDisputeCloseNotification:
		var notifier = DisputeCloseNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeDisputeOpenNotification:
		var notifier = DisputeOpenNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeDisputeUpdateNotification:
		var notifier = DisputeUpdateNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeFollowNotification:
		var notifier = FollowNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeFulfillmentNotification:
		var notifier = FulfillmentNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeModeratorAddNotification:
		var notifier = ModeratorAddNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeModeratorDisputeExpiry:
		var notifier = ModeratorDisputeExpiry{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeModeratorRemoveNotification:
		var notifier = ModeratorRemoveNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeOrderCancelNotification:
		var notifier = OrderCancelNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeOrderConfirmationNotification:
		var notifier = OrderConfirmationNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeOrderDeclinedNotification:
		var notifier = OrderDeclinedNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeOrderNewNotification:
		var notifier = OrderNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypePaymentNotification:
		var notifier = PaymentNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeProcessingErrorNotification:
		var notifier = ProcessingErrorNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeRefundNotification:
		var notifier = RefundNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeUnfollowNotification:
		var notifier = UnfollowNotification{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeVendorFinalizedPayment:
		var notifier = VendorFinalizedPayment{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	case NotifierTypeVendorDisputeTimeout:
		var notifier = VendorDisputeTimeout{}
		if err := json.Unmarshal(payload.NotifierData, &notifier); err != nil {
			return err
		}
		n.NotifierData = notifier
	default:
		return fmt.Errorf("unmarshal notification: unknown type: %s\n", payload.NotifierType)
	}

	n.NotifierType = n.NotifierData.GetType()
	n.ID = n.NotifierData.GetID()
	return nil
}

// extractLegacyNotificationType indicates whether a JSON payload is likely a legacy notification
// based on the presence of the "notification" field in the payload and returns the type found
func extractLegacyNotificationType(data []byte) (NotificationType, bool) {
	var legacyPayload = make(map[string]interface{})
	if err := json.Unmarshal(data, &legacyPayload); err != nil {
		return NotificationType(""), false
	}
	if _, ok := legacyPayload["notification"]; !ok {
		return NotificationType(legacyPayload["type"].(string)), true
	}
	return NotificationType(""), false
}

type Thumbnail struct {
	Tiny  string `json:"tiny"`
	Small string `json:"small"`
}

type notificationWrapper struct {
	Notification Notifier `json:"notification"`
}

type messageWrapper struct {
	Message Notifier `json:"message"`
}

type walletWrapper struct {
	Message Notifier `json:"wallet"`
}

type messageReadWrapper struct {
	MessageRead Notifier `json:"messageRead"`
}

type messageTypingWrapper struct {
	MessageRead Notifier `json:"messageTyping"`
}

type OrderNotification struct {
	ID          string           `json:"notificationId"`
	Type        NotificationType `json:"type"`
	Title       string           `json:"title"`
	BuyerID     string           `json:"buyerId"`
	BuyerHandle string           `json:"buyerHandle"`
	Thumbnail   Thumbnail        `json:"thumbnail"`
	OrderId     string           `json:"orderId"`
	Slug        string           `json:"slug"`
}

func (n OrderNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n OrderNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n OrderNotification) GetID() string             { return n.ID }
func (n OrderNotification) GetType() NotificationType { return NotifierTypeOrderNewNotification }
func (n OrderNotification) GetSMTPTitleAndBody() (string, string, bool) {
	form := "You received an order \"%s\".\n\nOrder ID: %s\nBuyer: %s\nThumbnail: %s\n"
	return "Order received", fmt.Sprintf(form, n.Title, n.OrderId, n.getBuyerID(), n.Thumbnail.Small), true
}
func (n OrderNotification) getBuyerID() string {
	if n.BuyerHandle != "" {
		return n.BuyerHandle
	}
	return n.BuyerID
}

type PaymentNotification struct {
	ID           string           `json:"notificationId"`
	Type         NotificationType `json:"type"`
	OrderId      string           `json:"orderId"`
	FundingTotal uint64           `json:"fundingTotal"`
}

func (n PaymentNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n PaymentNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n PaymentNotification) GetID() string             { return n.ID }
func (n PaymentNotification) GetType() NotificationType { return NotifierTypePaymentNotification }
func (n PaymentNotification) GetSMTPTitleAndBody() (string, string, bool) {
	form := "Payment for order \"%s\" received (total %d)."
	return "Payment received", fmt.Sprintf(form, n.OrderId, n.FundingTotal), true
}

type OrderConfirmationNotification struct {
	ID           string           `json:"notificationId"`
	Type         NotificationType `json:"type"`
	OrderId      string           `json:"orderId"`
	Thumbnail    Thumbnail        `json:"thumbnail"`
	VendorHandle string           `json:"vendorHandle"`
	VendorID     string           `json:"vendorId"`
}

func (n OrderConfirmationNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n OrderConfirmationNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n OrderConfirmationNotification) GetID() string { return n.ID }
func (n OrderConfirmationNotification) GetType() NotificationType {
	return NotifierTypeOrderConfirmationNotification
}
func (n OrderConfirmationNotification) GetSMTPTitleAndBody() (string, string, bool) {
	form := "Order \"%s\" has been confirmed."
	return "Order confirmed", fmt.Sprintf(form, n.OrderId), true
}

type OrderDeclinedNotification struct {
	ID           string           `json:"notificationId"`
	Type         NotificationType `json:"type"`
	OrderId      string           `json:"orderId"`
	Thumbnail    Thumbnail        `json:"thumbnail"`
	VendorHandle string           `json:"vendorHandle"`
	VendorID     string           `json:"vendorId"`
}

func (n OrderDeclinedNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n OrderDeclinedNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n OrderDeclinedNotification) GetID() string { return n.ID }
func (n OrderDeclinedNotification) GetType() NotificationType {
	return NotifierTypeOrderDeclinedNotification
}
func (n OrderDeclinedNotification) GetSMTPTitleAndBody() (string, string, bool) { return "", "", false }

type OrderCancelNotification struct {
	ID          string           `json:"notificationId"`
	Type        NotificationType `json:"type"`
	OrderId     string           `json:"orderId"`
	Thumbnail   Thumbnail        `json:"thumbnail"`
	BuyerHandle string           `json:"buyerHandle"`
	BuyerID     string           `json:"buyerId"`
}

func (n OrderCancelNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n OrderCancelNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n OrderCancelNotification) GetID() string { return n.ID }
func (n OrderCancelNotification) GetType() NotificationType {
	return NotifierTypeOrderCancelNotification
}
func (n OrderCancelNotification) GetSMTPTitleAndBody() (string, string, bool) {
	form := "Order \"%s\" has been cancelled."
	return "Order cancelled", fmt.Sprintf(form, n.OrderId), true
}

type RefundNotification struct {
	ID           string           `json:"notificationId"`
	Type         NotificationType `json:"type"`
	OrderId      string           `json:"orderId"`
	Thumbnail    Thumbnail        `json:"thumbnail"`
	VendorHandle string           `json:"vendorHandle"`
	VendorID     string           `json:"vendorId"`
}

func (n RefundNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n RefundNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n RefundNotification) GetID() string             { return n.ID }
func (n RefundNotification) GetType() NotificationType { return NotifierTypeRefundNotification }
func (n RefundNotification) GetSMTPTitleAndBody() (string, string, bool) {
	form := "Payment refund for order \"%s\" received."
	return "Payment refunded", fmt.Sprintf(form, n.OrderId), true
}

type FulfillmentNotification struct {
	ID           string           `json:"notificationId"`
	Type         NotificationType `json:"type"`
	OrderId      string           `json:"orderId"`
	Thumbnail    Thumbnail        `json:"thumbnail"`
	VendorHandle string           `json:"vendorHandle"`
	VendorID     string           `json:"vendorId"`
}

func (n FulfillmentNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n FulfillmentNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n FulfillmentNotification) GetID() string { return n.ID }
func (n FulfillmentNotification) GetType() NotificationType {
	return NotifierTypeFulfillmentNotification
}
func (n FulfillmentNotification) GetSMTPTitleAndBody() (string, string, bool) {
	form := "Order \"%s\" was marked as fulfilled."
	return "Order fulfilled", fmt.Sprintf(form, n.OrderId), true
}

type ProcessingErrorNotification struct {
	ID           string           `json:"notificationId"`
	Type         NotificationType `json:"type"`
	OrderId      string           `json:"orderId"`
	Thumbnail    Thumbnail        `json:"thumbnail"`
	VendorHandle string           `json:"vendorHandle"`
	VendorID     string           `json:"vendorId"`
}

func (n ProcessingErrorNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n ProcessingErrorNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n ProcessingErrorNotification) GetID() string { return n.ID }
func (n ProcessingErrorNotification) GetType() NotificationType {
	return NotifierTypeProcessingErrorNotification
}
func (n ProcessingErrorNotification) GetSMTPTitleAndBody() (string, string, bool) {
	return "", "", false
}

type CompletionNotification struct {
	ID          string           `json:"notificationId"`
	Type        NotificationType `json:"type"`
	OrderId     string           `json:"orderId"`
	Thumbnail   Thumbnail        `json:"thumbnail"`
	BuyerHandle string           `json:"buyerHandle"`
	BuyerID     string           `json:"buyerId"`
}

func (n CompletionNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n CompletionNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n CompletionNotification) GetID() string             { return n.ID }
func (n CompletionNotification) GetType() NotificationType { return NotifierTypeCompletionNotification }
func (n CompletionNotification) GetSMTPTitleAndBody() (string, string, bool) {
	form := "Order \"%s\" was marked as completed."
	return "Order completed", fmt.Sprintf(form, n.OrderId), true
}

type DisputeOpenNotification struct {
	ID             string           `json:"notificationId"`
	Type           NotificationType `json:"type"`
	OrderId        string           `json:"orderId"`
	Thumbnail      Thumbnail        `json:"thumbnail"`
	DisputerID     string           `json:"disputerId"`
	DisputerHandle string           `json:"disputerHandle"`
	DisputeeID     string           `json:"disputeeId"`
	DisputeeHandle string           `json:"disputeeHandle"`
	Buyer          string           `json:"buyer"`
}

func (n DisputeOpenNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n DisputeOpenNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n DisputeOpenNotification) GetID() string { return n.ID }
func (n DisputeOpenNotification) GetType() NotificationType {
	return NotifierTypeDisputeOpenNotification
}
func (n DisputeOpenNotification) GetSMTPTitleAndBody() (string, string, bool) {
	form := "Dispute around order \"%s\" was opened."
	return "Dispute opened", fmt.Sprintf(form, n.OrderId), true
}

type DisputeUpdateNotification struct {
	ID             string           `json:"notificationId"`
	Type           NotificationType `json:"type"`
	OrderId        string           `json:"orderId"`
	Thumbnail      Thumbnail        `json:"thumbnail"`
	DisputerID     string           `json:"disputerId"`
	DisputerHandle string           `json:"disputerHandle"`
	DisputeeID     string           `json:"disputeeId"`
	DisputeeHandle string           `json:"disputeeHandle"`
	Buyer          string           `json:"buyer"`
}

func (n DisputeUpdateNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n DisputeUpdateNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n DisputeUpdateNotification) GetID() string { return n.ID }
func (n DisputeUpdateNotification) GetType() NotificationType {
	return NotifierTypeDisputeUpdateNotification
}
func (n DisputeUpdateNotification) GetSMTPTitleAndBody() (string, string, bool) {
	form := "Dispute around order \"%s\" was updated."
	return "Dispute updated", fmt.Sprintf(form, n.OrderId), true
}

type DisputeCloseNotification struct {
	ID               string           `json:"notificationId"`
	Type             NotificationType `json:"type"`
	OrderId          string           `json:"orderId"`
	Thumbnail        Thumbnail        `json:"thumbnail"`
	OtherPartyID     string           `json:"otherPartyId"`
	OtherPartyHandle string           `json:"otherPartyHandle"`
	Buyer            string           `json:"buyer"`
}

func (n DisputeCloseNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n DisputeCloseNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n DisputeCloseNotification) GetID() string { return n.ID }
func (n DisputeCloseNotification) GetType() NotificationType {
	return NotifierTypeDisputeCloseNotification
}
func (n DisputeCloseNotification) GetSMTPTitleAndBody() (string, string, bool) {
	form := "Dispute around order \"%s\" was closed."
	return "Dispute closed", fmt.Sprintf(form, n.OrderId), true
}

type DisputeAcceptedNotification struct {
	ID               string           `json:"notificationId"`
	Type             NotificationType `json:"type"`
	OrderId          string           `json:"orderId"`
	Thumbnail        Thumbnail        `json:"thumbnail"`
	OherPartyID      string           `json:"otherPartyId"`
	OtherPartyHandle string           `json:"otherPartyHandle"`
	Buyer            string           `json:"buyer"`
}

func (n DisputeAcceptedNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n DisputeAcceptedNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n DisputeAcceptedNotification) GetID() string { return n.ID }
func (n DisputeAcceptedNotification) GetType() NotificationType {
	return NotifierTypeDisputeAcceptedNotification
}
func (n DisputeAcceptedNotification) GetSMTPTitleAndBody() (string, string, bool) {
	return "", "", false
}

type FollowNotification struct {
	ID     string           `json:"notificationId"`
	Type   NotificationType `json:"type"`
	PeerId string           `json:"peerId"`
}

func (n FollowNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n FollowNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n FollowNotification) GetID() string { return n.ID }
func (n FollowNotification) GetType() NotificationType {
	return NotifierTypeFollowNotification
}
func (n FollowNotification) GetSMTPTitleAndBody() (string, string, bool) { return "", "", false }

type UnfollowNotification struct {
	ID     string           `json:"notificationId"`
	Type   NotificationType `json:"type"`
	PeerId string           `json:"peerId"`
}

func (n UnfollowNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n UnfollowNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n UnfollowNotification) GetID() string                               { return n.ID }
func (n UnfollowNotification) GetType() NotificationType                   { return NotifierTypeUnfollowNotification }
func (n UnfollowNotification) GetSMTPTitleAndBody() (string, string, bool) { return "", "", false }

type ModeratorAddNotification struct {
	ID     string           `json:"notificationId"`
	Type   NotificationType `json:"type"`
	PeerId string           `json:"peerId"`
}

func (n ModeratorAddNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n ModeratorAddNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n ModeratorAddNotification) GetID() string { return n.ID }
func (n ModeratorAddNotification) GetType() NotificationType {
	return NotifierTypeModeratorAddNotification
}
func (n ModeratorAddNotification) GetSMTPTitleAndBody() (string, string, bool) { return "", "", false }

type ModeratorRemoveNotification struct {
	ID     string           `json:"notificationId"`
	Type   NotificationType `json:"type"`
	PeerId string           `json:"peerId"`
}

func (n ModeratorRemoveNotification) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n ModeratorRemoveNotification) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n ModeratorRemoveNotification) GetID() string { return n.ID }
func (n ModeratorRemoveNotification) GetType() NotificationType {
	return NotifierTypeModeratorRemoveNotification
}
func (n ModeratorRemoveNotification) GetSMTPTitleAndBody() (string, string, bool) {
	return "", "", false
}

type StatusNotification struct {
	Status string `json:"status"`
}

func (n StatusNotification) Data() ([]byte, error)                       { return json.MarshalIndent(n, "", "    ") }
func (n StatusNotification) WebsocketData() ([]byte, error)              { return n.Data() }
func (n StatusNotification) GetID() string                               { return "" } // Not persisted, ID is ignored
func (n StatusNotification) GetType() NotificationType                   { return NotifierTypeStatusUpdateNotification }
func (n StatusNotification) GetSMTPTitleAndBody() (string, string, bool) { return "", "", false }

type ChatMessage struct {
	MessageId string    `json:"messageId"`
	PeerId    string    `json:"peerId"`
	Subject   string    `json:"subject"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	Outgoing  bool      `json:"outgoing"`
	Timestamp time.Time `json:"timestamp"`
}

func (n ChatMessage) Data() ([]byte, error)                       { return json.MarshalIndent(messageWrapper{n}, "", "    ") }
func (n ChatMessage) WebsocketData() ([]byte, error)              { return n.Data() }
func (n ChatMessage) GetID() string                               { return "" } // Not persisted, ID is ignored
func (n ChatMessage) GetType() NotificationType                   { return NotifierTypeChatMessage }
func (n ChatMessage) GetSMTPTitleAndBody() (string, string, bool) { return "", "", false }

type ChatRead struct {
	MessageId string `json:"messageId"`
	PeerId    string `json:"peerId"`
	Subject   string `json:"subject"`
}

func (n ChatRead) Data() ([]byte, error)                       { return json.MarshalIndent(messageReadWrapper{n}, "", "    ") }
func (n ChatRead) WebsocketData() ([]byte, error)              { return n.Data() }
func (n ChatRead) GetID() string                               { return "" } // Not persisted, ID is ignored
func (n ChatRead) GetType() NotificationType                   { return NotifierTypeChatRead }
func (n ChatRead) GetSMTPTitleAndBody() (string, string, bool) { return "", "", false }

type ChatTyping struct {
	MessageId string `json:"messageId"`
	PeerId    string `json:"peerId"`
	Subject   string `json:"subject"`
}

func (n ChatTyping) Data() ([]byte, error) {
	return json.MarshalIndent(messageTypingWrapper{n}, "", "    ")
}
func (n ChatTyping) WebsocketData() ([]byte, error)              { return n.Data() }
func (n ChatTyping) GetID() string                               { return "" } // Not persisted, ID is ignored
func (n ChatTyping) GetType() NotificationType                   { return NotifierTypeChatTyping }
func (n ChatTyping) GetSMTPTitleAndBody() (string, string, bool) { return "", "", false }

type IncomingTransaction struct {
	Txid          string    `json:"txid"`
	Value         int64     `json:"value"`
	Address       string    `json:"address"`
	Status        string    `json:"status"`
	Memo          string    `json:"memo"`
	Timestamp     time.Time `json:"timestamp"`
	Confirmations int32     `json:"confirmations"`
	OrderId       string    `json:"orderId"`
	Thumbnail     string    `json:"thumbnail"`
	Height        int32     `json:"height"`
	CanBumpFee    bool      `json:"canBumpFee"`
}

func (n IncomingTransaction) Data() ([]byte, error) {
	return json.MarshalIndent(walletWrapper{n}, "", "    ")
}
func (n IncomingTransaction) WebsocketData() ([]byte, error)              { return n.Data() }
func (n IncomingTransaction) GetID() string                               { return "" } // Not persisted, ID is ignored
func (n IncomingTransaction) GetType() NotificationType                   { return NotifierTypeIncomingTransaction }
func (n IncomingTransaction) GetSMTPTitleAndBody() (string, string, bool) { return "", "", false }

// VendorDisputeTimeout represents a notification about a sale
// which will soon be unable to dispute. The Type indicates the age of the
// purchase and OrderID references the purchases orderID in the database schema
type VendorDisputeTimeout struct {
	ID        string           `json:"notificationId"`
	Type      NotificationType `json:"type"`
	OrderID   string           `json:"purchaseOrderId"`
	ExpiresIn uint             `json:"expiresIn"`
	Thumbnail Thumbnail        `json:"thumbnail"`
}

func (n VendorDisputeTimeout) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n VendorDisputeTimeout) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n VendorDisputeTimeout) GetID() string             { return n.ID }
func (n VendorDisputeTimeout) GetType() NotificationType { return NotifierTypeVendorDisputeTimeout }
func (n VendorDisputeTimeout) GetSMTPTitleAndBody() (string, string, bool) {
	return "", "", false
}

// BuyerDisputeTimeout represents a notification about a purchase
// which will soon be unable to dispute.
type BuyerDisputeTimeout struct {
	ID        string           `json:"notificationId"`
	Type      NotificationType `json:"type"`
	OrderID   string           `json:"orderId"`
	ExpiresIn uint             `json:"expiresIn"`
	Thumbnail Thumbnail        `json:"thumbnail"`
}

func (n BuyerDisputeTimeout) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n BuyerDisputeTimeout) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n BuyerDisputeTimeout) GetID() string             { return n.ID }
func (n BuyerDisputeTimeout) GetType() NotificationType { return NotifierTypeBuyerDisputeTimeout }
func (n BuyerDisputeTimeout) GetSMTPTitleAndBody() (string, string, bool) {
	return "", "", false
}

// BuyerDisputeExpiry represents a notification about a purchase
// which has an open dispute that is expiring
type BuyerDisputeExpiry struct {
	ID        string           `json:"notificationId"`
	Type      NotificationType `json:"type"`
	OrderID   string           `json:"orderId"`
	ExpiresIn uint             `json:"expiresIn"`
	Thumbnail Thumbnail        `json:"thumbnail"`
}

func (n BuyerDisputeExpiry) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n BuyerDisputeExpiry) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n BuyerDisputeExpiry) GetID() string             { return n.ID }
func (n BuyerDisputeExpiry) GetType() NotificationType { return n.Type }
func (n BuyerDisputeExpiry) GetSMTPTitleAndBody() (string, string, bool) {
	return "", "", false
}

// VendorFinalizedPayment represents a notification about a purchase
// which will soon be unable to dispute.
type VendorFinalizedPayment struct {
	ID      string           `json:"notificationId"`
	Type    NotificationType `json:"type"`
	OrderID string           `json:"orderId"`
}

func (n VendorFinalizedPayment) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n VendorFinalizedPayment) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n VendorFinalizedPayment) GetID() string             { return n.ID }
func (n VendorFinalizedPayment) GetType() NotificationType { return NotifierTypeVendorFinalizedPayment }
func (n VendorFinalizedPayment) GetSMTPTitleAndBody() (string, string, bool) {
	return "", "", false
}

// ModeratorDisputeExpiry represents a notification about an open dispute
// which will soon be expired and automatically resolved. The Type indicates
// the age of the dispute case and the CaseID references the cases caseID
// in the database schema
type ModeratorDisputeExpiry struct {
	ID        string           `json:"notificationId"`
	Type      NotificationType `json:"type"`
	CaseID    string           `json:"disputeCaseId"`
	ExpiresIn uint             `json:"expiresIn"`
	Thumbnail Thumbnail        `json:"thumbnail"`
}

func (n ModeratorDisputeExpiry) Data() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n ModeratorDisputeExpiry) WebsocketData() ([]byte, error) {
	return json.MarshalIndent(notificationWrapper{n}, "", "    ")
}
func (n ModeratorDisputeExpiry) GetID() string                               { return n.ID }
func (n ModeratorDisputeExpiry) GetType() NotificationType                   { return n.Type }
func (n ModeratorDisputeExpiry) GetSMTPTitleAndBody() (string, string, bool) { return "", "", false }

type TestNotification struct{}

func (TestNotification) Data() ([]byte, error) {
	return json.MarshalIndent(TestNotification{}, "", "    ")
}
func (n TestNotification) WebsocketData() ([]byte, error) { return n.Data() }
func (TestNotification) GetID() string                    { return "" } // Not persisted, ID is ignored
func (TestNotification) GetType() NotificationType        { return NotifierTypeTestNotification }
func (TestNotification) GetSMTPTitleAndBody() (string, string, bool) {
	return "Test Notification Head", "Test Notification Body", true
}

// PremarshalledNotifier is a hack to allow []byte data to be transferred through
// the Notifier interface without having to do things the right way. You should not
// be using this and should prefer to use an existing Notifier struct or create
// a new one following the pattern of the TestNotification
type PremarshalledNotifier struct {
	Payload []byte
}

func (n PremarshalledNotifier) Data() ([]byte, error)                       { return n.Payload, nil }
func (n PremarshalledNotifier) WebsocketData() ([]byte, error)              { return n.Data() }
func (n PremarshalledNotifier) GetID() string                               { return "" } // Not persisted, ID is ignored
func (n PremarshalledNotifier) GetType() NotificationType                   { return NotifierTypePremarshalledNotifier }
func (n PremarshalledNotifier) GetSMTPTitleAndBody() (string, string, bool) { return "", "", false }

func NewNotificationID() string {
	b := make([]byte, 32)
	rand.Read(b)
	encoded, _ := mh.Encode(b, mh.SHA2_256)
	nId, _ := mh.Cast(encoded)
	return nId.B58String()
}
