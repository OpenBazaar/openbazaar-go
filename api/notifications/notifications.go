package notifications

import (
	"encoding/json"
)

type notificationWrapper struct {
	Notfication interface{} `json:"notification"`
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

type OrderNotification struct {
	Title             string `json:"title"`
	BuyerGuid         string `json:"buyerGuid"`
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

type FollowNotification struct {
	Follow string `json:"follow"`
}

type UnfollowNotification struct {
	Unfollow string `json:"unfollow"`
}

func Serialize(i interface{}) []byte {
	var n notificationWrapper
	switch i.(type) {
	case OrderNotification:
		n = notificationWrapper{
			orderWrapper{
				OrderNotification: i.(OrderNotification),
			},
		}
	case PaymentNotification:
		n = notificationWrapper{
			paymentWrapper{
				PaymentNotification: i.(PaymentNotification),
			},
		}
	case OrderConfirmationNotification:
		n = notificationWrapper{
			orderConfirmationWrapper{
				OrderConfirmationNotification: i.(OrderConfirmationNotification),
			},
		}
	case OrderCancelNotification:
		n = notificationWrapper{
			orderCancelWrapper{
				OrderCancelNotification: i.(OrderCancelNotification),
			},
		}
	case FollowNotification:
		n = notificationWrapper{
			i.(FollowNotification),
		}
	case UnfollowNotification:
		n = notificationWrapper{
			i.(UnfollowNotification),
		}
	}
	b, _ := json.MarshalIndent(n, "", "    ")
	return b
}
