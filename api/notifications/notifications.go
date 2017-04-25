package notifications

import (
	"encoding/json"
	"fmt"
	"time"
)

type Notification struct {
	ID        int       `json:"id"`
	Data      Data      `json:"notification"`
	Timestamp time.Time `json:"timestamp"`
	Read      bool      `json:"read"`
}

type Data interface {
	// TODO maybe should be made 'real interface', which will allow
	// to use typed channels, type checking and semantic dispatching
	// instead of typecase:

	// Serialize() []byte
	// Describe() (string, string)
}

type notificationWrapper struct {
	Notification Data `json:"notification"`
}

type messageWrapper struct {
	Message Data `json:"message"`
}

type walletWrapper struct {
	Message Data `json:"wallet"`
}

type messageReadWrapper struct {
	MessageRead interface{} `json:"messageRead"`
}

type messageTypingWrapper struct {
	MessageRead interface{} `json:"messageTyping"`
}

type orderWrapper struct {
	OrderNotification `json:"order"`
}

type paymentWrapper struct {
	PaymentNotification `json:"payment"`
}

type orderConfirmationWrapper struct {
	OrderConfirmationNotification `json:"orderConfirmation"`
}

type orderCancelWrapper struct {
	OrderCancelNotification `json:"orderConfirmation"`
}

type refundWrapper struct {
	RefundNotification `json:"refund"`
}

type fulfillmentWrapper struct {
	FulfillmentNotification `json:"orderFulfillment"`
}

type completionWrapper struct {
	CompletionNotification `json:"orderCompletion"`
}

type disputeOpenWrapper struct {
	DisputeOpenNotification `json:"disputeOpen"`
}

type disputeUpdateWrapper struct {
	DisputeUpdateNotification `json:"disputeUpdate"`
}

type disputeCloseWrapper struct {
	DisputeCloseNotification `json:"disputeClose"`
}

type OrderNotification struct {
	Title             string `json:"title"`
	BuyerId           string `json:"buyerId"`
	BuyerBlockchainId string `json:"buyerBlockchainId"`
	Thumbnail         string `json:"thumbnail"`
	Timestamp         int    `json:"timestamp"`
	OrderId           string `json:"orderId"`
}

type PaymentNotification struct {
	OrderId      string `json:"orderId"`
	FundingTotal uint64 `json:"fundingTotal"`
}

type OrderConfirmationNotification struct {
	OrderId string `json:"orderId"`
}

type OrderCancelNotification struct {
	OrderId string `json:"orderId"`
}

type RefundNotification struct {
	OrderId string `json:"orderId"`
}

type FulfillmentNotification struct {
	OrderId string `json:"orderId"`
}

type CompletionNotification struct {
	OrderId string `json:"orderId"`
}

type DisputeOpenNotification struct {
	OrderId string `json:"orderId"`
}

type DisputeUpdateNotification struct {
	OrderId string `json:"orderId"`
}

type DisputeCloseNotification struct {
	OrderId string `json:"orderId"`
}

type FollowNotification struct {
	Follow string `json:"follow"`
}

type UnfollowNotification struct {
	Unfollow string `json:"unfollow"`
}

type ModeratorAddNotification struct {
	ModeratorAdd string `json:"moderatorAdd"`
}

type ModeratorRemoveNotification struct {
	ModeratorRemove string `json:"moderatorRemove"`
}

type StatusNotification struct {
	Status string `json:"status"`
}

