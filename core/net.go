package core

import (
	"errors"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	multihash "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	libp2p "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
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
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), n.OfflineMessageFailoverTimeout)
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

// SendOfflineMessage Supply of a public key is optional, if nil is instead provided n.EncryptMessage does a lookup
func (n *OpenBazaarNode) SendOfflineMessage(p peer.ID, k *libp2p.PubKey, m *pb.Message) error {
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
		err := ipfs.PublishPointer(n.IpfsNode, ctx, pointer)
		if err != nil {
			log.Error(err)
		}

		// Push provider to our push nodes for redundancy
		for _, p := range n.PushNodes {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := ipfs.PutPointerToPeer(n.IpfsNode, ctx, p, pointer)
			if err != nil {
				log.Error(err)
			}
		}

		OfflineMessageWaitGroup.Done()
	}()
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := n.Pubsub.Publisher.Publish(ctx, ipfs.MessageTopicPrefix+pointer.Cid.String(), ciphertext)
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

// SendOrder - send order created msg to peer
func (n *OpenBazaarNode) SendOrder(peerID string, contract *pb.RicardianContract) (resp *pb.Message, err error) {
	p, err := peer.IDB58Decode(peerID)
	if err != nil {
		return resp, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), n.OfflineMessageFailoverTimeout)
	defer cancel()
	pbAny, err := ptypes.MarshalAny(contract)
	if err != nil {
		return resp, err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER,
		Payload:     pbAny,
	}

	resp, err = n.Service.SendRequest(ctx, p, &m)
	if err != nil {
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
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER_CONFIRMATION,
		Payload:     a,
	}
	k, err := libp2p.UnmarshalPublicKey(contract.GetBuyerOrder().GetBuyerID().GetPubkeys().Identity)
	if err != nil {
		return err
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
	return n.sendMessage(peerID, kp, m)
}

// SendReject - send order rejected msg to peer
func (n *OpenBazaarNode) SendReject(peerID string, rejectMessage *pb.OrderReject) error {
	a, err := ptypes.MarshalAny(rejectMessage)
	if err != nil {
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
			return err
		}
		kp = &k
	}
	return n.sendMessage(peerID, kp, m)
}

// SendRefund - send refund msg to peer
func (n *OpenBazaarNode) SendRefund(peerID string, refundMessage *pb.RicardianContract) error {
	a, err := ptypes.MarshalAny(refundMessage)
	if err != nil {
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_REFUND,
		Payload:     a,
	}
	k, err := libp2p.UnmarshalPublicKey(refundMessage.GetBuyerOrder().GetBuyerID().GetPubkeys().Identity)
	if err != nil {
		return err
	}
	return n.sendMessage(peerID, &k, m)
}

// SendOrderFulfillment - send order fulfillment msg to peer
func (n *OpenBazaarNode) SendOrderFulfillment(peerID string, k *libp2p.PubKey, fulfillmentMessage *pb.RicardianContract) error {
	a, err := ptypes.MarshalAny(fulfillmentMessage)
	if err != nil {
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER_FULFILLMENT,
		Payload:     a,
	}
	return n.sendMessage(peerID, k, m)
}

// SendOrderCompletion - send order completion msg to peer
func (n *OpenBazaarNode) SendOrderCompletion(peerID string, k *libp2p.PubKey, completionMessage *pb.RicardianContract) error {
	a, err := ptypes.MarshalAny(completionMessage)
	if err != nil {
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_ORDER_COMPLETION,
		Payload:     a,
	}
	if err != nil {
		return err
	}
	return n.sendMessage(peerID, k, m)
}

// SendDisputeOpen - send open dispute msg to peer
func (n *OpenBazaarNode) SendDisputeOpen(peerID string, k *libp2p.PubKey, disputeMessage *pb.RicardianContract) error {
	a, err := ptypes.MarshalAny(disputeMessage)
	if err != nil {
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
		return err
	}
	payload, err := ptypes.MarshalAny(&pb.VendorFinalizedPayment{OrderID: orderID})
	if err != nil {
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
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_CHAT,
		Payload:     a,
	}

	p, err := peer.IDB58Decode(peerID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = n.Service.SendMessage(ctx, p, &m)
	if err != nil && chatMessage.Flag != pb.Chat_TYPING {
		if err := n.SendOfflineMessage(p, nil, &m); err != nil {
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
	block, err := n.IpfsNode.Blocks.GetBlock(ctx, &id)
	if err != nil {
		return err
	}

	b := &pb.Block{
		Cid:     block.Cid().String(),
		RawData: block.RawData(),
	}
	a, err := ptypes.MarshalAny(b)
	if err != nil {
		return err
	}
	m := pb.Message{
		MessageType: pb.Message_BLOCK,
		Payload:     a,
	}

	p, err := peer.IDB58Decode(peerID)
	if err != nil {
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
		return err
	}

	m := pb.Message{
		MessageType: pb.Message_STORE,
		Payload:     a,
	}

	p, err := peer.IDB58Decode(peerID)
	if err != nil {
		return err
	}
	pmes, err := n.Service.SendRequest(context.Background(), p, &m)
	if err != nil {
		return err
	}
	defer n.Service.DisconnectFromPeer(p)
	if pmes.Payload == nil {
		return errors.New("Peer responded with nil payload")
	}
	if pmes.MessageType == pb.Message_ERROR {
		log.Errorf("Error response from %s: %s", peerID, string(pmes.Payload.Value))
		return errors.New("Peer responded with error message")
	}

	resp := new(pb.CidList)
	err = ptypes.UnmarshalAny(pmes.Payload, resp)
	if err != nil {
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
		if err := n.SendBlock(peerID, *decoded); err != nil {
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
