package core

import (
	"golang.org/x/net/context"
	"github.com/OpenBazaar/openbazaar-go/pb"
	peer "gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"
)

// TODO: Right now these outgoing messages are only sent directly to the other peer.
// TODO: Once offline messaging is hooked up then failed direct messages should be sent via offline messaging.

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

func (n *OpenBazaarNode) Follow(peerId string) error {
	n.Datastore.Following().Put(peerId)
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := pb.Message{MessageType: pb.Message_FOLLOW,}
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil {
		return err
	}
	return nil
}

func (n *OpenBazaarNode) Unfollow(peerId string) error {
	n.Datastore.Following().Delete(peerId)
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := pb.Message{MessageType: pb.Message_UNFOLLOW,}
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil {
		return err
	}
	return nil
}
