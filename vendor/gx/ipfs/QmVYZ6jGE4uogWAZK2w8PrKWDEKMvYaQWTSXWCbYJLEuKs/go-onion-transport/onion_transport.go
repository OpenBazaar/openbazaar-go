package torOnion

import (
	"fmt"
	"net"

	tpt "gx/ipfs/QmQVm7pWYKPStMeMrXNRpvAJE5rSm9ThtQoNmjNHC7sh3k/go-libp2p-transport"
	manet "gx/ipfs/QmX3U3YXCQ6UYBxq2LVWF8dARS1hPUTEYLrSx654Qyxyw6/go-multiaddr-net"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	mafmt "gx/ipfs/QmZQa5J7j7kd44GGC4aKX8J9JGGzCMqwGzcEFqGV1YD57A/mafmt"

	"context"
	"crypto"
	"crypto/rsa"
	"encoding/base32"
	"github.com/yawning/bulb"
	"golang.org/x/net/proxy"
	"net/url"
	"strconv"
	"strings"
)

type TorControl interface {
	Dialer(auth *proxy.Auth) (proxy.Dialer, error)
	Listener(port uint16, key crypto.PrivateKey) (net.Listener, error)
}

type ManualControl struct {
	dialer proxy.Dialer
}

func (m *ManualControl) Dialer(auth *proxy.Auth) (proxy.Dialer, error) {
	return m.dialer, nil
}

func (m *ManualControl) Listener(port uint16, key crypto.PrivateKey) (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(int(port)))
}

// IsValidOnionMultiAddr is used to validate that a multiaddr
// is representing a Tor onion service
func IsValidOnionMultiAddr(a ma.Multiaddr) bool {
	if len(a.Protocols()) != 1 {
		return false
	}

	// check for correct network type
	if a.Protocols()[0].Name != "onion" {
		return false
	}

	// split into onion address and port
	addr, err := a.ValueForProtocol(ma.P_ONION)
	if err != nil {
		return false
	}
	split := strings.Split(addr, ":")
	if len(split) != 2 {
		return false
	}

	// onion address without the ".onion" substring
	if len(split[0]) != 16 {
		fmt.Println(split[0])
		return false
	}
	_, err = base32.StdEncoding.DecodeString(strings.ToUpper(split[0]))
	if err != nil {
		return false
	}

	// onion port number
	i, err := strconv.Atoi(split[1])
	if err != nil {
		return false
	}
	if i >= 65536 || i < 1 {
		return false
	}

	return true
}

// OnionTransport implements go-libp2p-transport's Transport interface
type OnionTransport struct {
	controlConn TorControl
	auth        *proxy.Auth
	onionKey    *rsa.PrivateKey
	onlyOnion   bool
}

type TransportConfig struct {
	// AutoConfig controls whether the transport will use the bulb library to connect to the
	// Tor control port and attempt to automatically configure a Tor Hidden Service. It will also
	// use the Tor control to acquire the socks5 proxy. If AutoConfig is true the ControlAddr
	// and KeysDir must be provided.
	//
	// If AutoConfig is false the user must have manually configured a Tor Hidden Service and must
	// provide the onion url to when calling Listen(). If AutoConfig is false the socks5 proxy address
	// must be provided as we wont be able to get it from the control port.
	AutoConfig bool

	// Must be provided if AutoConfig is true
	ControlAddr string

	// The Tor Control auth if password authentication is set in the torrc file.
	Auth *proxy.Auth

	// The 1024 bit RSA private key used for the onion address. Must be provided if AutoConfig is true.
	OnionKey *rsa.PrivateKey

	// If OnlyOnion is true the dialer will only be used to dial out on onion addresses. This is a
	// useful configuration for dual stack nodes which want to dial to clearnet nodes using the clearnet.
	OnlyOnion bool

	// Must be provided is AutoConfig is false
	SocksAddr string
}

// NewOnionTransport creates a OnionTransport
func NewOnionTransport(config TransportConfig) (*OnionTransport, error) {
	var conn TorControl
	if config.AutoConfig {
		torControlConn, err := bulb.Dial("tcp4", config.ControlAddr)
		if err != nil {
			return nil, err
		}
		var pw string
		if config.Auth != nil {
			pw = config.Auth.Password
		}
		if err := torControlConn.Authenticate(pw); err != nil {
			return nil, fmt.Errorf("Authentication failed: %v", err)
		}
		conn = torControlConn
	} else {
		tbProxyURL, err := url.Parse("socks5://" + config.SocksAddr)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse proxy URL: %v\n", err)
		}
		tbDialer, err := proxy.FromURL(tbProxyURL, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("Failed to obtain proxy dialer: %v\n", err)
		}
		conn = &ManualControl{tbDialer}
	}
	o := OnionTransport{
		controlConn: conn,
		auth:        config.Auth,
		onionKey:    config.OnionKey,
		onlyOnion:   config.OnlyOnion,
	}
	return &o, nil
}

// Returns a proxy dialer gathered from the control interface.
// This isn't needed for the IPFS transport but it provides
// easy access to Tor for other functions.
func (t *OnionTransport) TorDialer() (proxy.Dialer, error) {
	dialer, err := t.controlConn.Dialer(t.auth)
	if err != nil {
		return nil, err
	}
	return dialer, nil
}

