package api

import "net/http"

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
	}
}

func get(i *restAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch path {
	case "/ob/status", "/ob/status/":
		i.GETStatus(w, r)
		return
	}
}
