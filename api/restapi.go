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
	"github.com/natefinch/lumberjack"
)

var logger = &logging.Logger{Module: "restAPI"}

var stdoutLogFormat = logging.MustStringFormatter(
	`%{color:reset}%{color}%{time:15:04:05.000} [%{shortfunc}] [%{level}] %{message}`,
)

var fileLogFormat = logging.MustStringFormatter(
	`%{time:15:04:05.000} [%{shortfunc}] [%{level}] %{message}`,
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
	path string
}

func newRestAPIHandler(node *core.IpfsNode, ctx commands.Context) (*restAPIHandler, error) {
	//set logging for the api
	w := &lumberjack.Logger{
		Filename:   path.Join(ctx.ConfigRoot, "logs", "api.log"),
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     30, //days
	}
	backendStdout := logging.NewLogBackend(os.Stdout, "", 0)
	backendFile := logging.NewLogBackend(w, "", 0)
	backendStdoutFormatter := logging.NewBackendFormatter(backendStdout, stdoutLogFormat)
	backendFileFormatter := logging.NewBackendFormatter(backendFile, fileLogFormat)
	logging.SetBackend(backendFileFormatter, backendStdoutFormatter)

	prefixes := []string{"/ob/"}
	i := &restAPIHandler{
		node:   node,
		config: RestAPIConfig{
			Writable:     true,
			BlockList:    &corehttp.BlockList{},
			PathPrefixes: prefixes,
		},
		path: ctx.ConfigRoot,
	}
	return i, nil
}

// TODO: Build out the api
func (i *restAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dump, err := httputil.DumpRequest(r, false)
	if err != nil {
		logger.Errorf("Error reading http request: ", err)
	}
	logger.Debugf("%s", dump)
	defer func() {
		if r := recover(); r != nil {
			logger.Error("A panic occurred in the rest api handler!")
			logger.Error(r)
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

	fmt.Fprint(w, `{"success": true}`)
}
