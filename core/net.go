package core

import (
	"encoding/base64"
	"errors"
	"fmt"
	"gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	"gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/net"

	libp2p "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	"gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	"gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"

	"github.com/OpenBazaar/openbazaar-go/ipfs"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"golang.org/x/net/context"
)

const (
	// ChatMessageMaxCharacters - limit for chat msg
	ChatMessageMaxCharacters = 20000
	// ChatSubjectMaxCharacters - limit for chat subject
	ChatSubjectMaxCharacters = 500
	// DefaultPointerPrefixLength - default ipfs pointer prefix
	DefaultPointerPrefixLength = 14
)

// OfflineMessageWaitGroup - used for offline msgs
var OfflineMessageWaitGroup sync.WaitGroup

func (n *OpenBazaarNode) sendMessage(peerID string, k *libp2p.PubKey, message pb.Message) error {
	p, err := peer.IDB58Decode(peerID)
	if err != nil {
		log.Errorf("failed to decode peerID: %v", err)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), n.OfflineMessageFailoverTimeout)
	n.SendRelayedMessage(p, k, &message) // send relayed message immediately
	defer cancel()
	err = n.Service.SendMessage(ctx, p, &message)
	if err != nil {
		go func() {
			if err := n.SendOfflineMessage(p, k, &message); err != nil {
				log.Errorf("Error sending offline message %s", err.Error())
			}
		}()
	}
	return nil
}

func (n *OpenBazaarNode) SendRelayedMessage(p peer.ID, k *libp2p.PubKey, m *pb.Message) error {
	messageBytes, err := n.getMessageBytes(m)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), n.OfflineMessageFailoverTimeout)
	defer cancel()
	if k == nil {
		var pubKey libp2p.PubKey
		keyval, err := n.IpfsNode.Repo.Datastore().Get(datastore.NewKey("/pubkey/" + p.Pretty()))
		if err != nil {
			pubKey, err = routing.GetPublicKey(n.IpfsNode.Routing, ctx, p)
			if err != nil {
				log.Errorf("Failed to find public key for %s", p.Pretty())
				return err
			}
		} else {
			pubKey, err = libp2p.UnmarshalPublicKey(keyval)
			if err != nil {
				log.Errorf("Failed to find public key for %s", p.Pretty())
				return err
			}
		}
		k = &pubKey
	}

	relayciphertext, err := net.Encrypt(*k, messageBytes)
	if err != nil {
		return fmt.Errorf("Error: %s", err.Error())
	}

	// Base64 encode
	encodedCipherText := base64.StdEncoding.EncodeToString(relayciphertext)

	n.WebRelayManager.SendRelayMessage(encodedCipherText, p.Pretty())

	return nil
}

func (n *OpenBazaarNode) getMessageBytes(m *pb.Message) ([]byte, error) {
	pubKeyBytes, err := n.IpfsNode.PrivateKey.GetPublic().Bytes()
	if err != nil {
		return nil, err
	}
	ser, err := proto.Marshal(m)
	if err != nil {
		return nil, err
	}
	sig, err := n.IpfsNode.PrivateKey.Sign(ser)
	if err != nil {
		return nil, err
	}

	env := pb.Envelope{Message: m, Pubkey: pubKeyBytes, Signature: sig}
	messageBytes, merr := proto.Marshal(&env)
	if merr != nil {
		return nil, merr
	}
	return messageBytes, nil
}