type ChatMessage struct {
	MessageId string    `json:"messageId"`
	PeerId    string    `json:"peerId"`
	Subject   string    `json:"subject"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type ChatRead struct {
	MessageId string `json:"messageId"`
	PeerId    string `json:"peerId"`
	Subject   string `json:"subject"`
}

type ChatTyping struct {
	PeerId  string `json:"peerId"`
	Subject string `json:"subject"`
}

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

func Wrap(i interface{}) interface{} {
	switch i.(type) {
	case OrderNotification:
		return orderWrapper{OrderNotification: i.(OrderNotification)}
	case PaymentNotification:
		return paymentWrapper{PaymentNotification: i.(PaymentNotification)}
	case OrderConfirmationNotification:
		return orderConfirmationWrapper{OrderConfirmationNotification: i.(OrderConfirmationNotification)}
	case OrderCancelNotification:
		return orderCancelWrapper{OrderCancelNotification: i.(OrderCancelNotification)}
	case RefundNotification:
		return refundWrapper{RefundNotification: i.(RefundNotification)}
	case FulfillmentNotification:
		return fulfillmentWrapper{FulfillmentNotification: i.(FulfillmentNotification)}
	case CompletionNotification:
		return completionWrapper{CompletionNotification: i.(CompletionNotification)}
	case DisputeOpenNotification:
		return disputeOpenWrapper{DisputeOpenNotification: i.(DisputeOpenNotification)}
	case DisputeUpdateNotification:
		return disputeUpdateWrapper{DisputeUpdateNotification: i.(DisputeUpdateNotification)}
	case DisputeCloseNotification:
		return disputeCloseWrapper{DisputeCloseNotification: i.(DisputeCloseNotification)}
	default:
		return i
	}
}

func wrapType(i interface{}) interface{} {
	switch i.(type) {
	case orderWrapper:
		return notificationWrapper{i}
	case paymentWrapper:
		return notificationWrapper{i}
	case orderConfirmationWrapper:
		return notificationWrapper{i}
	case orderCancelWrapper:
		return notificationWrapper{i}
	case refundWrapper:
		return notificationWrapper{i}
	case fulfillmentWrapper:
		return notificationWrapper{i}
	case completionWrapper:
		return notificationWrapper{i}
	case disputeOpenWrapper:
		return notificationWrapper{i}
	case disputeUpdateWrapper:
		return notificationWrapper{i}
	case disputeCloseWrapper:
		return notificationWrapper{i}
	case FollowNotification:
		return notificationWrapper{i}
	case UnfollowNotification:
		return notificationWrapper{i}
	case ModeratorAddNotification:
		return notificationWrapper{i}
	case ModeratorRemoveNotification:
		return notificationWrapper{i}
	case ChatMessage:
		return messageWrapper{i.(ChatMessage)}
	case ChatRead:
		return messageReadWrapper{i.(ChatRead)}
	case ChatTyping:
		return messageTypingWrapper{i.(ChatTyping)}
	case IncomingTransaction:
		return walletWrapper{i.(IncomingTransaction)}
	default:
		return i
	}
}

func Serialize(i interface{}) []byte {
	w := wrapType(Wrap(i))
	b, _ := json.MarshalIndent(w, "", "    ")
	return b
}

func Describe(i interface{}) (string, string) {
	var head, body string
	switch i.(type) {
	case OrderNotification:
		head = "Order received"

		n := i.(OrderNotification)
		var buyer string
		if n.BuyerBlockchainId != "" {
			buyer = n.BuyerBlockchainId
		} else {
			buyer = n.BuyerId
		}
		form := "You received an order \"%s\".\n\nOrder ID: %s\nBuyer: %s\nThumbnail: %s\nTimestamp: %d"
		body = fmt.Sprintf(form, n.Title, n.OrderId, buyer, n.Thumbnail, n.Timestamp)

	case PaymentNotification:
		head = "Payment received"

		n := i.(PaymentNotification)
		form := "Payment for order \"%s\" received (total %d)."
		body = fmt.Sprintf(form, n.OrderId, n.FundingTotal)

	case OrderConfirmationNotification:
		head = "Order confirmed"

		n := i.(OrderConfirmationNotification)
		form := "Order \"%s\" has been confirmed."
		body = fmt.Sprintf(form, n.OrderId)

	case OrderCancelNotification:
		head = "Order cancelled"

		n := i.(OrderCancelNotification)
		form := "Order \"%s\" has been cancelled."
		body = fmt.Sprintf(form, n.OrderId)

	case RefundNotification:
		head = "Payment refunded"

		n := i.(RefundNotification)
		form := "Payment refund for order \"%s\" received."
		body = fmt.Sprintf(form, n.OrderId)

	case FulfillmentNotification:
		head = "Order fulfilled"

		n := i.(FulfillmentNotification)
		form := "Order \"%s\" was marked as fulfilled."
		body = fmt.Sprintf(form, n.OrderId)

	case CompletionNotification:
		head = "Order completed"

		n := i.(CompletionNotification)
		form := "Order \"%s\" was marked as completed."
		body = fmt.Sprintf(form, n.OrderId)

	case DisputeOpenNotification:
		head = "Dispute opened"

		n := i.(DisputeOpenNotification)
		form := "Dispute around order \"%s\" was opened."
		body = fmt.Sprintf(form, n.OrderId)

	case DisputeUpdateNotification:
		head = "Dispute updated"

		n := i.(DisputeUpdateNotification)
		form := "Dispute around order \"%s\" was updated."
		body = fmt.Sprintf(form, n.OrderId)

	case DisputeCloseNotification:
		head = "Dispute closed"

		n := i.(DisputeCloseNotification)
		form := "Dispute around order \"%s\" was closed."
		body = fmt.Sprintf(form, n.OrderId)
	}
	return head, body
}
