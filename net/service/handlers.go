package service

import (
	"encoding/hex"
	"errors"
	"fmt"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	libp2p "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	"time"

	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	blocks "gx/ipfs/QmSn9Td7xgxm9EV7iEjTckpUWmWApggzPxu7eFGWkkpwin/go-block-format"
	"strconv"
)

func (service *OpenBazaarService) HandlerForMsgType(t pb.Message_MessageType) func(peer.ID, *pb.Message, interface{}) (*pb.Message, error) {
	switch t {
	case pb.Message_PING:
		return service.handlePing
	case pb.Message_FOLLOW:
		return service.handleFollow
	case pb.Message_UNFOLLOW:
		return service.handleUnFollow
	case pb.Message_OFFLINE_ACK:
		return service.handleOfflineAck
	case pb.Message_OFFLINE_RELAY:
		return service.handleOfflineRelay
	case pb.Message_ORDER:
		return service.handleOrder
	case pb.Message_ORDER_CONFIRMATION:
		return service.handleOrderConfirmation
	case pb.Message_ORDER_CANCEL:
		return service.handleOrderCancel
	case pb.Message_ORDER_REJECT:
		return service.handleReject
	case pb.Message_REFUND:
		return service.handleRefund
	case pb.Message_ORDER_FULFILLMENT:
		return service.handleOrderFulfillment
	case pb.Message_ORDER_COMPLETION:
		return service.handleOrderCompletion
	case pb.Message_DISPUTE_OPEN:
		return service.handleDisputeOpen
	case pb.Message_DISPUTE_UPDATE:
		return service.handleDisputeUpdate
	case pb.Message_DISPUTE_CLOSE:
		return service.handleDisputeClose
	case pb.Message_CHAT:
		return service.handleChat
	case pb.Message_MODERATOR_ADD:
		return service.handleModeratorAdd
	case pb.Message_MODERATOR_REMOVE:
		return service.handleModeratorRemove
	case pb.Message_BLOCK:
		return service.handleBlock
	case pb.Message_STORE:
		return service.handleStore
	case pb.Message_ERROR:
		return service.handleError
	default:
		return nil
	}
}

func (service *OpenBazaarService) handlePing(peer peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received PING message from %s", peer.Pretty())
	return pmes, nil
}

