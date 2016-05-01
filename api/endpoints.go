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
	case "/ob/contract", "/ob/contract/":
		i.POSTContract(w, r)
		return
	}
}