// SendOfflineMessage Supply of a public key is optional, if nil is instead provided n.EncryptMessage does a lookup
func (n *OpenBazaarNode) SendOfflineMessage(p peer.ID, k *libp2p.PubKey, m *pb.Message) error {
	messageBytes, err := n.getMessageBytes(m)
	if err != nil {
		return err
	}

	// TODO: this function blocks if the recipient's public key is not on the local machine
	ciphertext, cerr := n.EncryptMessage(p, k, messageBytes)
	if cerr != nil {
		return cerr
	}

	addr, aerr := n.MessageStorage.Store(p, ciphertext)
	if aerr != nil {
		return aerr
	}
	mh, mherr := multihash.FromB58String(p.Pretty())
	if mherr != nil {
		return mherr
	}
	/* TODO: We are just using a default prefix length for now. Eventually we will want to customize this,
	but we will need some way to get the recipient's desired prefix length. Likely will be in profile. */
	pointer, err := ipfs.NewPointer(mh, DefaultPointerPrefixLength, addr, ciphertext)
	if err != nil {
		return err
	}
	if m.MessageType != pb.Message_OFFLINE_ACK {
		pointer.Purpose = ipfs.MESSAGE
		pointer.CancelID = &p
		err = n.Datastore.Pointers().Put(pointer)
		if err != nil {
			return err
		}
	}
	log.Debugf("Sending offline message to: %s, Message Type: %s, PointerID: %s, Location: %s", p.Pretty(), m.MessageType.String(), pointer.Cid.String(), pointer.Value.Addrs[0].String())
	OfflineMessageWaitGroup.Add(2)
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := ipfs.PublishPointer(n.DHT, ctx, pointer)
		if err != nil {
			log.Error(err)
		}

		// Push provider to our push nodes for redundancy
		for _, p := range n.PushNodes {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := ipfs.PutPointerToPeer(n.DHT, ctx, p, pointer)
			if err != nil {
				log.Error(err)
			}
		}

		OfflineMessageWaitGroup.Done()
	}()
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := n.Pubsub.Publisher.Publish(ctx, pointer.Cid.String(), ciphertext)
		if err != nil {
			log.Error(err)
		}
		OfflineMessageWaitGroup.Done()
	}()
	return nil
}

// SendOfflineAck - send ack to offline peer
func (n *OpenBazaarNode) SendOfflineAck(peerID string, pointerID peer.ID) error {
	a := &any.Any{Value: []byte(pointerID.Pretty())}
	m := pb.Message{
		MessageType: pb.Message_OFFLINE_ACK,
		Payload:     a}
	return n.sendMessage(peerID, nil, m)
}

// GetPeerStatus - check if a peer is online/offline
func (n *OpenBazaarNode) GetPeerStatus(peerID string) (string, error) {
	p, err := peer.IDB58Decode(peerID)
	if err != nil {
		log.Errorf("failed to decode peerID: %v", err)
		return "", err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := pb.Message{MessageType: pb.Message_PING}
	_, err = n.Service.SendRequest(ctx, p, &m)
	if err != nil {
		return "offline", nil
	}
	return "online", nil
}

// Follow - follow a peer
func (n *OpenBazaarNode) Follow(peerID string) error {
	m := pb.Message{MessageType: pb.Message_FOLLOW}

	pubkey := n.IpfsNode.PrivateKey.GetPublic()
	pubkeyBytes, err := pubkey.Bytes()
	if err != nil {
		return err
	}
	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}
	data := &pb.SignedData_Command{
		PeerID:    peerID,
		Type:      pb.Message_FOLLOW,
		Timestamp: ts,
	}
	ser, err := proto.Marshal(data)
	if err != nil {
		return err
	}
	sig, err := n.IpfsNode.PrivateKey.Sign(ser)
	if err != nil {
		return err
	}
	sd := &pb.SignedData{
		SerializedData: ser,
		SenderPubkey:   pubkeyBytes,
		Signature:      sig,
	}
	pbAny, err := ptypes.MarshalAny(sd)
	if err != nil {
		log.Errorf("failed to marshal the signedData: %v", err)
		return err
	}
	m.Payload = pbAny

	err = n.sendMessage(peerID, nil, m)
	if err != nil {
		return err
	}
	err = n.Datastore.Following().Put(peerID)
	if err != nil {
		return err
	}
	err = n.UpdateFollow()
	if err != nil {
		return err
	}
	return nil
}

