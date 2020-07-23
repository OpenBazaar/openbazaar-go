package service

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"time"

	libp2p "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	"gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	blocks "gx/ipfs/QmYYLnAzR28nAQ4U5MFniLprnktu6eTFKibeNt96V21EZK/go-block-format"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	ut "github.com/OpenBazaar/openbazaar-go/util"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
)

var (
	// ErrEmptyPayload occurs when an inbound message is provided without the contents
	ErrEmptyPayload = errors.New("message payload is empty")
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
	case pb.Message_VENDOR_FINALIZED_PAYMENT:
		return service.handleVendorFinalizedPayment
	case pb.Message_STORE:
		return service.handleStore
	case pb.Message_ORDER_PAYMENT:
		return service.handleOrderPayment
	case pb.Message_ERROR:
		return service.handleError
	case pb.Message_ORDER_PROCESSING_FAILURE:
		return service.handleOrderProcessingFailure
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
		return nil, ErrEmptyPayload
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
		return nil, errors.New("follow message doesn't include correct peer ID")
	}
	if data.Type != pb.Message_FOLLOW {
		return nil, errors.New("data type is not follow")
	}
	good, err := pubkey.Verify(sd.SerializedData, sd.Signature)
	if err != nil || !good {
		return nil, errors.New("bad signature")
	}

	proof := append(sd.SerializedData, sd.Signature...)
	err = service.datastore.Followers().Put(id.Pretty(), proof)
	if err != nil {
		return nil, err
	}
	n := repo.FollowNotification{
		ID:     repo.NewNotificationID(),
		Type:   repo.NotifierTypeFollowNotification,
		PeerId: id.Pretty(),
	}
	service.broadcast <- n
	err = service.datastore.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		log.Error(err)
	}
	log.Debugf("Received FOLLOW message from %s", id.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleUnFollow(pid peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
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
		return nil, errors.New("unfollow message doesn't include correct peer ID")
	}
	if data.Type != pb.Message_UNFOLLOW {
		return nil, errors.New("data type is not unfollow")
	}
	good, err := pubkey.Verify(sd.SerializedData, sd.Signature)
	if err != nil || !good {
		return nil, errors.New("bad signature")
	}
	err = service.datastore.Followers().Delete(id.Pretty())
	if err != nil {
		return nil, err
	}
	n := repo.UnfollowNotification{
		ID:     repo.NewNotificationID(),
		Type:   repo.NotifierTypeUnfollowNotification,
		PeerId: id.Pretty(),
	}
	service.broadcast <- n
	log.Debugf("Received UNFOLLOW message from %s", id.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleOfflineAck(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, errors.New("decoding OFFLINE_ACK failed: payload is empty")
	}
	pid, err := peer.IDB58Decode(string(pmes.Payload.Value))
	if err != nil {
		return nil, fmt.Errorf("decoding OFFLINE_ACK failed: %s", err.Error())
	}
	pointer, err := service.datastore.Pointers().Get(pid)
	if err != nil {
		return nil, fmt.Errorf("discarding OFFLINE_ACK: no message pending ACK from %s", p.Pretty())
	}
	if pointer.CancelID == nil || pointer.CancelID.Pretty() != p.Pretty() {
		return nil, fmt.Errorf("ignoring OFFLINE_ACK: unauthorized attempt from %s", p.Pretty())
	}
	err = service.datastore.Pointers().Delete(pid)
	if err != nil {
		return nil, err
	}
	log.Debugf("received OFFLINE_ACK: %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleOfflineRelay(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	// This acts very similarly to attemptDecrypt&handleMessage in the Offline Message Retreiver
	// However it does not send an ACK, or worry about message ordering

	// Decrypt and unmarshal plaintext
	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
	}
	plaintext, err := net.Decrypt(service.node.IpfsNode.PrivateKey, pmes.Payload.Value)
	if err != nil {
		log.Errorf("handleOfflineRelayError: %s", err.Error())
		return nil, err
	}

	// Unmarshal plaintext
	env := pb.Envelope{}
	err = proto.Unmarshal(plaintext, &env)
	if err != nil {
		log.Errorf("handleOfflineRelayError: %s", err.Error())
		return nil, err
	}

	// Validate the signature
	ser, err := proto.Marshal(env.Message)
	if err != nil {
		log.Errorf("handleOfflineRelayError: %s", err.Error())
		return nil, err
	}
	pubkey, err := libp2p.UnmarshalPublicKey(env.Pubkey)
	if err != nil {
		log.Errorf("handleOfflineRelayError: %s", err.Error())
		return nil, err
	}
	valid, err := pubkey.Verify(ser, env.Signature)
	if err != nil || !valid {
		log.Errorf("handleOfflineRelayError: signature failed to verify")
		return nil, err
	}

	id, err := peer.IDFromPublicKey(pubkey)
	if err != nil {
		log.Errorf("handleOfflineRelayError: %s", err.Error())
		return nil, err
	}

	err = service.node.IpfsNode.Peerstore.AddPubKey(id, pubkey)
	if err != nil {
		log.Errorf("handleOfflineRelayError: %s", err.Error())
	}
	store := service.node.IpfsNode.Repo.Datastore()
	if err := ipfs.PutCachedPubkey(store, id.Pretty(), env.Pubkey); err != nil {
		log.Errorf("caching pubkey: %s", err.Error())
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

	if offline {
		log.Debugf("handling offline order from: %s", peer.Pretty())
	} else {
		log.Debugf("handling normal order from: %s", peer.Pretty())
	}

	contract := new(pb.RicardianContract)
	var orderId string
	errorResponse := func(errMsg string) *pb.Message {
		e := &pb.Error{
			Code:         0,
			ErrorMessage: errMsg,
			OrderID:      orderId,
		}
		a, err := ptypes.MarshalAny(e)
		m := &pb.Message{
			MessageType: pb.Message_ERROR,
			Payload:     a,
		}
		if err != nil {
			log.Errorf("failed marshaling errorResponse (%s) for order (%s): %s", e.ErrorMessage, orderId, err)
			return m
		}
		if offline {
			contract.Errors = []string{errMsg}
			if err := service.node.Datastore.Sales().Put(orderId, *contract, pb.OrderState_PROCESSING_ERROR, false); err != nil {
				log.Errorf("failed updating PROCESSING_ERROR on sale (%s): %s", orderId, err)
			}
		}
		return m
	}

	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
	}
	err := ptypes.UnmarshalAny(pmes.Payload, contract)
	if err != nil {
		return nil, err
	}

	orderId, err = service.node.CalcOrderID(contract.BuyerOrder)
	if err != nil {
		return errorResponse(err.Error()), err
	}

	err = service.node.Datastore.Messages().Put(
		fmt.Sprintf("%s-%d", orderId, int(pb.Message_ORDER)),
		orderId, pb.Message_ORDER, peer.Pretty(), repo.Message{Msg: *pmes},
		"", time.Now().UnixNano(), []byte(peer))
	if err != nil {
		log.Errorf("failed putting message (%s-%d): %v", orderId, int(pb.Message_ORDER), err)
	}

	pro, err := service.node.GetProfile()
	if err != nil {
		log.Errorf("unable to read local profile: %s", err)
		return errorResponse(fmt.Sprintf("unable to read local profile: %s", err.Error())), err
	}

	if !pro.Vendor {
		log.Debugf("sending message to buyer that our store is not accepting orders")
		return errorResponse("the vendor turned his store off and is not accepting orders at this time"), errors.New("store is turned off")
	}

	err = service.node.ValidateOrder(contract, !offline)
	if err != nil {
		if err != core.ErrPurchaseUnknownListing || !offline {
			return errorResponse(err.Error()), err
		} else {
			log.Debugf("error validating order from %s: %s", err, peer.Pretty())
		}
	}

	order, err := repo.ToV5Order(contract.BuyerOrder, service.node.LookupCurrency)
	if err != nil {
		return nil, err
	}

	wal, err := service.node.Multiwallet.WalletForCurrencyCode(order.Payment.AmountCurrency.Code)
	if err != nil {
		return errorResponse(err.Error()), err
	}
	log.Debugf("incoming order linked to %s wallet", contract.BuyerOrder.Payment.AmountCurrency.Code)

	if order.Payment.Method == pb.Order_Payment_ADDRESS_REQUEST {
		log.Debugf("received direct online order from %s", peer.Pretty())
		total, err := service.node.CalculateOrderTotal(contract)
		if err != nil {
			return errorResponse("Error calculating payment amount"), err
		}
		n, ok := new(big.Int).SetString(order.Payment.BigAmount, 10)
		if !ok {
			return errorResponse("invalid amount"), errors.New("invalid amount")
		}
		if !service.node.ValidatePaymentAmount(total, n) {
			return errorResponse("Calculated a different payment amount"), errors.New("calculated different payment amount")
		}
		contract, err = service.node.NewOrderConfirmation(contract, true)
		if err != nil {
			return errorResponse("Error building order confirmation"), err
		}
		a, err := ptypes.MarshalAny(contract)
		if err != nil {
			return errorResponse("Error building order confirmation"), err
		}
		log.Debugf("storing sales order %s into the database and awaiting payment", contract.VendorOrderConfirmation.OrderID)
		if err := service.node.Datastore.Sales().Put(contract.VendorOrderConfirmation.OrderID, *contract, pb.OrderState_AWAITING_PAYMENT, false); err != nil {
			log.Errorf("failed to put sale (%s): %s", contract.VendorOrderConfirmation.OrderID, err)
			return errorResponse("Error persisting order"), err
		}
		m := pb.Message{
			MessageType: pb.Message_ORDER_CONFIRMATION,
			Payload:     a,
		}
		log.Debugf("sending order confirmation message to %s", peer.Pretty())
		log.Debugf("received addr-req ORDER message from %s", peer.Pretty())
		return &m, nil
	} else if order.Payment.Method == pb.Order_Payment_DIRECT {
		log.Debugf("received offline direct order from %s", peer.Pretty())
		err := service.node.ValidateDirectPaymentAddress(order)
		if err != nil {
			return errorResponse(err.Error()), err
		}
		addr, err := wal.DecodeAddress(ut.NormalizeAddress(order.Payment.Address))
		if err != nil {
			return errorResponse(err.Error()), err
		}
		err = wal.AddWatchedAddresses(addr)
		if err != nil {
			log.Error(err)
		}
		log.Debugf("added address to wallet to watch: %s", addr)
		log.Debugf("storing sales order %s in database", orderId)
		err = service.node.Datastore.Sales().Put(orderId, *contract, pb.OrderState_AWAITING_PAYMENT, false)
		if err != nil {
			log.Error(err)
		}
		log.Debugf("successfully processed direct ORDER message from %s", peer.Pretty())
		return nil, nil
	} else if order.Payment.Method == pb.Order_Payment_MODERATED && !offline {
		log.Debugf("processing moderated online order from %s", peer.Pretty())
		total, err := service.node.CalculateOrderTotal(contract)
		if err != nil {
			return errorResponse("Error calculating payment amount"), errors.New("error calculating payment amount")
		}
		n, ok := new(big.Int).SetString(order.Payment.BigAmount, 10)
		if !ok {
			return errorResponse("invalid amount"), errors.New("invalid amount")
		}
		if !service.node.ValidatePaymentAmount(total, n) {
			return errorResponse("Calculated a different payment amount"), errors.New("calculated different payment amount")
		}
		timeout, err := time.ParseDuration(strconv.Itoa(int(contract.VendorListings[0].Metadata.EscrowTimeoutHours)) + "h")
		if err != nil {
			return errorResponse(err.Error()), err
		}
		err = service.node.ValidateModeratedPaymentAddress(order, timeout)
		if err != nil {
			return errorResponse(err.Error()), err
		}
		addr, err := wal.DecodeAddress(ut.NormalizeAddress(order.Payment.Address))
		if err != nil {
			return errorResponse(err.Error()), err
		}
		err = wal.AddWatchedAddresses(addr)
		if err != nil {
			log.Error(err)
		}
		log.Debugf("added watch address (%s) to wallet (%s)", addr, contract.BuyerOrder.Payment.AmountCurrency.Code)
		contract, err = service.node.NewOrderConfirmation(contract, false)
		if err != nil {
			return errorResponse("Error building order confirmation"), errors.New("error building order confirmation")
		}
		a, err := ptypes.MarshalAny(contract)
		if err != nil {
			return errorResponse("Error building order confirmation"), errors.New("error building order confirmation")
		}
		err = service.node.Datastore.Sales().Put(contract.VendorOrderConfirmation.OrderID, *contract, pb.OrderState_AWAITING_PAYMENT, false)
		if err != nil {
			log.Error(err)
		}
		log.Debugf("storing sales order %s in database", orderId)
		m := pb.Message{
			MessageType: pb.Message_ORDER_CONFIRMATION,
			Payload:     a,
		}
		log.Debugf("Received moderated ORDER message from %s", peer.Pretty())
		return &m, nil
	} else if order.Payment.Method == pb.Order_Payment_MODERATED && offline {
		log.Debugf("processing moderated offline order from %s", peer.Pretty())
		timeout, err := time.ParseDuration(strconv.Itoa(int(contract.VendorListings[0].Metadata.EscrowTimeoutHours)) + "h")
		if err != nil {
			log.Error(err)
			return errorResponse(err.Error()), err
		}
		err = service.node.ValidateModeratedPaymentAddress(order, timeout)
		if err != nil {
			log.Error(err)
			return errorResponse(err.Error()), err
		}
		addr, err := wal.DecodeAddress(order.Payment.Address)
		if err != nil {
			log.Error(err)
			return errorResponse(err.Error()), err
		}
		err = wal.AddWatchedAddresses(addr)
		if err != nil {
			log.Error(err)
		}
		log.Debugf("storing sales order %s in database", orderId)
		err = service.node.Datastore.Sales().Put(orderId, *contract, pb.OrderState_AWAITING_PAYMENT, false)
		if err != nil {
			log.Error(err)
		}
		log.Debugf("successfully processed offline moderated ORDER message from %s", peer.Pretty())
		return nil, nil
	}
	log.Errorf("Unrecognized payment type on order (%s)", contract.VendorOrderConfirmation.OrderID)
	return errorResponse("Unrecognized payment type"), errors.New("unrecognized payment type")
}

func (service *OpenBazaarService) handleOrderConfirmation(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {

	log.Debugf("received order confirmation message from %s", p.Pretty())

	// Unmarshal payload
	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
	}
	vendorContract := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, vendorContract)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal ORDER_CONFIRMATION from %s", p.Pretty())
	}

	if vendorContract.VendorOrderConfirmation == nil {
		return nil, errors.New("received ORDER_CONFIRMATION message with nil confirmation object")
	}

	// Calc order ID
	orderId := vendorContract.VendorOrderConfirmation.OrderID

	err = service.node.Datastore.Messages().Put(
		fmt.Sprintf("%s-%d", orderId, int(pb.Message_ORDER_CONFIRMATION)),
		orderId, pb.Message_ORDER_CONFIRMATION, p.Pretty(), repo.Message{Msg: *pmes},
		"", time.Now().UnixNano(), []byte(p))
	if err != nil {
		log.Errorf("failed putting message (%s-%d): %v", orderId, int(pb.Message_ORDER_CONFIRMATION), err)
	}

	// Load the order
	log.Debugf("loading order %s from the database", orderId)
	contract, state, funded, _, _, _, err := service.datastore.Purchases().GetByOrderId(orderId)
	if err != nil {
		log.Debugf("could not retrieve order %s from the database: %s", orderId, err.Error())
		if err := service.SendProcessingError(p.Pretty(), orderId, pb.Message_ORDER_CONFIRMATION, nil); err != nil {
			log.Errorf("failed sending ORDER_PROCESSING_FAILURE to peer (%s): %s", p.Pretty(), err)
		}
		return nil, net.OutOfOrderMessage
	}

	if funded && state == pb.OrderState_AWAITING_FULFILLMENT || !funded && state == pb.OrderState_AWAITING_PAYMENT {
		log.Debugf("dropping duplicate order confirmation")
		return nil, net.DuplicateMessage
	}

	if state != pb.OrderState_PENDING {
		log.Debugf("order state (%s) is not what is expected", state.String())
		if err := service.SendProcessingError(p.Pretty(), vendorContract.VendorOrderConfirmation.OrderID, pb.Message_ORDER_CONFIRMATION, contract); err != nil {
			log.Errorf("failed sending ORDER_PROCESSING_FAILURE to peer (%s): %s", p.Pretty(), err)
		}
		return nil, net.OutOfOrderMessage
	}

	// Validate the order confirmation
	log.Debugf("validating order confirmation message")
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
		log.Debugf("now awaiting fulfillment for order %s", orderId)
		err := service.datastore.Purchases().Put(orderId, *contract, pb.OrderState_AWAITING_FULFILLMENT, false)
		if err != nil {
			log.Errorf("failed setting order (%) to AWAITING_FULFILLMENT: %s", orderId, err.Error())
		}
	} else {
		// Set message state to AWAITING_PAYMENT
		log.Debugf("order not funded, awaiting payment for order %s", orderId)
		err := service.datastore.Purchases().Put(orderId, *contract, pb.OrderState_AWAITING_PAYMENT, false)
		if err != nil {
			log.Errorf("failed setting order (%) to AWAITING_PAYMENT: %s", orderId, err.Error())
		}
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
	log.Debugf("broadcasting notification to the websocket for order %s", orderId)
	n := repo.OrderConfirmationNotification{
		ID:           repo.NewNotificationID(),
		Type:         repo.NotifierTypeOrderConfirmationNotification,
		OrderId:      orderId,
		Thumbnail:    repo.Thumbnail{Tiny: thumbnailTiny, Small: thumbnailSmall},
		VendorHandle: vendorHandle,
		VendorID:     vendorID,
	}
	service.broadcast <- n
	err = service.datastore.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		log.Error(err)
	}
	log.Debugf("successfully processed ORDER_CONFIRMATION message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleOrderCancel(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
	}
	orderId := string(pmes.Payload.Value)

	// Load the order
	contract, state, _, _, _, _, err := service.datastore.Sales().GetByOrderId(orderId)
	if err != nil {
		if err := service.SendProcessingError(p.Pretty(), orderId, pb.Message_ORDER_CANCEL, nil); err != nil {
			log.Errorf("failed sending ORDER_PROCESSING_FAILURE to peer (%s): %s", p.Pretty(), err)
		}
		return nil, net.OutOfOrderMessage
	}

	err = service.node.Datastore.Messages().Put(
		fmt.Sprintf("%s-%d", orderId, int(pb.Message_ORDER_CANCEL)),
		orderId, pb.Message_ORDER_CANCEL, p.Pretty(), repo.Message{Msg: *pmes},
		"", time.Now().UnixNano(), []byte(p))
	if err != nil {
		log.Errorf("failed putting message (%s-%d): %v", orderId, int(pb.Message_ORDER_CANCEL), err)
	}

	if state == pb.OrderState_CANCELED {
		return nil, net.DuplicateMessage
	}

	// Set message state to canceled
	err = service.datastore.Sales().Put(orderId, *contract, pb.OrderState_CANCELED, false)
	if err != nil {
		log.Error(err)
	}

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
	n := repo.OrderCancelNotification{
		ID:          repo.NewNotificationID(),
		Type:        repo.NotifierTypeOrderCancelNotification,
		OrderId:     orderId,
		Thumbnail:   repo.Thumbnail{Tiny: thumbnailTiny, Small: thumbnailSmall},
		BuyerHandle: buyerHandle,
		BuyerID:     buyerID,
	}
	service.broadcast <- n
	err = service.datastore.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		log.Error(err)
	}
	log.Debugf("Received ORDER_CANCEL message from %s", p.Pretty())

	return nil, nil
}

