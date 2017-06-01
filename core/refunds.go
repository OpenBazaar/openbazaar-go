package core

import (
	"encoding/hex"
	"errors"

	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
)

func (n *OpenBazaarNode) RefundOrder(contract *pb.RicardianContract, records []*spvwallet.TransactionRecord) error {
	refundMsg := new(pb.Refund)
	orderId, err := n.CalcOrderId(contract.BuyerOrder)
	if err != nil {
		return err
	}
	refundMsg.OrderID = orderId
	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}
	refundMsg.Timestamp = ts
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

		refundAddress, err := n.Wallet.DecodeAddress(contract.BuyerOrder.RefundAddress)
		if err != nil {
			return err
		}
		var output spvwallet.TransactionOutput

		outputScript, err := n.Wallet.AddressToScript(refundAddress)
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
		refundMsg.Sigs = sigs
	} else {
		var outValue int64
		for _, r := range records {
			log.Notice(r.Value)
			if r.Value > 0 {
				outValue += r.Value
			}
		}
		refundAddr, err := n.Wallet.DecodeAddress(contract.BuyerOrder.RefundAddress)
		if err != nil {
			return err
		}
		_, err = n.Wallet.Spend(outValue, refundAddr, spvwallet.NORMAL)
		if err != nil {
			return err
		}
	}
	contract.Refund = refundMsg
	contract, err = n.SignRefund(contract)
	if err != nil {
		return err
	}
	n.SendRefund(contract.BuyerOrder.BuyerID.PeerID, contract)
	n.Datastore.Sales().Put(orderId, *contract, pb.OrderState_REFUNDED, true)
	return nil
}

func (n *OpenBazaarNode) SignRefund(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedRefund, err := proto.Marshal(contract.Refund)
	if err != nil {
		return contract, err
	}
	s := new(pb.Signature)
	s.Section = pb.Signature_REFUND
	if err != nil {
		return contract, err
	}
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedRefund)
	if err != nil {
		return contract, err
	}
	s.SignatureBytes = guidSig
	contract.Signatures = append(contract.Signatures, s)
	return contract, nil
}

func (n *OpenBazaarNode) VerifySignaturesOnRefund(contract *pb.RicardianContract) error {
	if err := verifyMessageSignature(
		contract.Refund,
		contract.VendorListings[0].VendorID.Pubkeys.Identity,
		contract.Signatures,
		pb.Signature_REFUND,
		contract.VendorListings[0].VendorID.PeerID,
	); err != nil {
		switch err.(type) {
		case noSigError:
			return errors.New("Contract does not contain a signature for the refund")
		case invalidSigError:
			return errors.New("Vendor's guid signature on contact failed to verify")
		case matchKeyError:
			return errors.New("Public key in order does not match reported vendor ID")
		default:
			return err
		}
	}
	return nil
}
