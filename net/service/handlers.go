package service

import (
	"encoding/hex"
	"errors"
	"fmt"
	libp2p "gx/ipfs/QmPGxZ1DP2w45WcogpW1h43BvseXbfke9N91qotpoQcUeS/go-libp2p-crypto"
	peer "gx/ipfs/QmWUswjn261LSyVxWAEpMVtPdy8zmKBJJfBpG3Qdpa8ZsE/go-libp2p-peer"
	"time"

	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
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
	default:
		return nil
	}
}

func (service *OpenBazaarService) handlePing(peer peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received PING message from %s", peer.Pretty())
	return pmes, nil
}

func (service *OpenBazaarService) handleFollow(peer peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received FOLLOW message from %s", peer.Pretty())
	err := service.datastore.Followers().Put(peer.Pretty())
	if err != nil {
		return nil, err
	}
	n := notifications.FollowNotification{peer.Pretty()}
	service.broadcast <- n
	service.datastore.Notifications().Put(n, time.Now())
	return nil, nil
}

func (service *OpenBazaarService) handleUnFollow(peer peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received UNFOLLOW message from %s", peer.Pretty())
	err := service.datastore.Followers().Delete(peer.Pretty())
	if err != nil {
		return nil, err
	}
	n := notifications.UnfollowNotification{peer.Pretty()}
	service.broadcast <- n
	service.datastore.Notifications().Put(n, time.Now())
	return nil, nil
}

func (service *OpenBazaarService) handleOfflineAck(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received OFFLINE_ACK message from %s", p.Pretty())
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
	return nil, nil
}

func (service *OpenBazaarService) handleOfflineRelay(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received OFFLINE_RELAY message from %s", p.Pretty())
	// This acts very similarly to attemptDecrypt&handleMessage in the Offline Message Retreiver
	// However it does not send an ACK, or worry about message ordering

	// Decrypt and unmarshal plaintext
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

	return nil, nil
}

func (service *OpenBazaarService) handleOrder(peer peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received ORDER message from %s", peer.Pretty())
	offline, _ := options.(bool)
	errorResponse := func(error string) *pb.Message {
		a := &any.Any{Value: []byte(error)}
		m := &pb.Message{
			MessageType: pb.Message_ERROR,
			Payload:     a,
		}
		return m
	}
	contract := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, contract)
	if err != nil {
		return errorResponse("Could not unmarshal order"), err
	}

	err = service.node.ValidateOrder(contract)
	if err != nil {
		log.Error(err)
		return errorResponse(err.Error()), nil
	}

	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_ADDRESS_REQUEST {
		total, err := service.node.CalculateOrderTotal(contract)
		if err != nil {
			log.Error("Error calculating payment amount")
			return errorResponse("Error calculating payment amount"), nil
		}
		if !service.node.ValidatePaymentAmount(total, contract.BuyerOrder.Payment.Amount) {
			log.Error("Calculated a different payment amount")
			return errorResponse("Calculated a different payment amount"), nil
		}
		contract, err = service.node.NewOrderConfirmation(contract, true)
		if err != nil {
			log.Error(err)
			return errorResponse("Error building order confirmation"), nil
		}
		a, err := ptypes.MarshalAny(contract)
		if err != nil {
			log.Error(err)
			return errorResponse("Error building order confirmation"), nil
		}
		service.node.Datastore.Sales().Put(contract.VendorOrderConfirmation.OrderID, *contract, pb.OrderState_AWAITING_PAYMENT, false)
		m := pb.Message{
			MessageType: pb.Message_ORDER_CONFIRMATION,
			Payload:     a,
		}
		return &m, nil
	} else if contract.BuyerOrder.Payment.Method == pb.Order_Payment_DIRECT {
		err := service.node.ValidateDirectPaymentAddress(contract.BuyerOrder)
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
		orderId, err := service.node.CalcOrderId(contract.BuyerOrder)
		if err != nil {
			log.Error(err)
			return errorResponse(err.Error()), err
		}
		service.node.Datastore.Sales().Put(orderId, *contract, pb.OrderState_AWAITING_PAYMENT, false)
		return nil, nil
	} else if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED && !offline {
		total, err := service.node.CalculateOrderTotal(contract)
		if err != nil {
			log.Error("Error calculating payment amount")
			return errorResponse("Error calculating payment amount"), nil
		}
		if !service.node.ValidatePaymentAmount(total, contract.BuyerOrder.Payment.Amount) {
			log.Error("Calculated a different payment amount")
			return errorResponse("Calculated a different payment amount"), nil
		}
		err = service.node.ValidateModeratedPaymentAddress(contract.BuyerOrder)
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
		contract, err = service.node.NewOrderConfirmation(contract, false)
		if err != nil {
			log.Error(err)
			return errorResponse("Error building order confirmation"), nil
		}
		a, err := ptypes.MarshalAny(contract)
		if err != nil {
			log.Error(err)
			return errorResponse("Error building order confirmation"), nil
		}
		service.node.Datastore.Sales().Put(contract.VendorOrderConfirmation.OrderID, *contract, pb.OrderState_AWAITING_PAYMENT, false)
		m := pb.Message{
			MessageType: pb.Message_ORDER_CONFIRMATION,
			Payload:     a,
		}
		return &m, nil
	} else if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED && offline {
		err := service.node.ValidateModeratedPaymentAddress(contract.BuyerOrder)
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
		orderId, err := service.node.CalcOrderId(contract.BuyerOrder)
		if err != nil {
			log.Error(err)
			return errorResponse(err.Error()), err
		}
		service.node.Datastore.Sales().Put(orderId, *contract, pb.OrderState_AWAITING_PAYMENT, false)
		return nil, nil
	}
	log.Error("Unrecognized payment type")
	return errorResponse("Unrecognized payment type"), nil
}

