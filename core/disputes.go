package core

import (
	"errors"
	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
	"github.com/golang/protobuf/ptypes/timestamp"
	"gx/ipfs/QmT6n4mspWYEya864BhCUJEgyxiRfmiSY9ruQwTUNpRKaM/protobuf/proto"
	"time"
)

func (n *OpenBazaarNode) OpenDispute(orderID string, contract *pb.RicardianContract, records []*spvwallet.TransactionRecord, claim string) error {
	var isPurchase bool
	if n.IpfsNode.Identity.Pretty() == contract.BuyerOrder.BuyerID.Guid {
		isPurchase = true
	}

	dispute := new(pb.Dispute)

	// Create timestamp
	ts := new(timestamp.Timestamp)
	ts.Seconds = time.Now().Unix()
	ts.Nanos = 0
	dispute.Timestamp = ts

	// Add claim
	dispute.Claim = claim

	// Create outpoints
	var outpoints []*pb.Dispute_Outpoint
	for _, r := range records {
		o := new(pb.Dispute_Outpoint)
		o.Hash = r.Txid
		o.Index = r.Index
		outpoints = append(outpoints, o)
	}
	dispute.Outpoints = outpoints

	// Maybe add rating keys
	if isPurchase {
		var keys [][]byte
		for _, k := range contract.BuyerOrder.RatingKeys {
			keys = append(keys, k)
		}
		dispute.RatingKeys = keys
	}

	// Serialize contract
	ser, err := proto.Marshal(contract)
	if err != nil {
		return err
	}
	dispute.SerializedContract = ser

	// Sign dispute
	rc := new(pb.RicardianContract)
	rc.Dispute = dispute
	rc, err = n.SignDispute(rc)
	if err != nil {
		return err
	}
	contract.Dispute = dispute
	contract.Signatures = append(contract.Signatures, rc.Signatures[0])

	// Send to moderator
	err = n.SendDisputeOpen(contract.BuyerOrder.Payment.Moderator, rc)
	if err != nil {
		return err
	}

	// Send to counterparty
	var counterparty string
	if isPurchase {
		counterparty = contract.VendorListings[0].VendorID.Guid
	} else {
		counterparty = contract.BuyerOrder.BuyerID.Guid
	}
	err = n.SendDisputeOpen(counterparty, rc)
	if err != nil {
		return err
	}

	// Update database
	if isPurchase {
		n.Datastore.Purchases().Put(orderID, *contract, pb.OrderState_DISPUTED, true)
	} else {
		n.Datastore.Sales().Put(orderID, *contract, pb.OrderState_DISPUTED, true)
	}
	return nil
}

func (n *OpenBazaarNode) SignDispute(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedDispute, err := proto.Marshal(contract.Dispute)
	if err != nil {
		return contract, err
	}
	s := new(pb.Signature)
	s.Section = pb.Signature_DISPUTE
	if err != nil {
		return contract, err
	}
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedDispute)
	if err != nil {
		return contract, err
	}
	s.SignatureBytes = guidSig
	contract.Signatures = append(contract.Signatures, s)
	return contract, nil
}

func (n *OpenBazaarNode) VerifySignatureOnDisputeOpen(contract *pb.RicardianContract, peerID string) error {
	var pubkey []byte
	deser := new(pb.RicardianContract)
	err := proto.Unmarshal(contract.Dispute.SerializedContract, deser)
	if err != nil {
		return err
	}
	if len(deser.VendorListings) == 0 || deser.BuyerOrder == nil {
		return errors.New("Invalid serialized contract")
	}
	if peerID == deser.BuyerOrder.BuyerID.Guid {
		pubkey = deser.BuyerOrder.BuyerID.Pubkeys.Guid
	} else if peerID == deser.VendorListings[0].VendorID.Guid {
		pubkey = deser.VendorListings[0].VendorID.Pubkeys.Guid
	} else {
		return errors.New("Peer ID doesn't match either buyer or vendor")
	}

	if err := verifyMessageSignature(
		contract.Dispute,
		pubkey,
		contract.Signatures,
		pb.Signature_DISPUTE,
		peerID,
	); err != nil {
		switch err.(type) {
		case noSigError:
			return errors.New("Contract does not contain a signature for the dispute")
		case invalidSigError:
			return errors.New("Guid signature on contact failed to verify")
		case matchKeyError:
			return errors.New("Public key in dispute does not match reported ID")
		default:
			return err
		}
	}
	return nil
}