// Unfollow - unfollow a peer
func (n *OpenBazaarNode) Unfollow(peerID string) error {
	m := pb.Message{MessageType: pb.Message_UNFOLLOW}

	pubkey := n.IpfsNode.PrivateKey.GetPublic()
	pubkeyBytes, err := pubkey.Bytes()
	if err != nil {
		return err
	}
	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}
	data := &pb.SignedData_Command{
		PeerID:    peerID,
		Type:      pb.Message_UNFOLLOW,
		Timestamp: ts,
	}
	ser, err := proto.Marshal(data)
	if err != nil {
		return err
	}
	sig, err := n.IpfsNode.PrivateKey.Sign(ser)
	if err != nil {
		return err
	}
	sd := &pb.SignedData{
		SerializedData: ser,
		SenderPubkey:   pubkeyBytes,
		Signature:      sig,
	}
	pbAny, err := ptypes.MarshalAny(sd)
	if err != nil {
		log.Errorf("failed to marshal the signedData: %v", err)
		return err
	}
	m.Payload = pbAny

	err = n.sendMessage(peerID, nil, m)
	if err != nil {
		return err
	}
	err = n.Datastore.Following().Delete(peerID)
	if err != nil {
		return err
	}
	err = n.UpdateFollow()
	if err != nil {
		return err
	}
	return nil
}

// ResendCachedOrderMessage will retrieve the ORDER message from the datastore and resend it to the peerID
// for which it was originally intended
func (n *OpenBazaarNode) ResendCachedOrderMessage(orderID string, msgType pb.Message_MessageType) error {
	if _, ok := pb.Message_MessageType_name[int32(msgType)]; !ok {
		return fmt.Errorf("invalid order message type (%d)", int(msgType))
	}

	msg, peerID, err := n.Datastore.Messages().GetByOrderIDType(orderID, msgType)
	if err != nil || msg == nil || msg.Msg.GetPayload() == nil {
		return fmt.Errorf("unable to find message for order ID (%s) and message type (%s)", orderID, msgType.String())
	}

	p, err := peer.IDB58Decode(peerID)
	if err != nil {
		return fmt.Errorf("unable to decode invalid peer ID for order (%s) and message type (%s)", orderID, msgType.String())
	}

	n.SendRelayedMessage(p, nil, &msg.Msg) // send relayed message immediately

	ctx, cancel := context.WithTimeout(context.Background(), n.OfflineMessageFailoverTimeout)
	defer cancel()

	if err = n.Service.SendMessage(ctx, p, &msg.Msg); err != nil {
		go func() {
			if err := n.SendOfflineMessage(p, nil, &msg.Msg); err != nil {
				log.Errorf("error resending offline message for order id (%s) and message type (%+v): %s", orderID, msgType, err.Error())
			}
		}()
	}
	return nil
}

// SendOrder - send order created msg to peer
func (n *OpenBazaarNode) SendOrder(peerID string, contract *pb.RicardianContract) (resp *pb.Message, err error) {
	p, err := peer.IDB58Decode(peerID)
	if err != nil {
		log.Errorf("failed to decode peerID: %v", err)
		return resp, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), n.OfflineMessageFailoverTimeout)
	defer cancel()
	pbAny, err := ptypes.MarshalAny(contract)
	if err != nil {
		log.Errorf("failed to marshal the contract: %v", err)
		return resp, err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER,
		Payload:     pbAny,
	}
	orderID0, err := n.CalcOrderID(contract.BuyerOrder)
	if err != nil {
		log.Errorf("failed calculating order id: %v", err)
	} else {
		err = n.Datastore.Messages().Put(
			fmt.Sprintf("%s-%d", orderID0, int(pb.Message_ORDER)),
			orderID0, pb.Message_ORDER, peerID, repo.Message{Msg: m})
		if err != nil {
			log.Errorf("failed putting message (%s-%d): %v", orderID0, int(pb.Message_ORDER), err)
		}
	}
	resp, err = n.Service.SendRequest(ctx, p, &m)
	if err != nil {
		log.Errorf("failed to send order request: %v", err)
		return resp, err
	}
	return resp, nil
}

// SendError - send error msg to peer
func (n *OpenBazaarNode) SendError(peerID string, k *libp2p.PubKey, errorMessage pb.Message) error {
	return n.sendMessage(peerID, k, errorMessage)
}

