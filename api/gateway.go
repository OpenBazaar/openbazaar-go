package api

import (
	"net"
	"net/http"
	"time"
	"github.com/ipfs/go-ipfs/core/corehttp"
	core "github.com/ipfs/go-ipfs/core"
	"gx/ipfs/QmQopLATEYMNg7dVqZRNDfeE2S1yKy8zrRh5xnYiuqeZBn/goprocess"
	manet "gx/ipfs/QmYVqhVfbK4BKvbW88Lhm26b3ud14sTBvcm1H7uWUx1Fkp/go-multiaddr-net"
	logging "gx/ipfs/Qmazh5oNUVsDZTs2g59rq8aYQqwpss8tcUWQzor5sCCEuH/go-log"
)

var log = logging.Logger("core/server")

func makeHandler(n *core.IpfsNode, l net.Listener, options ...corehttp.ServeOption) (http.Handler, error) {
	topMux := http.NewServeMux()
	restAPI, err := newRestAPIHandler(n)
	if err != nil {
		return nil, err
	}
	topMux.Handle("/ob/", restAPI)
	mux := topMux
	for _, option := range options {
		var err error
		mux, err = option(n, l, mux)
		if err != nil {
			return nil, err
		}
	}
	return topMux, nil
}

func Serve(node *core.IpfsNode, lis net.Listener, options ...corehttp.ServeOption) error {
	handler, err := makeHandler(node, lis, options...)
	if err != nil {
		return err
	}

	addr, err := manet.FromNetAddr(lis.Addr())
	if err != nil {
		return err
	}

	// if the server exits beforehand
	var serverError error
	serverExited := make(chan struct{})

	node.Process().Go(func(p goprocess.Process) {
		serverError = http.Serve(lis, handler)
		close(serverExited)
	})

	// wait for server to exit.
	select {
	case <-serverExited:

	// if node being closed before server exits, close server
	case <-node.Process().Closing():
		log.Infof("server at %s terminating...", addr)

		lis.Close()

		outer:
		for {
			// wait until server exits
			select {
			case <-serverExited:
			// if the server exited as we are closing, we really dont care about errors
				serverError = nil
				break outer
			case <-time.After(5 * time.Second):
				log.Infof("waiting for server at %s to terminate...", addr)
			}
		}
	}

	log.Infof("server at %s terminated", addr)
	return serverError
}