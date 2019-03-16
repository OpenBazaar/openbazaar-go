package net

import (
	"context"
	"errors"

	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	inet "gx/ipfs/QmXuRkCR7BNQa9uqfpTiFWsTQLzmTWYg91Ja1w95gnqb6u/go-libp2p-net"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

var (
	OutOfOrderMessage = errors.New("message arrived out of order")
	DuplicateMessage  = errors.New("duplicate message")
)

type NetworkService interface {
	// Handle incoming streams
	HandleNewStream(s inet.Stream)

	// Get handler for message type
	HandlerForMsgType(t pb.Message_MessageType) func(peer.ID, *pb.Message, interface{}) (*pb.Message, error)

	// Send request to a peer and wait for the response
	SendRequest(ctx context.Context, p peer.ID, pmes *pb.Message) (*pb.Message, error)

	// Send a message to a peer without requiring a response
	SendMessage(ctx context.Context, p peer.ID, pmes *pb.Message) error

	// Disconnect from the given peer
	DisconnectFromPeer(p peer.ID) error

	// Block until the service is available
	WaitForReady()
}