// SendOrderConfirmation - send order confirmed msg to peer
func (n *OpenBazaarNode) SendOrderConfirmation(peerID string, contract *pb.RicardianContract) error {
	a, err := ptypes.MarshalAny(contract)
	if err != nil {
		log.Errorf("failed to marshal the contract: %v", err)
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER_CONFIRMATION,
		Payload:     a,
	}
	k, err := libp2p.UnmarshalPublicKey(contract.GetBuyerOrder().GetBuyerID().GetPubkeys().Identity)
	if err != nil {
		log.Errorf("failed to unmarshal the publicKey: %v", err)
		return err
	}
	orderID0 := contract.VendorOrderConfirmation.OrderID
	if orderID0 == "" {
		log.Errorf("failed fetching orderID")
	} else {
		err = n.Datastore.Messages().Put(
			fmt.Sprintf("%s-%d", orderID0, int(pb.Message_ORDER_CONFIRMATION)),
			orderID0, pb.Message_ORDER_CONFIRMATION, peerID, repo.Message{Msg: m})
		if err != nil {
			log.Errorf("failed putting message (%s-%d): %v", orderID0, int(pb.Message_ORDER_CONFIRMATION), err)
		}
	}
	return n.sendMessage(peerID, &k, m)
}

// SendCancel - send order canceled msg to peer
func (n *OpenBazaarNode) SendCancel(peerID, orderID string) error {
	a := &any.Any{Value: []byte(orderID)}
	m := pb.Message{
		MessageType: pb.Message_ORDER_CANCEL,
		Payload:     a,
	}
	//try to get public key from order
	order, _, _, _, _, _, err := n.Datastore.Purchases().GetByOrderId(orderID)
	var kp *libp2p.PubKey
	if err != nil { //probably implies we can't find the order in the Datastore
		kp = nil //instead SendOfflineMessage can try to get the key from the peerId
	} else {
		k, err := libp2p.UnmarshalPublicKey(order.GetVendorListings()[0].GetVendorID().GetPubkeys().Identity)
		if err != nil {
			return err
		}
		kp = &k
	}
	err = n.Datastore.Messages().Put(
		fmt.Sprintf("%s-%d", orderID, int(pb.Message_ORDER_CANCEL)),
		orderID, pb.Message_ORDER_CANCEL, peerID, repo.Message{Msg: m})
	if err != nil {
		log.Errorf("failed putting message (%s-%d): %v", orderID, int(pb.Message_ORDER_CANCEL), err)
	}
	return n.sendMessage(peerID, kp, m)
}

// SendReject - send order rejected msg to peer
func (n *OpenBazaarNode) SendReject(peerID string, rejectMessage *pb.OrderReject) error {
	a, err := ptypes.MarshalAny(rejectMessage)
	if err != nil {
		log.Errorf("failed to marshal the contract: %v", err)
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER_REJECT,
		Payload:     a,
	}
	var kp *libp2p.PubKey
	//try to get public key from order
	order, _, _, _, _, _, err := n.Datastore.Sales().GetByOrderId(rejectMessage.OrderID)
	if err != nil { //probably implies we can't find the order in the Datastore
		kp = nil //instead SendOfflineMessage can try to get the key from the peerId
	} else {
		k, err := libp2p.UnmarshalPublicKey(order.GetBuyerOrder().GetBuyerID().GetPubkeys().Identity)
		if err != nil {
			log.Errorf("failed to unmarshal publicKey: %v", err)
			return err
		}
		kp = &k
	}
	err = n.Datastore.Messages().Put(
		fmt.Sprintf("%s-%d", rejectMessage.OrderID, int(pb.Message_ORDER_REJECT)),
		rejectMessage.OrderID, pb.Message_ORDER_REJECT, peerID, repo.Message{Msg: m})
	if err != nil {
		log.Errorf("failed putting message (%s-%d): %v", rejectMessage.OrderID, int(pb.Message_ORDER_REJECT), err)
	}
	return n.sendMessage(peerID, kp, m)
}

