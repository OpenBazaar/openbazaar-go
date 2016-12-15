package core

import (
	peer "gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
	libp2p "gx/ipfs/QmUWER4r4qMvaCnX5zREcfyiWN7cXN9g3a7fkRqNz8qWPP/go-libp2p-crypto"
	multihash "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"golang.org/x/net/context"
)

const (
	CHAT_MESSAGE_MAX_CHARACTERS = 20000
	CHAT_SUBJECT_MAX_CHARACTERS = 500
)

//supply of a public key is optional, if nil is instead provided n.EncryptMessage does a lookup
func (n *OpenBazaarNode) SendOfflineMessage(p peer.ID, k *libp2p.PubKey, m *pb.Message) error {
	log.Debugf("Sending offline message to %s", p.Pretty())
	pubKeyBytes, err := n.IpfsNode.PrivateKey.GetPublic().Bytes()
	if err != nil {
		return err
	}
	ser, err := proto.Marshal(m)
	if err != nil {
		return err
	}
	sig, err := n.IpfsNode.PrivateKey.Sign(ser)
	if err != nil {
		return err
	}
	env := pb.Envelope{Message: m, Pubkey: pubKeyBytes, Signature: sig}
	messageBytes, merr := proto.Marshal(&env)
	if merr != nil {
		return merr
	}
	ciphertext, cerr := n.EncryptMessage(p, k, messageBytes)
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
	/* TODO: We are just using a default prefix length for now. Eventually we will want to customize this,
	   but we will need some way to get the recipient's desired prefix length. Likely will be in profile. */
	pointer, err := ipfs.PublishPointer(n.IpfsNode, ctx, mh, 16, addr)
	if err != nil {
		return err
	}
	if m.MessageType != pb.Message_OFFLINE_ACK {
		pointer.Purpose = ipfs.MESSAGE
		err = n.Datastore.Pointers().Put(pointer)
		if err != nil {
			return err
		}
	}
	return nil
}