func (n *OpenBazaarNode) ProcessDisputeOpen(rc *pb.RicardianContract, peerID string) error {
	if rc.Dispute == nil {
		return errors.New("Dispute message is nil")
	}

	// Deserialize contract
	contract := new(pb.RicardianContract)
	err := proto.Unmarshal(rc.Dispute.SerializedContract, contract)
	if err != nil {
		return err
	}
	if len(contract.VendorListings) == 0 || contract.BuyerOrder == nil || contract.BuyerOrder.Payment == nil {
		return errors.New("Serialized contract is malformatted")
	}

	orderId, err := n.CalcOrderId(contract.BuyerOrder)
	if err != nil {
		return err
	}

	// Figure out what role we have in this dispute and process it
	if contract.BuyerOrder.Payment.Moderator == n.IpfsNode.Identity.Pretty() { // Moderator
		var err error
		if contract.VendorListings[0].VendorID.Guid == peerID {
			err = n.Datastore.Cases().Put(orderId, nil, contract, pb.OrderState_DISPUTED, false, false, rc.Dispute.Claim)
		} else if contract.BuyerOrder.BuyerID.Guid == peerID {
			err = n.Datastore.Cases().Put(orderId, contract, nil, pb.OrderState_DISPUTED, false, true, rc.Dispute.Claim)
		} else {
			return errors.New("Peer ID doesn't match either buyer or vendor")
		}
		if err != nil {
			return err
		}
		// TODO: all the signatures in the contract need to be validate and we need a way to
		// save validation failures and display them in the UI.
	} else if contract.VendorListings[0].VendorID.Guid == n.IpfsNode.Identity.Pretty() { // Vendor
		// Load out version of the contract from the db
		myContract, state, _, _, _, err := n.Datastore.Sales().GetByOrderId(orderId)
		if err != nil {
			return err
		}
		// Check this order is currently in a state which can be disputed
		if state == pb.OrderState_COMPLETE || state == pb.OrderState_DISPUTED || state == pb.OrderState_RESOLVED || state == pb.OrderState_REFUNDED || state == pb.OrderState_CANCELED || state == pb.OrderState_REJECTED {
			return errors.New("Contact can no longer be disputed")
		}

		// Build dispute update message
		update := new(pb.DisputeUpdate)
		ser, err := proto.Marshal(myContract)
		if err != nil {
			return err
		}
		update.SerializedContract = ser
		update.OrderId = orderId
		update.PayoutAddress = n.Wallet.CurrentAddress(spvwallet.EXTERNAL).EncodeAddress()

		// Send the message
		err = n.SendDisputeUpdate(myContract.BuyerOrder.Payment.Moderator, update)
		if err != nil {
			return err
		}

		// Append the dispute and signature
		myContract.Dispute = rc.Dispute
		for _, sig := range rc.Signatures {
			if sig.Section == pb.Signature_DISPUTE {
				myContract.Signatures = append(myContract.Signatures, sig)
			}
		}
		// Save it back to the db with the new state
		err = n.Datastore.Sales().Put(orderId, *myContract, pb.OrderState_DISPUTED, false)
		if err != nil {
			return err
		}
	} else if contract.BuyerOrder.BuyerID.Guid == n.IpfsNode.Identity.Pretty() { // Buyer
		// Load out version of the contract from the db
		myContract, state, _, _, _, err := n.Datastore.Purchases().GetByOrderId(orderId)
		if err != nil {
			return err
		}
		// Check this order is currently in a state which can be disputed
		if state == pb.OrderState_COMPLETE || state == pb.OrderState_DISPUTED || state == pb.OrderState_RESOLVED || state == pb.OrderState_REFUNDED || state == pb.OrderState_CANCELED || state == pb.OrderState_REJECTED {
			return errors.New("Contact can no longer be disputed")
		}

		// Build dispute update message
		update := new(pb.DisputeUpdate)
		ser, err := proto.Marshal(myContract)
		if err != nil {
			return err
		}
		update.SerializedContract = ser
		update.OrderId = orderId
		update.PayoutAddress = n.Wallet.CurrentAddress(spvwallet.EXTERNAL).EncodeAddress()

		// Send the message
		err = n.SendDisputeUpdate(myContract.BuyerOrder.Payment.Moderator, update)
		if err != nil {
			return err
		}

		// Append the dispute and signature
		myContract.Dispute = rc.Dispute
		for _, sig := range rc.Signatures {
			if sig.Section == pb.Signature_DISPUTE {
				myContract.Signatures = append(myContract.Signatures, sig)
			}
		}
		// Save it back to the db with the new state
		err = n.Datastore.Purchases().Put(orderId, *myContract, pb.OrderState_DISPUTED, false)
		if err != nil {
			return err
		}
	} else {
		return errors.New("We are not involved in this dispute")
	}

	notif := notifications.Serialize(notifications.DisputeOpenNotification{orderId})
	n.Broadcast <- notif

	return nil
}
