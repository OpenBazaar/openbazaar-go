package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/golang/protobuf/proto"
	crypto "gx/ipfs/QmUWER4r4qMvaCnX5zREcfyiWN7cXN9g3a7fkRqNz8qWPP/go-libp2p-crypto"
	"gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
)

func (n *OpenBazaarNode) FulfillOrder(fulfillment *pb.OrderFulfillment, contract *pb.RicardianContract, records []*spvwallet.TransactionRecord) error {
	rc := new(pb.RicardianContract)
	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
		payout := new(pb.OrderFulfillment_Payout)
		payout.PayoutAddress = n.Wallet.CurrentAddress(spvwallet.EXTERNAL).EncodeAddress()
		payout.PayoutFeePerByte = n.Wallet.GetFeePerByte(spvwallet.NORMAL)
		var ins []spvwallet.TransactionInput
		var outValue int64
		for _, r := range records {
			if !r.Spent && r.Value > 0 {
				outpointHash, err := hex.DecodeString(r.Txid)
				if err != nil {
					return err
				}
				outValue += r.Value
				in := spvwallet.TransactionInput{OutpointIndex: r.Index, OutpointHash: outpointHash}
				ins = append(ins, in)
			}
		}

		refundAddress, err := btcutil.DecodeAddress(contract.BuyerOrder.RefundAddress, n.Wallet.Params())
		if err != nil {
			return err
		}
		var output spvwallet.TransactionOutput

		outputScript, err := txscript.PayToAddrScript(refundAddress)
		if err != nil {
			return err
		}
		output.ScriptPubKey = outputScript
		output.Value = outValue

		chaincode, err := hex.DecodeString(contract.BuyerOrder.Payment.Chaincode)
		if err != nil {
			return err
		}
		parentFP := []byte{0x00, 0x00, 0x00, 0x00}
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
			chaincode,
			parentFP,
			0,
			0,
			true)

		vendorKey, err := hdKey.Child(0)
		if err != nil {
			return err
		}
		redeemScript, err := hex.DecodeString(contract.BuyerOrder.Payment.RedeemScript)

		signatures, err := n.Wallet.CreateMultisigSignature(ins, []spvwallet.TransactionOutput{output}, vendorKey, redeemScript, payout.PayoutFeePerByte)
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
	for i, listing := range contract.VendorListings {
		if listing.Slug == fulfillment.Slug {
			keyIndex = i
			break
		}
	}

	rs := new(pb.RatingSignature)
	metadata := new(pb.RatingSignature_TransactionMetadata)
	metadata.RatingKey = contract.BuyerOrder.RatingKeys[keyIndex]
	metadata.ListingSlug = fulfillment.Slug
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
	err = n.SendOrderFulfillment(contract.BuyerOrder.BuyerID.Guid, rc)
	if err != nil {
		return err
	}
	contract.VendorOrderFulfillment = append(contract.VendorOrderFulfillment, fulfillment)
	for _, sig := range rc.SignaturePairs {
		if sig.Section == pb.SignaturePair_ORDER_FULFILLMENT {
			contract.SignaturePairs = append(contract.SignaturePairs, sig)
		}
	}
	n.Datastore.Sales().Put(contract.VendorOrderConfirmation.OrderID, *contract, pb.OrderState_FULFILLED, false)
	return nil
}

func (n *OpenBazaarNode) SignOrderFulfillment(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedOrderFulfil, err := proto.Marshal(contract.VendorOrderFulfillment[0])
	if err != nil {
		return contract, err
	}
	s := new(pb.SignaturePair)
	s.Section = pb.SignaturePair_ORDER_FULFILLMENT
	if err != nil {
		return contract, err
	}
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedOrderFulfil)
	if err != nil {
		return contract, err
	}
	priv, err := n.Wallet.MasterPrivateKey().ECPrivKey()
	if err != nil {
		return contract, err
	}
	hashed := sha256.Sum256(serializedOrderFulfil)
	bitcoinSig, err := priv.Sign(hashed[:])
	if err != nil {
		return contract, err
	}
	s.Guid = guidSig
	s.Bitcoin = bitcoinSig.Serialize()
	contract.SignaturePairs = append(contract.SignaturePairs, s)
	return contract, nil
}

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

	pubkey, err := crypto.UnmarshalPublicKey(contract.VendorListings[0].VendorID.Pubkeys.Guid)
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
		_, err := btcutil.DecodeAddress(fulfillment.Payout.PayoutAddress, n.Wallet.Params())
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
	for i, fulfil := range contract.VendorOrderFulfillment {
		guidPubkeyBytes := contract.VendorListings[0].VendorID.Pubkeys.Guid
		bitcoinPubkeyBytes := contract.VendorListings[0].VendorID.Pubkeys.Bitcoin
		guid := contract.VendorListings[0].VendorID.Guid
		ser, err := proto.Marshal(fulfil)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(ser)
		guidPubkey, err := crypto.UnmarshalPublicKey(guidPubkeyBytes)
		if err != nil {
			return err
		}
		bitcoinPubkey, err := btcec.ParsePubKey(bitcoinPubkeyBytes, btcec.S256())
		if err != nil {
			return err
		}
		var guidSig []byte
		var bitcoinSig *btcec.Signature
		var sig *pb.SignaturePair
		sigExists := false
		a := 0
		for _, s := range contract.SignaturePairs {
			if s.Section == pb.SignaturePair_ORDER_FULFILLMENT {
				if a == i {
					sig = s
					sigExists = true
				}
				a++
				break
			}
		}
		if !sigExists {
			return errors.New("Contract does not contain a signature for the order fulfilment")
		}
		guidSig = sig.Guid
		bitcoinSig, err = btcec.ParseSignature(sig.Bitcoin, btcec.S256())
		if err != nil {
			return err
		}
		valid, err := guidPubkey.Verify(ser, guidSig)
		if err != nil {
			return err
		}
		if !valid {
			return errors.New("Vendor's guid signature on contact failed to verify")
		}
		checkKeyHash, err := guidPubkey.Hash()
		if err != nil {
			return err
		}
		guidMH, err := multihash.FromB58String(guid)
		if err != nil {
			return err
		}
		if !bytes.Equal(guidMH, checkKeyHash) {
			return errors.New("Public key in order does not match reported vendor ID")
		}
		valid = bitcoinSig.Verify(hash[:], bitcoinPubkey)
		if !valid {
			return errors.New("Vendors's bitcoin signature on contact failed to verify")
		}
	}
	return nil
}

func (n *OpenBazaarNode) IsFulfilled(contract *pb.RicardianContract) bool {
	if len(contract.VendorOrderFulfillment) < len(contract.VendorListings) {
		return false
	}
	return true
}
