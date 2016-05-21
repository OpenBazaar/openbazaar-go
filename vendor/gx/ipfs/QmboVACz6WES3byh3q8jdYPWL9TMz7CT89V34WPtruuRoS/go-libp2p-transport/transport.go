package transport

import (
	"net"
	"time"

	manet "gx/ipfs/QmYp8PC6b9M3UY4awdHkdRUim68KpGSmRmz27bANHteen6/go-multiaddr-net"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	logging "gx/ipfs/Qmazh5oNUVsDZTs2g59rq8aYQqwpss8tcUWQzor5sCCEuH/go-log"
)

var log = logging.Logger("transport")

// Conn is an extension of the net.Conn interface that provides multiaddr
// information, and an accessor for the transport used to create the conn
type Conn interface {
	manet.Conn

	Transport() Transport
}

// Transport represents any device by which you can connect to and accept
// connections from other peers. The built-in transports provided are TCP and UTP
// but many more can be implemented, sctp, audio signals, sneakernet, UDT, a
// network of drones carrying usb flash drives, and so on.
type Transport interface {
	Dialer(laddr ma.Multiaddr, opts ...DialOpt) (Dialer, error)
	Listen(laddr ma.Multiaddr) (Listener, error)
	Matches(ma.Multiaddr) bool
}

// Dialer is an abstraction that is normally filled by an object containing
// information/options around how to perform the dial. An example would be
// setting TCP dial timeout for all dials made, or setting the local address
// that we dial out from.
type Dialer interface {
	Dial(raddr ma.Multiaddr) (Conn, error)
	Matches(ma.Multiaddr) bool
}

// Listener is an interface closely resembling the net.Listener interface.  The
// only real difference is that Accept() returns Conn's of the type in this
// package, and also exposes a Multiaddr method as opposed to a regular Addr
// method
type Listener interface {
	Accept() (Conn, error)
	Close() error
	Addr() net.Addr
	Multiaddr() ma.Multiaddr
}

type connWrap struct {
	manet.Conn
	transport Transport
}

func (cw *connWrap) Transport() Transport {
	return cw.transport
}

// DialOpt is an option used for configuring dialer behaviour
type DialOpt interface{}

type TimeoutOpt time.Duration
type ReuseportOpt bool

var ReusePorts ReuseportOpt = true
