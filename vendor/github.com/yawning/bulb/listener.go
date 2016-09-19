// listener.go - Tor backed net.Listener.
//
// To the extent possible under law, Yawning Angel waived all copyright
// and related or neighboring rights to bulb, using the creative
// commons "cc0" public domain dedication. See LICENSE or
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

// Listener returns a net.Listener backed by a Onion Service, optionally
// having Tor generate an ephemeral private key.  Regardless of the status of
// the returned Listener, the Onion Service will be torn down when the control
// connection is closed.
//
// WARNING: Only one port can be listened to per PrivateKey if this interface
// is used.  To bind to more ports, use the  AddOnion call directly.
func (c *Conn) Listener(port uint16, key crypto.PrivateKey) (net.Listener, error) {
	const (
		loopbackAddr = "127.0.0.1:0"
	)

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

	// Create the onion.
	ports := []OnionPortSpec{{port, strconv.FormatUint((uint64)(tAddr.Port), 10)}}
	oi, err := c.AddOnion(ports, key, key == nil)
	if err != nil {
		tcpListener.Close()
		return nil, err
	}

	oa := &onionAddr{info: oi, port: port}
	ol := &onionListener{addr: oa, ctrlConn: c, listener: tcpListener}

	return ol, nil
}
