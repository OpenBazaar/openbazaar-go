package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	"strconv"
	"sync"
	"time"
)

var DisputeWg = new(sync.WaitGroup)

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
	DisputeWg.Add(1)
	defer DisputeWg.Done()

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
		validationErrors := n.ValidateCaseContract(contract)
		var err error
		if contract.VendorListings[0].VendorID.Guid == peerID {
			err = n.Datastore.Cases().Put(orderId, nil, contract, []string{}, validationErrors, pb.OrderState_DISPUTED, false, false, rc.Dispute.Claim)
		} else if contract.BuyerOrder.BuyerID.Guid == peerID {
			err = n.Datastore.Cases().Put(orderId, contract, nil, validationErrors, []string{}, pb.OrderState_DISPUTED, false, true, rc.Dispute.Claim)
		} else {
			return errors.New("Peer ID doesn't match either buyer or vendor")
		}
		if err != nil {
			return err
		}
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

func (n *OpenBazaarNode) CloseDispute(buyerContract, vendorContract *pb.RicardianContract, buyerPercentage, vendorPercentage, moderatorPercentage float32, resolution string) error {
	if buyerPercentage+vendorPercentage+moderatorPercentage != 100 {
		return errors.New("Payout percentages must sum to 100")
	}
	return nil
}

