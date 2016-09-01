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

type OrderNotification struct {
	Title             string `json:"title"`
	BuyerGuid         string `json:"buyerGuid"`
	BuyerBlockchainId string `json:"buyerBlockchainId"`
	Thumbnail         string `json:"thumbnail"`
	Timestamp         int    `json:"timestamp"`
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
