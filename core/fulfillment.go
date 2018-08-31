package core

import (
	"bytes"
	"encoding/hex"
	"errors"
	"time"

	"github.com/OpenBazaar/wallet-interface"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"

	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

var (
	// MaxTXIDSize - max length for order txnID
	MaxTXIDSize = 512
)

// FulfillOrder - fulfill the order
func (n *OpenBazaarNode) FulfillOrder(fulfillment *pb.OrderFulfillment, contract *pb.RicardianContract, records []*wallet.TransactionRecord) error {
	if fulfillment.Slug == "" && len(contract.VendorListings) == 1 {
		fulfillment.Slug = contract.VendorListings[0].Slug
	} else if fulfillment.Slug == "" && len(contract.VendorListings) > 1 {
		return errors.New("Slug must be specified when an order contains multiple items")
	}
	rc := new(pb.RicardianContract)
	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
		payout := new(pb.OrderFulfillment_Payout)
		currentAddress := n.Wallet.CurrentAddress(wallet.EXTERNAL)
		payout.PayoutAddress = currentAddress.EncodeAddress()
		payout.PayoutFeePerByte = n.Wallet.GetFeePerByte(wallet.NORMAL)
		var ins []wallet.TransactionInput
		var outValue int64
		for _, r := range records {
			if !r.Spent && r.Value > 0 {
				outpointHash, err := hex.DecodeString(r.Txid)
				if err != nil {
					return err
				}
				outValue += r.Value
				in := wallet.TransactionInput{OutpointIndex: r.Index, OutpointHash: outpointHash, Value: r.Value}
				ins = append(ins, in)
			}
		}

		var output = wallet.TransactionOutput{
			Address: currentAddress,
			Value:   outValue,
		}
		chaincode, err := hex.DecodeString(contract.BuyerOrder.Payment.Chaincode)
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
		vendorKey, err := n.Wallet.ChildKey(mECKey.Serialize(), chaincode, true)
		if err != nil {
			return err
		}
		redeemScript, err := hex.DecodeString(contract.BuyerOrder.Payment.RedeemScript)
		if err != nil {
			return err
		}

		signatures, err := n.Wallet.CreateMultisigSignature(ins, []wallet.TransactionOutput{output}, vendorKey, redeemScript, payout.PayoutFeePerByte)
		if err != nil {
			return err
		}
		var sigs []*pb.BitcoinSignature
		for _, s := range signatures {
			pbSig := &pb.BitcoinSignature{Signature: s.Signature, InputIndex: s.InputIndex}
			sigs = append(sigs, pbSig)
		}
		payout.Sigs = sigs
		fulfillment.Payout = payout
	}
	var keyIndex int
	var listing *pb.Listing
	for i, list := range contract.VendorListings {
		if list.Slug == fulfillment.Slug {
			keyIndex = i
			listing = list
			break
		}
	}

	if listing.Metadata.ContractType == pb.Listing_Metadata_CRYPTOCURRENCY {
		err := validateCryptocurrencyFulfillment(fulfillment)
		if err != nil {
			return err
		}
	}

	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}
	fulfillment.Timestamp = ts

	rs := new(pb.RatingSignature)
	metadata := new(pb.RatingSignature_TransactionMetadata)
	metadata.RatingKey = contract.BuyerOrder.RatingKeys[keyIndex]
	metadata.ListingSlug = fulfillment.Slug

	if contract.BuyerOrder.Version > 0 {
		metadata.ListingTitle = listing.Item.Title
		if len(listing.Item.Images) > 0 {
			metadata.Thumbnail = &pb.RatingSignature_TransactionMetadata_Image{
				Tiny:     listing.Item.Images[0].Tiny,
				Small:    listing.Item.Images[0].Small,
				Medium:   listing.Item.Images[0].Medium,
				Large:    listing.Item.Images[0].Large,
				Original: listing.Item.Images[0].Original,
			}
		}
	}

	ser, err := proto.Marshal(metadata)
	if err != nil {
		return err
	}
	signature, err := n.IpfsNode.PrivateKey.Sign(ser)
	if err != nil {
		return err
	}
	rs.Metadata = metadata
	rs.Signature = signature

	fulfillment.RatingSignature = rs

	fulfils := []*pb.OrderFulfillment{}

	rc.VendorOrderFulfillment = append(fulfils, fulfillment)
	rc, err = n.SignOrderFulfillment(rc)
	if err != nil {
		return err
	}
	buyerkey, err := crypto.UnmarshalPublicKey(contract.BuyerOrder.BuyerID.Pubkeys.Identity)
	if err != nil {
		return err
	}
	err = n.SendOrderFulfillment(contract.BuyerOrder.BuyerID.PeerID, &buyerkey, rc)
	if err != nil {
		return err
	}
	contract.VendorOrderFulfillment = append(contract.VendorOrderFulfillment, fulfillment)
	for _, sig := range rc.Signatures {
		if sig.Section == pb.Signature_ORDER_FULFILLMENT {
			contract.Signatures = append(contract.Signatures, sig)
		}
	}
	if n.IsFulfilled(rc) {
		n.Datastore.Sales().Put(contract.VendorOrderConfirmation.OrderID, *contract, pb.OrderState_FULFILLED, false)
	} else {
		n.Datastore.Sales().Put(contract.VendorOrderConfirmation.OrderID, *contract, pb.OrderState_PARTIALLY_FULFILLED, false)
	}
	return nil
}