func (service *OpenBazaarService) handleOrderConfirmation(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received ORDER_CONFIRMATION message from %s", p.Pretty())

	// Unmarshal payload
	vendorContract := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, vendorContract)
	if err != nil {
		return nil, fmt.Errorf("Could not unmarshal ORDER_CONFIRMATION from %s", p.Pretty())
	}

	// Calc order ID
	orderId := vendorContract.VendorOrderConfirmation.OrderID

	// Load the order
	contract, _, funded, _, _, err := service.datastore.Purchases().GetByOrderId(orderId)
	if err != nil {
		return nil, err
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

	// Send notification to websocket
	n := notifications.OrderConfirmationNotification{orderId}
	service.broadcast <- n
	service.datastore.Notifications().Put(notifications.Wrap(n), time.Now())

	return nil, nil
}

func (service *OpenBazaarService) handleOrderCancel(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received ORDER_CANCEL message from %s", p.Pretty())

	orderId := string(pmes.Payload.Value)

	// Load the order
	contract, _, _, _, _, err := service.datastore.Sales().GetByOrderId(orderId)
	if err != nil {
		return nil, err
	}

	// Set message state to canceled
	service.datastore.Sales().Put(orderId, *contract, pb.OrderState_CANCELED, false)

	return nil, nil
}

