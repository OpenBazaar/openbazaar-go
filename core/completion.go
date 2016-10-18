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
	"github.com/golang/protobuf/ptypes/timestamp"
	"gx/ipfs/QmT6n4mspWYEya864BhCUJEgyxiRfmiSY9ruQwTUNpRKaM/protobuf/proto"
	crypto "gx/ipfs/QmUWER4r4qMvaCnX5zREcfyiWN7cXN9g3a7fkRqNz8qWPP/go-libp2p-crypto"
	"gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	"time"
)

const (
	RatingMin           = 1
	RatingMax           = 5
	ReviewMaxCharacters = 3000
)

type RatingData struct {
	OrderId         string `json:"orderId"`
	Overall         int    `json:"overall"`
	Quality         int    `json:"quality"`
	Description     int    `json:"description"`
	DeliverySpeed   int    `json:"deliverySpeed"`
	CustomerService int    `json:"customerService"`
	Review          string `json:"rewview"`
	Anonymous       bool   `json:"anonymous"`
}

func (n *OpenBazaarNode) CompleteOrder(ratingData *RatingData, contract *pb.RicardianContract, records []*spvwallet.TransactionRecord) error {

	orderId, err := n.CalcOrderId(contract.BuyerOrder)
	if err != nil {
		return err
	}

	oc := new(pb.OrderCompletion)
	oc.OrderId = orderId

	rating := new(pb.OrderCompletion_Rating)

	rd := new(pb.OrderCompletion_Rating_RatingData)
	rd.RatingKey = contract.BuyerOrder.RatingKey
	if !ratingData.Anonymous {
		rd.BuyerID = contract.BuyerOrder.BuyerID
	}
	rd.VendorID = contract.VendorListings[0].VendorID

	// TODO: when this is a closing of a disputed order we need to use the rating signature
	// from the order confirmation and add the moderators signature
	rd.VendorSig = contract.VendorOrderFulfillment[0].RatingSignature

	rd.Overall = uint32(ratingData.Overall)
	rd.Quality = uint32(ratingData.Quality)
	rd.Description = uint32(ratingData.Description)
	rd.CustomerService = uint32(ratingData.CustomerService)
	rd.DeliverySpeed = uint32(ratingData.DeliverySpeed)
	rd.Review = ratingData.Review

	ts := new(timestamp.Timestamp)
	ts.Seconds = time.Now().Unix()
	ts.Nanos = 0
	rd.Timestamp = ts
	rating.RatingData = rd

	ser, err := proto.Marshal(rating.RatingData)
	if err != nil {
		return err
	}

	ratingKey, err := n.Wallet.MasterPrivateKey().Child(uint32(contract.BuyerOrder.Timestamp.Seconds))
	if err != nil {
		return err
	}
	ecRatingKey, err := ratingKey.ECPrivKey()
	if err != nil {
		return err
	}
	sig, err := ecRatingKey.Sign(ser)
	if err != nil {
		return err
	}
	rating.Signature = sig.Serialize()
	oc.Rating = rating

	// Payout order if moderated
	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
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

		payoutAddress, err := btcutil.DecodeAddress(contract.VendorOrderFulfillment[0].Payout.PayoutAddress, n.Wallet.Params())
		if err != nil {
			return err
		}
		var output spvwallet.TransactionOutput
		outputScript, err := txscript.PayToAddrScript(payoutAddress)
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

		buyerKey, err := hdKey.Child(0)
		if err != nil {
			return err
		}
		redeemScript, err := hex.DecodeString(contract.BuyerOrder.Payment.RedeemScript)
		if err != nil {
			return err
		}

		buyerSignatures, err := n.Wallet.CreateMultisigSignature(ins, []spvwallet.TransactionOutput{output}, buyerKey, redeemScript, contract.VendorOrderFulfillment[0].Payout.PayoutFeePerByte)
		if err != nil {
			return err
		}
		var pbSigs []*pb.BitcoinSignature
		for _, s := range buyerSignatures {
			sig := new(pb.BitcoinSignature)
			sig.InputIndex = s.InputIndex
			sig.Signature = s.Signature
			pbSigs = append(pbSigs, sig)
		}
		oc.PayoutSigs = pbSigs
		var vendorSignatures []spvwallet.Signature
		for _, s := range contract.VendorOrderFulfillment[0].Payout.Sigs {
			sig := spvwallet.Signature{InputIndex: s.InputIndex, Signature: s.Signature}
			vendorSignatures = append(vendorSignatures, sig)
		}
		err = n.Wallet.Multisign(ins, []spvwallet.TransactionOutput{output}, buyerSignatures, vendorSignatures, redeemScript, contract.VendorOrderFulfillment[0].Payout.PayoutFeePerByte)
		if err != nil {
			return err
		}
	}

	rc := new(pb.RicardianContract)
	rc.BuyerOrderCompletion = oc

	rc, err = n.SignOrderCompletion(rc)
	if err != nil {
		return err
	}

	err = n.SendOrderCompletion(contract.VendorListings[0].VendorID.Guid, rc)
	if err != nil {
		return err
	}

	// TODO: commit rating to ipfs

	contract.BuyerOrderCompletion = oc
	for _, sig := range rc.Signatures {
		if sig.Section == pb.Signatures_ORDER_COMPLETION {
			contract.Signatures = append(contract.Signatures, sig)
		}
	}
	err = n.Datastore.Purchases().Put(orderId, *contract, pb.OrderState_COMPLETE, true)
	if err != nil {
		return err
	}
	return nil
}

func (n *OpenBazaarNode) SignOrderCompletion(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedOrderFulfil, err := proto.Marshal(contract.BuyerOrderCompletion)
	if err != nil {
		return contract, err
	}
	s := new(pb.Signatures)
	s.Section = pb.Signatures_ORDER_COMPLETION
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
	contract.Signatures = append(contract.Signatures, s)
	return contract, nil
}

func (n *OpenBazaarNode) ValidateOrderCompletion(contract *pb.RicardianContract) error {
	if err := verifySignaturesOnOrderCompletion(contract); err != nil {
		return err
	}

	// TODO: validate bitcoin signatures of moderated

	return nil
}

func verifySignaturesOnOrderCompletion(contract *pb.RicardianContract) error {
	guidPubkeyBytes := contract.BuyerOrder.BuyerID.Pubkeys.Guid
	bitcoinPubkeyBytes := contract.BuyerOrder.BuyerID.Pubkeys.Bitcoin
	guid := contract.BuyerOrder.BuyerID.Guid
	ser, err := proto.Marshal(contract.BuyerOrderCompletion)
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
	var sig *pb.Signatures
	sigExists := false
	for _, s := range contract.Signatures {
		if s.Section == pb.Signatures_ORDER_COMPLETION {
			sig = s
			sigExists = true
			break
		}
	}
	if !sigExists {
		return errors.New("Contract does not contain a signature for the order completion")
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
		return errors.New("Buyer's guid signature on contact failed to verify")
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
		return errors.New("Public key in order does not match reported buyer ID")
	}
	valid = bitcoinSig.Verify(hash[:], bitcoinPubkey)
	if !valid {
		return errors.New("Buyer's bitcoin signature on contact failed to verify")
	}

	return nil
}