func (service *OpenBazaarService) handleReject(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
	}
	rejectMsg := new(pb.OrderReject)
	err := ptypes.UnmarshalAny(pmes.Payload, rejectMsg)
	if err != nil {
		return nil, err
	}

	// Load the order
	contract, state, _, records, _, _, err := service.datastore.Purchases().GetByOrderId(rejectMsg.OrderID)
	if err != nil {
		if err := service.SendProcessingError(p.Pretty(), rejectMsg.OrderID, pb.Message_ORDER_REJECT, nil); err != nil {
			log.Errorf("failed sending ORDER_PROCESSING_FAILURE to peer (%s): %s", p.Pretty(), err)
		}
		return nil, net.OutOfOrderMessage
	}

	if state == pb.OrderState_DECLINED {
		return nil, net.DuplicateMessage
	}

	order, err := repo.ToV5Order(contract.BuyerOrder, service.node.LookupCurrency)
	if err != nil {
		return nil, err
	}

	err = service.node.Datastore.Messages().Put(
		fmt.Sprintf("%s-%d", rejectMsg.OrderID, int(pb.Message_ORDER_REJECT)),
		rejectMsg.OrderID, pb.Message_ORDER_REJECT, p.Pretty(), repo.Message{Msg: *pmes},
		"", time.Now().UnixNano(), []byte(p))
	if err != nil {
		log.Errorf("failed putting message (%s-%d): %v", rejectMsg.OrderID, int(pb.Message_ORDER_REJECT), err)
	}

	wal, err := service.node.Multiwallet.WalletForCurrencyCode(order.Payment.AmountCurrency.Code)
	if err != nil {
		return nil, err
	}

	if order.Payment.Method != pb.Order_Payment_MODERATED {
		escrowWallet, ok := wal.(wallet.EscrowWallet)
		if !ok {
			return nil, errors.New("wallet does not support escrow")
		}
		// Sweep the address into our wallet
		var txInputs []wallet.TransactionInput
		for _, r := range records {
			if !r.Spent && r.Value.Cmp(big.NewInt(0)) > 0 {
				hash, err := hex.DecodeString(ut.NormalizeAddress(r.Txid))
				if err != nil {
					return nil, err
				}
				addr, err := wal.DecodeAddress(ut.NormalizeAddress(r.Address))
				if err != nil {
					return nil, err
				}
				u := wallet.TransactionInput{
					OutpointHash:  hash,
					OutpointIndex: r.Index,
					LinkedAddress: addr,
					Value:         r.Value,
				}

				txInputs = append(txInputs, u)
			}
		}

		chaincode, err := hex.DecodeString(order.Payment.Chaincode)
		if err != nil {
			return nil, err
		}
		mECKey, err := service.node.MasterPrivateKey.ECPrivKey()
		if err != nil {
			return nil, err
		}
		buyerKey, err := wal.ChildKey(mECKey.Serialize(), chaincode, true)
		if err != nil {
			return nil, err
		}
		redeemScript, err := hex.DecodeString(order.Payment.RedeemScript)
		if err != nil {
			return nil, err
		}
		refundAddress, err := wal.DecodeAddress(ut.NormalizeAddress(order.RefundAddress))
		if err != nil {
			return nil, err
		}
		_, err = escrowWallet.SweepAddress(txInputs, &refundAddress, buyerKey, &redeemScript, wallet.NORMAL)
		if err != nil {
			return nil, err
		}
	} else {
		escrowWallet, ok := wal.(wallet.EscrowWallet)
		if !ok {
			return nil, errors.New("wallet does not support escrow")
		}
		var ins []wallet.TransactionInput
		outValue := big.NewInt(0)
		for _, r := range records {
			if !r.Spent && r.Value.Cmp(big.NewInt(0)) > 0 {
				outpointHash, err := hex.DecodeString(ut.NormalizeAddress(r.Txid))
				if err != nil {
					return nil, err
				}
				outValue = new(big.Int).Add(outValue, &r.Value)
				in := wallet.TransactionInput{OutpointIndex: r.Index, OutpointHash: outpointHash, Value: r.Value}
				ins = append(ins, in)
			}
		}

		refundAddress, err := wal.DecodeAddress(ut.NormalizeAddress(order.RefundAddress))
		if err != nil {
			return nil, err
		}
		var output = wallet.TransactionOutput{
			Address: refundAddress,
			Value:   *outValue,
		}

		chaincode, err := hex.DecodeString(order.Payment.Chaincode)
		if err != nil {
			return nil, err
		}
		mECKey, err := service.node.MasterPrivateKey.ECPrivKey()
		if err != nil {
			return nil, err
		}
		buyerKey, err := wal.ChildKey(mECKey.Serialize(), chaincode, true)
		if err != nil {
			return nil, err
		}
		redeemScript, err := hex.DecodeString(order.Payment.RedeemScript)
		if err != nil {
			return nil, err
		}
		fee, ok := new(big.Int).SetString(order.BigRefundFee, 10)
		if !ok {
			return nil, errors.New("invalid amount")
		}
		buyerSignatures, err := escrowWallet.CreateMultisigSignature(ins, []wallet.TransactionOutput{output}, buyerKey, redeemScript, *fee)
		if err != nil {
			return nil, err
		}
		var vendorSignatures []wallet.Signature
		for _, s := range rejectMsg.Sigs {
			sig := wallet.Signature{InputIndex: s.InputIndex, Signature: s.Signature}
			vendorSignatures = append(vendorSignatures, sig)
		}
		_, err = escrowWallet.Multisign(ins, []wallet.TransactionOutput{output}, buyerSignatures, vendorSignatures, redeemScript, *fee, true)
		if err != nil {
			return nil, err
		}
	}

	// Set message state to rejected
	err = service.datastore.Purchases().Put(rejectMsg.OrderID, *contract, pb.OrderState_DECLINED, false)
	if err != nil {
		log.Error(err)
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
	n := repo.OrderDeclinedNotification{
		ID:           repo.NewNotificationID(),
		Type:         repo.NotifierTypeOrderDeclinedNotification,
		OrderId:      rejectMsg.OrderID,
		Thumbnail:    repo.Thumbnail{Tiny: thumbnailTiny, Small: thumbnailSmall},
		VendorHandle: vendorHandle,
		VendorID:     vendorID,
	}
	service.broadcast <- n

	err = service.datastore.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		log.Error(err)
	}
	log.Debugf("Received REJECT message from %s", p.Pretty())

	return nil, nil
}