func (service *OpenBazaarService) handleFollow(pid peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	sd := new(pb.SignedData)
	err := ptypes.UnmarshalAny(pmes.Payload, sd)
	if err != nil {
		return nil, err
	}
	pubkey, err := libp2p.UnmarshalPublicKey(sd.SenderPubkey)
	if err != nil {
		return nil, err
	}
	id, err := peer.IDFromPublicKey(pubkey)
	if err != nil {
		return nil, err
	}
	data := new(pb.SignedData_Command)
	err = proto.Unmarshal(sd.SerializedData, data)
	if err != nil {
		return nil, err
	}
	if data.PeerID != service.node.IpfsNode.Identity.Pretty() {
		return nil, errors.New("Follow message doesn't include correct peer ID")
	}
	if data.Type != pb.Message_FOLLOW {
		return nil, errors.New("Data type is not follow")
	}
	good, err := pubkey.Verify(sd.SerializedData, sd.Signature)
	if err != nil || !good {
		return nil, errors.New("Bad signature")
	}

	proof := append(sd.SerializedData, sd.Signature...)
	err = service.datastore.Followers().Put(id.Pretty(), proof)
	if err != nil {
		return nil, err
	}
	n := notifications.FollowNotification{notifications.NewID(), "follow", id.Pretty()}
	service.broadcast <- n
	service.datastore.Notifications().Put(n.ID, n, n.Type, time.Now())
	log.Debugf("Received FOLLOW message from %s", id.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleUnFollow(pid peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	sd := new(pb.SignedData)
	err := ptypes.UnmarshalAny(pmes.Payload, sd)
	if err != nil {
		return nil, err
	}
	pubkey, err := libp2p.UnmarshalPublicKey(sd.SenderPubkey)
	if err != nil {
		return nil, err
	}
	id, err := peer.IDFromPublicKey(pubkey)
	if err != nil {
		return nil, err
	}
	data := new(pb.SignedData_Command)
	err = proto.Unmarshal(sd.SerializedData, data)
	if err != nil {
		return nil, err
	}
	if data.PeerID != service.node.IpfsNode.Identity.Pretty() {
		return nil, errors.New("Unfollow message doesn't include correct peer ID")
	}
	if data.Type != pb.Message_UNFOLLOW {
		return nil, errors.New("Data type is not unfollow")
	}
	good, err := pubkey.Verify(sd.SerializedData, sd.Signature)
	if err != nil || !good {
		return nil, errors.New("Bad signature")
	}
	err = service.datastore.Followers().Delete(id.Pretty())
	if err != nil {
		return nil, err
	}
	n := notifications.UnfollowNotification{notifications.NewID(), "unfollow", id.Pretty()}
	service.broadcast <- n
	log.Debugf("Received UNFOLLOW message from %s", id.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleOfflineAck(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	pid, err := peer.IDB58Decode(string(pmes.Payload.Value))
	if err != nil {
		return nil, err
	}
	pointer, err := service.datastore.Pointers().Get(pid)
	if err != nil {
		return nil, err
	}
	if pointer.CancelID == nil || pointer.CancelID.Pretty() != p.Pretty() {
		return nil, errors.New("Peer is not authorized to delete pointer")
	}
	err = service.datastore.Pointers().Delete(pid)
	if err != nil {
		return nil, err
	}
	log.Debugf("Received OFFLINE_ACK message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleOfflineRelay(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	// This acts very similarly to attemptDecrypt&handleMessage in the Offline Message Retreiver
	// However it does not send an ACK, or worry about message ordering

	// Decrypt and unmarshal plaintext
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	plaintext, err := net.Decrypt(service.node.IpfsNode.PrivateKey, pmes.Payload.Value)
	if err != nil {
		return nil, err
	}

	// Unmarshal plaintext
	env := pb.Envelope{}
	err = proto.Unmarshal(plaintext, &env)
	if err != nil {
		return nil, err
	}

	// Validate the signature
	ser, err := proto.Marshal(env.Message)
	if err != nil {
		return nil, err
	}
	pubkey, err := libp2p.UnmarshalPublicKey(env.Pubkey)
	if err != nil {
		return nil, err
	}
	valid, err := pubkey.Verify(ser, env.Signature)
	if err != nil || !valid {
		return nil, err
	}

	id, err := peer.IDFromPublicKey(pubkey)
	if err != nil {
		return nil, err
	}

	// Get handler for this message type
	handler := service.HandlerForMsgType(env.Message.MessageType)
	if handler == nil {
		log.Debug("Got back nil handler from HandlerForMsgType")
		return nil, nil
	}

	// Dispatch handler
	_, err = handler(id, env.Message, true)
	if err != nil {
		log.Errorf("Handle message error: %s", err)
		return nil, err
	}
	log.Debugf("Received OFFLINE_RELAY message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleOrder(peer peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	offline, _ := options.(bool)
	contract := new(pb.RicardianContract)
	var orderId string
	errorResponse := func(error string) *pb.Message {
		e := &pb.Error{
			Code:         0,
			ErrorMessage: error,
			OrderID:      orderId,
		}
		a, _ := ptypes.MarshalAny(e)
		m := &pb.Message{
			MessageType: pb.Message_ERROR,
			Payload:     a,
		}
		if offline {
			contract.Errors = []string{error}
			service.node.Datastore.Sales().Put(orderId, *contract, pb.OrderState_PROCESSING_ERROR, false)
		}
		return m
	}

	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	err := ptypes.UnmarshalAny(pmes.Payload, contract)
	if err != nil {
		return nil, err
	}

	orderId, err = service.node.CalcOrderId(contract.BuyerOrder)
	if err != nil {
		return errorResponse(err.Error()), err
	}

	pro, _ := service.node.GetProfile()
	if !pro.Vendor {
		return errorResponse("The vendor turned his store off and is not accepting orders at this time"), errors.New("Store is turned off")
	}

	err = service.node.ValidateOrder(contract, !offline)
	if err != nil && (err != core.ErrPurchaseUnknownListing || !offline) {
		return errorResponse(err.Error()), err
	}
	currentTime := time.Now()
	purchaseTime := time.Unix(contract.BuyerOrder.Timestamp.Seconds, int64(contract.BuyerOrder.Timestamp.Nanos))

	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_ADDRESS_REQUEST {
		total, err := service.node.CalculateOrderTotal(contract)
		if err != nil {
			return errorResponse("Error calculating payment amount"), err
		}
		if !service.node.ValidatePaymentAmount(total, contract.BuyerOrder.Payment.Amount) {
			return errorResponse("Calculated a different payment amount"), errors.New("Calculated different payment amount")
		}
		contract, err = service.node.NewOrderConfirmation(contract, true, false)
		if err != nil {
			return errorResponse("Error building order confirmation"), err
		}
		a, err := ptypes.MarshalAny(contract)
		if err != nil {
			return errorResponse("Error building order confirmation"), err
		}
		service.node.Datastore.Sales().Put(contract.VendorOrderConfirmation.OrderID, *contract, pb.OrderState_AWAITING_PAYMENT, false)
		if currentTime.After(purchaseTime) {
			service.node.Datastore.Sales().SetNeedsResync(contract.VendorOrderConfirmation.OrderID, true)
		}
		m := pb.Message{
			MessageType: pb.Message_ORDER_CONFIRMATION,
			Payload:     a,
		}
		log.Debugf("Received addr-req ORDER message from %s", peer.Pretty())
		return &m, nil
	} else if contract.BuyerOrder.Payment.Method == pb.Order_Payment_DIRECT {
		err := service.node.ValidateDirectPaymentAddress(contract.BuyerOrder)
		if err != nil {
			return errorResponse(err.Error()), err
		}
		addr, err := service.node.Wallet.DecodeAddress(contract.BuyerOrder.Payment.Address)
		if err != nil {
			return errorResponse(err.Error()), err
		}
		script, err := service.node.Wallet.AddressToScript(addr)
		if err != nil {
			return errorResponse(err.Error()), err
		}
		service.node.Wallet.AddWatchedScript(script)
		service.node.Datastore.Sales().Put(orderId, *contract, pb.OrderState_AWAITING_PAYMENT, false)
		if currentTime.After(purchaseTime) {
			service.node.Datastore.Sales().SetNeedsResync(orderId, true)
		}
		log.Debugf("Received direct ORDER message from %s", peer.Pretty())
		return nil, nil
	} else if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED && !offline {
		total, err := service.node.CalculateOrderTotal(contract)
		if err != nil {
			return errorResponse("Error calculating payment amount"), errors.New("Error calculating payment amount")
		}
		if !service.node.ValidatePaymentAmount(total, contract.BuyerOrder.Payment.Amount) {
			return errorResponse("Calculated a different payment amount"), errors.New("Calculated different payment amount")
		}
		timeout, err := time.ParseDuration(strconv.Itoa(int(contract.VendorListings[0].Metadata.EscrowTimeoutHours)) + "h")
		if err != nil {
			return errorResponse(err.Error()), err
		}
		err = service.node.ValidateModeratedPaymentAddress(contract.BuyerOrder, timeout)
		if err != nil {
			return errorResponse(err.Error()), err
		}
		addr, err := service.node.Wallet.DecodeAddress(contract.BuyerOrder.Payment.Address)
		if err != nil {
			return errorResponse(err.Error()), err
		}
		script, err := service.node.Wallet.AddressToScript(addr)
		if err != nil {
			return errorResponse(err.Error()), err
		}
		service.node.Wallet.AddWatchedScript(script)
		contract, err = service.node.NewOrderConfirmation(contract, false, false)
		if err != nil {
			return errorResponse("Error building order confirmation"), errors.New("Error building order confirmation")
		}
		a, err := ptypes.MarshalAny(contract)
		if err != nil {
			return errorResponse("Error building order confirmation"), errors.New("Error building order confirmation")
		}
		service.node.Datastore.Sales().Put(contract.VendorOrderConfirmation.OrderID, *contract, pb.OrderState_AWAITING_PAYMENT, false)
		if currentTime.After(purchaseTime) {
			service.node.Datastore.Sales().SetNeedsResync(contract.VendorOrderConfirmation.OrderID, true)
		}
		m := pb.Message{
			MessageType: pb.Message_ORDER_CONFIRMATION,
			Payload:     a,
		}
		log.Debugf("Received moderated ORDER message from %s", peer.Pretty())
		return &m, nil
	} else if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED && offline {
		timeout, err := time.ParseDuration(strconv.Itoa(int(contract.VendorListings[0].Metadata.EscrowTimeoutHours)) + "h")
		if err != nil {
			log.Error(err)
			return errorResponse(err.Error()), err
		}
		err = service.node.ValidateModeratedPaymentAddress(contract.BuyerOrder, timeout)
		if err != nil {
			log.Error(err)
			return errorResponse(err.Error()), err
		}
		addr, err := service.node.Wallet.DecodeAddress(contract.BuyerOrder.Payment.Address)
		if err != nil {
			log.Error(err)
			return errorResponse(err.Error()), err
		}
		script, err := service.node.Wallet.AddressToScript(addr)
		if err != nil {
			log.Error(err)
			return errorResponse(err.Error()), err
		}
		service.node.Wallet.AddWatchedScript(script)
		log.Debugf("Received offline moderated ORDER message from %s", peer.Pretty())
		service.node.Datastore.Sales().Put(orderId, *contract, pb.OrderState_AWAITING_PAYMENT, false)
		if currentTime.After(purchaseTime) {
			service.node.Datastore.Sales().SetNeedsResync(orderId, true)
		}
		return nil, nil
	}
	log.Error("Unrecognized payment type")
	return errorResponse("Unrecognized payment type"), errors.New("Unrecognized payment type")
}

func (service *OpenBazaarService) handleOrderConfirmation(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {

	// Unmarshal payload
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	vendorContract := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, vendorContract)
	if err != nil {
		return nil, fmt.Errorf("Could not unmarshal ORDER_CONFIRMATION from %s", p.Pretty())
	}

	if vendorContract.VendorOrderConfirmation == nil {
		return nil, errors.New("Received ORDER_CONFIRMATION message with nil confirmation object")
	}

	// Calc order ID
	orderId := vendorContract.VendorOrderConfirmation.OrderID

	// Load the order
	contract, state, funded, _, _, err := service.datastore.Purchases().GetByOrderId(orderId)
	if err != nil {
		return nil, net.OutOfOrderMessage
	}

	if funded && state == pb.OrderState_AWAITING_FULFILLMENT || !funded && state == pb.OrderState_AWAITING_PAYMENT {
		return nil, net.DuplicateMessage
	}

	// Validate the order confirmation
	err = service.node.ValidateOrderConfirmation(vendorContract, false)
	if err != nil {
		return nil, err
	}

	// Append the order confirmation
	contract.VendorOrderConfirmation = vendorContract.VendorOrderConfirmation
	for _, sig := range vendorContract.Signatures {
		if sig.Section == pb.Signature_ORDER_CONFIRMATION {
			contract.Signatures = append(contract.Signatures, sig)
		}
	}

	if funded {
		// Set message state to AWAITING_FULFILLMENT
		service.datastore.Purchases().Put(orderId, *contract, pb.OrderState_AWAITING_FULFILLMENT, false)
	} else {
		// Set message state to AWAITING_PAYMENT
		service.datastore.Purchases().Put(orderId, *contract, pb.OrderState_AWAITING_PAYMENT, false)
	}

	var thumbnailTiny string
	var thumbnailSmall string
	var vendorID string
	var vendorHandle string
	if len(contract.VendorListings) > 0 && contract.VendorListings[0].Item != nil && len(contract.VendorListings[0].Item.Images) > 0 {
		thumbnailTiny = contract.VendorListings[0].Item.Images[0].Tiny
		thumbnailSmall = contract.VendorListings[0].Item.Images[0].Small
		if contract.VendorListings[0].VendorID != nil {
			vendorID = contract.VendorListings[0].VendorID.PeerID
			vendorHandle = contract.VendorListings[0].VendorID.Handle
		}
	}

	// Send notification to websocket
	n := notifications.OrderConfirmationNotification{notifications.NewID(), "orderConfirmation", orderId, notifications.Thumbnail{thumbnailTiny, thumbnailSmall}, vendorHandle, vendorID}
	service.broadcast <- n
	service.datastore.Notifications().Put(n.ID, n, n.Type, time.Now())
	log.Debugf("Received ORDER_CONFIRMATION message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleOrderCancel(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	orderId := string(pmes.Payload.Value)

	// Load the order
	contract, state, _, _, _, err := service.datastore.Sales().GetByOrderId(orderId)
	if err != nil {
		return nil, net.OutOfOrderMessage
	}

	if state == pb.OrderState_CANCELED {
		return nil, net.DuplicateMessage
	}

	// Set message state to canceled
	service.datastore.Sales().Put(orderId, *contract, pb.OrderState_CANCELED, false)

	var thumbnailTiny string
	var thumbnailSmall string
	var buyerID string
	var buyerHandle string
	if len(contract.VendorListings) > 0 && contract.VendorListings[0].Item != nil && len(contract.VendorListings[0].Item.Images) > 0 {
		thumbnailTiny = contract.VendorListings[0].Item.Images[0].Tiny
		thumbnailSmall = contract.VendorListings[0].Item.Images[0].Small
		if contract.BuyerOrder != nil && contract.BuyerOrder.BuyerID != nil {
			buyerID = contract.BuyerOrder.BuyerID.PeerID
			buyerHandle = contract.BuyerOrder.BuyerID.Handle
		}
	}

	// Send notification to websocket
	n := notifications.OrderCancelNotification{notifications.NewID(), "canceled", orderId, notifications.Thumbnail{thumbnailTiny, thumbnailSmall}, buyerHandle, buyerID}
	service.broadcast <- n
	service.datastore.Notifications().Put(n.ID, n, n.Type, time.Now())
	log.Debugf("Received ORDER_CANCEL message from %s", p.Pretty())

	return nil, nil
}

func (service *OpenBazaarService) handleReject(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	rejectMsg := new(pb.OrderReject)
	err := ptypes.UnmarshalAny(pmes.Payload, rejectMsg)
	if err != nil {
		return nil, err
	}

	// Load the order
	contract, state, _, records, _, err := service.datastore.Purchases().GetByOrderId(rejectMsg.OrderID)
	if err != nil {
		return nil, net.OutOfOrderMessage
	}

	if state == pb.OrderState_DECLINED {
		return nil, net.DuplicateMessage
	}

	if contract.BuyerOrder.Payment.Method != pb.Order_Payment_MODERATED {
		// Sweep the address into our wallet
		var utxos []wallet.Utxo
		for _, r := range records {
			if !r.Spent && r.Value > 0 {
				u := wallet.Utxo{}
				scriptBytes, err := hex.DecodeString(r.ScriptPubKey)
				if err != nil {
					return nil, err
				}
				u.ScriptPubkey = scriptBytes
				hash, err := chainhash.NewHashFromStr(r.Txid)
				if err != nil {
					return nil, err
				}
				outpoint := wire.NewOutPoint(hash, r.Index)
				u.Op = *outpoint
				u.Value = r.Value
				utxos = append(utxos, u)
			}
		}

		chaincode, err := hex.DecodeString(contract.BuyerOrder.Payment.Chaincode)
		if err != nil {
			return nil, err
		}
		parentFP := []byte{0x00, 0x00, 0x00, 0x00}
		mPrivKey := service.node.Wallet.MasterPrivateKey()
		if err != nil {
			return nil, err
		}
		mECKey, err := mPrivKey.ECPrivKey()
		if err != nil {
			return nil, err
		}
		hdKey := hd.NewExtendedKey(
			service.node.Wallet.Params().HDPrivateKeyID[:],
			mECKey.Serialize(),
			chaincode,
			parentFP,
			0,
			0,
			true)

		buyerKey, err := hdKey.Child(0)
		if err != nil {
			return nil, err
		}
		redeemScript, err := hex.DecodeString(contract.BuyerOrder.Payment.RedeemScript)
		refundAddress, err := service.node.Wallet.DecodeAddress(contract.BuyerOrder.RefundAddress)
		if err != nil {
			return nil, err
		}
		_, err = service.node.Wallet.SweepAddress(utxos, &refundAddress, buyerKey, &redeemScript, wallet.NORMAL)
		if err != nil {
			return nil, err
		}
	} else {
		var ins []wallet.TransactionInput
		var outValue int64
		for _, r := range records {
			if !r.Spent && r.Value > 0 {
				outpointHash, err := hex.DecodeString(r.Txid)
				if err != nil {
					return nil, err
				}
				outValue += r.Value
				in := wallet.TransactionInput{OutpointIndex: r.Index, OutpointHash: outpointHash, Value: r.Value}
				ins = append(ins, in)
			}
		}

		refundAddress, err := service.node.Wallet.DecodeAddress(contract.BuyerOrder.RefundAddress)
		if err != nil {
			return nil, err
		}
		var output wallet.TransactionOutput
		outputScript, err := service.node.Wallet.AddressToScript(refundAddress)
		if err != nil {
			return nil, err
		}
		output.ScriptPubKey = outputScript
		output.Value = outValue

		chaincode, err := hex.DecodeString(contract.BuyerOrder.Payment.Chaincode)
		if err != nil {
			return nil, err
		}
		parentFP := []byte{0x00, 0x00, 0x00, 0x00}
		mPrivKey := service.node.Wallet.MasterPrivateKey()
		if err != nil {
			return nil, err
		}
		mECKey, err := mPrivKey.ECPrivKey()
		if err != nil {
			return nil, err
		}
		hdKey := hd.NewExtendedKey(
			service.node.Wallet.Params().HDPrivateKeyID[:],
			mECKey.Serialize(),
			chaincode,
			parentFP,
			0,
			0,
			true)

		buyerKey, err := hdKey.Child(0)
		if err != nil {
			return nil, err
		}
		redeemScript, err := hex.DecodeString(contract.BuyerOrder.Payment.RedeemScript)

		buyerSignatures, err := service.node.Wallet.CreateMultisigSignature(ins, []wallet.TransactionOutput{output}, buyerKey, redeemScript, contract.BuyerOrder.RefundFee)
		if err != nil {
			return nil, err
		}
		var vendorSignatures []wallet.Signature
		for _, s := range rejectMsg.Sigs {
			sig := wallet.Signature{InputIndex: s.InputIndex, Signature: s.Signature}
			vendorSignatures = append(vendorSignatures, sig)
		}
		_, err = service.node.Wallet.Multisign(ins, []wallet.TransactionOutput{output}, buyerSignatures, vendorSignatures, redeemScript, contract.BuyerOrder.RefundFee, true)
		if err != nil {
			return nil, err
		}
	}

	// Set message state to rejected
	service.datastore.Purchases().Put(rejectMsg.OrderID, *contract, pb.OrderState_DECLINED, false)

	var thumbnailTiny string
	var thumbnailSmall string
	var vendorID string
	var vendorHandle string
	if len(contract.VendorListings) > 0 && contract.VendorListings[0].Item != nil && len(contract.VendorListings[0].Item.Images) > 0 {
		thumbnailTiny = contract.VendorListings[0].Item.Images[0].Tiny
		thumbnailSmall = contract.VendorListings[0].Item.Images[0].Small
		if contract.VendorListings[0].VendorID != nil {
			vendorID = contract.VendorListings[0].VendorID.PeerID
			vendorHandle = contract.VendorListings[0].VendorID.Handle
		}
	}

	// Send notification to websocket
	n := notifications.OrderDeclinedNotification{notifications.NewID(), "declined", rejectMsg.OrderID, notifications.Thumbnail{thumbnailTiny, thumbnailSmall}, vendorHandle, vendorID}
	service.broadcast <- n

	service.datastore.Notifications().Put(n.ID, n, n.Type, time.Now())
	log.Debugf("Received REJECT message from %s", p.Pretty())

	return nil, nil
}

func (service *OpenBazaarService) handleRefund(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	rc := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, rc)
	if err != nil {
		return nil, err
	}

	if rc.Refund == nil {
		return nil, errors.New("Received REFUND message with nil refund object")
	}

	if err := service.node.VerifySignaturesOnRefund(rc); err != nil {
		return nil, err
	}

	// Load the order
	contract, state, _, records, _, err := service.datastore.Purchases().GetByOrderId(rc.Refund.OrderID)
	if err != nil {
		return nil, net.OutOfOrderMessage
	}

	if !(state == pb.OrderState_PARTIALLY_FULFILLED || state == pb.OrderState_AWAITING_FULFILLMENT) {
		return nil, net.DuplicateMessage
	}

	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
		var ins []wallet.TransactionInput
		var outValue int64
		for _, r := range records {
			if !r.Spent && r.Value > 0 {
				outpointHash, err := hex.DecodeString(r.Txid)
				if err != nil {
					return nil, err
				}
				outValue += r.Value
				in := wallet.TransactionInput{OutpointIndex: r.Index, OutpointHash: outpointHash, Value: r.Value}
				ins = append(ins, in)
			}
		}

		refundAddress, err := service.node.Wallet.DecodeAddress(contract.BuyerOrder.RefundAddress)
		if err != nil {
			return nil, err
		}
		var output wallet.TransactionOutput
		outputScript, err := service.node.Wallet.AddressToScript(refundAddress)
		if err != nil {
			return nil, err
		}
		output.ScriptPubKey = outputScript
		output.Value = outValue

		chaincode, err := hex.DecodeString(contract.BuyerOrder.Payment.Chaincode)
		if err != nil {
			return nil, err
		}
		parentFP := []byte{0x00, 0x00, 0x00, 0x00}
		mPrivKey := service.node.Wallet.MasterPrivateKey()
		if err != nil {
			return nil, err
		}
		mECKey, err := mPrivKey.ECPrivKey()
		if err != nil {
			return nil, err
		}
		hdKey := hd.NewExtendedKey(
			service.node.Wallet.Params().HDPrivateKeyID[:],
			mECKey.Serialize(),
			chaincode,
			parentFP,
			0,
			0,
			true)

		buyerKey, err := hdKey.Child(0)
		if err != nil {
			return nil, err
		}
		redeemScript, err := hex.DecodeString(contract.BuyerOrder.Payment.RedeemScript)
		if err != nil {
			return nil, err
		}

		buyerSignatures, err := service.node.Wallet.CreateMultisigSignature(ins, []wallet.TransactionOutput{output}, buyerKey, redeemScript, contract.BuyerOrder.RefundFee)
		if err != nil {
			return nil, err
		}
		var vendorSignatures []wallet.Signature
		for _, s := range rc.Refund.Sigs {
			sig := wallet.Signature{InputIndex: s.InputIndex, Signature: s.Signature}
			vendorSignatures = append(vendorSignatures, sig)
		}
		_, err = service.node.Wallet.Multisign(ins, []wallet.TransactionOutput{output}, buyerSignatures, vendorSignatures, redeemScript, contract.BuyerOrder.RefundFee, true)
		if err != nil {
			return nil, err
		}
	}
	contract.Refund = rc.Refund
	for _, sig := range contract.Signatures {
		if sig.Section == pb.Signature_REFUND {
			contract.Signatures = append(contract.Signatures, sig)
		}
	}

	// Set message state to refunded
	service.datastore.Purchases().Put(contract.Refund.OrderID, *contract, pb.OrderState_REFUNDED, false)

	var thumbnailTiny string
	var thumbnailSmall string
	var vendorID string
	var vendorHandle string
	if len(contract.VendorListings) > 0 && contract.VendorListings[0].Item != nil && len(contract.VendorListings[0].Item.Images) > 0 {
		thumbnailTiny = contract.VendorListings[0].Item.Images[0].Tiny
		thumbnailSmall = contract.VendorListings[0].Item.Images[0].Small
		if contract.VendorListings[0].VendorID != nil {
			vendorID = contract.VendorListings[0].VendorID.PeerID
			vendorHandle = contract.VendorListings[0].VendorID.Handle
		}
	}

	// Send notification to websocket
	n := notifications.RefundNotification{notifications.NewID(), "refund", contract.Refund.OrderID, notifications.Thumbnail{thumbnailTiny, thumbnailSmall}, vendorHandle, vendorID}
	service.broadcast <- n
	service.datastore.Notifications().Put(n.ID, n, n.Type, time.Now())
	log.Debugf("Received REFUND message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleOrderFulfillment(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	rc := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, rc)
	if err != nil {
		return nil, err
	}

	if len(rc.VendorOrderFulfillment) == 0 {
		return nil, errors.New("Received FULFILLMENT message with no VendorOrderFulfillment objects")
	}

	// Load the order
	contract, state, _, _, _, err := service.datastore.Purchases().GetByOrderId(rc.VendorOrderFulfillment[0].OrderId)
	if err != nil {
		return nil, net.OutOfOrderMessage
	}

	if state == pb.OrderState_PENDING || state == pb.OrderState_AWAITING_PAYMENT {
		return nil, net.OutOfOrderMessage
	}

	if !(state == pb.OrderState_PARTIALLY_FULFILLED || state == pb.OrderState_AWAITING_FULFILLMENT) {
		return nil, net.DuplicateMessage
	}

	contract.VendorOrderFulfillment = append(contract.VendorOrderFulfillment, rc.VendorOrderFulfillment[0])
	for _, sig := range rc.Signatures {
		if sig.Section == pb.Signature_ORDER_FULFILLMENT {
			contract.Signatures = append(contract.Signatures, sig)
		}
	}

	if err := service.node.ValidateOrderFulfillment(rc.VendorOrderFulfillment[0], contract); err != nil {
		return nil, err
	}

	// Set message state to fulfilled if all listings have a matching fulfillment message
	if service.node.IsFulfilled(contract) {
		service.datastore.Purchases().Put(rc.VendorOrderFulfillment[0].OrderId, *contract, pb.OrderState_FULFILLED, false)
	} else {
		service.datastore.Purchases().Put(rc.VendorOrderFulfillment[0].OrderId, *contract, pb.OrderState_PARTIALLY_FULFILLED, false)
	}

	var thumbnailTiny string
	var thumbnailSmall string
	var vendorHandle string
	var vendorID string
	if len(contract.VendorListings) > 0 && contract.VendorListings[0].Item != nil && len(contract.VendorListings[0].Item.Images) > 0 {
		thumbnailTiny = contract.VendorListings[0].Item.Images[0].Tiny
		thumbnailSmall = contract.VendorListings[0].Item.Images[0].Small
		if contract.VendorListings[0].VendorID != nil {
			vendorID = contract.VendorListings[0].VendorID.PeerID
			vendorHandle = contract.VendorListings[0].VendorID.Handle
		}
	}

	// Send notification to websocket
	n := notifications.FulfillmentNotification{notifications.NewID(), "fulfillment", rc.VendorOrderFulfillment[0].OrderId, notifications.Thumbnail{thumbnailTiny, thumbnailSmall}, vendorHandle, vendorID}
	service.broadcast <- n
	service.datastore.Notifications().Put(n.ID, n, n.Type, time.Now())
	log.Debugf("Received ORDER_FULFILLMENT message from %s", p.Pretty())

	return nil, nil
}

func (service *OpenBazaarService) handleOrderCompletion(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {

	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	rc := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, rc)
	if err != nil {
		return nil, err
	}

	if rc.BuyerOrderCompletion == nil {
		return nil, errors.New("Received ORDER_COMPLETION with nil BuyerOrderCompletion object")
	}

	// Load the order
	contract, state, _, records, _, err := service.datastore.Sales().GetByOrderId(rc.BuyerOrderCompletion.OrderId)
	if err != nil {
		return nil, net.OutOfOrderMessage
	}

	if state == pb.OrderState_COMPLETED {
		return nil, net.DuplicateMessage
	}

	contract.BuyerOrderCompletion = rc.BuyerOrderCompletion
	for _, sig := range rc.Signatures {
		if sig.Section == pb.Signature_ORDER_COMPLETION {
			contract.Signatures = append(contract.Signatures, sig)
		}
	}

	if err := service.node.ValidateOrderCompletion(contract); err != nil {
		return nil, err
	}
	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED && state != pb.OrderState_DISPUTED && state != pb.OrderState_DECIDED && state != pb.OrderState_RESOLVED && state != pb.OrderState_PAYMENT_FINALIZED {
		var ins []wallet.TransactionInput
		var outValue int64
		for _, r := range records {
			if !r.Spent && r.Value > 0 {
				outpointHash, err := hex.DecodeString(r.Txid)
				if err != nil {
					return nil, err
				}
				outValue += r.Value
				in := wallet.TransactionInput{OutpointIndex: r.Index, OutpointHash: outpointHash}
				ins = append(ins, in)
			}
		}
		var payoutAddress btcutil.Address
		if len(contract.VendorOrderFulfillment) > 0 {
			payoutAddress, err = service.node.Wallet.DecodeAddress(contract.VendorOrderFulfillment[0].Payout.PayoutAddress)
			if err != nil {
				return nil, err
			}
		} else {
			payoutAddress = service.node.Wallet.CurrentAddress(wallet.EXTERNAL)
		}
		var output wallet.TransactionOutput
		outputScript, err := service.node.Wallet.AddressToScript(payoutAddress)
		if err != nil {
			return nil, err
		}
		output.ScriptPubKey = outputScript
		output.Value = outValue

		redeemScript, err := hex.DecodeString(contract.BuyerOrder.Payment.RedeemScript)
		if err != nil {
			return nil, err
		}

		var vendorSignatures []wallet.Signature
		for _, s := range contract.VendorOrderFulfillment[0].Payout.Sigs {
			sig := wallet.Signature{InputIndex: s.InputIndex, Signature: s.Signature}
			vendorSignatures = append(vendorSignatures, sig)
		}
		var buyerSignatures []wallet.Signature
		for _, s := range contract.BuyerOrderCompletion.PayoutSigs {
			sig := wallet.Signature{InputIndex: s.InputIndex, Signature: s.Signature}
			buyerSignatures = append(buyerSignatures, sig)
		}

		_, err = service.node.Wallet.Multisign(ins, []wallet.TransactionOutput{output}, buyerSignatures, vendorSignatures, redeemScript, contract.VendorOrderFulfillment[0].Payout.PayoutFeePerByte, true)
		if err != nil {
			return nil, err
		}
	}

	err = service.node.ValidateAndSaveRating(contract)
	if err != nil {
		log.Error("Error validating rating:", err)
	}

	// Set message state to complete
	service.datastore.Sales().Put(rc.BuyerOrderCompletion.OrderId, *contract, pb.OrderState_COMPLETED, false)

	var thumbnailTiny string
	var thumbnailSmall string
	var buyerID string
	var buyerHandle string
	if len(contract.VendorListings) > 0 && contract.VendorListings[0].Item != nil && len(contract.VendorListings[0].Item.Images) > 0 {
		thumbnailTiny = contract.VendorListings[0].Item.Images[0].Tiny
		thumbnailSmall = contract.VendorListings[0].Item.Images[0].Small
		if contract.BuyerOrder != nil && contract.BuyerOrder.BuyerID != nil {
			buyerID = contract.BuyerOrder.BuyerID.PeerID
			buyerHandle = contract.BuyerOrder.BuyerID.Handle
		}
	}

	// Send notification to websocket
	n := notifications.CompletionNotification{notifications.NewID(), "orderComplete", rc.BuyerOrderCompletion.OrderId, notifications.Thumbnail{thumbnailTiny, thumbnailSmall}, buyerHandle, buyerID}
	service.broadcast <- n
	service.datastore.Notifications().Put(n.ID, n, n.Type, time.Now())
	log.Debugf("Received ORDER_COMPLETION message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleDisputeOpen(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {

	// Unmarshall
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	rc := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, rc)
	if err != nil {
		return nil, err
	}

	// Verify signature
	err = service.node.VerifySignatureOnDisputeOpen(rc, p.Pretty())
	if err != nil {
		return nil, err
	}

	// Process message
	err = service.node.ProcessDisputeOpen(rc, p.Pretty())
	if err != nil {
		return nil, err
	}
	log.Debugf("Received DISPUTE_OPEN message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleDisputeUpdate(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {

	// Make sure we aren't currently processing any disputes before proceeding
	core.DisputeWg.Wait()

	// Unmarshall
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	update := new(pb.DisputeUpdate)
	err := ptypes.UnmarshalAny(pmes.Payload, update)
	if err != nil {
		return nil, err
	}
	buyerContract, vendorContract, _, _, _, _, _, err := service.node.Datastore.Cases().GetPayoutDetails(update.OrderId)
	if err != nil {
		return nil, net.OutOfOrderMessage
	}
	rc := new(pb.RicardianContract)
	err = proto.Unmarshal(update.SerializedContract, rc)
	if err != nil {
		return nil, err
	}
	var thumbnailTiny string
	var thumbnailSmall string
	var disputerID string
	var disputerHandle string
	var disputeeID string
	var disputeeHandle string
	var buyer string
	if buyerContract == nil {
		buyerValidationErrors := service.node.ValidateCaseContract(rc)
		err = service.node.Datastore.Cases().UpdateBuyerInfo(update.OrderId, rc, buyerValidationErrors, update.PayoutAddress, update.Outpoints)
		if err != nil {
			return nil, err
		}
		if len(vendorContract.VendorListings) > 0 && vendorContract.VendorListings[0].Item != nil && len(vendorContract.VendorListings[0].Item.Images) > 0 {
			thumbnailTiny = vendorContract.VendorListings[0].Item.Images[0].Tiny
			thumbnailSmall = vendorContract.VendorListings[0].Item.Images[0].Small
			if vendorContract.VendorListings[0].VendorID != nil {
				disputerID = vendorContract.VendorListings[0].VendorID.PeerID
				disputerHandle = vendorContract.VendorListings[0].VendorID.Handle
			}
			if vendorContract.BuyerOrder.BuyerID != nil {
				buyer = vendorContract.BuyerOrder.BuyerID.PeerID
				disputeeID = vendorContract.BuyerOrder.BuyerID.PeerID
				disputeeHandle = vendorContract.BuyerOrder.BuyerID.Handle
			}
		}
	} else if vendorContract == nil {
		vendorValidationErrors := service.node.ValidateCaseContract(rc)
		err = service.node.Datastore.Cases().UpdateVendorInfo(update.OrderId, rc, vendorValidationErrors, update.PayoutAddress, update.Outpoints)
		if err != nil {
			return nil, err
		}
		if len(buyerContract.VendorListings) > 0 && buyerContract.VendorListings[0].Item != nil && len(buyerContract.VendorListings[0].Item.Images) > 0 {
			thumbnailTiny = buyerContract.VendorListings[0].Item.Images[0].Tiny
			thumbnailSmall = buyerContract.VendorListings[0].Item.Images[0].Small
			if buyerContract.VendorListings[0].VendorID != nil {
				disputeeID = buyerContract.VendorListings[0].VendorID.PeerID
				disputeeHandle = buyerContract.VendorListings[0].VendorID.Handle
			}
			if buyerContract.BuyerOrder.BuyerID != nil {
				buyer = buyerContract.BuyerOrder.BuyerID.PeerID
				disputerID = buyerContract.BuyerOrder.BuyerID.PeerID
				disputerHandle = buyerContract.BuyerOrder.BuyerID.Handle
			}
		}
	} else {
		return nil, errors.New("All contracts have already been received")
	}

	// Send notification to websocket
	n := notifications.DisputeUpdateNotification{notifications.NewID(), "disputeUpdate", update.OrderId, notifications.Thumbnail{thumbnailTiny, thumbnailSmall}, disputerID, disputerHandle, disputeeID, disputeeHandle, buyer}
	service.broadcast <- n

	service.datastore.Notifications().Put(n.ID, n, n.Type, time.Now())
	log.Debugf("Received DISPUTE_UPDATE message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleDisputeClose(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {

	// Unmarshall
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	rc := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, rc)
	if err != nil {
		return nil, err
	}

	// Load the order
	isPurchase := false
	var contract *pb.RicardianContract
	var state pb.OrderState
	var otherPartyID string
	var otherPartyHandle string
	var buyer string
	contract, state, _, _, _, err = service.datastore.Sales().GetByOrderId(rc.DisputeResolution.OrderId)
	if err != nil {
		contract, state, _, _, _, err = service.datastore.Purchases().GetByOrderId(rc.DisputeResolution.OrderId)
		if err != nil {
			return nil, net.OutOfOrderMessage
		}
		isPurchase = true
		if len(contract.VendorListings) > 0 && contract.VendorListings[0].VendorID != nil {
			otherPartyID = contract.VendorListings[0].VendorID.PeerID
			otherPartyHandle = contract.VendorListings[0].VendorID.Handle
			if contract.BuyerOrder != nil && contract.BuyerOrder.BuyerID != nil {
				buyer = contract.BuyerOrder.BuyerID.PeerID
			}
		}
	} else {
		if contract.BuyerOrder != nil && contract.BuyerOrder.BuyerID != nil {
			otherPartyID = contract.BuyerOrder.BuyerID.PeerID
			otherPartyHandle = contract.BuyerOrder.BuyerID.Handle
			buyer = contract.BuyerOrder.BuyerID.PeerID
		}
	}

	if state != pb.OrderState_DISPUTED {
		return nil, net.OutOfOrderMessage
	}

	// Validate
	contract.DisputeResolution = rc.DisputeResolution
	for _, sig := range rc.Signatures {
		if sig.Section == pb.Signature_DISPUTE_RESOLUTION {
			contract.Signatures = append(contract.Signatures, sig)
		}
	}
	err = service.node.ValidateDisputeResolution(contract)
	if err != nil {
		return nil, err
	}

	if isPurchase {
		// Set message state to complete
		err = service.datastore.Purchases().Put(rc.DisputeResolution.OrderId, *contract, pb.OrderState_DECIDED, false)
	} else {
		err = service.datastore.Sales().Put(rc.DisputeResolution.OrderId, *contract, pb.OrderState_DECIDED, false)
	}
	if err != nil {
		return nil, err
	}

	var thumbnailTiny string
	var thumbnailSmall string
	if len(contract.VendorListings) > 0 && contract.VendorListings[0].Item != nil && len(contract.VendorListings[0].Item.Images) > 0 {
		thumbnailTiny = contract.VendorListings[0].Item.Images[0].Tiny
		thumbnailSmall = contract.VendorListings[0].Item.Images[0].Small
	}

	// Send notification to websocket
	n := notifications.DisputeCloseNotification{notifications.NewID(), "disputeClose", rc.DisputeResolution.OrderId, notifications.Thumbnail{thumbnailTiny, thumbnailSmall}, otherPartyID, otherPartyHandle, buyer}
	service.broadcast <- n

	service.datastore.Notifications().Put(n.ID, n, n.Type, time.Now())
	log.Debugf("Received DISPUTE_CLOSE message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleChat(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {

	// Unmarshall
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	chat := new(pb.Chat)
	err := ptypes.UnmarshalAny(pmes.Payload, chat)
	if err != nil {
		return nil, err
	}

	if chat.Flag == pb.Chat_TYPING {
		n := notifications.ChatTyping{
			PeerId:  p.Pretty(),
			Subject: chat.Subject,
		}
		service.broadcast <- notifications.Serialize(n)
		return nil, nil
	}
	if chat.Flag == pb.Chat_READ {
		n := notifications.ChatRead{
			PeerId:    p.Pretty(),
			Subject:   chat.Subject,
			MessageId: chat.MessageId,
		}
		service.broadcast <- n
		_, _, err = service.datastore.Chat().MarkAsRead(p.Pretty(), chat.Subject, true, chat.MessageId)
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	// Validate
	if len(chat.Subject) > core.CHAT_SUBJECT_MAX_CHARACTERS {
		return nil, errors.New("Chat subject over max characters")
	}
	if len(chat.Message) > core.CHAT_MESSAGE_MAX_CHARACTERS {
		return nil, errors.New("Chat message over max characters")
	}

	// Use correct timestamp
	offline, _ := options.(bool)
	var t time.Time
	if !offline {
		t = time.Now()
	} else {
		if chat.Timestamp == nil {
			return nil, errors.New("Invalid timestamp")
		}
		t, err = ptypes.Timestamp(chat.Timestamp)
		if err != nil {
			return nil, err
		}
	}

	// Put to database
	err = service.datastore.Chat().Put(chat.MessageId, p.Pretty(), chat.Subject, chat.Message, t, false, false)
	if err != nil {
		return nil, err
	}

	if chat.Subject != "" {
		go func() {
			service.datastore.Purchases().MarkAsUnread(chat.Subject)
			service.datastore.Sales().MarkAsUnread(chat.Subject)
			service.datastore.Cases().MarkAsUnread(chat.Subject)
		}()
	}

	// Push to websocket
	n := notifications.ChatMessage{
		MessageId: chat.MessageId,
		PeerId:    p.Pretty(),
		Subject:   chat.Subject,
		Message:   chat.Message,
		Timestamp: t,
	}
	service.broadcast <- n
	log.Debugf("Received CHAT message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleModeratorAdd(pid peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	sd := new(pb.SignedData)
	err := ptypes.UnmarshalAny(pmes.Payload, sd)
	if err != nil {
		return nil, err
	}
	pubkey, err := libp2p.UnmarshalPublicKey(sd.SenderPubkey)
	if err != nil {
		return nil, err
	}
	id, err := peer.IDFromPublicKey(pubkey)
	if err != nil {
		return nil, err
	}
	data := new(pb.SignedData_Command)
	err = proto.Unmarshal(sd.SerializedData, data)
	if err != nil {
		return nil, err
	}
	if data.PeerID != service.node.IpfsNode.Identity.Pretty() {
		return nil, errors.New("Moderator add message doesn't include correct peer ID")
	}
	if data.Type != pb.Message_MODERATOR_ADD {
		return nil, errors.New("Data type is not moderator_add")
	}
	good, err := pubkey.Verify(sd.SerializedData, sd.Signature)
	if err != nil || !good {
		return nil, errors.New("Bad signature")
	}
	err = service.datastore.ModeratedStores().Put(id.Pretty())
	if err != nil {
		return nil, err
	}
	n := notifications.ModeratorAddNotification{notifications.NewID(), "moderatorAdd", id.Pretty()}
	service.broadcast <- n

	service.datastore.Notifications().Put(n.ID, n, n.Type, time.Now())
	log.Debugf("Received MODERATOR_ADD message from %s", id.Pretty())

	return nil, nil
}

func (service *OpenBazaarService) handleModeratorRemove(pid peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	sd := new(pb.SignedData)
	err := ptypes.UnmarshalAny(pmes.Payload, sd)
	if err != nil {
		return nil, err
	}
	pubkey, err := libp2p.UnmarshalPublicKey(sd.SenderPubkey)
	if err != nil {
		return nil, err
	}
	id, err := peer.IDFromPublicKey(pubkey)
	if err != nil {
		return nil, err
	}
	data := new(pb.SignedData_Command)
	err = proto.Unmarshal(sd.SerializedData, data)
	if err != nil {
		return nil, err
	}
	if data.PeerID != service.node.IpfsNode.Identity.Pretty() {
		return nil, errors.New("Moderator remove message doesn't include correct peer ID")
	}
	if data.Type != pb.Message_MODERATOR_REMOVE {
		return nil, errors.New("Data type is not moderator_remove")
	}
	good, err := pubkey.Verify(sd.SerializedData, sd.Signature)
	if err != nil || !good {
		return nil, errors.New("Bad signature")
	}
	err = service.datastore.ModeratedStores().Delete(id.Pretty())
	if err != nil {
		return nil, err
	}
	n := notifications.ModeratorRemoveNotification{notifications.NewID(), "moderatorRemove", id.Pretty()}
	service.broadcast <- n

	service.datastore.Notifications().Put(n.ID, n, n.Type, time.Now())
	log.Debugf("Received MODERATOR_REMOVE message from %s", id.Pretty())

	return nil, nil
}

func (service *OpenBazaarService) handleBlock(pid peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	// If we aren't accepting store requests then ban this peer
	if !service.node.AcceptStoreRequests {
		service.node.BanManager.AddBlockedId(pid)
		return nil, nil
	}

	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	b := new(pb.Block)
	err := ptypes.UnmarshalAny(pmes.Payload, b)
	if err != nil {
		return nil, err
	}
	id, err := cid.Decode(b.Cid)
	if err != nil {
		return nil, err
	}
	block, err := blocks.NewBlockWithCid(b.RawData, id)
	if err != nil {
		return nil, err
	}
	_, err = service.node.IpfsNode.Blocks.AddBlock(block)
	if err != nil {
		return nil, err
	}
	log.Debugf("Received BLOCK message from %s", pid.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleStore(pid peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	// If we aren't accepting store requests then ban this peer
	if !service.node.AcceptStoreRequests {
		service.node.BanManager.AddBlockedId(pid)
		return nil, nil
	}

	errorResponse := func(error string) *pb.Message {
		a := &any.Any{Value: []byte(error)}
		m := &pb.Message{
			MessageType: pb.Message_ERROR,
			Payload:     a,
		}
		return m
	}

	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	cList := new(pb.CidList)
	err := ptypes.UnmarshalAny(pmes.Payload, cList)
	if err != nil {
		return errorResponse("Could not unmarshall message"), err
	}
	var need []string
	for _, id := range cList.Cids {
		decoded, err := cid.Decode(id)
		if err != nil {
			continue
		}
		has, err := service.node.IpfsNode.Blockstore.Has(decoded)
		if err != nil || !has {
			need = append(need, decoded.String())
		}
	}
	log.Debugf("Received STORE message from %s", pid.Pretty())
	log.Debugf("Requesting %d blocks from %s", len(need), pid.Pretty())

	resp := new(pb.CidList)
	resp.Cids = need
	a, err := ptypes.MarshalAny(resp)
	if err != nil {
		return errorResponse("Error marshalling response"), err
	}
	m := &pb.Message{
		MessageType: pb.Message_STORE,
		Payload:     a,
	}
	return m, nil
}

func (service *OpenBazaarService) handleError(peer peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, errors.New("Payload is nil")
	}
	errorMessage := new(pb.Error)
	err := ptypes.UnmarshalAny(pmes.Payload, errorMessage)
	if err != nil {
		return nil, err
	}

	// Load the order
	contract, state, _, _, _, err := service.datastore.Purchases().GetByOrderId(errorMessage.OrderID)
	if err != nil {
		return nil, err
	}

	if state == pb.OrderState_PROCESSING_ERROR {
		return nil, net.DuplicateMessage
	}

	contract.Errors = []string{errorMessage.ErrorMessage}
	service.datastore.Purchases().Put(errorMessage.OrderID, *contract, pb.OrderState_PROCESSING_ERROR, false)

	var thumbnailTiny string
	var thumbnailSmall string
	var vendorHandle string
	var vendorID string
	if len(contract.VendorListings) > 0 && contract.VendorListings[0].Item != nil && len(contract.VendorListings[0].Item.Images) > 0 {
		thumbnailTiny = contract.VendorListings[0].Item.Images[0].Tiny
		thumbnailSmall = contract.VendorListings[0].Item.Images[0].Small
		if contract.VendorListings[0].VendorID != nil {
			vendorID = contract.VendorListings[0].VendorID.PeerID
			vendorHandle = contract.VendorListings[0].VendorID.Handle
		}
	}

	// Send notification to websocket
	n := notifications.ProcessingErrorNotification{notifications.NewID(), "processingError", errorMessage.OrderID, notifications.Thumbnail{thumbnailTiny, thumbnailSmall}, vendorHandle, vendorID}
	service.broadcast <- n
	service.datastore.Notifications().Put(n.ID, n, n.Type, time.Now())
	log.Debugf("Received ERROR message from %s:%s", peer.Pretty(), errorMessage.ErrorMessage)
	return nil, nil
}