// SendRefund - send refund msg to peer
func (n *OpenBazaarNode) SendRefund(peerID string, refundMessage *pb.RicardianContract) error {
	a, err := ptypes.MarshalAny(refundMessage)
	if err != nil {
		log.Errorf("failed to marshal the contract: %v", err)
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_REFUND,
		Payload:     a,
	}
	k, err := libp2p.UnmarshalPublicKey(refundMessage.GetBuyerOrder().GetBuyerID().GetPubkeys().Identity)
	if err != nil {
		log.Errorf("failed to unmarshal publicKey: %v", err)
		return err
	}
	return n.sendMessage(peerID, &k, m)
}

// SendOrderFulfillment - send order fulfillment msg to peer
func (n *OpenBazaarNode) SendOrderFulfillment(peerID string, k *libp2p.PubKey, fulfillmentMessage *pb.RicardianContract) error {
	a, err := ptypes.MarshalAny(fulfillmentMessage)
	if err != nil {
		log.Errorf("failed to marshal the contract: %v", err)
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER_FULFILLMENT,
		Payload:     a,
	}
	orderID0 := fulfillmentMessage.VendorOrderFulfillment[0].OrderId
	if orderID0 != "" {
		log.Errorf("failed fetching orderID")
	} else {
		err = n.Datastore.Messages().Put(
			fmt.Sprintf("%s-%d", orderID0, int(pb.Message_ORDER_FULFILLMENT)),
			orderID0, pb.Message_ORDER_FULFILLMENT, peerID, repo.Message{Msg: m})
		if err != nil {
			log.Errorf("failed putting message (%s-%d): %v", orderID0, int(pb.Message_ORDER_FULFILLMENT), err)
		}
	}
	return n.sendMessage(peerID, k, m)
}

// SendOrderCompletion - send order completion msg to peer
func (n *OpenBazaarNode) SendOrderCompletion(peerID string, k *libp2p.PubKey, completionMessage *pb.RicardianContract) error {
	a, err := ptypes.MarshalAny(completionMessage)
	if err != nil {
		log.Errorf("failed to marshal the contract: %v", err)
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER_COMPLETION,
		Payload:     a,
	}
	orderID0 := completionMessage.BuyerOrderCompletion.OrderId
	if orderID0 == "" {
		log.Errorf("failed fetching orderID")
	} else {
		err = n.Datastore.Messages().Put(
			fmt.Sprintf("%s-%d", orderID0, int(pb.Message_ORDER_COMPLETION)),
			orderID0, pb.Message_ORDER_COMPLETION, peerID, repo.Message{Msg: m})
		if err != nil {
			log.Errorf("failed putting message (%s-%d): %v", orderID0, int(pb.Message_ORDER_COMPLETION), err)
		}
	}
	return n.sendMessage(peerID, k, m)
}

// SendDisputeOpen - send open dispute msg to peer
func (n *OpenBazaarNode) SendDisputeOpen(peerID string, k *libp2p.PubKey, disputeMessage *pb.RicardianContract) error {
	a, err := ptypes.MarshalAny(disputeMessage)
	if err != nil {
		log.Errorf("failed to marshal the contract: %v", err)
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_DISPUTE_OPEN,
		Payload:     a,
	}
	return n.sendMessage(peerID, k, m)
}

// SendDisputeUpdate - send update dispute msg to peer
func (n *OpenBazaarNode) SendDisputeUpdate(peerID string, updateMessage *pb.DisputeUpdate) error {
	a, err := ptypes.MarshalAny(updateMessage)
	if err != nil {
		log.Errorf("failed to marshal the contract: %v", err)
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_DISPUTE_UPDATE,
		Payload:     a,
	}
	return n.sendMessage(peerID, nil, m)
}

// SendDisputeClose - send dispute closed msg to peer
func (n *OpenBazaarNode) SendDisputeClose(peerID string, k *libp2p.PubKey, resolutionMessage *pb.RicardianContract) error {
	a, err := ptypes.MarshalAny(resolutionMessage)
	if err != nil {
		log.Errorf("failed to marshal the contract: %v", err)
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_DISPUTE_CLOSE,
		Payload:     a,
	}
	return n.sendMessage(peerID, k, m)
}

