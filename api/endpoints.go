package api

import (
	"net/http"
	"strings"
)

func put(i *jsonAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(path, "/ob/profile"):
		i.PUTProfile(w, r)
	case strings.HasPrefix(path, "/ob/settings"):
		i.PUTSettings(w, r)
	case strings.HasPrefix(path, "/ob/moderator"):
		i.PUTModerator(w, r)
	case strings.HasPrefix(path, "/ob/listing"):
		i.PUTListing(w, r)
	default:
		ErrorResponse(w, http.StatusNotFound, "Not Found")
	}
}

func post(i *jsonAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(path, "/ob/listing"):
		i.POSTListing(w, r)
	case strings.HasPrefix(path, "/ob/purchase"):
		i.POSTPurchase(w, r)
	case strings.HasPrefix(path, "/ob/follow"):
		i.POSTFollow(w, r)
	case strings.HasPrefix(path, "/ob/unfollow"):
		i.POSTUnfollow(w, r)
	case strings.HasPrefix(path, "/ob/profile"):
		i.POSTProfile(w, r)
	case strings.HasPrefix(path, "/ob/images"):
		i.POSTImage(w, r)
	case strings.HasPrefix(path, "/wallet/spend"):
		i.POSTSpendCoins(w, r)
	case strings.HasPrefix(path, "/ob/settings"):
		i.POSTSettings(w, r)
	case strings.HasPrefix(path, "/ob/inventory"):
		i.POSTInventory(w, r)
	case strings.HasPrefix(path, "/ob/avatar"):
		i.POSTAvatar(w, r)
	case strings.HasPrefix(path, "/ob/header"):
		i.POSTHeader(w, r)
	case strings.HasPrefix(path, "/ob/orderconfirmation"):
		i.POSTOrderConfirmation(w, r)
	case strings.HasPrefix(path, "/ob/ordercancel"):
		i.POSTOrderCancel(w, r)
	case strings.HasPrefix(path, "/ob/orderfulfillment"):
		i.POSTOrderFulfill(w, r)
	case strings.HasPrefix(path, "/ob/ordercompletion"):
		i.POSTOrderComplete(w, r)
	case strings.HasPrefix(path, "/ob/refund"):
		i.POSTRefund(w, r)
	case strings.HasPrefix(path, "/wallet/resyncblockchain"):
		i.POSTResyncBlockchain(w, r)
	case strings.HasPrefix(path, "/wallet/bumpfee"):
		i.POSTBumpFee(w, r)
	case strings.HasPrefix(path, "/ob/opendispute"):
		i.POSTOpenDispute(w, r)
	case strings.HasPrefix(path, "/ob/closedispute"):
		i.POSTCloseDispute(w, r)
	case strings.HasPrefix(path, "/ob/releasefunds"):
		i.POSTReleaseFunds(w, r)
	case strings.HasPrefix(path, "/ob/chat"):
		i.POSTChat(w, r)
	case strings.HasPrefix(path, "/ob/markchatasread"):
		i.POSTMarkChatAsRead(w, r)
	case strings.HasPrefix(path, "/ob/marknotificationasread"):
		i.POSTMarkNotificationAsRead(w, r)
	case strings.HasPrefix(path, "/ob/fetchprofiles"):
		i.POSTFetchProfiles(w, r)
	case strings.HasPrefix(path, "/ob/blocknode"):
		i.POSTBlockNode(w, r)
	case strings.HasPrefix(path, "/ob/shutdown"):
		i.POSTShutdown(w, r)
	case strings.HasPrefix(path, "/ob/estimatetotal"):
		i.POSTEstimateTotal(w, r)
	default:
		ErrorResponse(w, http.StatusNotFound, "Not Found")
	}
}

func get(i *jsonAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(path, "/ob/status"):
		i.GETStatus(w, r)
	case strings.HasPrefix(path, "/ob/peers"):
		i.GETPeers(w, r)
	case strings.HasPrefix(path, "/ob/config"):
		i.GETConfig(w, r)
	case strings.HasPrefix(path, "/wallet/address"):
		i.GETAddress(w, r)
	case strings.HasPrefix(path, "/wallet/mnemonic"):
		i.GETMnemonic(w, r)
	case strings.HasPrefix(path, "/wallet/balance"):
		i.GETBalance(w, r)
	case strings.HasPrefix(path, "/wallet/transactions"):
		i.GETTransactions(w, r)
	case strings.HasPrefix(path, "/ob/settings"):
		i.GETSettings(w, r)
	case strings.HasPrefix(path, "/ob/closestpeers"):
		i.GETClosestPeers(w, r)
	case strings.HasPrefix(path, "/ob/exchangerate"):
		i.GETExchangeRate(w, r)
	case strings.HasPrefix(path, "/ob/followers"):
		i.GETFollowers(w, r)
	case strings.HasPrefix(path, "/ob/following"):
		i.GETFollowing(w, r)
	case strings.HasPrefix(path, "/ob/inventory"):
		i.GETInventory(w, r)
	case strings.HasPrefix(path, "/ob/profile"):
		i.GETProfile(w, r)
	case strings.HasPrefix(path, "/ob/listings"):
		i.GETListings(w, r)
	case strings.HasPrefix(path, "/ob/listing"):
		i.GETListing(w, r)
	case strings.HasPrefix(path, "/ob/followsme"):
		i.GETFollowsMe(w, r)
	case strings.HasPrefix(path, "/ob/isfollowing"):
		i.GETIsFollowing(w, r)
	case strings.HasPrefix(path, "/ob/order"):
		i.GETOrder(w, r)
	case strings.HasPrefix(path, "/ob/moderators"):
		i.GETModerators(w, r)
	case strings.HasPrefix(path, "/ob/case"):
		i.GETCase(w, r)
	case strings.HasPrefix(path, "/ob/chatmessages"):
		i.GETChatMessages(w, r)
	case strings.HasPrefix(path, "/ob/chatconversations"):
		i.GETChatConversations(w, r)
	case strings.HasPrefix(path, "/ob/notifications"):
		i.GETNotifications(w, r)
	case strings.HasPrefix(path, "/ob/images"):
		i.GETImage(w, r)
	case strings.HasPrefix(path, "/ob/purchases"):
		i.GETPurchases(w, r)
	case strings.HasPrefix(path, "/ob/sales"):
		i.GETSales(w, r)
	case strings.HasPrefix(path, "/ob/cases"):
		i.GETCases(w, r)
	case strings.HasPrefix(path, "/wallet/estimatefee"):
		i.GETEstimateFee(w, r)
	default:
		ErrorResponse(w, http.StatusNotFound, "Not Found")
	}
}

func patch(i *jsonAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(path, "/ob/settings"):
		i.PATCHSettings(w, r)
	case strings.HasPrefix(path, "/ob/profile"):
		i.PATCHProfile(w, r)
	default:
		ErrorResponse(w, http.StatusNotFound, "Not Found")
	}
}

func deleter(i *jsonAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(path, "/ob/moderator"):
		i.DELETEModerator(w, r)
	case strings.HasPrefix(path, "/ob/listing"):
		i.DELETEListing(w, r)
	case strings.HasPrefix(path, "/ob/chatmessage"):
		i.DELETEChatMessage(w, r)
	case strings.HasPrefix(path, "/ob/chatconversation"):
		i.DELETEChatConversation(w, r)
	case strings.HasPrefix(path, "/ob/notifications"):
		i.DELETENotification(w, r)
	case strings.HasPrefix(path, "/ob/blocknode"):
		i.DELETEBlockNode(w, r)
	default:
		ErrorResponse(w, http.StatusNotFound, "Not Found")
	}
}
