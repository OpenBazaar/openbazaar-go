package net

import (
	"context"
	inet "gx/ipfs/QmNa31VPzC561NWwRsJLE7nGYZYuuD2QfpK2b1q9BK54J1/go-libp2p-net"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"

	"errors"
	"github.com/OpenBazaar/openbazaar-go/pb"
)

var OutOfOrderMessage error = errors.New("Message arrived out of order")

type NetworkService interface {
	// Handle incoming streams
	HandleNewStream(s inet.Stream)

	// Get handler for mesage type
	HandlerForMsgType(t pb.Message_MessageType) func(peer.ID, *pb.Message, interface{}) (*pb.Message, error)

	// Send request to a peer and wait for the response
	SendRequest(ctx context.Context, p peer.ID, pmes *pb.Message) (*pb.Message, error)

	// Send a message to a peer without requiring a response
	SendMessage(ctx context.Context, p peer.ID, pmes *pb.Message) error
}
