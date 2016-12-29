package api

import (
	"net/http"
	"strings"
)

func put(i *jsonAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch path {
	case "/ob/profile", "/ob/profile/":
		i.PUTProfile(w, r)
	case "/ob/settings", "/ob/settings/":
		i.PUTSettings(w, r)
	case "/ob/moderator", "/ob/moderator/":
		i.PUTModerator(w, r)
	case "/ob/listing", "/ob/listing/":
		i.PUTListing(w, r)
	default:
		ErrorResponse(w, http.StatusNotFound, "Not Found")
	}
}

func post(i *jsonAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch path {
	case "/ob/listing", "/ob/listing/":
		i.POSTListing(w, r)
	case "/ob/purchase", "/ob/purchase/":
		i.POSTPurchase(w, r)
	case "/ob/follow", "/ob/follow/":
		i.POSTFollow(w, r)
	case "/ob/unfollow", "/ob/unfollow/":
		i.POSTUnfollow(w, r)
	case "/ob/profile", "/ob/profile/":
		i.POSTProfile(w, r)
	case "/ob/images", "/ob/images/":
		i.POSTImage(w, r)
	case "/wallet/spend", "/wallet/spend/":
		i.POSTSpendCoins(w, r)
	case "/ob/settings", "/ob/settings/":
		i.POSTSettings(w, r)
	case "/ob/inventory", "/ob/inventory/":
		i.POSTInventory(w, r)
	case "/ob/moderator", "/ob/moderator/":
		i.POSTModerator(w, r)
	case "/ob/avatar", "/ob/avatar/":
		i.POSTAvatar(w, r)
	case "/ob/header", "/ob/header/":
		i.POSTHeader(w, r)
	case "/ob/orderconfirmation", "/ob/orderconfirmation/":
		i.POSTOrderConfirmation(w, r)
	case "/ob/ordercancel", "/ob/ordercancel/":
		i.POSTOrderCancel(w, r)
	case "/ob/orderfulfillment", "/ob/orderfulfillment/":
		i.POSTOrderFulfill(w, r)
	case "/ob/ordercompletion", "/ob/ordercompletion/":
		i.POSTOrderComplete(w, r)
	case "/ob/refund", "/ob/refund/":
		i.POSTRefund(w, r)
	case "/wallet/resyncblockchain", "/wallet/resyncblockchain/":
		i.POSTResyncBlockchain(w, r)
	case "/ob/opendispute", "/ob/opendispute/":
		i.POSTOpenDispute(w, r)
	case "/ob/closedispute", "/ob/closedispute/":
		i.POSTCloseDispute(w, r)
	case "/ob/releasefunds", "/ob/releasefunds/":
		i.POSTReleaseFunds(w, r)
	case "/ob/chat", "/ob/chat/":
		i.POSTChat(w, r)
	case "/ob/markchatasread", "/ob/markchatasread/":
		i.POSTMarkChatAsRead(w, r)
	case "/ob/shutdown", "/ob/shutdown/":
		i.POSTShutdown(w, r)
	default:
		ErrorResponse(w, http.StatusNotFound, "Not Found")
	}
}

func get(i *jsonAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.Contains(path, "/ob/status"):
		i.GETStatus(w, r)
	case strings.Contains(path, "/ob/peers"):
		i.GETPeers(w, r)
	case strings.Contains(path, "/ob/config"):
		i.GETConfig(w, r)
	case strings.Contains(path, "/wallet/address"):
		i.GETAddress(w, r)
	case strings.Contains(path, "/wallet/mnemonic"):
		i.GETMnemonic(w, r)
	case strings.Contains(path, "/wallet/balance"):
		i.GETBalance(w, r)
	case strings.Contains(path, "/ob/settings"):
		i.GETSettings(w, r)
	case strings.Contains(path, "/ob/closestpeers"):
		i.GETClosestPeers(w, r)
	case strings.Contains(path, "/ob/exchangerate"):
		i.GETExchangeRate(w, r)
	case strings.Contains(path, "/ob/followers"):
		i.GETFollowers(w, r)
	case strings.Contains(path, "/ob/following"):
		i.GETFollowing(w, r)
	case strings.Contains(path, "/ob/inventory"):
		i.GETInventory(w, r)
	case strings.Contains(path, "/ob/profile"):
		i.GETProfile(w, r)
	case strings.Contains(path, "/ob/listings"):
		i.GETListings(w, r)
	case strings.Contains(path, "/ob/listing"):
		i.GETListing(w, r)
	case strings.Contains(path, "/ob/followsme"):
		i.GETFollowsMe(w, r)
	case strings.Contains(path, "/ob/isfollowing"):
		i.GETIsFollowing(w, r)
	case strings.Contains(path, "/ob/order"):
		i.GETOrder(w, r)
	case strings.Contains(path, "/ob/moderators"):
		i.GETModerators(w, r)
	case strings.Contains(path, "/ob/case"):
		i.GETCase(w, r)
	case strings.Contains(path, "/ob/chatmessages"):
		i.GETChatMessages(w, r)
	case strings.Contains(path, "/ob/chatconversations"):
		i.GETChatConversations(w, r)
	default:
		ErrorResponse(w, http.StatusNotFound, "Not Found")
	}
}

func patch(i *jsonAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch path {
	case "/ob/settings", "/ob/settings/":
		i.PATCHSettings(w, r)
	default:
		ErrorResponse(w, http.StatusNotFound, "Not Found")
	}
}

func deleter(i *jsonAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch path {
	case "/ob/moderator", "/ob/moderator/":
		i.DELETEModerator(w, r)
	case "/ob/listing", "/ob/listing/":
		i.DELETEListing(w, r)
	case "/ob/chatmessage", "/ob/chatmessage/":
		i.DELETEChatMessage(w, r)
	case "/ob/chatconversation", "/ob/chatconversation/":
		i.DELETEChatConversation(w, r)
	default:
		ErrorResponse(w, http.StatusNotFound, "Not Found")
	}
}