func (service *OpenBazaarService) handleRefund(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
	}
	rc := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, rc)
	if err != nil {
		return nil, err
	}

	if rc.Refund == nil {
		return nil, errors.New("received REFUND message with nil refund object")
	}

	if err := service.node.VerifySignaturesOnRefund(rc); err != nil {
		return nil, err
	}

	// Load the order
	contract, state, _, records, _, _, err := service.datastore.Purchases().GetByOrderId(rc.Refund.OrderID)
	if err != nil {
		if err := service.SendProcessingError(p.Pretty(), rc.Refund.OrderID, pb.Message_REFUND, nil); err != nil {
			log.Errorf("failed sending ORDER_PROCESSING_FAILURE to peer (%s): %s", p.Pretty(), err)
		}
		return nil, net.OutOfOrderMessage
	}

	err = service.node.Datastore.Messages().Put(
		fmt.Sprintf("%s-%d", rc.Refund.OrderID, int(pb.Message_REFUND)),
		rc.Refund.OrderID, pb.Message_REFUND, p.Pretty(), repo.Message{Msg: *pmes},
		"", time.Now().UnixNano(), []byte(p))
	if err != nil {
		log.Errorf("failed putting message (%s-%d): %v", rc.Refund.OrderID, int(pb.Message_REFUND), err)
	}

	if !(state == pb.OrderState_PARTIALLY_FULFILLED || state == pb.OrderState_AWAITING_FULFILLMENT) {
		return nil, net.DuplicateMessage
	}

	order, err := repo.ToV5Order(contract.BuyerOrder, service.node.LookupCurrency)
	if err != nil {
		return nil, err
	}

	wal, err := service.node.Multiwallet.WalletForCurrencyCode(order.Payment.AmountCurrency.Code)
	if err != nil {
		return nil, err
	}

	if order.Payment.Method == pb.Order_Payment_MODERATED {
		escrowWallet, ok := wal.(wallet.EscrowWallet)
		if !ok {
			return nil, errors.New("wallet does not support escrow")
		}
		var ins []wallet.TransactionInput
		outValue := big.NewInt(0)
		for _, r := range records {
			if !r.Spent && r.Value.Cmp(big.NewInt(0)) > 0 {
				outpointHash, err := hex.DecodeString(ut.NormalizeAddress(r.Txid))
				if err != nil {
					return nil, err
				}
				outValue = new(big.Int).Add(outValue, &r.Value)
				in := wallet.TransactionInput{OutpointIndex: r.Index, OutpointHash: outpointHash, Value: r.Value}
				ins = append(ins, in)
			}
		}

		refundAddress, err := wal.DecodeAddress(ut.NormalizeAddress(order.RefundAddress))
		if err != nil {
			return nil, err
		}
		var output = wallet.TransactionOutput{
			Address: refundAddress,
			Value:   *outValue,
		}

		chaincode, err := hex.DecodeString(order.Payment.Chaincode)
		if err != nil {
			return nil, err
		}
		mECKey, err := service.node.MasterPrivateKey.ECPrivKey()
		if err != nil {
			return nil, err
		}
		buyerKey, err := wal.ChildKey(mECKey.Serialize(), chaincode, true)
		if err != nil {
			return nil, err
		}
		redeemScript, err := hex.DecodeString(order.Payment.RedeemScript)
		if err != nil {
			return nil, err
		}
		fee, ok := new(big.Int).SetString(order.BigRefundFee, 10)
		if !ok {
			return nil, errors.New("invalid amount")
		}
		buyerSignatures, err := escrowWallet.CreateMultisigSignature(ins, []wallet.TransactionOutput{output}, buyerKey, redeemScript, *fee)
		if err != nil {
			return nil, err
		}
		var vendorSignatures []wallet.Signature
		for _, s := range rc.Refund.Sigs {
			sig := wallet.Signature{InputIndex: s.InputIndex, Signature: s.Signature}
			vendorSignatures = append(vendorSignatures, sig)
		}
		_, err = escrowWallet.Multisign(ins, []wallet.TransactionOutput{output}, buyerSignatures, vendorSignatures, redeemScript, *fee, true)
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
	err = service.datastore.Purchases().Put(contract.Refund.OrderID, *contract, pb.OrderState_REFUNDED, false)
	if err != nil {
		log.Error(err)
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
	n := repo.RefundNotification{
		ID:           repo.NewNotificationID(),
		Type:         repo.NotifierTypeRefundNotification,
		OrderId:      contract.Refund.OrderID,
		Thumbnail:    repo.Thumbnail{Tiny: thumbnailTiny, Small: thumbnailSmall},
		VendorHandle: vendorHandle,
		VendorID:     vendorID,
	}
	service.broadcast <- n
	err = service.datastore.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		log.Error(err)
	}
	log.Debugf("Received REFUND message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleOrderFulfillment(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {

	log.Debugf("received order fulfillment message from %s", p.Pretty())

	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
	}
	rc := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, rc)
	if err != nil {
		return nil, err
	}

	if len(rc.VendorOrderFulfillment) == 0 {
		return nil, errors.New("received FULFILLMENT message with no VendorOrderFulfillment objects")
	}

	err = service.node.Datastore.Messages().Put(
		fmt.Sprintf("%s-%d", rc.VendorOrderFulfillment[0].OrderId, int(pb.Message_ORDER_FULFILLMENT)),
		rc.VendorOrderFulfillment[0].OrderId, pb.Message_ORDER_FULFILLMENT, p.Pretty(), repo.Message{Msg: *pmes},
		"", time.Now().UnixNano(), []byte(p))
	if err != nil {
		log.Errorf("failed putting message (%s-%d): %v", rc.VendorOrderFulfillment[0].OrderId, int(pb.Message_ORDER_FULFILLMENT), err)
	}

	// Load the order
	contract, state, _, _, _, _, err := service.datastore.Purchases().GetByOrderId(rc.VendorOrderFulfillment[0].OrderId)
	if err != nil {
		log.Debugf("unable to find a matching order in the purchases table of the database")
		if err := service.SendProcessingError(p.Pretty(), rc.VendorOrderFulfillment[0].OrderId, pb.Message_ORDER_FULFILLMENT, nil); err != nil {
			log.Errorf("failed sending ORDER_PROCESSING_FAILURE to peer (%s): %s", p.Pretty(), err)
		}
		return nil, net.OutOfOrderMessage
	}

	if state == pb.OrderState_PENDING || state == pb.OrderState_AWAITING_PAYMENT {
		log.Debugf("order state (%s) is not what is expected", state.String())
		if err := service.SendProcessingError(p.Pretty(), rc.VendorOrderFulfillment[0].OrderId, pb.Message_ORDER_FULFILLMENT, contract); err != nil {
			log.Errorf("failed sending ORDER_PROCESSING_FAILURE to peer (%s): %s", p.Pretty(), err)
		}
		return nil, net.OutOfOrderMessage
	}

	if !(state == pb.OrderState_PARTIALLY_FULFILLED || state == pb.OrderState_AWAITING_FULFILLMENT) {
		log.Debugf("order state (%s) is not what is expected", state.String())
		return nil, net.DuplicateMessage
	}

	contract.VendorOrderFulfillment = append(contract.VendorOrderFulfillment, rc.VendorOrderFulfillment[0])
	for _, sig := range rc.Signatures {
		if sig.Section == pb.Signature_ORDER_FULFILLMENT {
			contract.Signatures = append(contract.Signatures, sig)
		}
	}

	log.Debugf("validating order fulfillment message")
	if err := service.node.ValidateOrderFulfillment(rc.VendorOrderFulfillment[0], contract); err != nil {
		return nil, err
	}

	// Set message state to fulfilled if all listings have a matching fulfillment message
	if service.node.IsFulfilled(contract) {
		log.Debugf("updating order %s in the database to fulfilled", rc.VendorOrderFulfillment[0].OrderId)
		err = service.datastore.Purchases().Put(rc.VendorOrderFulfillment[0].OrderId, *contract, pb.OrderState_FULFILLED, false)
		if err != nil {
			log.Error(err)
		}
	} else {
		log.Debugf("updating order %s in the database to partially fulfilled", rc.VendorOrderFulfillment[0].OrderId)
		err = service.datastore.Purchases().Put(rc.VendorOrderFulfillment[0].OrderId, *contract, pb.OrderState_PARTIALLY_FULFILLED, false)
		if err != nil {
			log.Error(err)
		}
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
	n := repo.FulfillmentNotification{
		ID:           repo.NewNotificationID(),
		Type:         repo.NotifierTypeFulfillmentNotification,
		OrderId:      rc.VendorOrderFulfillment[0].OrderId,
		Thumbnail:    repo.Thumbnail{Tiny: thumbnailTiny, Small: thumbnailSmall},
		VendorHandle: vendorHandle,
		VendorID:     vendorID,
	}
	service.broadcast <- n
	err = service.datastore.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		log.Error(err)
	}
	log.Debugf("successfully processed ORDER_FULFILLMENT message from %s", p.Pretty())

	return nil, nil
}

