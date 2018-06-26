// listener.go - Tor backed net.Listener.
//
// To the extent possible under law, Yawning Angel and Ivan Markin
// waived all copyright and related or neighboring rights to bulb, using
// the creative commons "cc0" public domain dedication. See LICENSE or
// <http://creativecommons.org/publicdomain/zero/1.0/> for full details.

package bulb

import (
	"crypto"
	"fmt"
	"net"
	"strconv"
)

type onionAddr struct {
	info *OnionInfo
	port uint16
}

func (a *onionAddr) Network() string {
	return "tcp"
}

func (a *onionAddr) String() string {
	return fmt.Sprintf("%s.onion:%d", a.info.OnionID, a.port)
}

type onionListener struct {
	addr     *onionAddr
	ctrlConn *Conn
	listener net.Listener
}

func (l *onionListener) Accept() (net.Conn, error) {
	return l.listener.Accept()
}

func (l *onionListener) Close() (err error) {
	if err = l.listener.Close(); err == nil {
		// Only delete the onion once.
		err = l.ctrlConn.DeleteOnion(l.addr.info.OnionID)
	}
	return err
}

func (l *onionListener) Addr() net.Addr {
	return l.addr
}

// NewListener returns a net.Listener backed by an Onion Service using configuration
// config, optionally having Tor generate an ephemeral private key (config is nil or
// config.PrivateKey is nil).
// All of virtual ports specified in vports will be mapped to the port to which
// the underlying TCP listener binded. PortSpecs in config will be ignored since
// there is only one mapping for a vports set is possible.
func (c *Conn) NewListener(config *NewOnionConfig, vports ...uint16) (net.Listener, error) {
	var cfg NewOnionConfig
	if config == nil {
		cfg = NewOnionConfig{
			DiscardPK: true,
		}
	} else {
		cfg = *config
	}

	const loopbackAddr = "127.0.0.1:0"

	// Listen on the loopback interface.
	tcpListener, err := net.Listen("tcp4", loopbackAddr)
	if err != nil {
		return nil, err
	}
	tAddr, ok := tcpListener.Addr().(*net.TCPAddr)
	if !ok {
		tcpListener.Close()
		return nil, newProtocolError("failed to extract local port")
	}

	if len(vports) < 1 {
		return nil, newProtocolError("no virual ports specified")
	}
	targetPortStr := strconv.FormatUint((uint64)(tAddr.Port), 10)
	var portSpecs []OnionPortSpec
	for _, vport := range vports {
		portSpecs = append(portSpecs, OnionPortSpec{
			VirtPort: vport,
			Target:   targetPortStr,
		})
	}
	cfg.PortSpecs = portSpecs
	// Create the onion.
	oi, err := c.NewOnion(&cfg)
	if err != nil {
		tcpListener.Close()
		return nil, err
	}

	oa := &onionAddr{info: oi, port: vports[0]}
	ol := &onionListener{addr: oa, ctrlConn: c, listener: tcpListener}

	return ol, nil
}

// [DEPRECATED] Listener returns a net.Listener backed by an Onion Service.
func (c *Conn) Listener(port uint16, key crypto.PrivateKey) (net.Listener, error) {
	cfg := &NewOnionConfig{}
	if key != nil {
		cfg.PrivateKey = key
		cfg.DiscardPK = false
	} else {
		cfg.DiscardPK = true
	}
	return c.NewListener(cfg, port)
}