func (n *OpenBazaarNode) ValidateCaseContract(contract *pb.RicardianContract) []string {
	var validationErrors []string

	// Contract should have a listing, order, and order confirmation to make it to this point
	if len(contract.VendorListings) == 0 {
		validationErrors = append(validationErrors, "Contract contains no listings")
	}
	if contract.BuyerOrder == nil {
		validationErrors = append(validationErrors, "Contract is missing the buyer's order")
	}
	if contract.VendorOrderConfirmation == nil {
		validationErrors = append(validationErrors, "Contract is missing the order confirmation")
	}
	if contract.VendorListings[0].VendorID == nil || contract.VendorListings[0].VendorID.Pubkeys == nil {
		validationErrors = append(validationErrors, "The listing is missing the vendor ID information. Unable to validate any signatures.")
		return validationErrors
	}
	if contract.BuyerOrder.BuyerID == nil || contract.BuyerOrder.BuyerID.Pubkeys == nil {
		validationErrors = append(validationErrors, "The listing is missing the buyer ID information. Unable to validate any signatures.")
		return validationErrors
	}

	vendorPubkey := contract.VendorListings[0].VendorID.Pubkeys.Guid
	vendorGuid := contract.VendorListings[0].VendorID.Guid

	buyerPubkey := contract.BuyerOrder.BuyerID.Pubkeys.Guid
	buyerGuid := contract.BuyerOrder.BuyerID.Guid

	// Make sure the order contains a payment object
	if contract.BuyerOrder.Payment == nil {
		validationErrors = append(validationErrors, "The buyer's order is missing the payment section")
	}

	// There needs to be one listing for each unique item in the order
	var listingHashes []string
	for _, item := range contract.BuyerOrder.Items {
		listingHashes = append(listingHashes, item.ListingHash)
	}
	for _, listing := range contract.VendorListings {
		ser, err := proto.Marshal(listing)
		if err != nil {
			continue
		}
		h := sha256.Sum256(ser)
		encoded, err := mh.Encode(h[:], mh.SHA2_256)
		if err != nil {
			continue
		}
		listingMH, err := mh.Cast(encoded)
		if err != nil {
			continue
		}
		for i, l := range listingHashes {
			if l == listingMH.B58String() {
				// Delete from listingHases
				listingHashes = append(listingHashes[:i], listingHashes[i+1:]...)
				break
			}
		}
	}
	// This should have a length of zero if there is one vendorListing for each item in buyerOrder
	if len(listingHashes) > 0 {
		validationErrors = append(validationErrors, "Not all items in the order have a matching vendor listing")
	}

	// There needs to be one listing signature for each listing
	var listingSigs []*pb.Signature
	for _, sig := range contract.Signatures {
		if sig.Section == pb.Signature_LISTING {
			listingSigs = append(listingSigs, sig)
		}
	}
	if len(listingSigs) < len(contract.VendorListings) {
		validationErrors = append(validationErrors, "Not all listings are signed by the vendor")
	}

	// Verify the listing signatures
	for i, listing := range contract.VendorListings {
		if err := verifyMessageSignature(listing, vendorPubkey, []*pb.Signature{listingSigs[i]}, pb.Signature_LISTING, vendorGuid); err != nil {
			validationErrors = append(validationErrors, "Invalid vendor signature on listing "+strconv.Itoa(i)+err.Error())
		}
		if i == len(listingSigs)-1 {
			break
		}
	}

	// Verify the order signature
	if err := verifyMessageSignature(contract.BuyerOrder, buyerPubkey, contract.Signatures, pb.Signature_ORDER, buyerGuid); err != nil {
		validationErrors = append(validationErrors, "Invalid buyer signature on order")
	}

	// Verify the order confirmation signature
	if err := verifyMessageSignature(contract.VendorOrderConfirmation, vendorPubkey, contract.Signatures, pb.Signature_ORDER_CONFIRMATION, vendorGuid); err != nil {
		validationErrors = append(validationErrors, "Invalid vendor signature on order confirmation")
	}

	// There should be one fulfilment signature for each vendorOrderFulfilment object
	var fulfilmentSigs []*pb.Signature
	for _, sig := range contract.Signatures {
		if sig.Section == pb.Signature_ORDER_FULFILLMENT {
			fulfilmentSigs = append(fulfilmentSigs, sig)
		}
	}
	if len(fulfilmentSigs) < len(contract.VendorOrderFulfillment) {
		validationErrors = append(validationErrors, "Not all order fulfilments are signed by the vendor")
	}

	// Verify the signature of the order fulfilments
	for i, f := range contract.VendorOrderFulfillment {
		if err := verifyMessageSignature(f, vendorPubkey, []*pb.Signature{fulfilmentSigs[i]}, pb.Signature_ORDER_FULFILLMENT, vendorGuid); err != nil {
			validationErrors = append(validationErrors, "Invalid vendor signature on fulfilment "+strconv.Itoa(i))
		}
		if i == len(fulfilmentSigs)-1 {
			break
		}
	}

	// Verify the buyer's bitcoin signature on his guid
	if err := verifyBitcoinSignature(
		contract.BuyerOrder.BuyerID.Pubkeys.Bitcoin,
		contract.BuyerOrder.BuyerID.BitcoinSig,
		contract.BuyerOrder.BuyerID.Guid,
	); err != nil {
		validationErrors = append(validationErrors, "The buyer's bitcoin signature which covers his guid is invalid. This could be an attempt to forge the buyer's identity.")
	}

	// Verify the vendor's bitcoin signature on his guid
	if err := verifyBitcoinSignature(
		contract.VendorListings[0].VendorID.Pubkeys.Bitcoin,
		contract.VendorListings[0].VendorID.BitcoinSig,
		contract.VendorListings[0].VendorID.Guid,
	); err != nil {
		validationErrors = append(validationErrors, "The vendor's bitcoin signature which covers his guid is invalid. This could be an attempt to forge the vendor's identity.")
	}

	// Verify the redeem script matches all the bitcoin keys
	if contract.BuyerOrder.Payment != nil {
		chaincode, err := hex.DecodeString(contract.BuyerOrder.Payment.Chaincode)
		if err != nil {
			validationErrors = append(validationErrors, "Error validating bitcoin address and redeem script")
			return validationErrors
		}
		parentFP := []byte{0x00, 0x00, 0x00, 0x00}
		mECKey, err := n.Wallet.MasterPublicKey().ECPubKey()
		if err != nil {
			validationErrors = append(validationErrors, "Error validating bitcoin address and redeem script")
			return validationErrors
		}
		hdKey := hd.NewExtendedKey(
			n.Wallet.Params().HDPublicKeyID[:],
			mECKey.SerializeCompressed(),
			chaincode,
			parentFP,
			0,
			0,
			false)
		moderatorKey, err := hdKey.Child(0)
		if err != nil {
			validationErrors = append(validationErrors, "Error validating bitcoin address and redeem script")
			return validationErrors
		}

		hdKey = hd.NewExtendedKey(
			n.Wallet.Params().HDPublicKeyID[:],
			contract.BuyerOrder.BuyerID.Pubkeys.Bitcoin,
			chaincode,
			parentFP,
			0,
			0,
			false)

		buyerKey, err := hdKey.Child(0)
		if err != nil {
			validationErrors = append(validationErrors, "Error validating bitcoin address and redeem script")
			return validationErrors
		}
		hdKey = hd.NewExtendedKey(
			n.Wallet.Params().HDPublicKeyID[:],
			contract.VendorListings[0].VendorID.Pubkeys.Bitcoin,
			chaincode,
			parentFP,
			0,
			0,
			false)
		vendorKey, err := hdKey.Child(0)
		if err != nil {
			validationErrors = append(validationErrors, "Error validating bitcoin address and redeem script")
			return validationErrors
		}
		addr, redeemScript, err := n.Wallet.GenerateMultisigScript([]hd.ExtendedKey{*buyerKey, *vendorKey, *moderatorKey}, 2)

		if contract.BuyerOrder.Payment.Address != addr.EncodeAddress() {
			validationErrors = append(validationErrors, "The calculated bitcoin address doesn't match the address in the order")
		}

		if hex.EncodeToString(redeemScript) != contract.BuyerOrder.Payment.RedeemScript {
			validationErrors = append(validationErrors, "The calculated redeem script doesn't match the redeem script in the order")
		}
	}

	return validationErrors
}