func (service *OpenBazaarService) handleOrderCompletion(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {

	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
	}
	rc := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, rc)
	if err != nil {
		return nil, err
	}

	if rc.BuyerOrderCompletion == nil {
		return nil, errors.New("received ORDER_COMPLETION with nil BuyerOrderCompletion object")
	}

	err = service.node.Datastore.Messages().Put(
		fmt.Sprintf("%s-%d", rc.BuyerOrderCompletion.OrderId, int(pb.Message_ORDER_COMPLETION)),
		rc.BuyerOrderCompletion.OrderId, pb.Message_ORDER_COMPLETION, p.Pretty(), repo.Message{Msg: *pmes},
		"", time.Now().UnixNano(), []byte(p))
	if err != nil {
		log.Errorf("failed putting message (%s-%d): %v", rc.BuyerOrderCompletion.OrderId, int(pb.Message_ORDER_COMPLETION), err)
	}

	// Load the order
	contract, state, _, records, _, _, err := service.datastore.Sales().GetByOrderId(rc.BuyerOrderCompletion.OrderId)
	if err != nil {
		if err := service.SendProcessingError(p.Pretty(), rc.BuyerOrderCompletion.OrderId, pb.Message_ORDER_COMPLETION, nil); err != nil {
			log.Errorf("failed sending ORDER_PROCESSING_FAILURE to peer (%s): %s", p.Pretty(), err)
		}
		return nil, net.OutOfOrderMessage
	}

	if state == pb.OrderState_COMPLETED {
		return nil, net.DuplicateMessage
	}

	order, err := repo.ToV5Order(contract.BuyerOrder, service.node.LookupCurrency)
	if err != nil {
		return nil, err
	}

	wal, err := service.node.Multiwallet.WalletForCurrencyCode(order.Payment.AmountCurrency.Code)
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
	if order.Payment.Method == pb.Order_Payment_MODERATED && state != pb.OrderState_DISPUTED && state != pb.OrderState_DECIDED && state != pb.OrderState_RESOLVED && state != pb.OrderState_PAYMENT_FINALIZED {
		escrowWallet, ok := wal.(wallet.EscrowWallet)
		if !ok {
			return nil, errors.New("wallet does not support escrow")
		}
		var ins []wallet.TransactionInput
		outValue := big.NewInt(0)
		for _, r := range records {
			if !r.Spent && r.Value.Cmp(big.NewInt(0)) > 0 {
				outpointHash, err := hex.DecodeString(ut.NormalizeAddress(r.Txid))
				if err != nil {
					return nil, err
				}
				outValue = new(big.Int).Add(outValue, &r.Value)
				in := wallet.TransactionInput{
					OutpointIndex: r.Index,
					OutpointHash:  outpointHash,
					Value:         r.Value,
				}
				ins = append(ins, in)
			}
		}
		var payoutAddress btcutil.Address
		if len(contract.VendorOrderFulfillment) > 0 {
			payoutAddress, err = wal.DecodeAddress(
				ut.NormalizeAddress(contract.VendorOrderFulfillment[0].Payout.PayoutAddress))
			if err != nil {
				return nil, err
			}
		} else {
			payoutAddress = wal.CurrentAddress(wallet.EXTERNAL)
		}
		var output = wallet.TransactionOutput{
			Address: payoutAddress,
			Value:   *outValue,
		}

		redeemScript, err := hex.DecodeString(order.Payment.RedeemScript)
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
		fulfillment := repo.ToV5OrderFulfillment(contract.VendorOrderFulfillment[0])
		payoutFee, ok := new(big.Int).SetString(fulfillment.Payout.BigPayoutFeePerByte, 10)
		if !ok {
			return nil, errors.New("invalid amount")
		}
		_, err = escrowWallet.Multisign(ins, []wallet.TransactionOutput{output}, buyerSignatures, vendorSignatures, redeemScript, *payoutFee, true)
		if err != nil {
			if err.Error() == "ERROR_INSUFFICIENT_FUNDS" {
				err0 := service.node.Datastore.Messages().Put(
					fmt.Sprintf("%s-%d", rc.BuyerOrderCompletion.OrderId, int(pb.Message_ORDER_COMPLETION)),
					rc.BuyerOrderCompletion.OrderId, pb.Message_ORDER_COMPLETION, p.Pretty(), repo.Message{Msg: *pmes},
					err.Error(), time.Now().UnixNano(), []byte(p))
				if err0 != nil {
					log.Errorf("failed putting message (%s-%d): %v", rc.BuyerOrderCompletion.OrderId, int(pb.Message_ORDER_COMPLETION), err0)
				}
			}

			return nil, err
		}
	}

	err = service.node.ValidateAndSaveRating(contract)
	if err != nil {
		log.Error("error validating rating:", err)
	}

	// Set message state to complete
	err = service.datastore.Sales().Put(rc.BuyerOrderCompletion.OrderId, *contract, pb.OrderState_COMPLETED, false)
	if err != nil {
		log.Error(err)
	}

	var thumbnailTiny string
	var thumbnailSmall string
	var buyerID string
	var buyerHandle string
	if len(contract.VendorListings) > 0 && contract.VendorListings[0].Item != nil && len(contract.VendorListings[0].Item.Images) > 0 {
		thumbnailTiny = contract.VendorListings[0].Item.Images[0].Tiny
		thumbnailSmall = contract.VendorListings[0].Item.Images[0].Small
		if order.BuyerID != nil {
			buyerID = order.BuyerID.PeerID
			buyerHandle = order.BuyerID.Handle
		}
	}

	// Send notification to websocket
	n := repo.CompletionNotification{
		ID:          repo.NewNotificationID(),
		Type:        repo.NotifierTypeCompletionNotification,
		OrderId:     rc.BuyerOrderCompletion.OrderId,
		Thumbnail:   repo.Thumbnail{Tiny: thumbnailTiny, Small: thumbnailSmall},
		BuyerHandle: buyerHandle,
		BuyerID:     buyerID,
	}
	service.broadcast <- n
	err = service.datastore.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		log.Error(err)
	}
	log.Debugf("received ORDER_COMPLETION message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleDisputeOpen(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {

	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
	}
	rc := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, rc)
	if err != nil {
		return nil, err
	}

	orderID, err := service.node.CalcOrderID(rc.BuyerOrder)
	if err != nil {
		log.Errorf("failed calc orderID")
	}

	if orderID != "" {
		err = service.node.Datastore.Messages().Put(
			fmt.Sprintf("%s-%d", orderID, int(pb.Message_DISPUTE_OPEN)),
			orderID, pb.Message_DISPUTE_OPEN, p.Pretty(), repo.Message{Msg: *pmes}, "", time.Now().UnixNano(), []byte(p))
		if err != nil {
			log.Errorf("failed putting message (%s-%d): %v", orderID, int(pb.Message_DISPUTE_OPEN), err)
		}
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
	log.Debugf("received DISPUTE_OPEN message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleDisputeUpdate(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {

	// Make sure we aren't currently processing any disputes before proceeding
	core.DisputeWg.Wait()

	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
	}
	update := new(pb.DisputeUpdate)
	err := ptypes.UnmarshalAny(pmes.Payload, update)
	if err != nil {
		return nil, err
	}

	orderID := update.OrderId
	if orderID != "" {
		err = service.node.Datastore.Messages().Put(
			fmt.Sprintf("%s-%d", orderID, int(pb.Message_DISPUTE_UPDATE)),
			orderID, pb.Message_DISPUTE_UPDATE, p.Pretty(), repo.Message{Msg: *pmes}, "", time.Now().UnixNano(), []byte(p))
		if err != nil {
			log.Errorf("failed putting message (%s-%d): %v", orderID, int(pb.Message_DISPUTE_UPDATE), err)
		}
	}

	dispute, err := service.node.Datastore.Cases().GetByCaseID(update.OrderId)
	if err != nil {
		if err := service.SendProcessingError(p.Pretty(), update.OrderId, pb.Message_DISPUTE_UPDATE, nil); err != nil {
			log.Errorf("failed sending ORDER_PROCESSING_FAILURE to peer (%s): %s", p.Pretty(), err)
		}
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
	if dispute.BuyerContract == nil {
		buyerValidationErrors := service.node.ValidateCaseContract(rc)
		err = service.node.Datastore.Cases().UpdateBuyerInfo(update.OrderId, rc, buyerValidationErrors, update.PayoutAddress, update.Outpoints)
		if err != nil {
			return nil, err
		}
		if len(dispute.VendorContract.VendorListings) > 0 && dispute.VendorContract.VendorListings[0].Item != nil && len(dispute.VendorContract.VendorListings[0].Item.Images) > 0 {
			thumbnailTiny = dispute.VendorContract.VendorListings[0].Item.Images[0].Tiny
			thumbnailSmall = dispute.VendorContract.VendorListings[0].Item.Images[0].Small
			if dispute.VendorContract.VendorListings[0].VendorID != nil {
				disputerID = dispute.VendorContract.VendorListings[0].VendorID.PeerID
				disputerHandle = dispute.VendorContract.VendorListings[0].VendorID.Handle
			}
			if dispute.VendorContract.BuyerOrder.BuyerID != nil {
				buyer = dispute.VendorContract.BuyerOrder.BuyerID.PeerID
				disputeeID = dispute.VendorContract.BuyerOrder.BuyerID.PeerID
				disputeeHandle = dispute.VendorContract.BuyerOrder.BuyerID.Handle
			}
		}
	} else if dispute.VendorContract == nil {
		vendorValidationErrors := service.node.ValidateCaseContract(rc)
		err = service.node.Datastore.Cases().UpdateVendorInfo(update.OrderId, rc, vendorValidationErrors, update.PayoutAddress, update.Outpoints)
		if err != nil {
			return nil, err
		}
		if len(dispute.BuyerContract.VendorListings) > 0 && dispute.BuyerContract.VendorListings[0].Item != nil && len(dispute.BuyerContract.VendorListings[0].Item.Images) > 0 {
			thumbnailTiny = dispute.BuyerContract.VendorListings[0].Item.Images[0].Tiny
			thumbnailSmall = dispute.BuyerContract.VendorListings[0].Item.Images[0].Small
			if dispute.BuyerContract.VendorListings[0].VendorID != nil {
				disputeeID = dispute.BuyerContract.VendorListings[0].VendorID.PeerID
				disputeeHandle = dispute.BuyerContract.VendorListings[0].VendorID.Handle
			}
			if dispute.BuyerContract.BuyerOrder.BuyerID != nil {
				buyer = dispute.BuyerContract.BuyerOrder.BuyerID.PeerID
				disputerID = dispute.BuyerContract.BuyerOrder.BuyerID.PeerID
				disputerHandle = dispute.BuyerContract.BuyerOrder.BuyerID.Handle
			}
		}
	} else {
		return nil, errors.New("all contracts have already been received")
	}

	// Send notification to websocket
	n := repo.DisputeUpdateNotification{
		ID:             repo.NewNotificationID(),
		Type:           repo.NotifierTypeDisputeUpdateNotification,
		OrderId:        update.OrderId,
		Thumbnail:      repo.Thumbnail{Tiny: thumbnailTiny, Small: thumbnailSmall},
		DisputerID:     disputerID,
		DisputerHandle: disputerHandle,
		DisputeeID:     disputeeID,
		DisputeeHandle: disputeeHandle,
		Buyer:          buyer,
	}
	service.broadcast <- n

	err = service.datastore.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		log.Error(err)
	}
	log.Debugf("received DISPUTE_UPDATE message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleDisputeClose(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {

	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
	}
	rc := new(pb.RicardianContract)
	err := ptypes.UnmarshalAny(pmes.Payload, rc)
	if err != nil {
		return nil, err
	}

	orderID := rc.DisputeResolution.OrderId
	if orderID != "" {
		err = service.node.Datastore.Messages().Put(
			fmt.Sprintf("%s-%d", orderID, int(pb.Message_DISPUTE_CLOSE)),
			orderID, pb.Message_DISPUTE_CLOSE, p.Pretty(), repo.Message{Msg: *pmes}, "", time.Now().UnixNano(), []byte(p))
		if err != nil {
			log.Errorf("failed putting message (%s-%d): %v", orderID, int(pb.Message_DISPUTE_CLOSE), err)
		}
	}

	// Load the order
	isPurchase := false
	var contract *pb.RicardianContract
	var state pb.OrderState
	var otherPartyID string
	var otherPartyHandle string
	var buyer string
	contract, state, _, _, _, _, err = service.datastore.Sales().GetByOrderId(rc.DisputeResolution.OrderId)
	if err != nil {
		contract, state, _, _, _, _, err = service.datastore.Purchases().GetByOrderId(rc.DisputeResolution.OrderId)
		if err != nil {
			if err := service.SendProcessingError(p.Pretty(), rc.DisputeResolution.OrderId, pb.Message_DISPUTE_CLOSE, nil); err != nil {
				log.Errorf("failed sending ORDER_PROCESSING_FAILURE to peer (%s): %s", p.Pretty(), err)
			}
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
		if err := service.SendProcessingError(p.Pretty(), rc.DisputeResolution.OrderId, pb.Message_DISPUTE_CLOSE, contract); err != nil {
			log.Errorf("failed sending ORDER_PROCESSING_FAILURE to peer (%s): %s", p.Pretty(), err)
		}
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

	// If DisputeAcceptance is already set then move the state directly to RESOLVED
	if isPurchase {
		if contract.DisputeAcceptance != nil {
			err = service.datastore.Purchases().Put(rc.DisputeResolution.OrderId, *contract, pb.OrderState_RESOLVED, false)
		} else {
			err = service.datastore.Purchases().Put(rc.DisputeResolution.OrderId, *contract, pb.OrderState_DECIDED, false)
		}
	} else {
		if contract.DisputeAcceptance != nil {
			err = service.datastore.Sales().Put(rc.DisputeResolution.OrderId, *contract, pb.OrderState_RESOLVED, false)
		} else {
			err = service.datastore.Sales().Put(rc.DisputeResolution.OrderId, *contract, pb.OrderState_DECIDED, false)
		}
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
	n := repo.DisputeCloseNotification{
		ID:               repo.NewNotificationID(),
		Type:             repo.NotifierTypeDisputeCloseNotification,
		OrderId:          rc.DisputeResolution.OrderId,
		Thumbnail:        repo.Thumbnail{Tiny: thumbnailTiny, Small: thumbnailSmall},
		OtherPartyID:     otherPartyID,
		OtherPartyHandle: otherPartyHandle,
		Buyer:            buyer,
	}
	service.broadcast <- n

	err = service.datastore.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		log.Error(err)
	}
	log.Debugf("received DISPUTE_CLOSE message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleOrderProcessingFailure(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
	}
	procFailure := new(pb.OrderProcessingFailure)
	if err := ptypes.UnmarshalAny(pmes.Payload, procFailure); err != nil {
		return nil, err
	}

	var (
		localContract *pb.RicardianContract
	)
	if contract, _, _, _, _, _, err := service.node.Datastore.Sales().GetByOrderId(procFailure.OrderID); err != nil {
		localContract = contract
	} else if contract, _, _, _, _, _, err := service.node.Datastore.Purchases().GetByOrderId(procFailure.OrderID); err != nil {
		localContract = contract
	}
	if localContract == nil {
		return nil, fmt.Errorf("no contract found for order ID (%s)", procFailure.OrderID)
	}
	// TODO: Validate we aren't in a loop, exit if we are
	missingMessageTypes, err := analyzeForMissingMessages(localContract, procFailure)
	if err != nil {
		return nil, err
	}
	if missingMessageTypes == nil {
		err := fmt.Errorf("unable to determine missing message types for order ID (%s)", procFailure.OrderID)
		log.Error(err.Error())
		return nil, err
	}
	for _, msgType := range missingMessageTypes {
		log.Debugf("resending missing ORDER message (%s) to peer (%s)", msgType.String(), p.Pretty())
		if err := service.node.ResendCachedOrderMessage(procFailure.OrderID, msgType); err != nil {
			err := fmt.Errorf("resending message type (%s) for order (%s): %s", msgType.String(), procFailure.OrderID, err.Error())
			log.Error(err.Error())
			// TODO: Can we attempt to recreate MessageType?
			return nil, err
		}
	}
	return nil, nil
}

// analyzeForMissingMessages compares the local RicardianContract with the one provided with the
// error message to determine which messages might be missing and need to be resent to the remote
// peer
func analyzeForMissingMessages(lc *pb.RicardianContract, e *pb.OrderProcessingFailure) ([]pb.Message_MessageType, error) {
	var msgsToResend = []pb.Message_MessageType{}
	if lc == nil {
		return nil, fmt.Errorf("no local contract for order ID (%s) to compare to, unable to recover missing messages", e.OrderID)
	}
	if lc.BuyerOrder != nil && (e.Contract == nil || e.Contract.BuyerOrder == nil) {
		msgsToResend = append(msgsToResend, pb.Message_ORDER)
	}
	if lc.VendorOrderConfirmation != nil && (e.Contract == nil || e.Contract.VendorOrderConfirmation == nil) {
		msgsToResend = append(msgsToResend, pb.Message_ORDER_CONFIRMATION)
	}
	// TODO: How do we detect ORDER_REJECT diff?
	// TODO: How do we detect ORDER_CANCEL diff?
	if len(lc.VendorOrderFulfillment) != 0 && (e.Contract == nil || len(e.Contract.VendorOrderFulfillment) == 0) {
		msgsToResend = append(msgsToResend, pb.Message_ORDER_FULFILLMENT)
	}
	if lc.Dispute != nil && (e.Contract == nil || e.Contract.Dispute == nil) {
		msgsToResend = append(msgsToResend, pb.Message_DISPUTE_OPEN)
		msgsToResend = append(msgsToResend, pb.Message_DISPUTE_UPDATE)
	}
	if lc.DisputeResolution != nil && (e.Contract == nil || e.Contract.DisputeResolution == nil) {
		// TODO: Re-broadcast error to moderator so they can resend DISPUTE_CLOSE
		msgsToResend = append(msgsToResend, pb.Message_DISPUTE_CLOSE)
	}
	if lc.DisputeAcceptance != nil && (e.Contract == nil || e.Contract.DisputeAcceptance == nil) {
		// TODO: This msg occurs after order is PENDING, FULFILLED, and DISPUTED states, is
		// there a better check for resending FINALIZED_PAYMENT?
		msgsToResend = append(msgsToResend, pb.Message_VENDOR_FINALIZED_PAYMENT)
	}
	if lc.Refund != nil && (e.Contract == nil || e.Contract.Refund == nil) {
		msgsToResend = append(msgsToResend, pb.Message_REFUND)
	}
	if lc.BuyerOrderCompletion != nil && (e.Contract == nil || e.Contract.BuyerOrderCompletion == nil) {
		msgsToResend = append(msgsToResend, pb.Message_ORDER_COMPLETION)
	}
	return msgsToResend, nil
}

func (service *OpenBazaarService) handleChat(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {

	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
	}
	chat := new(pb.Chat)
	err := ptypes.UnmarshalAny(pmes.Payload, chat)
	if err != nil {
		return nil, err
	}

	if chat.Flag == pb.Chat_TYPING {
		n := repo.ChatTyping{
			PeerId:    p.Pretty(),
			Subject:   chat.Subject,
			MessageId: chat.MessageId,
		}
		service.broadcast <- n
		return nil, nil
	}
	if chat.Flag == pb.Chat_READ {
		n := repo.ChatRead{
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
	if len(chat.Subject) > core.ChatSubjectMaxCharacters {
		return nil, errors.New("chat subject over max characters")
	}
	if len(chat.Message) > core.ChatMessageMaxCharacters {
		return nil, errors.New("chat message over max characters")
	}

	// Use correct timestamp
	offline, _ := options.(bool)
	var t time.Time
	if !offline {
		t = time.Now()
	} else {
		if chat.Timestamp == nil {
			return nil, errors.New("invalid timestamp")
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
			err = service.datastore.Purchases().MarkAsUnread(chat.Subject)
			if err != nil {
				log.Error(err)
			}
			err = service.datastore.Sales().MarkAsUnread(chat.Subject)
			if err != nil {
				log.Error(err)
			}
			err = service.datastore.Cases().MarkAsUnread(chat.Subject)
			if err != nil {
				log.Error(err)
			}
		}()
	}

	// Push to websocket
	n := repo.ChatMessageNotification{
		MessageId: chat.MessageId,
		PeerId:    p.Pretty(),
		Subject:   chat.Subject,
		Message:   chat.Message,
		Timestamp: repo.NewAPITime(t),
	}
	service.broadcast <- n
	log.Debugf("received CHAT message from %s", p.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleModeratorAdd(pid peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
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
		return nil, errors.New("moderator add message doesn't include correct peer ID")
	}
	if data.Type != pb.Message_MODERATOR_ADD {
		return nil, errors.New("data type is not moderator_add")
	}
	good, err := pubkey.Verify(sd.SerializedData, sd.Signature)
	if err != nil || !good {
		return nil, errors.New("bad signature")
	}
	err = service.datastore.ModeratedStores().Put(id.Pretty())
	if err != nil {
		return nil, err
	}

	log.Debugf("received MODERATOR_ADD message from %s", id.Pretty())

	return nil, nil
}

func (service *OpenBazaarService) handleModeratorRemove(pid peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
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
		return nil, errors.New("moderator remove message doesn't include correct peer ID")
	}
	if data.Type != pb.Message_MODERATOR_REMOVE {
		return nil, errors.New("data type is not moderator_remove")
	}
	good, err := pubkey.Verify(sd.SerializedData, sd.Signature)
	if err != nil || !good {
		return nil, errors.New("bad signature")
	}
	err = service.datastore.ModeratedStores().Delete(id.Pretty())
	if err != nil {
		return nil, err
	}

	log.Debugf("received MODERATOR_REMOVE message from %s", id.Pretty())

	return nil, nil
}

func (service *OpenBazaarService) handleBlock(pid peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	// If we aren't accepting store requests then ban this peer
	if !service.node.AcceptStoreRequests {
		service.node.BanManager.AddBlockedId(pid)
		return nil, nil
	}

	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
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
	err = service.node.IpfsNode.Blocks.AddBlock(block)
	if err != nil {
		return nil, err
	}
	log.Debugf("received BLOCK message from %s", pid.Pretty())
	return nil, nil
}

func (service *OpenBazaarService) handleVendorFinalizedPayment(pid peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		return nil, ErrEmptyPayload
	}
	paymentFinalizedMessage := new(pb.VendorFinalizedPayment)
	if err := ptypes.UnmarshalAny(pmes.Payload, paymentFinalizedMessage); err != nil {
		return nil, err
	}

	contract, state, _, _, _, _, err := service.datastore.Purchases().GetByOrderId(paymentFinalizedMessage.OrderID)
	if err != nil {
		return nil, err
	}

	if state != pb.OrderState_PENDING && state != pb.OrderState_FULFILLED && state != pb.OrderState_DISPUTED {
		return nil, errors.New("release escrow can only be called when sale is pending, fulfilled, or disputed")
	}
	err = service.datastore.Purchases().Put(paymentFinalizedMessage.OrderID, *contract, pb.OrderState_PAYMENT_FINALIZED, false)
	if err != nil {
		log.Error(err)
	}

	n := repo.VendorFinalizedPayment{
		ID:      repo.NewNotificationID(),
		Type:    repo.NotifierTypeVendorFinalizedPayment,
		OrderID: paymentFinalizedMessage.OrderID,
	}
	err = service.datastore.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		log.Error(err)
	}
	service.broadcast <- n
	log.Debugf("received VENDOR_FINALIZED_PAYMENT message from %s", pid.Pretty())
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
		return nil, ErrEmptyPayload
	}
	cList := new(pb.CidList)
	err := ptypes.UnmarshalAny(pmes.Payload, cList)
	if err != nil {
		return errorResponse("could not unmarshall message"), err
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

func (service *OpenBazaarService) handleOrderPayment(peer peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("received order payment message from: %s", peer.Pretty())

	if pmes.Payload == nil {
		return nil, errors.New("payload is nil")
	}
	paymentDetails := new(pb.OrderPaymentTxn)
	err := ptypes.UnmarshalAny(pmes.Payload, paymentDetails)
	if err != nil {
		return nil, err
	}

	orderID := paymentDetails.OrderID
	err = service.node.Datastore.Messages().Put(
		fmt.Sprintf("%s-%d", orderID, int(pb.Message_ORDER_PAYMENT)),
		orderID, pb.Message_ORDER_PAYMENT, peer.Pretty(), repo.Message{Msg: *pmes}, "", time.Now().UnixNano(), []byte(peer))
	if err != nil {
		log.Errorf("failed putting message (%s-%d): %v", orderID, int(pb.Message_ORDER_PAYMENT), err)
	}

	wal, err := service.node.Multiwallet.WalletForCurrencyCode(paymentDetails.GetCoin())
	if err != nil {
		return nil, err
	}

	wal0, ok := wal.(wallet.WalletMustManuallyAssociateTransactionToOrder)
	if !ok {
		return nil, nil
	}

	chash, err := chainhash.NewHashFromStr(paymentDetails.GetTransactionID())
	if err != nil {
		return nil, err
	}

	log.Debugf("retrieving %s transaction %s", paymentDetails.Coin, chash.String())
	txn, err := wal.GetTransaction(chash.String())
	if err != nil {
		return nil, err
	}

	log.Debugf("retrieving order %s from the database", paymentDetails.OrderID)
	contract, _, _, _, _, _, err := service.datastore.Sales().GetByOrderId(paymentDetails.OrderID)
	isVendor := true
	if err != nil {
		contract, _, _, _, _, _, err = service.datastore.Purchases().GetByOrderId(paymentDetails.OrderID)
		if err != nil {
			log.Debugf("no matching order was found in the database")
			return nil, net.OutOfOrderMessage
		}
		isVendor = false
	}

	if contract.VendorOrderConfirmation != nil &&
		contract.BuyerOrder.Payment.Method != pb.Order_Payment_MODERATED {

		// the seller has confirmed the direct order, so a simple check of
		// the addresses and we are good to proceed
		if !ut.AreAddressesEqual(contract.VendorOrderConfirmation.PaymentAddress, txn.ToAddress) {
			log.Errorf("mismatched payment address details: orderID: %s, expectedAddr: %s, actualAddr: %s",
				paymentDetails.OrderID, contract.VendorOrderConfirmation.PaymentAddress, txn.ToAddress)
			return nil, errors.New("mismatched payment addresses")

		}

	} else {
		// the seller has not confirmed or this is a moderated purchase,
		// so we need to compare the peerID in the vendorListing
		// to the node peerID
		if isVendor {
			if contract.VendorListings[0].VendorID.PeerID !=
				service.node.IpfsNode.Identity.Pretty() {
				log.Errorf("mismatched peerID. wrong node is processing: orderID: %s, contractPeerID: %s",
					paymentDetails.OrderID, contract.VendorListings[0].VendorID.PeerID)
				return nil, errors.New("mismatched peer id")
			}
		} else {
			if contract.BuyerOrder.BuyerID.PeerID !=
				service.node.IpfsNode.Identity.Pretty() {
				log.Errorf("mismatched peerID. wrong node is processing: orderID: %s, contractPeerID: %s",
					paymentDetails.OrderID, contract.BuyerOrder.BuyerID.PeerID)
				return nil, errors.New("mismatched peer id")
			}
		}

	}

	toAddress, err := wal.DecodeAddress(contract.BuyerOrder.Payment.RedeemScript)
	if err != nil {
		log.Error(err)
	}

	tvalue, ok := new(big.Int).SetString(txn.Value, 10)
	if ok && tvalue.Cmp(big.NewInt(0)) == 0 {
		toAddress, err = wal.DecodeAddress(contract.BuyerOrder.Payment.RedeemScript)
		if err != nil {
			log.Error(err)
		}
	}

	outputs := []wallet.TransactionOutput{}
	for _, o := range txn.Outputs {
		output := wallet.TransactionOutput{
			Address: toAddress,
			Value:   o.Value,
			Index:   o.Index,
			OrderID: paymentDetails.OrderID,
		}
		outputs = append(outputs, output)
	}

	input := wallet.TransactionInput{}

	txnValue, ok := new(big.Int).SetString(txn.Value, 10)
	if !ok {
		return nil, errors.New("invalid amount")
	}
	if paymentDetails.WithInput {
		input = wallet.TransactionInput{
			OutpointHash:  []byte(txn.Txid[:32]),
			OutpointIndex: 1,
			LinkedAddress: toAddress,
			Value:         *txnValue,
			OrderID:       paymentDetails.OrderID,
		}
	}

	cb := wallet.TransactionCallback{
		Txid:      txn.Txid,
		Outputs:   outputs,
		Inputs:    []wallet.TransactionInput{input},
		Height:    0,
		Timestamp: time.Now(),
		Value:     *txnValue,
		WatchOnly: false,
	}

	log.Debugf("associating tx %s with order %s", txn.Txid, paymentDetails.OrderID)
	wal0.AssociateTransactionWithOrder(cb)

	return nil, nil
}

func (service *OpenBazaarService) handleError(peer peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	if pmes.Payload == nil {
		log.Debugf("received empty ERROR message from peer (%s)", peer.Pretty())
		return nil, ErrEmptyPayload
	}
	errorMessage := new(pb.Error)
	err := ptypes.UnmarshalAny(pmes.Payload, errorMessage)
	if err != nil {
		log.Debugf("received unmarshalable ERROR message from peer (%s)", peer.Pretty())
		return nil, err
	}
	log.Debugf("received ERROR message from peer (%s): %s", peer.Pretty(), errorMessage.ErrorMessage)

	// Load the order
	contract, state, _, _, _, _, err := service.datastore.Purchases().GetByOrderId(errorMessage.OrderID)
	if err != nil {
		return nil, err
	}

	if state == pb.OrderState_PROCESSING_ERROR {
		return nil, net.DuplicateMessage
	}

	contract.Errors = []string{errorMessage.ErrorMessage}
	err = service.datastore.Purchases().Put(errorMessage.OrderID, *contract, pb.OrderState_PROCESSING_ERROR, false)
	if err != nil {
		log.Error(err)
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
	n := repo.ProcessingErrorNotification{
		ID:           repo.NewNotificationID(),
		Type:         repo.NotifierTypeProcessingErrorNotification,
		OrderId:      errorMessage.OrderID,
		Thumbnail:    repo.Thumbnail{Tiny: thumbnailTiny, Small: thumbnailSmall},
		VendorHandle: vendorHandle,
		VendorID:     vendorID,
	}
	service.broadcast <- n
	err = service.datastore.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		log.Error(err)
	}
	log.Debugf("received ERROR message from peer (%s): %s", peer.Pretty(), errorMessage.ErrorMessage)
	return nil, nil
}

// SendProcessingError produces a ORDER_PROCESSING_FAILURE error message to be sent to peerID regarding
// orderID. This message should cause the peerID to reproduce the missing messages omitted in latestContract
// up-to-and-including the last attemptedMessage
func (service *OpenBazaarService) SendProcessingError(peerID, orderID string, attemptedMessage pb.Message_MessageType, latestContract *pb.RicardianContract) error {
	return service.node.SendProcessingError(peerID, orderID, attemptedMessage, latestContract)
}
