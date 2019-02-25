package tcpreuse

import (
	"net"

	reuseport "gx/ipfs/QmSvfeW68LC13nVt3BiwHKFneSa4DCdq3erG8RNtJvq7Ni/go-reuseport"
	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	manet "gx/ipfs/Qmaabb1tJZ2CX5cp6MuuiGgns71NYoxdgQP6Xdid1dVceC/go-multiaddr-net"
)

type listener struct {
	manet.Listener
	network *network
}

func (l *listener) Close() error {
	l.network.mu.Lock()
	delete(l.network.listeners, l)
	l.network.dialer = nil
	l.network.mu.Unlock()
	return l.Listener.Close()
}

// Listen listens on the given multiaddr.
//
// If reuseport is supported, it will be enabled for this listener and future
// dials from this transport may reuse the port.
//
// Note: You can listen on the same multiaddr as many times as you want
// (although only *one* listener will end up handling the inbound connection).
func (t *Transport) Listen(laddr ma.Multiaddr) (manet.Listener, error) {
	nw, naddr, err := manet.DialArgs(laddr)
	if err != nil {
		return nil, err
	}
	var n *network
	switch nw {
	case "tcp4":
		n = &t.v4
	case "tcp6":
		n = &t.v6
	default:
		return nil, ErrWrongProto
	}

	if !reuseport.Available() {
		return manet.Listen(laddr)
	}
	nl, err := reuseport.Listen(nw, naddr)
	if err != nil {
		return manet.Listen(laddr)
	}

	if _, ok := nl.Addr().(*net.TCPAddr); !ok {
		nl.Close()
		return nil, ErrWrongProto
	}

	malist, err := manet.WrapNetListener(nl)
	if err != nil {
		nl.Close()
		return nil, err
	}

	list := &listener{
		Listener: malist,
		network:  n,
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	if n.listeners == nil {
		n.listeners = make(map[*listener]struct{})
	}
	n.listeners[list] = struct{}{}
	n.dialer = nil

	return list, nil
}
