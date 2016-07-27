package api

import (
	"net/http"
	"strings"
)

func put(i *restAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch path {
	case "/ob/profile", "/ob/profile/":
		i.PUTProfile(w, r)
		return
	case "/ob/avatar", "/ob/avatar/":
		i.PUTAvatar(w, r)
		return
	case "/ob/header", "/ob/header/":
		i.PUTHeader(w, r)
		return
	case "/ob/images", "/ob/images/":
		i.PUTImage(w, r)
		return
	case "/ob/settings", "/ob/settings/":
		i.PUTSettings(w, r)
		return
	}
}

func post(i *restAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
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
	case "/wallet/spend", "/wallet/spend/":
		i.POSTSpendCoins(w, r)
		return
	case "/ob/settings", "/ob/settings/":
		i.POSTSettings(w, r)
		return
	case "/ob/login":
		i.POSTLogin(w, r)
		return
	}
}

func get(i *restAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
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
}

func patch(i *restAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch path {
	case "/ob/settings", "/ob/settings/":
		i.PATCHSettings(w, r)
		return
	}
}
