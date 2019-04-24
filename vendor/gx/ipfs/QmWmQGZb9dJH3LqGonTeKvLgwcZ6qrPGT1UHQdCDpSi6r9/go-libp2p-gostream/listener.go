package gostream

import (
	"context"
	"net"

	pnet "gx/ipfs/QmY3ArotKMKaL7YGfbQfyDrib6RVraLqZYWXZvVgZktBxp/go-libp2p-net"
	host "gx/ipfs/QmYrWiWM4qtrnCeT3R14jY3ZZyirDNJgwK57q4qFYePgbd/go-libp2p-host"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
)

// listener is an implementation of net.Listener which handles
// http-tagged streams from a libp2p connection.
// A listener can be built with Listen()
type listener struct {
	host     host.Host
	ctx      context.Context
	tag      protocol.ID
	cancel   func()
	streamCh chan pnet.Stream
}

// Accept returns the next a connection to this listener.
// It blocks if there are no connections. Under the hood,
// connections are libp2p streams.
func (l *listener) Accept() (net.Conn, error) {
	select {
	case s := <-l.streamCh:
		return newConn(s), nil
	case <-l.ctx.Done():
		return nil, l.ctx.Err()
	}
}

// Close terminates this listener. It will no longer handle any
// incoming streams
func (l *listener) Close() error {
	l.cancel()
	l.host.RemoveStreamHandler(l.tag)
	return nil
}

// Addr returns the address for this listener, which is its libp2p Peer ID.
func (l *listener) Addr() net.Addr {
	return &addr{l.host.ID()}
}

// Listen provides a standard net.Listener ready to accept "connections".
// Under the hood, these connections are libp2p streams tagged with the
// given protocol.ID.
func Listen(h host.Host, tag protocol.ID) (net.Listener, error) {
	ctx, cancel := context.WithCancel(context.Background())

	l := &listener{
		host:     h,
		ctx:      ctx,
		cancel:   cancel,
		tag:      tag,
		streamCh: make(chan pnet.Stream),
	}

	h.SetStreamHandler(tag, func(s pnet.Stream) {
		select {
		case l.streamCh <- s:
		case <-ctx.Done():
			s.Reset()
		}
	})

	return l, nil
}
