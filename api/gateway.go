package api

import (
	"net"
	"net/http"
	"time"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("api")

// Gateway represents an HTTP API gateway
type Gateway struct {
	listener   net.Listener
	handler    http.Handler
	config     schema.APIConfig
	shutdownCh chan struct{}
}

// NewGateway instantiates a new `Gateway`
func NewGateway(n *core.OpenBazaarNode, authCookie http.Cookie, l net.Listener, config schema.APIConfig, logger logging.Backend, options ...corehttp.ServeOption) (*Gateway, error) {

	log.SetBackend(logging.AddModuleLevel(logger))
	topMux := http.NewServeMux()

	jsonAPI, err := newJsonAPIHandler(n, authCookie, config)
	if err != nil {
		return nil, err
	}
	wsAPI, err := newWSAPIHandler(n, n.Context, authCookie, config)
	if err != nil {
		return nil, err
	}
	n.Broadcast = manageNotifications(n, wsAPI.h.Broadcast)

	topMux.Handle("/ob/", jsonAPI)
	topMux.Handle("/wallet/", jsonAPI)
	topMux.Handle("/ws", wsAPI)

	mux := topMux
	for _, option := range options {
		mux, err = option(n.IpfsNode, l, mux)
		if err != nil {
			return nil, err
		}
	}

	return &Gateway{
		listener:   l,
		handler:    topMux,
		config:     config,
		shutdownCh: make(chan struct{}),
	}, nil
}

// Close shutsdown the Gateway listener
func (g *Gateway) Close() error {
	log.Infof("server at %s terminating...", g.listener.Addr())

	// Print shutdown message every few seconds if we're taking too long
	go func() {
		select {
		case <-g.shutdownCh:
			return
		case <-time.After(5 * time.Second):
			log.Infof("waiting for server at %s to terminate...", g.listener.Addr())

		}
	}()

	// Shutdown the listener
	close(g.shutdownCh)
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
