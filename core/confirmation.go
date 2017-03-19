package core

import (
	"bytes"
	"encoding/hex"
	"errors"
	crypto "gx/ipfs/QmPGxZ1DP2w45WcogpW1h43BvseXbfke9N91qotpoQcUeS/go-libp2p-crypto"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	"time"
)

func (n *OpenBazaarNode) NewOrderConfirmation(contract *pb.RicardianContract, addressRequest bool) (*pb.RicardianContract, error) {
	oc := new(pb.OrderConfirmation)
	// Calculate order ID
	orderID, err := n.CalcOrderId(contract.BuyerOrder)
	if err != nil {
		return nil, err
	}
	oc.OrderID = orderID
	if addressRequest {
		addr := n.Wallet.NewAddress(spvwallet.EXTERNAL)
		oc.PaymentAddress = addr.EncodeAddress()
	}

	ts := new(timestamp.Timestamp)
	ts.Seconds = time.Now().Unix()
	ts.Nanos = 0
	oc.Timestamp = ts

	oc.RatingSignatures = []*pb.RatingSignature{}
	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
		for i, listing := range contract.VendorListings {
			buyerKey := contract.BuyerOrder.RatingKeys[i]
			moderatorKey, err := hex.DecodeString(ExtraModeratorKeyFromReddemScript(contract.BuyerOrder.Payment.RedeemScript))
			if err != nil {
				return nil, err
			}
			metadata := new(pb.RatingSignature_TransactionMetadata)
			metadata.ListingSlug = listing.Slug
			metadata.ModeratorKey = moderatorKey
			metadata.RatingKey = buyerKey

			ser, err := proto.Marshal(metadata)
			if err != nil {
				return nil, err
			}
			sig, err := n.IpfsNode.PrivateKey.Sign(ser)
			if err != nil {
				return nil, err
			}

			rs := new(pb.RatingSignature)
			rs.Metadata = metadata
			rs.Signature = sig

			oc.RatingSignatures = append(oc.RatingSignatures, rs)
		}
		oc.PaymentAddress = contract.BuyerOrder.Payment.Address
		oc.PayoutFee = n.Wallet.GetFeePerByte(spvwallet.NORMAL)
	}

	oc.RequestedAmount, err = n.CalculateOrderTotal(contract)
	if err != nil {
		return nil, err
	}
	contract.VendorOrderConfirmation = oc
	contract, err = n.SignOrderConfirmation(contract)
	if err != nil {
		return nil, err
	}
	return contract, nil
}

func (n *OpenBazaarNode) ConfirmOfflineOrder(contract *pb.RicardianContract, records []*spvwallet.TransactionRecord) error {
	contract, err := n.NewOrderConfirmation(contract, false)
	if err != nil {
		return err
	}
	if contract.BuyerOrder.Payment.Method != pb.Order_Payment_MODERATED {
		// Sweep the temp address into our wallet
		var utxos []spvwallet.Utxo
		for _, r := range records {
			if !r.Spent && r.Value > 0 {
				u := spvwallet.Utxo{}
				scriptBytes, err := hex.DecodeString(r.ScriptPubKey)
				if err != nil {
					return err
				}
				u.ScriptPubkey = scriptBytes
				hash, err := chainhash.NewHashFromStr(r.Txid)
				if err != nil {
					return err
				}
				outpoint := wire.NewOutPoint(hash, r.Index)
				u.Op = *outpoint
				u.Value = r.Value
				utxos = append(utxos, u)
			}
		}

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
		_, err = n.Wallet.SweepAddress(utxos, nil, vendorKey, &redeemScript, spvwallet.NORMAL)
		if err != nil {
			return err
		}
	}
	err = n.SendOrderConfirmation(contract.BuyerOrder.BuyerID.Guid, contract)
	if err != nil {
		return err
	}
	n.Datastore.Sales().Put(contract.VendorOrderConfirmation.OrderID, *contract, pb.OrderState_FUNDED, false)
	return nil
}

