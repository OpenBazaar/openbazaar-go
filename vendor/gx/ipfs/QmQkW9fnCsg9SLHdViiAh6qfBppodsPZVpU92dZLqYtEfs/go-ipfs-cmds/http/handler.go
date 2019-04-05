package http

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	cors "gx/ipfs/QmNNk4iczWp8Q4R1mXQ2mrrjQvWisYqMqbW1an8qGbJZsM/cors"
	cmds "gx/ipfs/QmQkW9fnCsg9SLHdViiAh6qfBppodsPZVpU92dZLqYtEfs/go-ipfs-cmds"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"
)

var log = logging.Logger("cmds/http")

var (
	ErrNotFound           = errors.New("404 page not found")
	errApiVersionMismatch = errors.New("api version mismatch")
)

const (
	StreamErrHeader          = "X-Stream-Error"
	streamHeader             = "X-Stream-Output"
	channelHeader            = "X-Chunked-Output"
	extraContentLengthHeader = "X-Content-Length"
	uaHeader                 = "User-Agent"
	contentTypeHeader        = "Content-Type"
	contentDispHeader        = "Content-Disposition"
	transferEncodingHeader   = "Transfer-Encoding"
	originHeader             = "origin"

	applicationJson        = "application/json"
	applicationOctetStream = "application/octet-stream"
	plainText              = "text/plain"
)

func skipAPIHeader(h string) bool {
	switch h {
	case "Access-Control-Allow-Origin":
		return true
	case "Access-Control-Allow-Methods":
		return true
	case "Access-Control-Allow-Credentials":
		return true
	default:
		return false
	}
}

// the internal handler for the API
type handler struct {
	root *cmds.Command
	cfg  *ServerConfig
	env  cmds.Environment
}

func NewHandler(env cmds.Environment, root *cmds.Command, cfg *ServerConfig) http.Handler {
	if cfg == nil {
		panic("must provide a valid ServerConfig")
	}

	c := cors.New(*cfg.corsOpts)

	var h http.Handler

	h = &handler{
		env:  env,
		root: root,
		cfg:  cfg,
	}

	if cfg.APIPath != "" {
		h = newPrefixHandler(cfg.APIPath, h) // wrap with path prefix checker and trimmer
	}
	h = c.Handler(h) // wrap with CORS handler

	return h
}

type requestLogger interface {
	LogRequest(*cmds.Request) func()
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debug("incoming API request: ", r.URL)

	defer func() {
		if r := recover(); r != nil {
			log.Error("a panic has occurred in the commands handler!")
			log.Error(r)
			log.Errorf("stack trace:\n%s", debug.Stack())
		}
	}()

	ctx := h.env.Context()
	if ctx == nil {
		log.Error("no root context found, using background")
		ctx = context.Background()
	}

	if !allowOrigin(r, h.cfg) || !allowReferer(r, h.cfg) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("403 - Forbidden"))
		log.Warningf("API blocked request to %s. (possible CSRF)", r.URL)
		return
	}

	// If we have a request body, make sure the preamble
	// knows that it should close the body if it wants to
	// write before completing reading.
	// FIXME: https://github.com/ipfs/go-ipfs/issues/5168
	// FIXME: https://github.com/golang/go/issues/15527
	var bodyEOFChan chan struct{}
	if r.Body != http.NoBody {
		bodyEOFChan = make(chan struct{})
		var once sync.Once
		bw := bodyWrapper{
			ReadCloser: r.Body,
			onEOF: func() {
				once.Do(func() { close(bodyEOFChan) })
			},
		}
		r.Body = bw
	}

	req, err := parseRequest(ctx, r, h.root)
	if err != nil {
		if err == ErrNotFound {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		w.Write([]byte(err.Error()))
		return
	}

	// Handle the timeout up front.
	var cancel func()
	if timeoutStr, ok := req.Options[cmds.TimeoutOpt]; ok {
		timeout, err := time.ParseDuration(timeoutStr.(string))
		if err != nil {
			return
		}
		req.Context, cancel = context.WithTimeout(req.Context, timeout)
	} else {
		req.Context, cancel = context.WithCancel(req.Context)
	}
	defer cancel()

	req.Context = logging.ContextWithLoggable(req.Context, uuidLoggable())
	if cn, ok := w.(http.CloseNotifier); ok {
		clientGone := cn.CloseNotify()
		go func() {
			select {
			case <-clientGone:
			case <-req.Context.Done():
			}
			cancel()
		}()
	}

	re, err := NewResponseEmitter(w, r.Method, req, withRequestBodyEOFChan(bodyEOFChan))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	if reqLogger, ok := h.env.(requestLogger); ok {
		done := reqLogger.LogRequest(req)
		defer done()
	}

	// set user's headers first.
	for k, v := range h.cfg.Headers {
		if !skipAPIHeader(k) {
			w.Header()[k] = v
		}
	}

	h.root.Call(req, re, h.env)
}

func uuidLoggable() logging.Loggable {
	ids := make([]byte, 16)
	rand.Read(ids)

	return logging.Metadata{
		"requestId": base32.HexEncoding.EncodeToString(ids),
	}
}

func sanitizedErrStr(err error) string {
	s := err.Error()
	s = strings.Split(s, "\n")[0]
	s = strings.Split(s, "\r")[0]
	return s
}
