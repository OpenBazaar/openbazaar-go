package torOnion

import (
	"crypto"
	"fmt"
	"net"

	manet "gx/ipfs/QmT6Cp31887FpAc25z25YHgpFJohZedrYLWPPspRtj1Brp/go-multiaddr-net"
	ma "gx/ipfs/QmUAQaWbKxGCUTuoQVvvicbQNZ9APF5pDGWyAZSe93AtKH/go-multiaddr"
	tpt "gx/ipfs/QmWMia2fBVBesMerbtApQY7Tj2sgTaziveBACfCRUcv45f/go-libp2p-transport"

	"context"
	"encoding/base32"
	"github.com/yawning/bulb"
	"golang.org/x/net/proxy"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// IsValidOnionMultiAddr is used to validate that a multiaddr
// is representing a Tor onion service
func IsValidOnionMultiAddr(a ma.Multiaddr) bool {
	netaddr, err := manet.ToNetAddr(a)
	if err != nil {
		return false
	}

	// check for correct network type
	if netaddr.Network() != "onion" {
		return false
	}

	// split into onion address and port
	addr := strings.Split(netaddr.String(), ":")
	if len(addr) != 2 {
		return false
	}

	// onion address without the ".onion" substring
	if len(addr[0]) != 16 {
		return false
	}
	_, err = base32.StdEncoding.DecodeString(strings.ToUpper(addr[0]))
	if err != nil {
		return false
	}

	// onion port number
	i, err := strconv.Atoi(addr[1])
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
	controlConn *bulb.Conn
	auth        *proxy.Auth
	keysDir     string
	keys        map[string]crypto.PrivateKey
}

// NewOnionTransport creates a OnionTransport
//
// controlNet and controlAddr contain the connecting information
// for the tor control port; either TCP or UNIX domain socket.
//
// keysDir is the key material for the Tor onion service.
// The onion key needs to be generated separately from this function
// and should be in the keys directory.
func NewOnionTransport(controlNet, controlAddr string, auth *proxy.Auth, keysDir string) (*OnionTransport, error) {
	conn, err := bulb.Dial(controlNet, controlAddr)
	if err != nil {
		return nil, err
	}
	o := OnionTransport{
		controlConn: conn,
		auth:        auth,
		keysDir:     keysDir,
	}
	keys, err := o.loadKeys()
	if err != nil {
		return nil, err
	}
	o.keys = keys
	return &o, nil
}

// loadKeys loads keys into our keys map from files in the keys directory
func (t *OnionTransport) loadKeys() (map[string]crypto.PrivateKey, error) {
	keys := make(map[string]crypto.PrivateKey)
	absPath, err := filepath.Abs(t.keysDir)
	if err != nil {
		return nil, err
	}
	walkpath := func(path string, f os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".onion_key") {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			var key []byte
			_, err = file.Read(key)
			if err != nil {
				return err
			}
			onionName := strings.Replace(file.Name(), ".onion_key", "", 1)
			keys[onionName] = key
		}
		return nil
	}
	err = filepath.Walk(absPath, walkpath)
	return keys, err
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
	netaddr, err := manet.ToNetAddr(laddr)
	if err != nil {
		return nil, err
	}

	// retreive onion service virtport
	addr := strings.Split(netaddr.String(), ":")
	if len(addr) != 2 {
		return nil, fmt.Errorf("failed to parse onion address")
	}

	// convert port string to int
	port, err := strconv.Atoi(addr[1])
	if err != nil {
		return nil, fmt.Errorf("failed to convert onion service port to int")
	}

	onionKey, ok := t.keys[addr[0]]
	if !ok {
		return nil, fmt.Errorf("missing onion service key material for %s", addr[0])
	}

	listener := OnionListener{
		port:  uint16(port),
		key:   onionKey,
		laddr: laddr,
	}

	// setup bulb listener
	listener.listener, err = t.controlConn.Listener(uint16(port), onionKey)
	if err != nil {
		return nil, err
	}

	return &listener, nil
}

// Matches returns true if the given multiaddr represents a Tor onion service
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
	if err != nil {
		return nil, err
	}
	onionConn := OnionConn{
		transport: tpt.Transport(d.transport),
		laddr:     d.laddr,
		raddr:     &raddr,
	}
	onionConn.Conn, err = dialer.Dial(netaddr.Network(), netaddr.String())
	if err != nil {
		return nil, err
	}
	return &onionConn, nil
}

func (d *OnionDialer) DialContext(ctx context.Context, raddr ma.Multiaddr) (tpt.Conn, error) {
	dialer, err := d.transport.controlConn.Dialer(d.auth)
	if err != nil {
		return nil, err
	}
	netaddr, err := manet.ToNetAddr(raddr)
	if err != nil {
		return nil, err
	}
	onionConn := OnionConn{
		transport: tpt.Transport(d.transport),
		laddr:     d.laddr,
		raddr:     &raddr,
	}
	onionConn.Conn, err = dialer.Dial(netaddr.Network(), netaddr.String())
	if err != nil {
		return nil, err
	}
	return &onionConn, nil
}

// Matches returns true if the given multiaddr represents a Tor onion service
func (d *OnionDialer) Matches(a ma.Multiaddr) bool {
	return IsValidOnionMultiAddr(a)
}

// OnionListener implements go-libp2p-transport's Listener interface
type OnionListener struct {
	port      uint16
	key       crypto.PrivateKey
	laddr     ma.Multiaddr
	listener  net.Listener
	transport tpt.Transport
}

// Accept blocks until a connection is received returning
// go-libp2p-transport's Conn interface or an error if
/// something went wrong
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
