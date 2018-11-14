package net

import (
	"context"
	"errors"
	inet "gx/ipfs/QmXuRkCR7BNQa9uqfpTiFWsTQLzmTWYg91Ja1w95gnqb6u/go-libp2p-net"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

var (
	OutOfOrderMessage error = errors.New("Message arrived out of order")
	DuplicateMessage  error = errors.New("Duplicate Message")
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
}