// SendFundsReleasedByVendor - send funds released by vendor msg to peer
func (n *OpenBazaarNode) SendFundsReleasedByVendor(peerID string, marshalledPeerPublicKey []byte, orderID string) error {
	peerKey, err := libp2p.UnmarshalPublicKey(marshalledPeerPublicKey)
	if err != nil {
		log.Errorf("failed to unmarshal the publicKey: %v", err)
		return err
	}
	payload, err := ptypes.MarshalAny(&pb.VendorFinalizedPayment{OrderID: orderID})
	if err != nil {
		log.Errorf("failed to marshal the finalized payment: %v", err)
		return err
	}
	message := pb.Message{
		MessageType: pb.Message_VENDOR_FINALIZED_PAYMENT,
		Payload:     payload,
	}
	return n.sendMessage(peerID, &peerKey, message)
}

// SendChat - send chat msg to peer
func (n *OpenBazaarNode) SendChat(peerID string, chatMessage *pb.Chat) error {
	a, err := ptypes.MarshalAny(chatMessage)
	if err != nil {
		log.Errorf("failed to marshal the chat message: %v", err)
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_CHAT,
		Payload:     a,
	}

	p, err := peer.IDB58Decode(peerID)
	if err != nil {
		log.Errorf("failed to decode peerID: %v", err)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), n.OfflineMessageFailoverTimeout)
	n.SendRelayedMessage(p, nil, &m) // send relayed message immediately
	defer cancel()
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil && chatMessage.Flag != pb.Chat_TYPING {
		if err := n.SendOfflineMessage(p, nil, &m); err != nil {
			log.Errorf("failed to send offline message: %v", err)
			return err
		}
	}
	return nil
}

// SendModeratorAdd - send add moderator msg to peer
func (n *OpenBazaarNode) SendModeratorAdd(peerID string) error {
	m := pb.Message{MessageType: pb.Message_MODERATOR_ADD}

	pubkey := n.IpfsNode.PrivateKey.GetPublic()
	pubkeyBytes, err := pubkey.Bytes()
	if err != nil {
		return err
	}
	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}
	data := &pb.SignedData_Command{
		PeerID:    peerID,
		Type:      pb.Message_MODERATOR_ADD,
		Timestamp: ts,
	}
	ser, err := proto.Marshal(data)
	if err != nil {
		return err
	}
	sig, err := n.IpfsNode.PrivateKey.Sign(ser)
	if err != nil {
		return err
	}
	sd := &pb.SignedData{
		SerializedData: ser,
		SenderPubkey:   pubkeyBytes,
		Signature:      sig,
	}
	pbAny, err := ptypes.MarshalAny(sd)
	if err != nil {
		log.Errorf("failed to marshal the signed data: %v", err)
		return err
	}
	m.Payload = pbAny

	err = n.sendMessage(peerID, nil, m)
	if err != nil {
		return err
	}
	return nil
}

// SendModeratorRemove - send remove moderator msg to peer
func (n *OpenBazaarNode) SendModeratorRemove(peerID string) error {
	m := pb.Message{MessageType: pb.Message_MODERATOR_REMOVE}

	pubkey := n.IpfsNode.PrivateKey.GetPublic()
	pubkeyBytes, err := pubkey.Bytes()
	if err != nil {
		return err
	}
	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}
	data := &pb.SignedData_Command{
		PeerID:    peerID,
		Type:      pb.Message_MODERATOR_REMOVE,
		Timestamp: ts,
	}
	ser, err := proto.Marshal(data)
	if err != nil {
		return err
	}
	sig, err := n.IpfsNode.PrivateKey.Sign(ser)
	if err != nil {
		return err
	}
	sd := &pb.SignedData{
		SerializedData: ser,
		SenderPubkey:   pubkeyBytes,
		Signature:      sig,
	}
	pbAny, err := ptypes.MarshalAny(sd)
	if err != nil {
		log.Errorf("failed to marshal the signedData: %v", err)
		return err
	}
	m.Payload = pbAny

	err = n.sendMessage(peerID, nil, m)
	if err != nil {
		return err
	}
	return nil
}

