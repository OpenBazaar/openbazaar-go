package api

import (
	"net/http"
	"fmt"
	"runtime/debug"
	"net/url"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/ipfs/go-ipfs/core"

)

type RestAPIConfig struct {
	Headers      map[string][]string
	BlockList    *corehttp.BlockList
	Writable     bool
	PathPrefixes []string
}

type restAPIHandler struct {
	node   *core.IpfsNode
	config RestAPIConfig
}

func newRestAPIHandler(node *core.IpfsNode) (*restAPIHandler, error) {
	prefixes := []string{"/ob/"}
	i := &restAPIHandler{
		node:   node,
		config: RestAPIConfig{
			Writable:     true,
			BlockList:    &corehttp.BlockList{},
			PathPrefixes: prefixes,
		},
	}
	return i, nil
}

// TODO: Build out the api
func (i *restAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("A panic occurred in the rest api handler!")
			log.Error(r)
			debug.PrintStack()
		}
	}()

	u, err := url.Parse(r.URL.Path)
	if err != nil {
		panic(err)
	}
	if i.config.Writable {
		switch r.Method {
		case "POST":
			post(i, u.String(), w, r)
			return
		case "PUT":
			fmt.Fprint(w, "put")
			return
		case "DELETE":
			fmt.Fprint(w, "delete")
			return
		}
	}

	if r.Method == "GET" {
		fmt.Fprint(w, "get")
		return
	}
}

func (i *restAPIHandler) POSTProfile (w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "post")
}