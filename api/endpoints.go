package api
import "net/http"

func post(i *restAPIHandler, path string, w http.ResponseWriter, r *http.Request) {
	switch path {
	case "/ob/profile", "/ob/profile/":
		i.POSTProfile(w, r)
		return
	}
}
