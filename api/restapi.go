package api

import (
	"io"
	"path"
	"net/http"
	"fmt"
	"runtime/debug"
	"net/url"
	"os"
	"encoding/json"
	"net/http/httputil"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/ipfs/go-ipfs/core"
	"github.com/op/go-logging"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
)

var log = logging.MustGetLogger("restAPI")

type RestAPIConfig struct {
	Headers      map[string][]string
	BlockList    *corehttp.BlockList
	Writable     bool
	PathPrefixes []string
}

type restAPIHandler struct {
	node   *core.IpfsNode
	config RestAPIConfig
	path string
	context commands.Context
}

func newRestAPIHandler(node *core.IpfsNode, ctx commands.Context) (*restAPIHandler, error) {
	prefixes := []string{"/ob/"}
	i := &restAPIHandler{
		node:   node,
		config: RestAPIConfig{
			Writable:     true,
			BlockList:    &corehttp.BlockList{},
			PathPrefixes: prefixes,
		},
		path: ctx.ConfigRoot,
		context: ctx,
	}
	return i, nil
}

// TODO: Build out the api
func (i *restAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dump, err := httputil.DumpRequest(r, false)
	if err != nil {
		log.Errorf("Error reading http request: ", err)
	}
	log.Debugf("%s", dump)
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
			fmt.Fprint(w, "post")
			return
		case "PUT":
			put(i, u.String(), w, r)
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

func (i *restAPIHandler) PUTProfile (w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	f, err := os.Create(path.Join(i.path, "node", "profile"))
	if err != nil {
		fmt.Fprint(w, `{"success": false, "reason": %s}`, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()

	dec := json.NewDecoder(r.Body)
	for {
		var v map[string]interface{}
		if err := dec.Decode(&v); err == io.EOF{
			break
		}
		b, err := json.MarshalIndent(v, "", "    ")
		if err != nil {
			fmt.Fprint(w, `{"success": false, "reason": %s}`, err)
		}
		if _, err := f.WriteString(string(b)); err != nil {
			fmt.Fprint(w, `{"success": false, "reason": %s}`, err)
		}
	}
	_, aerr := ipfs.AddDirectory(i.context, path.Join(i.path, "node"))
	if aerr != nil {
		fmt.Fprint(w, `{"success": false, "reason": %s}`, aerr)
		return
	}
	fmt.Fprint(w, `{"success": true}`)
}

func (i *restAPIHandler) PUTAvatar (w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	file, _, err := r.FormFile("avatar")

	if err != nil {
		fmt.Fprint(w, `{"success": false, "reason": %s}`, err)
		return
	}
	defer file.Close()

	out, err := os.Create(path.Join(i.path, "node", "avatar"))

	if err != nil {
		fmt.Fprint(w, `{"success": false, "reason": %s}`, err)
		return
	}

	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		fmt.Fprint(w, `{"success": false, "reason": %s}`, err)
		return
	}
	_, aerr := ipfs.AddDirectory(i.context, path.Join(i.path, "node"))
	if aerr != nil {
		fmt.Fprint(w, `{"success": false, "reason": %s}`, aerr)
		return
	}

	fmt.Fprint(w, `{"success": true}`)
}

func (i *restAPIHandler) PUTHeader (w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	file, _, err := r.FormFile("header")

	if err != nil {
		fmt.Fprint(w, `{"success": false, "reason": %s}`, err)
		return
	}
	defer file.Close()

	out, err := os.Create(path.Join(i.path, "node", "header"))

	if err != nil {
		fmt.Fprint(w, `{"success": false, "reason": %s}`, err)
		return
	}

	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		fmt.Fprint(w, `{"success": false, "reason": %s}`, err)
		return
	}
	_, aerr := ipfs.AddDirectory(i.context, path.Join(i.path, "node"))
	if aerr != nil {
		fmt.Fprint(w, `{"success": false, "reason": %s}`, aerr)
		return
	}
	fmt.Fprint(w, `{"success": true}`)
}