func (n *OpenBazaarNode) SendOfflineAck(peerId string, pointerID peer.ID) error {
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := &any.Any{Value: []byte(pointerID.Pretty())}
	m := pb.Message{
		MessageType: pb.Message_OFFLINE_ACK,
		Payload:     a}
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil { // Could not connect directly to peer. Likely offline.
		if err := n.SendOfflineMessage(p, nil, &m); err != nil {
			return err
		}
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
	m := pb.Message{MessageType: pb.Message_PING}
	_, err = n.Service.SendRequest(ctx, p, &m)
	if err != nil {
		return "offline"
	}
	return "online"
}

func (n *OpenBazaarNode) Follow(peerId string) error {
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := pb.Message{MessageType: pb.Message_FOLLOW}
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil { // Could not connect directly to peer. Likely offline.
		if err := n.SendOfflineMessage(p, nil, &m); err != nil {
			return err
		}
	}
	err = n.Datastore.Following().Put(peerId)
	if err != nil {
		return err
	}
	err = n.UpdateFollow()
	if err != nil {
		return err
	}
	return nil
}

func (n *OpenBazaarNode) Unfollow(peerId string) error {
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := pb.Message{MessageType: pb.Message_UNFOLLOW}
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil {
		if err := n.SendOfflineMessage(p, nil, &m); err != nil {
			return err
		}
	}
	err = n.Datastore.Following().Delete(peerId)
	if err != nil {
		return err
	}
	err = n.UpdateFollow()
	if err != nil {
		return err
	}
	return nil
}

func (n *OpenBazaarNode) SendOrder(peerId string, contract *pb.RicardianContract) (resp *pb.Message, err error) {
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return resp, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	any, err := ptypes.MarshalAny(contract)
	if err != nil {
		return resp, err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER,
		Payload:     any,
	}

	resp, err = n.Service.SendRequest(ctx, p, &m)
	if err != nil {
		return resp, err
	}
	return resp, nil
}

func (n *OpenBazaarNode) SendOrderConfirmation(peerId string, contract *pb.RicardianContract) error {
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	any, err := ptypes.MarshalAny(contract)
	if err != nil {
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER_CONFIRMATION,
		Payload:     any,
	}
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil {
		if k, err := libp2p.UnmarshalPublicKey(contract.VendorListings[0].VendorID.Pubkeys.Guid); err != nil {
			if err := n.SendOfflineMessage(p, &k, &m); err != nil {
				return err
			}
		}
	}
	return nil
}

func (n *OpenBazaarNode) SendCancel(peerId, orderId string) error {
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &any.Any{Value: []byte(orderId)}
	m := pb.Message{
		MessageType: pb.Message_ORDER_CANCEL,
		Payload:     a,
	}
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil {
		if err := n.SendOfflineMessage(p, nil, &m); err != nil {
			return err
		}
	}
	return nil
}

func (n *OpenBazaarNode) SendReject(peerId string, rejectMessage *pb.OrderReject) error {
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := ptypes.MarshalAny(rejectMessage)
	if err != nil {
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER_REJECT,
		Payload:     a,
	}
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil {
		if err := n.SendOfflineMessage(p, nil, &m); err != nil {
			return err
		}
	}
	return nil
}

func (n *OpenBazaarNode) SendRefund(peerId string, refundMessage *pb.RicardianContract) error {
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := ptypes.MarshalAny(refundMessage)
	if err != nil {
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_REFUND,
		Payload:     a,
	}
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil {
		if k, err := libp2p.UnmarshalPublicKey(refundMessage.VendorListings[0].VendorID.Pubkeys.Guid); err != nil {
			if err := n.SendOfflineMessage(p, &k, &m); err != nil {
				return err
			}
		}
	}
	return nil
}

func (n *OpenBazaarNode) SendOrderFulfillment(peerId string, fulfillmentMessage *pb.RicardianContract) error {
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := ptypes.MarshalAny(fulfillmentMessage)
	if err != nil {
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER_FULFILLMENT,
		Payload:     a,
	}
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil {
		if k, err := libp2p.UnmarshalPublicKey(fulfillmentMessage.VendorListings[0].VendorID.Pubkeys.Guid); err != nil {
			if err := n.SendOfflineMessage(p, &k, &m); err != nil {
				return err
			}
		}
	}
	return nil
}

func (n *OpenBazaarNode) SendOrderCompletion(peerId string, completionMessage *pb.RicardianContract) error {
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := ptypes.MarshalAny(completionMessage)
	if err != nil {
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER_COMPLETION,
		Payload:     a,
	}
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil {
		if k, err := libp2p.UnmarshalPublicKey(completionMessage.VendorListings[0].VendorID.Pubkeys.Guid); err != nil {
			if err := n.SendOfflineMessage(p, &k, &m); err != nil {
				return err
			}
		}
	}
	return nil
}

func (n *OpenBazaarNode) SendDisputeOpen(peerId string, disputeMessage *pb.RicardianContract) error {
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := ptypes.MarshalAny(disputeMessage)
	if err != nil {
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_DISPUTE_OPEN,
		Payload:     a,
	}
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil {
		if err := n.SendOfflineMessage(p, &m); err != nil {
			return err
		}
	}
	return nil
}

func (n *OpenBazaarNode) SendDisputeUpdate(peerId string, updateMessage *pb.DisputeUpdate) error {
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := ptypes.MarshalAny(updateMessage)
	if err != nil {
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_DISPUTE_UPDATE,
		Payload:     a,
	}
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil {
		if err := n.SendOfflineMessage(p, &m); err != nil {
			return err
		}
	}
	return nil
}

func (n *OpenBazaarNode) SendDisputeClose(peerId string, resolutionMessage *pb.RicardianContract) error {
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := ptypes.MarshalAny(resolutionMessage)
	if err != nil {
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_DISPUTE_CLOSE,
		Payload:     a,
	}
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil {
		if err := n.SendOfflineMessage(p, &m); err != nil {
			return err
		}
	}
	return nil
}

func (n *OpenBazaarNode) SendChat(peerId string, chatMessage *pb.Chat) error {
	p, err := peer.IDB58Decode(peerId)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := ptypes.MarshalAny(chatMessage)
	if err != nil {
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_CHAT,
		Payload:     a,
	}
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil {
		if err := n.SendOfflineMessage(p, &m); err != nil {
			return err
		}
	}
	return nil
}
