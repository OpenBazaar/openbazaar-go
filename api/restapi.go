package api

import (
	"net/http"
	"fmt"
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
	fmt.Fprint(w, "hello world")
}