// SignOrderFulfillment - add signature to order fulfillment
func (n *OpenBazaarNode) SignOrderFulfillment(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedOrderFulfil, err := proto.Marshal(contract.VendorOrderFulfillment[0])
	if err != nil {
		return contract, err
	}
	s := new(pb.Signature)
	s.Section = pb.Signature_ORDER_FULFILLMENT
	if err != nil {
		return contract, err
	}
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedOrderFulfil)
	if err != nil {
		return contract, err
	}
	s.SignatureBytes = guidSig
	contract.Signatures = append(contract.Signatures, s)
	return contract, nil
}

// ValidateOrderFulfillment - validate order details
func (n *OpenBazaarNode) ValidateOrderFulfillment(fulfillment *pb.OrderFulfillment, contract *pb.RicardianContract) error {
	if err := verifySignaturesOnOrderFulfilment(contract); err != nil {
		return err
	}

	slugExists := func(a string, list []string) bool {
		for _, b := range list {
			if b == a {
				return true
			}
		}
		return false
	}
	keyExists := func(a []byte, list [][]byte) bool {
		for _, b := range list {
			if bytes.Equal(b, a) {
				return true
			}
		}
		return false
	}

	var listingSlugs []string
	for _, listing := range contract.VendorListings {
		listingSlugs = append(listingSlugs, listing.Slug)
	}
	if !slugExists(fulfillment.Slug, listingSlugs) {
		return errors.New("Slug in rating signature does not exist in order")
	}
	if !keyExists(fulfillment.RatingSignature.Metadata.RatingKey, contract.BuyerOrder.RatingKeys) {
		return errors.New("Rating key in vendor's rating signature is invalid")
	}

	pubkey, err := crypto.UnmarshalPublicKey(contract.VendorListings[0].VendorID.Pubkeys.Identity)
	if err != nil {
		return err
	}

	ser, err := proto.Marshal(fulfillment.RatingSignature.Metadata)
	if err != nil {
		return err
	}
	valid, err := pubkey.Verify(ser, fulfillment.RatingSignature.Signature)
	if err != nil || !valid {
		return errors.New("Failed to verify signature on rating keys")
	}

	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
		if fulfillment.Payout == nil {
			return errors.New("Payout object for multisig is nil")
		}
		_, err := n.Wallet.DecodeAddress(fulfillment.Payout.PayoutAddress)
		if err != nil {
			return errors.New("Invalid payout address")
		}
	}
	if n.IsFulfilled(contract) {
		var listingSlugs []string
		for _, listing := range contract.VendorListings {
			listingSlugs = append(listingSlugs, listing.Slug)
		}
		var ratingSlugs []string
		for _, fulfil := range contract.VendorOrderFulfillment {
			ratingSlugs = append(ratingSlugs, fulfil.RatingSignature.Metadata.ListingSlug)
		}
		for _, ls := range listingSlugs {
			if !slugExists(ls, ratingSlugs) {
				return errors.New("Vendor failed to send rating signatures covering all purchased listings")
			}
		}
		var vendorSignedKeys [][]byte
		for _, fulfil := range contract.VendorOrderFulfillment {
			vendorSignedKeys = append(vendorSignedKeys, fulfil.RatingSignature.Metadata.RatingKey)
		}
		for _, bk := range contract.BuyerOrder.RatingKeys {
			if !keyExists(bk, vendorSignedKeys) {
				return errors.New("Vendor failed to send rating signatures covering all ratingKeys")
			}
		}
	}
	return nil
}

func verifySignaturesOnOrderFulfilment(contract *pb.RicardianContract) error {
	for _, fulfil := range contract.VendorOrderFulfillment {
		if err := verifyMessageSignature(
			fulfil,
			contract.VendorListings[0].VendorID.Pubkeys.Identity,
			contract.Signatures,
			pb.Signature_ORDER_FULFILLMENT,
			contract.VendorListings[0].VendorID.PeerID,
		); err != nil {
			switch err.(type) {
			case noSigError:
				return errors.New("Contract does not contain a signature for the order fulfilment")
			case invalidSigError:
				return errors.New("Vendor's guid signature on contact failed to verify")
			case matchKeyError:
				return errors.New("Public key in order does not match reported vendor ID")
			default:
				return err
			}
		}
	}
	return nil
}

func validateCryptocurrencyFulfillment(fulfillment *pb.OrderFulfillment) error {
	if len(fulfillment.PhysicalDelivery)+len(fulfillment.DigitalDelivery) > 0 {
		return ErrFulfillIncorrectDeliveryType
	}

	for _, delivery := range fulfillment.CryptocurrencyDelivery {
		if delivery.TransactionID == "" {
			return ErrFulfillCryptocurrencyTXIDNotFound
		}
		if len(delivery.TransactionID) > MaxTXIDSize {
			return ErrFulfillCryptocurrencyTXIDTooLong
		}
	}

	return nil
}

// IsFulfilled - check is order is fulfilled
func (n *OpenBazaarNode) IsFulfilled(contract *pb.RicardianContract) bool {
	return len(contract.VendorOrderFulfillment) >= len(contract.VendorListings)
}
