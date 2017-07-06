package conn

import (
	"context"
	"errors"
	"net"
	"time"

	ic "gx/ipfs/QmP1DfoUjiWH2ZBo1PBH6FupdBucbDepx3HpWmEY6JMUpY/go-libp2p-crypto"
	tpt "gx/ipfs/QmSKoeo64CA54WNWqzVXHi5aT2LbgzP7B9VzQjW5yh7d4H/go-libp2p-transport"
	secio "gx/ipfs/QmV2e7FjHs7N27D4gRoe2wqRMhMCtfiMmm2teeKqvk7K4Y/go-libp2p-secio"
	iconn "gx/ipfs/QmcXRdAP5bCCm51X7XfDUrQ8Q9PsrKbU75pyvB18iuKob5/go-libp2p-interface-conn"
	ma "gx/ipfs/QmcyqRMCAXVtYPS4DiBrA7sezL9rRGfW8Ctx7cywL4TXJj/go-multiaddr"
	peer "gx/ipfs/QmdS9KpbDyPrieswibZhkod1oXqRwZJrUPzxCofAMWpFGq/go-libp2p-peer"
)

// secureConn wraps another Conn object with an encrypted channel.
type secureConn struct {
	insecure iconn.Conn    // the wrapped conn
	secure   secio.Session // secure Session
}

// newConn constructs a new connection
func newSecureConn(ctx context.Context, sk ic.PrivKey, insecure iconn.Conn) (iconn.Conn, error) {

	if insecure == nil {
		return nil, errors.New("insecure is nil")
	}
	if insecure.LocalPeer() == "" {
		return nil, errors.New("insecure.LocalPeer() is nil")
	}
	if sk == nil {
		return nil, errors.New("private key is nil")
	}

	// NewSession performs the secure handshake, which takes multiple RTT
	sessgen := secio.SessionGenerator{LocalID: insecure.LocalPeer(), PrivateKey: sk}
	secure, err := sessgen.NewSession(ctx, insecure)
	if err != nil {
		return nil, err
	}

	conn := &secureConn{
		insecure: insecure,
		secure:   secure,
	}
	return conn, nil
}

func (c *secureConn) Close() error {
	return c.secure.Close()
}

// ID is an identifier unique to this connection.
func (c *secureConn) ID() string {
	return iconn.ID(c)
}

func (c *secureConn) String() string {
	return iconn.String(c, "secureConn")
}

func (c *secureConn) LocalAddr() net.Addr {
	return c.insecure.LocalAddr()
}

func (c *secureConn) RemoteAddr() net.Addr {
	return c.insecure.RemoteAddr()
}

func (c *secureConn) SetDeadline(t time.Time) error {
	return c.insecure.SetDeadline(t)
}

func (c *secureConn) SetReadDeadline(t time.Time) error {
	return c.insecure.SetReadDeadline(t)
}

func (c *secureConn) SetWriteDeadline(t time.Time) error {
	return c.insecure.SetWriteDeadline(t)
}

// LocalMultiaddr is the Multiaddr on this side
func (c *secureConn) LocalMultiaddr() ma.Multiaddr {
	return c.insecure.LocalMultiaddr()
}

// RemoteMultiaddr is the Multiaddr on the remote side
func (c *secureConn) RemoteMultiaddr() ma.Multiaddr {
	return c.insecure.RemoteMultiaddr()
}

// LocalPeer is the Peer on this side
func (c *secureConn) LocalPeer() peer.ID {
	return c.secure.LocalPeer()
}

// RemotePeer is the Peer on the remote side
func (c *secureConn) RemotePeer() peer.ID {
	return c.secure.RemotePeer()
}

// LocalPrivateKey is the public key of the peer on this side
func (c *secureConn) LocalPrivateKey() ic.PrivKey {
	return c.secure.LocalPrivateKey()
}

// RemotePubKey is the public key of the peer on the remote side
func (c *secureConn) RemotePublicKey() ic.PubKey {
	return c.secure.RemotePublicKey()
}

// Read reads data, net.Conn style
func (c *secureConn) Read(buf []byte) (int, error) {
	return c.secure.ReadWriter().Read(buf)
}

// Write writes data, net.Conn style
func (c *secureConn) Write(buf []byte) (int, error) {
	return c.secure.ReadWriter().Write(buf)
}

// ReleaseMsg releases a buffer
func (c *secureConn) ReleaseMsg(m []byte) {
	c.secure.ReadWriter().ReleaseMsg(m)
}

func (c *secureConn) Transport() tpt.Transport {
	return c.insecure.Transport()
}
