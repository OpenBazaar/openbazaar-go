package connsec

import (
	"context"
	"net"

	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	inet "gx/ipfs/QmXuRkCR7BNQa9uqfpTiFWsTQLzmTWYg91Ja1w95gnqb6u/go-libp2p-net"
)

// A Transport turns inbound and outbound unauthenticated,
// plain-text connections into authenticated, encrypted connections.
type Transport interface {
	// SecureInbound secures an inbound connection.
	SecureInbound(ctx context.Context, insecure net.Conn) (Conn, error)

	// SecureOutbound secures an outbound connection.
	SecureOutbound(ctx context.Context, insecure net.Conn, p peer.ID) (Conn, error)
}

// Conn is an authenticated, encrypted connection.
type Conn interface {
	net.Conn
	inet.ConnSecurity
}