func (n *OpenBazaarNode) RejectOfflineOrder(contract *pb.RicardianContract, records []*spvwallet.TransactionRecord) error {
	orderId, err := n.CalcOrderId(contract.BuyerOrder)
	if err != nil {
		return err
	}
	rejectMsg := new(pb.OrderReject)
	rejectMsg.OrderID = orderId
	ts := new(timestamp.Timestamp)
	ts.Seconds = time.Now().Unix()
	ts.Nanos = 0
	rejectMsg.Timestamp = ts
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

		signatures, err := n.Wallet.CreateMultisigSignature(ins, []spvwallet.TransactionOutput{output}, vendorKey, redeemScript, contract.BuyerOrder.RefundFee)
		if err != nil {
			return err
		}
		var sigs []*pb.BitcoinSignature
		for _, s := range signatures {
			pbSig := &pb.BitcoinSignature{Signature: s.Signature, InputIndex: s.InputIndex}
			sigs = append(sigs, pbSig)
		}
		rejectMsg.Sigs = sigs
	}
	err = n.SendReject(contract.BuyerOrder.BuyerID.Guid, rejectMsg)
	if err != nil {
		return err
	}
	n.Datastore.Sales().Put(orderId, *contract, pb.OrderState_REJECTED, true)
	return nil
}

func (n *OpenBazaarNode) ValidateOrderConfirmation(contract *pb.RicardianContract, validateAddress bool) error {
	orderID, err := n.CalcOrderId(contract.BuyerOrder)
	if err != nil {
		return err
	}
	if contract.VendorOrderConfirmation.OrderID != orderID {
		return errors.New("Vendor's response contained invalid order ID")
	}
	if contract.VendorOrderConfirmation.RequestedAmount != contract.BuyerOrder.Payment.Amount {
		return errors.New("Vendor requested an amount different from what we calculated")
	}
	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
		keyExists := func(a []byte, list [][]byte) bool {
			for _, b := range list {
				if bytes.Equal(b, a) {
					return true
				}
			}
			return false
		}
		for _, sig := range contract.VendorOrderConfirmation.RatingSignatures {
			exists := false
			for _, listing := range contract.VendorListings {
				if sig.Metadata.ListingSlug == listing.Slug {
					exists = true
				}
			}
			if !exists {
				return errors.New("Rating signatures do not cover all purchased listings")
			}
			pubkey, err := crypto.UnmarshalPublicKey(contract.VendorListings[0].VendorID.Pubkeys.Guid)
			if err != nil {
				return err
			}
			moderatorKey, err := hex.DecodeString(ExtraModeratorKeyFromReddemScript(contract.BuyerOrder.Payment.RedeemScript))
			if err != nil {
				return err
			}
			if !keyExists(sig.Metadata.RatingKey, contract.BuyerOrder.RatingKeys) {
				return errors.New("Not all ratings keys are signed by ratings signatures")
			}

			if !bytes.Equal(sig.Metadata.ModeratorKey, moderatorKey) {
				return errors.New("Rating signature does not contain moderatory key")
			}
			ser, err := proto.Marshal(sig.Metadata)
			if err != nil {
				return err
			}
			valid, err := pubkey.Verify(ser, sig.Signature)
			if err != nil || !valid {
				return errors.New("Failed to verify signature on rating keys")
			}
		}
	}
	if validateAddress {
		_, err = btcutil.DecodeAddress(contract.VendorOrderConfirmation.PaymentAddress, n.Wallet.Params())
		if err != nil {
			return err
		}
	}
	err = verifySignaturesOnOrderConfirmation(contract)
	if err != nil {
		return err
	}
	return nil
}

func (n *OpenBazaarNode) SignOrderConfirmation(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedOrderConf, err := proto.Marshal(contract.VendorOrderConfirmation)
	if err != nil {
		return contract, err
	}
	s := new(pb.Signature)
	s.Section = pb.Signature_ORDER_CONFIRMATION
	if err != nil {
		return contract, err
	}
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedOrderConf)
	if err != nil {
		return contract, err
	}
	s.SignatureBytes = guidSig
	contract.Signatures = append(contract.Signatures, s)
	return contract, nil
}

func verifySignaturesOnOrderConfirmation(contract *pb.RicardianContract) error {
	if err := verifyMessageSignature(
		contract.VendorOrderConfirmation,
		contract.VendorListings[0].VendorID.Pubkeys.Guid,
		contract.Signatures,
		pb.Signature_ORDER_CONFIRMATION,
		contract.VendorListings[0].VendorID.Guid,
	); err != nil {
		switch err.(type) {
		case noSigError:
			return errors.New("Contract does not contain a signature for the order confirmation")
		case invalidSigError:
			return errors.New("Vendor's guid signature on contact failed to verify")
		case matchKeyError:
			return errors.New("Public key in order confirmation does not match reported vendor ID")
		default:
			return err
		}
	}
	return nil
}

func ExtraModeratorKeyFromReddemScript(redeemScript string) string {
	return redeemScript[134:200]
}
