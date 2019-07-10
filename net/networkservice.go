package net

import (
	"context"
	"errors"

	inet "gx/ipfs/QmY3ArotKMKaL7YGfbQfyDrib6RVraLqZYWXZvVgZktBxp/go-libp2p-net"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"

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
	DisconnectFromPeer(p peer.ID)

	// Block until the service is available
	WaitForReady()
}
