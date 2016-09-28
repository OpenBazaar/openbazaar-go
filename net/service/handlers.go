package service

import (
	"encoding/hex"
	"fmt"
	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	peer "gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
	"gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
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
	case pb.Message_ORDER:
		return service.handleOrder
	case pb.Message_ORDER_CONFIRMATION:
		return service.handleOrderConfirmation
	case pb.Message_ORDER_CANCEL:
		return service.handleOrderCancel
	case pb.Message_ORDER_REJECT:
		return service.handleReject
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
	service.broadcast <- notifications.Serialize(notifications.FollowNotification{peer.Pretty()})
	return nil, nil
}

func (service *OpenBazaarService) handleUnFollow(peer peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received UNFOLLOW message from %s", peer.Pretty())
	err := service.datastore.Followers().Delete(peer.Pretty())
	if err != nil {
		return nil, err
	}
	service.broadcast <- notifications.Serialize(notifications.UnfollowNotification{peer.Pretty()})
	return nil, nil
}

func (service *OpenBazaarService) handleOfflineAck(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received OFFLINE_ACK message from %s", p.Pretty())
	pid, err := peer.IDB58Decode(string(pmes.Payload.Value))
	if err != nil {
		return nil, err
	}
	err = service.datastore.Pointers().Delete(pid)
	if err != nil {
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
	err := proto.Unmarshal(pmes.Payload.Value, contract)
	if err != nil {
		return errorResponse("Could not unmarshal order"), nil
	}

	err = service.node.ValidateOrder(contract)
	if err != nil {
		return errorResponse(err.Error()), nil
	}

	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_ADDRESS_REQUEST {
		total, err := service.node.CalculateOrderTotal(contract)
		if err != nil {
			return errorResponse("Error calculating payment amount"), nil
		}
		if total != contract.BuyerOrder.Payment.Amount {
			return errorResponse("Calculated a different payment amount"), nil
		}
		contract, err = service.node.NewOrderConfirmation(contract, true)
		if err != nil {
			return errorResponse("Error building order confirmation"), nil
		}
		a, err := ptypes.MarshalAny(contract)
		if err != nil {
			return errorResponse("Error building order confirmation"), nil
		}
		service.node.Datastore.Sales().Put(contract.VendorOrderConfirmation.OrderID, *contract, pb.OrderState_CONFIRMED, false)
		m := pb.Message{
			MessageType: pb.Message_ORDER_CONFIRMATION,
			Payload:     a,
		}
		return &m, nil
	} else if contract.BuyerOrder.Payment.Method == pb.Order_Payment_DIRECT {
		err := service.node.ValidateDirectPaymentAddress(contract.BuyerOrder)
		if err != nil {
			return errorResponse(err.Error()), nil
		}
		addr, err := btcutil.DecodeAddress(contract.BuyerOrder.Payment.Address, service.node.Wallet.Params())
		if err != nil {
			return errorResponse(err.Error()), nil
		}
		script, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return errorResponse(err.Error()), nil
		}
		service.node.Wallet.AddWatchedScript(script)
		orderId, err := service.node.CalcOrderId(contract.BuyerOrder)
		if err != nil {
			return errorResponse(err.Error()), nil
		}
		service.node.Datastore.Sales().Put(orderId, *contract, pb.OrderState_PENDING, false)
		return nil, nil
	} else if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED && !offline {
		total, err := service.node.CalculateOrderTotal(contract)
		if err != nil {
			return errorResponse("Error calculating payment amount"), nil
		}
		if total != contract.BuyerOrder.Payment.Amount {
			return errorResponse("Calculated a different payment amount"), nil
		}
		err = service.node.ValidateModeratedPaymentAddress(contract.BuyerOrder)
		if err != nil {
			return errorResponse(err.Error()), nil
		}
		addr, err := btcutil.DecodeAddress(contract.BuyerOrder.Payment.Address, service.node.Wallet.Params())
		if err != nil {
			return errorResponse(err.Error()), nil
		}
		script, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return errorResponse(err.Error()), nil
		}
		service.node.Wallet.AddWatchedScript(script)
		contract, err = service.node.NewOrderConfirmation(contract, false)
		if err != nil {
			return errorResponse("Error building order confirmation"), nil
		}
		a, err := ptypes.MarshalAny(contract)
		if err != nil {
			return errorResponse("Error building order confirmation"), nil
		}
		service.node.Datastore.Sales().Put(contract.VendorOrderConfirmation.OrderID, *contract, pb.OrderState_CONFIRMED, false)
		m := pb.Message{
			MessageType: pb.Message_ORDER_CONFIRMATION,
			Payload:     a,
		}
		return &m, nil
	} else if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED && offline {
		err := service.node.ValidateModeratedPaymentAddress(contract.BuyerOrder)
		if err != nil {
			return errorResponse(err.Error()), nil
		}
		addr, err := btcutil.DecodeAddress(contract.BuyerOrder.Payment.Address, service.node.Wallet.Params())
		if err != nil {
			return errorResponse(err.Error()), nil
		}
		script, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return errorResponse(err.Error()), nil
		}
		service.node.Wallet.AddWatchedScript(script)
		orderId, err := service.node.CalcOrderId(contract.BuyerOrder)
		if err != nil {
			return errorResponse(err.Error()), nil
		}
		service.node.Datastore.Sales().Put(orderId, *contract, pb.OrderState_PENDING, false)
		return nil, nil
	}
	return errorResponse("Unrecognized payment type"), nil
}

func (service *OpenBazaarService) handleOrderConfirmation(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received ORDER_CONFIRMATION message from %s", p.Pretty())

	// Unmarshal payload
	vendorContract := new(pb.RicardianContract)
	err := proto.Unmarshal(pmes.Payload.Value, vendorContract)
	if err != nil {
		return nil, fmt.Errorf("Could not unmarshal ORDER_CONFIRMATION from %s", p.Pretty())
	}

	// Calc order ID
	orderId := vendorContract.VendorOrderConfirmation.OrderID

	// Load the order
	contract, _, _, _, _, err := service.datastore.Purchases().GetByOrderId(orderId)
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
		if sig.Section == pb.Signatures_ORDER_CONFIRMATION {
			contract.Signatures = append(contract.Signatures, sig)
		}
	}

	// Set message state to confirmed
	service.datastore.Purchases().Put(orderId, *contract, pb.OrderState_CONFIRMED, false)

	// Send notification to websocket
	n := notifications.Serialize(notifications.OrderConfirmationNotification{orderId})
	service.broadcast <- n

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

	// Set message state to confirmed
	service.datastore.Sales().Put(orderId, *contract, pb.OrderState_CANCELED, false)

	return nil, nil
}

func (service *OpenBazaarService) handleReject(p peer.ID, pmes *pb.Message, options interface{}) (*pb.Message, error) {
	log.Debugf("Received REJECT message from %s", p.Pretty())
	orderId := string(pmes.Payload.Value)

	// Load the order
	contract, _, _, records, _, err := service.datastore.Purchases().GetByOrderId(orderId)
	if err != nil {
		return nil, err
	}

	// Sweep the temp address into our wallet
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
	err = service.node.Wallet.SweepMultisig(utxos, buyerKey, redeemScript, spvwallet.NORMAL)
	if err != nil {
		return nil, err
	}

	// Set message state to confirmed
	service.datastore.Purchases().Put(orderId, *contract, pb.OrderState_REJECTED, false)

	// Send notification to websocket
	n := notifications.Serialize(notifications.OrderCancelNotification{orderId})
	service.broadcast <- n

	return nil, nil
}
