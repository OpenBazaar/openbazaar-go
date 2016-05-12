package core

import (
	"golang.org/x/net/context"
	peer "gx/ipfs/QmZwZjMVGss5rqYsJVGy18gNbkTJffFyq2x1uJ4e4p3ZAt/go-libp2p-peer"
	"github.com/OpenBazaar/openbazaar-go/pb"
)

func (n *OpenBazaarNode) GetPeerStatus(peerId string) string {
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return "error parsing peerId"
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := pb.Message{MessageType: pb.Message_PING,}
	_, err = n.Service.SendRequest(ctx, p, &m)
	if err != nil {
		return "offline"
	}
	return "online"
}