// SendBlock - send requested ipfs block to peer
func (n *OpenBazaarNode) SendBlock(peerID string, id cid.Cid) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	block, err := n.IpfsNode.Blocks.GetBlock(ctx, id)
	if err != nil {
		return err
	}

	b := &pb.Block{
		Cid:     block.Cid().String(),
		RawData: block.RawData(),
	}
	a, err := ptypes.MarshalAny(b)
	if err != nil {
		log.Errorf("failed to marshal the block: %v", err)
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_BLOCK,
		Payload:     a,
	}

	p, err := peer.IDB58Decode(peerID)
	if err != nil {
		log.Errorf("failed to decode peerID: %v", err)
		return err
	}
	return n.Service.SendMessage(context.Background(), p, &m)
}

// SendStore - send requested stores to peer
func (n *OpenBazaarNode) SendStore(peerID string, ids []cid.Cid) error {
	var s []string
	for _, d := range ids {
		s = append(s, d.String())
	}
	cList := new(pb.CidList)
	cList.Cids = s

	a, err := ptypes.MarshalAny(cList)
	if err != nil {
		log.Errorf("failed to marshal the cidList: %v", err)
		return err
	}

	m := pb.Message{
		MessageType: pb.Message_STORE,
		Payload:     a,
	}

	p, err := peer.IDB58Decode(peerID)
	if err != nil {
		log.Errorf("failed to decode peerID: %v", err)
		return err
	}
	pmes, err := n.Service.SendRequest(context.Background(), p, &m)
	if err != nil {
		return err
	}
	defer n.Service.DisconnectFromPeer(p)
	if pmes.Payload == nil {
		return errors.New("peer responded with nil payload")
	}
	if pmes.MessageType == pb.Message_ERROR {
		log.Errorf("Error response from %s: %s", peerID, string(pmes.Payload.Value))
		return errors.New("peer responded with error message")
	}

	resp := new(pb.CidList)
	err = ptypes.UnmarshalAny(pmes.Payload, resp)
	if err != nil {
		log.Errorf("failed to unmarshal the cidList: %v", err)
		return err
	}
	if len(resp.Cids) == 0 {
		log.Debugf("peer %s requested no blocks", peerID)
		return nil
	}
	log.Debugf("Sending %d blocks to %s", len(resp.Cids), peerID)
	for _, id := range resp.Cids {
		decoded, err := cid.Decode(id)
		if err != nil {
			log.Debugf("failed decoding store block (%s) for peer (%s)", id, peerID)
			continue
		}
		if err := n.SendBlock(peerID, decoded); err != nil {
			log.Debugf("failed sending store block (%s) to peer (%s)", id, peerID)
			continue
		}
	}
	return nil
}

// SendOfflineRelay - send and offline relay message to the peer. Used for relaying messages from
// a client node to another peer.
func (n *OpenBazaarNode) SendOfflineRelay(peerID string, encryptedMessage []byte) error {
	m := pb.Message{
		MessageType: pb.Message_OFFLINE_RELAY,
		Payload:     &any.Any{Value: encryptedMessage},
	}
	return n.sendMessage(peerID, nil, m)
}

// SendOrderPayment - send order payment msg to seller from buyer
func (n *OpenBazaarNode) SendOrderPayment(peerID string, paymentMessage *pb.OrderPaymentTxn) error {
	a, err := ptypes.MarshalAny(paymentMessage)
	if err != nil {
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER_PAYMENT,
		Payload:     a,
	}

	p, err := peer.IDB58Decode(peerID)
	if err != nil {
		return err
	}

	n.SendRelayedMessage(p, nil, &m) // send relayed message immediately

	ctx, cancel := context.WithTimeout(context.Background(), n.OfflineMessageFailoverTimeout)
	err = n.Service.SendMessage(ctx, p, &m)
	cancel()
	if err != nil {
		if err := n.SendOfflineMessage(p, nil, &m); err != nil {
			return err
		}
	}
	return nil
}