// Dialer creates and returns a go-libp2p-transport Dialer
func (t *OnionTransport) Dialer(laddr ma.Multiaddr, opts ...tpt.DialOpt) (tpt.Dialer, error) {
	dialer := OnionDialer{
		auth:      t.auth,
		laddr:     &laddr,
		transport: t,
	}
	return &dialer, nil
}

// Listen creates and returns a go-libp2p-transport Listener
func (t *OnionTransport) Listen(laddr ma.Multiaddr) (tpt.Listener, error) {

	// convert to net.Addr
	netaddr, err := laddr.ValueForProtocol(ma.P_ONION)
	if err != nil {

	}

	// retreive onion service virtport
	addr := strings.Split(netaddr, ":")
	if len(addr) != 2 {
		return nil, fmt.Errorf("failed to parse onion address")
	}

	// convert port string to int
	port, err := strconv.Atoi(addr[1])
	if err != nil {
		return nil, fmt.Errorf("failed to convert onion service port to int")
	}

	listener := OnionListener{
		laddr: laddr,
	}

	// setup listener
	listener.listener, err = t.controlConn.Listener(uint16(port), t.onionKey)
	if err != nil {
		return nil, err
	}

	return &listener, nil
}

// Matches returns true if the address is a valid onion multiaddr
func (t *OnionTransport) Matches(a ma.Multiaddr) bool {
	return IsValidOnionMultiAddr(a)
}

// OnionDialer implements go-libp2p-transport's Dialer interface
type OnionDialer struct {
	auth      *proxy.Auth
	conn      *OnionConn
	laddr     *ma.Multiaddr
	transport *OnionTransport
}

// Dial connects to the specified multiaddr and returns
// a go-libp2p-transport Conn interface
func (d *OnionDialer) Dial(raddr ma.Multiaddr) (tpt.Conn, error) {
	dialer, err := d.transport.controlConn.Dialer(d.auth)
	if err != nil {
		return nil, err
	}
	netaddr, err := manet.ToNetAddr(raddr)
	var onionAddress string
	if err != nil {
		onionAddress, err = raddr.ValueForProtocol(ma.P_ONION)
		if err != nil {
			return nil, err
		}
	}
	onionConn := OnionConn{
		transport: tpt.Transport(d.transport),
		laddr:     d.laddr,
		raddr:     &raddr,
	}
	if onionAddress != "" {
		split := strings.Split(onionAddress, ":")
		onionConn.Conn, err = dialer.Dial("tcp4", split[0]+".onion:"+split[1])
	} else {
		onionConn.Conn, err = dialer.Dial(netaddr.Network(), netaddr.String())
	}
	if err != nil {
		return nil, err
	}
	return &onionConn, nil
}

func (d *OnionDialer) DialContext(ctx context.Context, raddr ma.Multiaddr) (tpt.Conn, error) {
	return d.Dial(raddr)
}

// If onlyOnion is set, Matches returns true only for onion addrs.
// Otherwise TCP addrs can use this dialer in addition to onion.
func (d *OnionDialer) Matches(a ma.Multiaddr) bool {
	if d.transport.onlyOnion {
		// only dial out on onion addresses
		return IsValidOnionMultiAddr(a)
	} else {
		return IsValidOnionMultiAddr(a) || mafmt.TCP.Matches(a)
	}
}

// OnionListener implements go-libp2p-transport's Listener interface
type OnionListener struct {
	laddr     ma.Multiaddr
	listener  net.Listener
	transport tpt.Transport
}

// Accept blocks until a connection is received returning
// go-libp2p-transport's Conn interface or an error if
// something went wrong
func (l *OnionListener) Accept() (tpt.Conn, error) {
	conn, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}
	raddr, err := manet.FromNetAddr(conn.RemoteAddr())
	if err != nil {
		return nil, err
	}
	onionConn := OnionConn{
		Conn:      conn,
		transport: l.transport,
		laddr:     &l.laddr,
		raddr:     &raddr,
	}
	return &onionConn, nil
}

// Close shuts down the listener
func (l *OnionListener) Close() error {
	return l.listener.Close()
}

// Addr returns the net.Addr interface which represents
// the local multiaddr we are listening on
func (l *OnionListener) Addr() net.Addr {
	netaddr, _ := manet.ToNetAddr(l.laddr)
	return netaddr
}

// Multiaddr returns the local multiaddr we are listening on
func (l *OnionListener) Multiaddr() ma.Multiaddr {
	return l.laddr
}

// OnionConn implement's go-libp2p-transport's Conn interface
type OnionConn struct {
	net.Conn
	transport tpt.Transport
	laddr     *ma.Multiaddr
	raddr     *ma.Multiaddr
}

// Transport returns the OnionTransport associated
// with this OnionConn
func (c *OnionConn) Transport() tpt.Transport {
	return c.transport
}

// LocalMultiaddr returns the local multiaddr for this connection
func (c *OnionConn) LocalMultiaddr() ma.Multiaddr {
	return *c.laddr
}

// RemoteMultiaddr returns the remote multiaddr for this connection
func (c *OnionConn) RemoteMultiaddr() ma.Multiaddr {
	return *c.raddr
}
