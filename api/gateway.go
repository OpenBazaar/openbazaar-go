package api

import (
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/op/go-logging"
	"gx/ipfs/QmQopLATEYMNg7dVqZRNDfeE2S1yKy8zrRh5xnYiuqeZBn/goprocess"
	manet "gx/ipfs/QmUBa4w6CbHJUMeGJPDiMEDWsM93xToK1fTnFXnrC8Hksw/go-multiaddr-net"
	"net"
	"net/http"
	"time"
)

var log = logging.MustGetLogger("api")

func makeHandler(n *core.OpenBazaarNode, ctx commands.Context, l net.Listener, options ...corehttp.ServeOption) (http.Handler, error) {
	topMux := http.NewServeMux()
	restAPI, err := newRestAPIHandler(n)
	if err != nil {
		return nil, err
	}
	wsAPI := newWSAPIHandler(ctx)
	n.Broadcast = wsAPI.h.Broadcast

	topMux.Handle("/ob/", restAPI)
	topMux.Handle("/ws", wsAPI)

	mux := topMux
	for _, option := range options {
		var err error
		mux, err = option(n.IpfsNode, l, mux)
		if err != nil {
			return nil, err
		}
	}
	return topMux, nil
}

func Serve(cb chan<- bool, node *core.OpenBazaarNode, ctx commands.Context, lis net.Listener, options ...corehttp.ServeOption) error {
	handler, err := makeHandler(node, ctx, lis, options...)
	cb <- true
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

	node.IpfsNode.Process().Go(func(p goprocess.Process) {
		serverError = http.Serve(lis, handler)
		close(serverExited)
	})

	// wait for server to exit.
	select {
	case <-serverExited:

	// if node being closed before server exits, close server
	case <-node.IpfsNode.Process().Closing():
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
