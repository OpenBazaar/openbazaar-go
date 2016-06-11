package core

import (
	"golang.org/x/net/context"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/golang/protobuf/proto"
	peer "gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"
	multihash "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
)

// TODO: Right now these outgoing messages are only sent directly to the other peer.
// TODO: Once offline messaging is hooked up then failed direct messages should be sent via offline messaging.

func (n *OpenBazaarNode) SendOfflineMessage(p peer.ID, m *pb.Message) error {
	log.Debugf("Sending offline message to %s", p.Pretty())
	env := pb.Envelope{Message: m, PeerID: p.Pretty()}
	messageBytes, merr := proto.Marshal(&env)
	if merr != nil {
		return merr
	}
	ciphertext, cerr := n.EncryptMessage(p, messageBytes)
	if cerr != nil {
		return cerr
	}
	addr, aerr := n.MessageStorage.Store(p, ciphertext)
	if aerr != nil {
		return aerr
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mh, mherr := multihash.FromB58String(p.Pretty())
	if mherr != nil {
		return mherr
	}
	// TODO: We're just using a default prefix length for now. Eventually we will want to customize this,
	// but we will need some way to get the recipient's desired prefix length. Likely will be in profile.
	if err := ipfs.AddPointer(n.IpfsNode, ctx, mh, 20, addr); err != nil {
		return err
	}
	return nil
}

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
	if err != nil { // Couldn't connect directly to peer. Likely offline.
		if err := n.SendOfflineMessage(p, &m); err != nil {
			return err
		}
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
		if err := n.SendOfflineMessage(p, &m); err != nil {
			return err
		}
	}
	return nil
}