func (service *OpenBazaarService) handleReject(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received REJECT message from %s", p.Pretty())
	rejectMsg := new(pb.OrderReject)
	err := ptypes.UnmarshalAny(pmes.Payload, rejectMsg)
	if err != nil {
		return nil, err
	}

	// Load the order
	contract, _, _, records, _, err := service.datastore.Purchases().GetByOrderId(rejectMsg.OrderID)
	if err != nil {
		return nil, err
	}

	if contract.BuyerOrder.Payment.Method != pb.Order_Payment_MODERATED {
		// Sweep the address into our wallet
		var utxos []spvwallet.Utxo
		for _, r := range records {
			if !r.Spent && r.Value > 0 {
				u := spvwallet.Utxo{}
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
		_, err = service.node.Wallet.SweepAddress(utxos, &refundAddress, buyerKey, &redeemScript, spvwallet.NORMAL)
		if err != nil {
			return nil, err
		}
	} else {
		var ins []spvwallet.TransactionInput
		var outValue int64
		for _, r := range records {
			if !r.Spent && r.Value > 0 {
				outpointHash, err := hex.DecodeString(r.Txid)
				if err != nil {
					return nil, err
				}
				outValue += r.Value
				in := spvwallet.TransactionInput{OutpointIndex: r.Index, OutpointHash: outpointHash}
				ins = append(ins, in)
			}
		}

		refundAddress, err := service.node.Wallet.DecodeAddress(contract.BuyerOrder.RefundAddress)
		if err != nil {
			return nil, err
		}
		var output spvwallet.TransactionOutput
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

		buyerSignatures, err := service.node.Wallet.CreateMultisigSignature(ins, []spvwallet.TransactionOutput{output}, buyerKey, redeemScript, contract.BuyerOrder.RefundFee)
		if err != nil {
			return nil, err
		}
		var vendorSignatures []spvwallet.Signature
		for _, s := range rejectMsg.Sigs {
			sig := spvwallet.Signature{InputIndex: s.InputIndex, Signature: s.Signature}
			vendorSignatures = append(vendorSignatures, sig)
		}
		_, err = service.node.Wallet.Multisign(ins, []spvwallet.TransactionOutput{output}, buyerSignatures, vendorSignatures, redeemScript, contract.BuyerOrder.RefundFee, true)
		if err != nil {
			return nil, err
		}
	}

	// Set message state to rejected
	service.datastore.Purchases().Put(rejectMsg.OrderID, *contract, pb.OrderState_DECLINED, false)

	// Send notification to websocket
	n := notifications.OrderCancelNotification{rejectMsg.OrderID}
	service.broadcast <- n
	service.datastore.Notifications().Put(notifications.Wrap(n), time.Now())

	return nil, nil
}

func (service *OpenBazaarService) handleRefund(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received REFUND message from %s", p.Pretty())
	rc := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, rc)
	if err != nil {
		return nil, err
	}

	if err := service.node.VerifySignaturesOnRefund(rc); err != nil {
		return nil, err
	}

	// Load the order
	contract, _, _, records, _, err := service.datastore.Purchases().GetByOrderId(rc.Refund.OrderID)
	if err != nil {
		return nil, err
	}

	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
		var ins []spvwallet.TransactionInput
		var outValue int64
		for _, r := range records {
			if !r.Spent && r.Value > 0 {
				outpointHash, err := hex.DecodeString(r.Txid)
				if err != nil {
					return nil, err
				}
				outValue += r.Value
				in := spvwallet.TransactionInput{OutpointIndex: r.Index, OutpointHash: outpointHash}
				ins = append(ins, in)
			}
		}

		refundAddress, err := service.node.Wallet.DecodeAddress(contract.BuyerOrder.RefundAddress)
		if err != nil {
			return nil, err
		}
		var output spvwallet.TransactionOutput
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

		buyerSignatures, err := service.node.Wallet.CreateMultisigSignature(ins, []spvwallet.TransactionOutput{output}, buyerKey, redeemScript, contract.BuyerOrder.RefundFee)
		if err != nil {
			return nil, err
		}
		var vendorSignatures []spvwallet.Signature
		for _, s := range rc.Refund.Sigs {
			sig := spvwallet.Signature{InputIndex: s.InputIndex, Signature: s.Signature}
			vendorSignatures = append(vendorSignatures, sig)
		}
		_, err = service.node.Wallet.Multisign(ins, []spvwallet.TransactionOutput{output}, buyerSignatures, vendorSignatures, redeemScript, contract.BuyerOrder.RefundFee, true)
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

	// Send notification to websocket
	n := notifications.RefundNotification{contract.Refund.OrderID}
	service.broadcast <- n
	service.datastore.Notifications().Put(notifications.Wrap(n), time.Now())

	return nil, nil
}

func (service *OpenBazaarService) handleOrderFulfillment(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received ORDER_FULFILLMENT message from %s", p.Pretty())

	rc := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, rc)
	if err != nil {
		return nil, err
	}

	// Load the order
	contract, _, _, _, _, err := service.datastore.Purchases().GetByOrderId(rc.VendorOrderFulfillment[0].OrderId)
	if err != nil {
		return nil, err
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

	// Send notification to websocket
	n := notifications.FulfillmentNotification{rc.VendorOrderFulfillment[0].OrderId}
	service.broadcast <- n
	service.datastore.Notifications().Put(notifications.Wrap(n), time.Now())

	return nil, nil
}

func (service *OpenBazaarService) handleOrderCompletion(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received ORDER_COMPLETION message from %s", p.Pretty())

	rc := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, rc)
	if err != nil {
		return nil, err
	}

	// Load the order
	contract, state, _, records, _, err := service.datastore.Sales().GetByOrderId(rc.BuyerOrderCompletion.OrderId)
	if err != nil {
		return nil, err
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
	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED && state != pb.OrderState_RESOLVED {
		var ins []spvwallet.TransactionInput
		var outValue int64
		for _, r := range records {
			if !r.Spent && r.Value > 0 {
				outpointHash, err := hex.DecodeString(r.Txid)
				if err != nil {
					return nil, err
				}
				outValue += r.Value
				in := spvwallet.TransactionInput{OutpointIndex: r.Index, OutpointHash: outpointHash}
				ins = append(ins, in)
			}
		}

		payoutAddress, err := service.node.Wallet.DecodeAddress(contract.VendorOrderFulfillment[0].Payout.PayoutAddress)
		if err != nil {
			return nil, err
		}
		var output spvwallet.TransactionOutput
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

		var vendorSignatures []spvwallet.Signature
		for _, s := range contract.VendorOrderFulfillment[0].Payout.Sigs {
			sig := spvwallet.Signature{InputIndex: s.InputIndex, Signature: s.Signature}
			vendorSignatures = append(vendorSignatures, sig)
		}
		var buyerSignatures []spvwallet.Signature
		for _, s := range contract.BuyerOrderCompletion.PayoutSigs {
			sig := spvwallet.Signature{InputIndex: s.InputIndex, Signature: s.Signature}
			buyerSignatures = append(buyerSignatures, sig)
		}

		_, err = service.node.Wallet.Multisign(ins, []spvwallet.TransactionOutput{output}, buyerSignatures, vendorSignatures, redeemScript, contract.VendorOrderFulfillment[0].Payout.PayoutFeePerByte, true)
		if err != nil {
			return nil, err
		}
	}

	err = service.node.ValidateAndSaveRating(contract)
	if err != nil {
		log.Error(err)
	}

	// Set message state to complete
	service.datastore.Sales().Put(rc.BuyerOrderCompletion.OrderId, *contract, pb.OrderState_COMPLETED, false)

	// Send notification to websocket
	n := notifications.CompletionNotification{rc.BuyerOrderCompletion.OrderId}
	service.broadcast <- n
	service.datastore.Notifications().Put(notifications.Wrap(n), time.Now())

	return nil, nil
}

func (service *OpenBazaarService) handleDisputeOpen(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received DISPUTE_OPEN message from %s", p.Pretty())

	// Unmarshall
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

	return nil, nil
}

func (service *OpenBazaarService) handleDisputeUpdate(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received DISPUTE_UPDATE message from %s", p.Pretty())

	// Make sure we aren't currently processing any disputes before proceeding
	core.DisputeWg.Wait()

	// Unmarshall
	update := new(pb.DisputeUpdate)
	err := ptypes.UnmarshalAny(pmes.Payload, update)
	if err != nil {
		return nil, err
	}
	buyerContract, vendorContract, _, _, _, _, _, err := service.node.Datastore.Cases().GetPayoutDetails(update.OrderId)
	if err != nil {
		return nil, err
	}
	rc := new(pb.RicardianContract)
	err = proto.Unmarshal(update.SerializedContract, rc)
	if err != nil {
		return nil, err
	}
	if buyerContract == nil {
		buyerValidationErrors := service.node.ValidateCaseContract(rc)
		err = service.node.Datastore.Cases().UpdateBuyerInfo(update.OrderId, rc, buyerValidationErrors, update.PayoutAddress, update.Outpoints)
		if err != nil {
			return nil, err
		}
	} else if vendorContract == nil {
		vendorValidationErrors := service.node.ValidateCaseContract(rc)
		err = service.node.Datastore.Cases().UpdateVendorInfo(update.OrderId, rc, vendorValidationErrors, update.PayoutAddress, update.Outpoints)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("All contracts have already been received")
	}
	// Send notification to websocket
	n := notifications.DisputeUpdateNotification{update.OrderId}
	service.broadcast <- n
	service.datastore.Notifications().Put(notifications.Wrap(n), time.Now())

	return nil, nil
}

func (service *OpenBazaarService) handleDisputeClose(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received DISPUTE_CLOSE message from %s", p.Pretty())

	// Unmarshall
	rc := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, rc)
	if err != nil {
		return nil, err
	}

	// Load the order
	isPurchase := false
	var contract *pb.RicardianContract
	contract, _, _, _, _, err = service.datastore.Sales().GetByOrderId(rc.DisputeResolution.OrderId)
	if err != nil {
		contract, _, _, _, _, err = service.datastore.Purchases().GetByOrderId(rc.DisputeResolution.OrderId)
		if err != nil {
			return nil, err
		}
		isPurchase = true
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

	// Save to database
	contract.DisputeResolution = rc.DisputeResolution
	for _, sig := range rc.Signatures {
		if sig.Section == pb.Signature_DISPUTE_RESOLUTION {
			contract.Signatures = append(contract.Signatures, sig)
		}
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

	// Send notification to websocket
	n := notifications.DisputeCloseNotification{rc.DisputeResolution.OrderId}
	service.broadcast <- n
	service.datastore.Notifications().Put(notifications.Wrap(n), time.Now())
	return nil, nil
}

func (service *OpenBazaarService) handleChat(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received CHAT message from %s", p.Pretty())

	// Unmarshall
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
	return nil, nil
}

func (service *OpenBazaarService) handleModeratorAdd(peer peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received MODERATOR_ADD message from %s", peer.Pretty())
	err := service.datastore.ModeratedStores().Put(peer.Pretty())
	if err != nil {
		return nil, err
	}
	n := notifications.ModeratorAddNotification{peer.Pretty()}
	service.broadcast <- n
	service.datastore.Notifications().Put(n, time.Now())
	return nil, nil
}

func (service *OpenBazaarService) handleModeratorRemove(peer peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received MODERATOR_REMOVE message from %s", peer.Pretty())
	err := service.datastore.ModeratedStores().Delete(peer.Pretty())
	if err != nil {
		return nil, err
	}
	n := notifications.ModeratorRemoveNotification{peer.Pretty()}
	service.broadcast <- n
	service.datastore.Notifications().Put(n, time.Now())
	return nil, nil
}
