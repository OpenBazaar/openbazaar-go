package api

import (
	"encoding/json"
	"fmt"

	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/golang/protobuf/proto"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
)

type JSONAPIConfig struct {
	Headers       map[string]interface{}
	Enabled       bool
	Cors          *string
	Authenticated bool
	AllowedIPs    map[string]bool
	Cookie        http.Cookie
	Username      string
	Password      string
}

type jsonAPIHandler struct {
	config JSONAPIConfig
	node   *core.OpenBazaarNode
}

func newJSONAPIHandler(node *core.OpenBazaarNode, authCookie http.Cookie, config schema.APIConfig) *jsonAPIHandler {
	allowedIPs := make(map[string]bool)
	for _, ip := range config.AllowedIPs {
		allowedIPs[ip] = true
	}
	i := &jsonAPIHandler{
		config: JSONAPIConfig{
			Enabled:       config.Enabled,
			Cors:          config.CORS,
			Headers:       config.HTTPHeaders,
			Authenticated: config.Authenticated,
			AllowedIPs:    allowedIPs,
			Cookie:        authCookie,
			Username:      config.Username,
			Password:      config.Password,
		},
		node: node,
	}
	return i
}

func (i *jsonAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse(r.URL.Path)
	if err != nil {
		log.Error(err)
		return
	}
	if !i.config.Enabled && !gatewayAllowedPath(u.Path, r.Method) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "403 - Forbidden")
		return
	}

	checkForUnauthorizedIP(w, i.config.AllowedIPs, r.RemoteAddr)

	if i.config.Cors != nil {
		enableCORS(w, i.config.Cors)
	}

	for k, v := range i.config.Headers {
		w.Header()[k] = v.([]string)
	}

	if i.config.Authenticated {
		configureAPIAuthentication(r, w, i.config.Username, i.config.Password, i.config.Cookie.Value)
	}

	// Stop here if its Preflighted OPTIONS request
	if r.Method == "OPTIONS" {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Error("A panic occurred in the rest api handler!")
			log.Error(r)
			debug.PrintStack()
		}
	}()

	w.Header().Add("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		get(i, u.String(), w, r)
	case "POST":
		post(i, u.String(), w, r)
	case "PUT":
		put(i, u.String(), w, r)
	case "DELETE":
		deleter(i, u.String(), w, r)
	case "PATCH":
		patch(i, u.String(), w, r)
	}

	writeToAPILog(r)
}

func ErrorResponse(w http.ResponseWriter, errorCode int, reason string) {
	type APIError struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
	}
	reason = strings.Replace(reason, `"`, `'`, -1)
	err := APIError{false, reason}
	resp, _ := json.MarshalIndent(err, "", "    ")
	w.WriteHeader(errorCode)
	fmt.Fprint(w, string(resp))
}

func JSONErrorResponse(w http.ResponseWriter, errorCode int, err error) {
	w.WriteHeader(errorCode)
	fmt.Fprint(w, err.Error())
}

func RenderJSONOrStringError(w http.ResponseWriter, errorCode int, err error) {
	errStr := err.Error()
	var jsonObj map[string]interface{}
	if json.Unmarshal([]byte(errStr), &jsonObj) == nil {
		JSONErrorResponse(w, http.StatusInternalServerError, err)
		return
	}

	ErrorResponse(w, http.StatusInternalServerError, errStr)
}

func SanitizedResponse(w http.ResponseWriter, response string) {
	ret, err := SanitizeJSON([]byte(response))
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, string(ret))
}

func SanitizedResponseM(w http.ResponseWriter, response string, m proto.Message) {
	out, err := SanitizeProtobuf(response, m)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, string(out))
}

func isNullJSON(jsonBytes []byte) bool {
	return string(jsonBytes) == "null"
}

func (i *jsonAPIHandler) POSTShutdown(w http.ResponseWriter, r *http.Request) {
	shutdown := func() {
		log.Info("OpenBazaar Server shutting down...")
		time.Sleep(time.Second)
		if core.Node != nil {
			core.Node.Datastore.Close()
			repoLockFile := filepath.Join(core.Node.RepoPath, fsrepo.LockFile)
			os.Remove(repoLockFile)
			core.Node.Multiwallet.Close()
			core.Node.IpfsNode.Close()
		}
		os.Exit(1)
	}
	go shutdown()
	SanitizedResponse(w, `{}`)
}
