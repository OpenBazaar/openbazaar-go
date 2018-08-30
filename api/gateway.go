package api

import (
	"net"
	"net/http"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("api")

// Gateway represents an HTTP API gateway
type Gateway struct {
	listener net.Listener
	handler  http.Handler
	config   schema.APIConfig
}

// NewGateway instantiates a new `Gateway`
func NewGateway(n *core.OpenBazaarNode, authCookie http.Cookie, l net.Listener, config schema.APIConfig, logger logging.Backend, options ...corehttp.ServeOption) (*Gateway, error) {

	log.SetBackend(logging.AddModuleLevel(logger))
	topMux := http.NewServeMux()

	jsonAPI := newJsonAPIHandler(n, authCookie, config)
	wsAPI := newWSAPIHandler(n, authCookie, config)
	n.Broadcast = manageNotifications(n, wsAPI.h.Broadcast)

	topMux.Handle("/ob/", jsonAPI)
	topMux.Handle("/wallet/", jsonAPI)
	topMux.Handle("/ws", wsAPI)

	var (
		err error
		mux = topMux
	)
	for _, option := range options {
		mux, err = option(n.IpfsNode, l, mux)
		if err != nil {
			return nil, err
		}
	}

	return &Gateway{
		listener: l,
		handler:  topMux,
		config:   config,
	}, nil
}

// Close shutsdown the Gateway listener
func (g *Gateway) Close() error {
	log.Infof("server at %s terminating...", g.listener.Addr())
	return g.listener.Close()
}

// Serve begins listening on the configured address
func (g *Gateway) Serve() error {
	var err error
	if g.config.SSL {
		err = http.ListenAndServeTLS(g.listener.Addr().String(), g.config.SSLCert, g.config.SSLKey, g.handler)
	} else {
		err = http.Serve(g.listener, g.handler)
	}
	return err
}
