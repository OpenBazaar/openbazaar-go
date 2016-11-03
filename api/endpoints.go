package api

import (
	"net/http"
	"strings"
)

func put(i *jsonAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch path {
	case "/ob/profile", "/ob/profile/":
		i.PUTProfile(w, r)
		return
	case "/ob/settings", "/ob/settings/":
		i.PUTSettings(w, r)
		return
	case "/ob/moderator", "/ob/moderator/":
		i.PUTModerator(w, r)
		return
	case "/ob/listing", "/ob/listing/":
		i.PUTListing(w, r)
		return
	}
}

func post(i *jsonAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch path {
	case "/ob/listing", "/ob/listing/":
		i.POSTListing(w, r)
		return
	case "/ob/purchase", "/ob/purchase/":
		i.POSTPurchase(w, r)
		return
	case "/ob/follow", "/ob/follow/":
		i.POSTFollow(w, r)
		return
	case "/ob/unfollow", "/ob/unfollow/":
		i.POSTUnfollow(w, r)
		return
	case "/ob/profile", "/ob/profile/":
		i.POSTProfile(w, r)
		return
	case "/ob/images", "/ob/images/":
		i.POSTImage(w, r)
		return
	case "/wallet/spend", "/wallet/spend/":
		i.POSTSpendCoins(w, r)
		return
	case "/ob/settings", "/ob/settings/":
		i.POSTSettings(w, r)
		return
	case "/ob/inventory", "/ob/inventory/":
		i.POSTInventory(w, r)
		return
	case "/ob/moderator", "/ob/moderator/":
		i.POSTModerator(w, r)
		return
	case "/ob/avatar", "/ob/avatar/":
		i.POSTAvatar(w, r)
		return
	case "/ob/header", "/ob/header/":
		i.POSTHeader(w, r)
		return
	case "/ob/orderconfirmation", "/ob/orderconfirmation/":
		i.POSTOrderConfirmation(w, r)
		return
	case "/ob/ordercancel", "/ob/ordercancel/":
		i.POSTOrderCancel(w, r)
		return
	case "/ob/orderfulfillment", "/ob/orderfulfillment/":
		i.POSTOrderFulfill(w, r)
		return
	case "/ob/ordercompletion", "/ob/ordercompletion/":
		i.POSTOrderComplete(w, r)
		return
	case "/ob/refund", "/ob/refund/":
		i.POSTRefund(w, r)
		return
	case "/wallet/resyncblockchain", "/wallet/resyncblockchain/":
		i.POSTResyncBlockchain(w, r)
		return
	case "/ob/shutdown", "/ob/shutdown/":
		i.POSTShutdown(w, r)
		return
	}
}

func get(i *jsonAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	if strings.Contains(path, "/ob/status/") {
		i.GETStatus(w, r)
		return
	}
	if strings.Contains(path, "/ob/peers") {
		i.GETPeers(w, r)
		return
	}
	if strings.Contains(path, "/ob/config") {
		i.GETConfig(w, r)
		return
	}
	if strings.Contains(path, "/wallet/address") {
		i.GETAddress(w, r)
		return
	}
	if strings.Contains(path, "/wallet/mnemonic") {
		i.GETMnemonic(w, r)
		return
	}
	if strings.Contains(path, "/wallet/balance") {
		i.GETBalance(w, r)
		return
	}
	if strings.Contains(path, "/ob/settings") {
		i.GETSettings(w, r)
		return
	}
	if strings.Contains(path, "/ob/closestpeers") {
		i.GETClosestPeers(w, r)
		return
	}
	if strings.Contains(path, "/ob/exchangerate") {
		i.GETExchangeRate(w, r)
		return
	}
	if strings.Contains(path, "/ob/followers") {
		i.GETFollowers(w, r)
		return
	}
	if strings.Contains(path, "/ob/following") {
		i.GETFollowing(w, r)
		return
	}
	if strings.Contains(path, "/ob/inventory") {
		i.GETInventory(w, r)
		return
	}
	if strings.Contains(path, "/ob/listing") {
		i.GETListing(w, r)
		return
	}
	if strings.Contains(path, "/ob/followsme") {
		i.GETFollowsMe(w, r)
		return
	}
	if strings.Contains(path, "/ob/isfollowing") {
		i.GETIsFollowing(w, r)
		return
	}
	if strings.Contains(path, "/ob/order") {
		i.GETOrder(w, r)
		return
	}
	if strings.Contains(path, "/ob/moderators") {
		i.GETModerators(w, r)
		return
	}
}

func patch(i *jsonAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch path {
	case "/ob/settings", "/ob/settings/":
		i.PATCHSettings(w, r)
		return
	}
}

func deleter(i *jsonAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch path {
	case "/ob/moderator", "/ob/moderator/":
		i.DELETEModerator(w, r)
		return
	case "/ob/listing", "/ob/listing/":
		i.DELETEListing(w, r)
		return
	}
}
