package core

import (
	"encoding/hex"
	"errors"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"math/big"
	"strings"
	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
)

// RefundOrder - refund buyer
func (n *OpenBazaarNode) RefundOrder(contract *pb.RicardianContract, records []*wallet.TransactionRecord) error {
	refundMsg := new(pb.Refund)
	orderID, err := n.CalcOrderID(contract.BuyerOrder)
	if err != nil {
		return err
	}
	refundMsg.OrderID = orderID
	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}
	refundMsg.Timestamp = ts
	order, err := repo.ToV5Order(contract.BuyerOrder, n.LookupCurrency)
	if err != nil {
		return err
	}
	wal, err := n.Multiwallet.WalletForCurrencyCode(order.Payment.AmountCurrency.Code)
	if err != nil {
		return err
	}
	escrowWallet, ok := wal.(wallet.EscrowWallet)
	if !ok {
		return errors.New("wallet does not support escrow")
	}
	if order.Payment.Method == pb.Order_Payment_MODERATED {
		var ins []wallet.TransactionInput
		outValue := big.NewInt(0)
		for _, r := range records {
			if !r.Spent && r.Value.Cmp(big.NewInt(0)) > 0 {
				outpointHash, err := hex.DecodeString(strings.TrimPrefix(r.Txid, "0x"))
				if err != nil {
					return err
				}
				outValue = new(big.Int).Add(outValue, &r.Value)
				in := wallet.TransactionInput{OutpointIndex: r.Index, OutpointHash: outpointHash, Value: r.Value}
				ins = append(ins, in)
			}
		}

		refundAddress, err := wal.DecodeAddress(order.RefundAddress)
		if err != nil {
			return err
		}
		output := wallet.TransactionOutput{
			Address: refundAddress,
			Value:   *outValue,
		}

		chaincode, err := hex.DecodeString(order.Payment.Chaincode)
		if err != nil {
			return err
		}
		mECKey, err := n.MasterPrivateKey.ECPrivKey()
		if err != nil {
			return err
		}
		vendorKey, err := wal.ChildKey(mECKey.Serialize(), chaincode, true)
		if err != nil {
			return err
		}
		redeemScript, err := hex.DecodeString(order.Payment.RedeemScript)
		if err != nil {
			return err
		}
		f, _ := new(big.Int).SetString(order.BigRefundFee, 10)
		signatures, err := escrowWallet.CreateMultisigSignature(ins, []wallet.TransactionOutput{output}, vendorKey, redeemScript, *f)
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
		outValue := big.NewInt(0)
		for _, r := range records {
			if r.Value.Cmp(big.NewInt(0)) > 0 {
				outValue = new(big.Int).Add(outValue, &r.Value)
			}
		}
		refundAddr, err := wal.DecodeAddress(order.RefundAddress)
		if err != nil {
			return err
		}
		txid, err := wal.Spend(*outValue, refundAddr, wallet.NORMAL, orderID, false)
		if err != nil {
			return err
		}
		txinfo := new(pb.Refund_TransactionInfo)
		txinfo.Txid = txid
		txinfo.BigValue = outValue.String()
		txinfo.ValueCurrency = contract.BuyerOrder.Payment.AmountCurrency
		refundMsg.RefundTransaction = txinfo
	}
	contract.Refund = refundMsg
	contract, err = n.SignRefund(contract)
	if err != nil {
		return err
	}
	err = n.SendRefund(order.BuyerID.PeerID, contract)
	if err != nil {
		// TODO: do we retry a failed refund send?
		log.Error(err)
	}
	err = n.Datastore.Sales().Put(orderID, *contract, pb.OrderState_REFUNDED, true)
	if err != nil {
		log.Error(err)
	}
	return nil
}

// SignRefund - add signature to refund
func (n *OpenBazaarNode) SignRefund(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedRefund, err := proto.Marshal(contract.Refund)
	if err != nil {
		return contract, err
	}
	s := new(pb.Signature)
	s.Section = pb.Signature_REFUND
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedRefund)
	if err != nil {
		return contract, err
	}
	s.SignatureBytes = guidSig
	contract.Signatures = append(contract.Signatures, s)
	return contract, nil
}

// VerifySignaturesOnRefund - verify signatures on refund
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
			return errors.New("contract does not contain a signature for the refund")
		case invalidSigError:
			return errors.New("vendor's guid signature on contact failed to verify")
		case matchKeyError:
			return errors.New("public key in order does not match reported vendor ID")
		default:
			return err
		}
	}
	return nil
}
