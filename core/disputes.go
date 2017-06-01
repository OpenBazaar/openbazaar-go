package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"sync"
	"time"

	libp2p "gx/ipfs/QmPGxZ1DP2w45WcogpW1h43BvseXbfke9N91qotpoQcUeS/go-libp2p-crypto"
	peer "gx/ipfs/QmWUswjn261LSyVxWAEpMVtPdy8zmKBJJfBpG3Qdpa8ZsE/go-libp2p-peer"
	mh "gx/ipfs/QmbZ6Cee2uHjG7hf19qLHppgKDRtaG4CVtMzdmK9VCVqLu/go-multihash"

	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/ipfs/go-ipfs/routing/dht"
	"golang.org/x/net/context"
)

var DisputeWg = new(sync.WaitGroup)

var ErrCaseNotFound = errors.New("Case not found")

func (n *OpenBazaarNode) OpenDispute(orderID string, contract *pb.RicardianContract, records []*spvwallet.TransactionRecord, claim string) error {
	var isPurchase bool
	if n.IpfsNode.Identity.Pretty() == contract.BuyerOrder.BuyerID.PeerID {
		isPurchase = true
	}

	dispute := new(pb.Dispute)

	// Create timestamp
	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}
	dispute.Timestamp = ts

	// Add claim
	dispute.Claim = claim

	// Create outpoints
	var outpoints []*pb.Outpoint
	for _, r := range records {
		o := new(pb.Outpoint)
		o.Hash = r.Txid
		o.Index = r.Index
		o.Value = uint64(r.Value)
		outpoints = append(outpoints, o)
	}
	dispute.Outpoints = outpoints

	// Add payout address
	dispute.PayoutAddress = n.Wallet.CurrentAddress(spvwallet.EXTERNAL).EncodeAddress()

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
	err = n.SendDisputeOpen(contract.BuyerOrder.Payment.Moderator, nil, rc)
	if err != nil {
		return err
	}

	// Send to counterparty
	var counterparty string
	var counterkey libp2p.PubKey
	if isPurchase {
		counterparty = contract.VendorListings[0].VendorID.PeerID
		counterkey, err = libp2p.UnmarshalPublicKey(contract.VendorListings[0].VendorID.Pubkeys.Identity)
		if err != nil {
			return nil
		}
	} else {
		counterparty = contract.BuyerOrder.BuyerID.PeerID
		counterkey, err = libp2p.UnmarshalPublicKey(contract.BuyerOrder.BuyerID.Pubkeys.Identity)
		if err != nil {
			return nil
		}
	}
	err = n.SendDisputeOpen(counterparty, &counterkey, rc)
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
	if peerID == deser.BuyerOrder.BuyerID.PeerID {
		pubkey = deser.BuyerOrder.BuyerID.Pubkeys.Identity
	} else if peerID == deser.VendorListings[0].VendorID.PeerID {
		pubkey = deser.VendorListings[0].VendorID.Pubkeys.Identity
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
		if contract.VendorListings[0].VendorID.PeerID == peerID {
			err = n.Datastore.Cases().Put(orderId, pb.OrderState_DISPUTED, false, rc.Dispute.Claim)
			if err != nil {
				return err
			}
			err = n.Datastore.Cases().UpdateVendorInfo(orderId, contract, validationErrors, rc.Dispute.PayoutAddress, rc.Dispute.Outpoints)
			if err != nil {
				return err
			}
		} else if contract.BuyerOrder.BuyerID.PeerID == peerID {
			err = n.Datastore.Cases().Put(orderId, pb.OrderState_DISPUTED, true, rc.Dispute.Claim)
			if err != nil {
				return err
			}
			err = n.Datastore.Cases().UpdateBuyerInfo(orderId, contract, validationErrors, rc.Dispute.PayoutAddress, rc.Dispute.Outpoints)
			if err != nil {
				return err
			}
		} else {
			return errors.New("Peer ID doesn't match either buyer or vendor")
		}
		if err != nil {
			return err
		}
	} else if contract.VendorListings[0].VendorID.PeerID == n.IpfsNode.Identity.Pretty() { // Vendor
		// Load out version of the contract from the db
		myContract, state, _, records, _, err := n.Datastore.Sales().GetByOrderId(orderId)
		if err != nil {
			return err
		}
		// Check this order is currently in a state which can be disputed
		if state == pb.OrderState_COMPLETED || state == pb.OrderState_DISPUTED || state == pb.OrderState_DECIDED || state == pb.OrderState_RESOLVED || state == pb.OrderState_REFUNDED || state == pb.OrderState_CANCELED || state == pb.OrderState_DECLINED {
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

		var outpoints []*pb.Outpoint
		for _, r := range records {
			o := new(pb.Outpoint)
			o.Hash = r.Txid
			o.Index = r.Index
			o.Value = uint64(r.Value)
			outpoints = append(outpoints, o)
		}
		update.Outpoints = outpoints

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
	} else if contract.BuyerOrder.BuyerID.PeerID == n.IpfsNode.Identity.Pretty() { // Buyer
		// Load out version of the contract from the db
		myContract, state, _, records, _, err := n.Datastore.Purchases().GetByOrderId(orderId)
		if err != nil {
			return err
		}
		// Check this order is currently in a state which can be disputed
		if state == pb.OrderState_COMPLETED || state == pb.OrderState_DISPUTED || state == pb.OrderState_DECIDED || state == pb.OrderState_RESOLVED || state == pb.OrderState_REFUNDED || state == pb.OrderState_CANCELED || state == pb.OrderState_DECLINED {
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

		var outpoints []*pb.Outpoint
		for _, r := range records {
			o := new(pb.Outpoint)
			o.Hash = r.Txid
			o.Index = r.Index
			o.Value = uint64(r.Value)
			outpoints = append(outpoints, o)
		}
		update.Outpoints = outpoints

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

	notif := notifications.DisputeOpenNotification{orderId}
	n.Broadcast <- notif
	n.Datastore.Notifications().Put(notifications.Wrap(notif), time.Now())

	return nil
}

func (n *OpenBazaarNode) CloseDispute(orderId string, buyerPercentage, vendorPercentage float32, resolution string) error {
	if buyerPercentage+vendorPercentage != 100 {
		return errors.New("Payout percentages must sum to 100")
	}

	buyerContract, vendorContract, buyerPayoutAddress, vendorPayoutAddress, buyerOutpoints, vendorOutpoints, state, err := n.Datastore.Cases().GetPayoutDetails(orderId)
	if err != nil {
		return ErrCaseNotFound
	}
	if state != pb.OrderState_DISPUTED {
		return errors.New("A dispute for this order is not open")
	}

	d := new(pb.DisputeResolution)

	// Add timestamp
	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}
	d.Timestamp = ts

	// Add orderId
	d.OrderId = orderId

	// Set self (moderator) as the party that made the resolution proposal
	d.ProposedBy = n.IpfsNode.Identity.Pretty()

	// Sign buyer rating key
	if buyerContract != nil {
		for _, key := range buyerContract.BuyerOrder.RatingKeys {
			sig, err := n.IpfsNode.PrivateKey.Sign(key)
			if err != nil {
				return err
			}
			d.ModeratorRatingSigs = append(d.ModeratorRatingSigs, sig)
		}
	}

	// Set resolution
	d.Resolution = resolution

	// Decide whose contract to use
	var buyerPayout bool
	var vendorPayout bool
	var moderatorPayout bool
	var outpoints []*pb.Outpoint
	var redeemScript string
	var chaincode string
	var feePerByte uint64
	var vendorId string
	var vendorKey libp2p.PubKey
	var buyerId string
	var buyerKey libp2p.PubKey
	if buyerPercentage > 0 && vendorPercentage == 0 {
		buyerPayout = true
		outpoints = buyerOutpoints
		redeemScript = buyerContract.BuyerOrder.Payment.RedeemScript
		chaincode = buyerContract.BuyerOrder.Payment.Chaincode
		feePerByte = buyerContract.BuyerOrder.RefundFee
		buyerId = buyerContract.BuyerOrder.BuyerID.PeerID
		buyerKey, err = libp2p.UnmarshalPublicKey(buyerContract.BuyerOrder.BuyerID.Pubkeys.Identity)
		if err != nil {
			return err
		}
		vendorId = buyerContract.VendorListings[0].VendorID.PeerID
		vendorKey, err = libp2p.UnmarshalPublicKey(buyerContract.VendorListings[0].VendorID.Pubkeys.Identity)
		if err != nil {
			return err
		}
	} else if vendorPercentage > 0 && buyerPercentage == 0 {
		vendorPayout = true
		outpoints = vendorOutpoints
		redeemScript = vendorContract.BuyerOrder.Payment.RedeemScript
		chaincode = vendorContract.BuyerOrder.Payment.Chaincode
		if len(vendorContract.VendorOrderFulfillment) > 0 && vendorContract.VendorOrderFulfillment[0].Payout != nil {
			feePerByte = vendorContract.VendorOrderFulfillment[0].Payout.PayoutFeePerByte
		} else {
			feePerByte = n.Wallet.GetFeePerByte(spvwallet.NORMAL)
		}
		buyerId = vendorContract.BuyerOrder.BuyerID.PeerID
		buyerKey, err = libp2p.UnmarshalPublicKey(vendorContract.BuyerOrder.BuyerID.Pubkeys.Identity)
		if err != nil {
			return err
		}
		vendorId = vendorContract.VendorListings[0].VendorID.PeerID
		vendorKey, err = libp2p.UnmarshalPublicKey(vendorContract.VendorListings[0].VendorID.Pubkeys.Identity)
		if err != nil {
			return err
		}
	} else if vendorPercentage > buyerPercentage {
		buyerPayout = true
		vendorPayout = true
		outpoints = vendorOutpoints
		redeemScript = vendorContract.BuyerOrder.Payment.RedeemScript
		chaincode = vendorContract.BuyerOrder.Payment.Chaincode
		if len(vendorContract.VendorOrderFulfillment) > 0 && vendorContract.VendorOrderFulfillment[0].Payout != nil {
			feePerByte = vendorContract.VendorOrderFulfillment[0].Payout.PayoutFeePerByte
		} else {
			feePerByte = n.Wallet.GetFeePerByte(spvwallet.NORMAL)
		}
		buyerId = vendorContract.BuyerOrder.BuyerID.PeerID
		buyerKey, err = libp2p.UnmarshalPublicKey(vendorContract.BuyerOrder.BuyerID.Pubkeys.Identity)
		if err != nil {
			return err
		}
		vendorId = vendorContract.VendorListings[0].VendorID.PeerID
		vendorKey, err = libp2p.UnmarshalPublicKey(vendorContract.VendorListings[0].VendorID.Pubkeys.Identity)
		if err != nil {
			return err
		}
	} else if buyerPercentage >= vendorPercentage {
		buyerPayout = true
		vendorPayout = true
		outpoints = buyerOutpoints
		redeemScript = buyerContract.BuyerOrder.Payment.RedeemScript
		chaincode = buyerContract.BuyerOrder.Payment.Chaincode
		feePerByte = buyerContract.BuyerOrder.RefundFee
		buyerId = buyerContract.BuyerOrder.BuyerID.PeerID
		buyerKey, err = libp2p.UnmarshalPublicKey(buyerContract.BuyerOrder.BuyerID.Pubkeys.Identity)
		if err != nil {
			return err
		}
		vendorId = buyerContract.VendorListings[0].VendorID.PeerID
		vendorKey, err = libp2p.UnmarshalPublicKey(buyerContract.VendorListings[0].VendorID.Pubkeys.Identity)
		if err != nil {
			return err
		}
	}

	// Calculate total out value
	var totalOut uint64
	for _, o := range outpoints {
		totalOut += o.Value
	}

	// Create outputs using full value. We will subtract the fee off each output later.
	var outputs []spvwallet.TransactionOutput
	var modAddr btcutil.Address
	var modValue uint64
	modAddr = n.Wallet.CurrentAddress(spvwallet.EXTERNAL)
	modValue, err = n.GetModeratorFee(totalOut)
	var modOutputScript []byte
	if err != nil {
		return err
	}
	if modValue > 0 {
		modOutputScript, err = n.Wallet.AddressToScript(modAddr)
		if err != nil {
			return err
		}
		out := spvwallet.TransactionOutput{
			ScriptPubKey: modOutputScript,
			Value:        int64(modValue),
		}
		outputs = append(outputs, out)
		moderatorPayout = true
	}

	var buyerAddr btcutil.Address
	var buyerValue uint64
	var buyerOutputScript []byte
	if buyerPayout {
		buyerAddr, err = n.Wallet.DecodeAddress(buyerPayoutAddress)
		if err != nil {
			return err
		}
		buyerValue = uint64((float64(totalOut) - float64(modValue)) * (float64(buyerPercentage) / 100))
		buyerOutputScript, err = n.Wallet.AddressToScript(buyerAddr)
		if err != nil {
			return err
		}
		out := spvwallet.TransactionOutput{
			ScriptPubKey: buyerOutputScript,
			Value:        int64(buyerValue),
		}
		outputs = append(outputs, out)
	}
	var vendorAddr btcutil.Address
	var vendorValue uint64
	var vendorOutputScript []byte
	if vendorPayout {
		vendorAddr, err = n.Wallet.DecodeAddress(vendorPayoutAddress)
		if err != nil {
			return err
		}
		vendorValue = uint64((float64(totalOut) - float64(modValue)) * (float64(vendorPercentage) / 100))
		vendorOutputScript, err = n.Wallet.AddressToScript(vendorAddr)
		if err != nil {
			return err
		}
		out := spvwallet.TransactionOutput{
			ScriptPubKey: vendorOutputScript,
			Value:        int64(vendorValue),
		}
		outputs = append(outputs, out)
	}

	if len(outputs) == 0 {
		return errors.New("Transaction has no outputs")
	}

	// Create inputs
	var inputs []spvwallet.TransactionInput
	for _, o := range outpoints {
		decodedHash, err := hex.DecodeString(o.Hash)
		if err != nil {
			return err
		}
		input := spvwallet.TransactionInput{
			OutpointHash:  decodedHash,
			OutpointIndex: o.Index,
		}
		inputs = append(inputs, input)
	}

	if len(inputs) == 0 {
		return errors.New("Transaction has no inputs")
	}

	// Calculate total fee
	txFee := n.Wallet.EstimateFee(inputs, outputs, feePerByte)

	// Subtract fee from each output in proportion to output value
	var outs []spvwallet.TransactionOutput
	for _, output := range outputs {
		outPercentage := float64(output.Value) / float64(totalOut)
		outputShareOfFee := outPercentage * float64(txFee)
		o := spvwallet.TransactionOutput{
			Value:        output.Value - int64(outputShareOfFee),
			ScriptPubKey: output.ScriptPubKey,
			Index:        output.Index,
		}
		outs = append(outs, o)
	}

	// Create moderator key
	parentFP := []byte{0x00, 0x00, 0x00, 0x00}
	chaincodeBytes, err := hex.DecodeString(chaincode)
	if err != nil {
		return err
	}
	mPrivKey := n.Wallet.MasterPrivateKey()
	if err != nil {
		return err
	}
	mECKey, err := mPrivKey.ECPrivKey()
	if err != nil {
		return err
	}
	hdKey := hd.NewExtendedKey(
		n.Wallet.Params().HDPrivateKeyID[:],
		mECKey.Serialize(),
		chaincodeBytes,
		parentFP,
		0,
		0,
		true)

	moderatorKey, err := hdKey.Child(0)
	if err != nil {
		return err
	}

	// Create signatures
	redeemScriptBytes, err := hex.DecodeString(redeemScript)
	if err != nil {
		return err
	}
	sigs, err := n.Wallet.CreateMultisigSignature(inputs, outs, moderatorKey, redeemScriptBytes, 0)
	if err != nil {
		return err
	}
	var bitcoinSigs []*pb.BitcoinSignature
	for _, sig := range sigs {
		s := new(pb.BitcoinSignature)
		s.InputIndex = sig.InputIndex
		s.Signature = sig.Signature
		bitcoinSigs = append(bitcoinSigs, s)
	}

	// Create payout object
	payout := new(pb.DisputeResolution_Payout)
	payout.Inputs = outpoints
	payout.Sigs = bitcoinSigs
	if buyerPayout {
		outputShareOfFee := (float64(buyerValue) / float64(totalOut)) * float64(txFee)
		payout.BuyerOutput = &pb.DisputeResolution_Payout_Output{Script: hex.EncodeToString(buyerOutputScript), Amount: buyerValue - uint64(outputShareOfFee)}
	}
	if vendorPayout {
		outputShareOfFee := (float64(vendorValue) / float64(totalOut)) * float64(txFee)
		payout.VendorOutput = &pb.DisputeResolution_Payout_Output{Script: hex.EncodeToString(vendorOutputScript), Amount: vendorValue - uint64(outputShareOfFee)}
	}
	if moderatorPayout {
		outputShareOfFee := (float64(modValue) / float64(totalOut)) * float64(txFee)
		payout.ModeratorOutput = &pb.DisputeResolution_Payout_Output{Script: hex.EncodeToString(modOutputScript), Amount: modValue - uint64(outputShareOfFee)}
	}

	d.Payout = payout

	rc := new(pb.RicardianContract)
	rc.DisputeResolution = d
	rc, err = n.SignDisputeResolution(rc)
	if err != nil {
		return err
	}

	err = n.SendDisputeClose(buyerId, &buyerKey, rc)
	if err != nil {
		return err
	}
	err = n.SendDisputeClose(vendorId, &vendorKey, rc)
	if err != nil {
		return err
	}

	err = n.Datastore.Cases().MarkAsClosed(orderId, d)
	if err != nil {
		return err
	}
	return nil
}

func (n *OpenBazaarNode) SignDisputeResolution(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedDR, err := proto.Marshal(contract.DisputeResolution)
	if err != nil {
		return contract, err
	}
	s := new(pb.Signature)
	s.Section = pb.Signature_DISPUTE_RESOLUTION
	if err != nil {
		return contract, err
	}
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedDR)
	if err != nil {
		return contract, err
	}
	s.SignatureBytes = guidSig
	contract.Signatures = append(contract.Signatures, s)
	return contract, nil
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

	vendorPubkey := contract.VendorListings[0].VendorID.Pubkeys.Identity
	vendorGuid := contract.VendorListings[0].VendorID.PeerID

	buyerPubkey := contract.BuyerOrder.BuyerID.Pubkeys.Identity
	buyerGuid := contract.BuyerOrder.BuyerID.PeerID

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
		contract.BuyerOrder.BuyerID.PeerID,
	); err != nil {
		validationErrors = append(validationErrors, "The buyer's bitcoin signature which covers his guid is invalid. This could be an attempt to forge the buyer's identity.")
	}

	// Verify the vendor's bitcoin signature on his guid
	if err := verifyBitcoinSignature(
		contract.VendorListings[0].VendorID.Pubkeys.Bitcoin,
		contract.VendorListings[0].VendorID.BitcoinSig,
		contract.VendorListings[0].VendorID.PeerID,
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

func (n *OpenBazaarNode) ValidateDisputeResolution(contract *pb.RicardianContract) error {
	err := n.verifySignatureOnDisputeResolution(contract)
	if err != nil {
		return err
	}
	if contract.DisputeResolution.Payout == nil || len(contract.DisputeResolution.Payout.Sigs) == 0 {
		return errors.New("DisputeResolution contains invalid payout")
	}
	checkWeOwnAddress := func(scriptPubKey string) error {
		scriptBytes, err := hex.DecodeString(scriptPubKey)
		if err != nil {
			return err
		}
		addr, err := n.Wallet.ScriptToAddress(scriptBytes)
		if err != nil {
			return err
		}
		if !n.Wallet.HasKey(addr) {
			return errors.New("Moderator payout sends coins to an address we don't control")
		}
		return nil
	}

	if contract.VendorListings[0].VendorID.PeerID == n.IpfsNode.Identity.Pretty() && contract.DisputeResolution.Payout.VendorOutput != nil {
		return checkWeOwnAddress(contract.DisputeResolution.Payout.VendorOutput.Script)
	} else if contract.BuyerOrder.BuyerID.PeerID == n.IpfsNode.Identity.Pretty() && contract.DisputeResolution.Payout.BuyerOutput != nil {
		return checkWeOwnAddress(contract.DisputeResolution.Payout.BuyerOutput.Script)
	}
	return nil
}

func (n *OpenBazaarNode) verifySignatureOnDisputeResolution(contract *pb.RicardianContract) error {

	moderatorID, err := peer.IDB58Decode(contract.BuyerOrder.Payment.Moderator)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pubkey, err := n.IpfsNode.Routing.(*dht.IpfsDHT).GetPublicKey(ctx, moderatorID)
	if err != nil {
		log.Errorf("Failed to find public key for %s", moderatorID.Pretty())
		return err
	}
	pubKeyBytes, err := pubkey.Bytes()
	if err != nil {
		return err
	}

	if err := verifyMessageSignature(
		contract.DisputeResolution,
		pubKeyBytes,
		contract.Signatures,
		pb.Signature_DISPUTE_RESOLUTION,
		moderatorID.Pretty(),
	); err != nil {
		switch err.(type) {
		case noSigError:
			return errors.New("Contract does not contain a signature for the dispute resolution")
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

func (n *OpenBazaarNode) ReleaseFunds(contract *pb.RicardianContract, records []*spvwallet.TransactionRecord) error {
	// Create inputs
	var inputs []spvwallet.TransactionInput
	for _, o := range contract.DisputeResolution.Payout.Inputs {
		decodedHash, err := hex.DecodeString(o.Hash)
		if err != nil {
			return err
		}
		input := spvwallet.TransactionInput{
			OutpointHash:  decodedHash,
			OutpointIndex: o.Index,
		}
		inputs = append(inputs, input)
	}

	if len(inputs) == 0 {
		return errors.New("Transaction has no inputs")
	}

	// Create outputs
	var outputs []spvwallet.TransactionOutput
	if contract.DisputeResolution.Payout.BuyerOutput != nil {
		decodedScript, err := hex.DecodeString(contract.DisputeResolution.Payout.BuyerOutput.Script)
		if err != nil {
			return err
		}
		output := spvwallet.TransactionOutput{
			ScriptPubKey: decodedScript,
			Value:        int64(contract.DisputeResolution.Payout.BuyerOutput.Amount),
		}
		outputs = append(outputs, output)
	}
	if contract.DisputeResolution.Payout.VendorOutput != nil {
		decodedScript, err := hex.DecodeString(contract.DisputeResolution.Payout.VendorOutput.Script)
		if err != nil {
			return err
		}
		output := spvwallet.TransactionOutput{
			ScriptPubKey: decodedScript,
			Value:        int64(contract.DisputeResolution.Payout.VendorOutput.Amount),
		}
		outputs = append(outputs, output)
	}
	if contract.DisputeResolution.Payout.ModeratorOutput != nil {
		decodedScript, err := hex.DecodeString(contract.DisputeResolution.Payout.ModeratorOutput.Script)
		if err != nil {
			return err
		}
		output := spvwallet.TransactionOutput{
			ScriptPubKey: decodedScript,
			Value:        int64(contract.DisputeResolution.Payout.ModeratorOutput.Amount),
		}
		outputs = append(outputs, output)
	}

	// Create signing key
	parentFP := []byte{0x00, 0x00, 0x00, 0x00}
	chaincodeBytes, err := hex.DecodeString(contract.BuyerOrder.Payment.Chaincode)
	if err != nil {
		return err
	}
	mPrivKey := n.Wallet.MasterPrivateKey()
	if err != nil {
		return err
	}
	mECKey, err := mPrivKey.ECPrivKey()
	if err != nil {
		return err
	}
	hdKey := hd.NewExtendedKey(
		n.Wallet.Params().HDPrivateKeyID[:],
		mECKey.Serialize(),
		chaincodeBytes,
		parentFP,
		0,
		0,
		true)

	signingKey, err := hdKey.Child(0)
	if err != nil {
		return err
	}

	// Create signatures
	redeemScriptBytes, err := hex.DecodeString(contract.BuyerOrder.Payment.RedeemScript)
	if err != nil {
		return err
	}
	mySigs, err := n.Wallet.CreateMultisigSignature(inputs, outputs, signingKey, redeemScriptBytes, 0)
	if err != nil {
		return err
	}

	var moderatorSigs []spvwallet.Signature
	for _, sig := range contract.DisputeResolution.Payout.Sigs {
		s := spvwallet.Signature{
			Signature:  sig.Signature,
			InputIndex: sig.InputIndex,
		}
		moderatorSigs = append(moderatorSigs, s)
	}

	_, err = n.Wallet.Multisign(inputs, outputs, mySigs, moderatorSigs, redeemScriptBytes, 0, true)
	if err != nil {
		return err
	}
	return nil
}
