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

func (n *OpenBazaarNode) RefundOrder(contract *pb.RicardianContract, records []*spvwallet.TransactionRecord) error {
	refundMsg := new(pb.Refund)
	orderId, err := n.CalculateOrderId(contract.BuyerOrder)
	if err != nil {
		return err
	}
	refundMsg.OrderID = orderId
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
		refundMsg.Sigs = sigs
	} else {
		var outValue int64
		for _, r := range records {
			log.Notice(r.Value)
			if r.Value > 0 {
				outValue += r.Value
			}
		}
		refundAddr, err := btcutil.DecodeAddress(contract.BuyerOrder.RefundAddress, n.Wallet.Params())
		if err != nil {
			return err
		}
		err = n.Wallet.Spend(outValue, refundAddr, spvwallet.NORMAL)
		if err != nil {
			return err
		}
	}
	contract.Refund = refundMsg
	contract, err = n.SignRefund(contract)
	if err != nil {
		return err
	}
	n.SendRefund(contract.BuyerOrder.BuyerID.Guid, contract)
	n.Datastore.Sales().Put(orderId, *contract, pb.OrderState_REFUNDED, true)
	return nil
}

func (n *OpenBazaarNode) SignRefund(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedRefund, err := proto.Marshal(contract.Refund)
	if err != nil {
		return contract, err
	}
	s := new(pb.Signatures)
	s.Section = pb.Signatures_REFUND
	if err != nil {
		return contract, err
	}
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedRefund)
	if err != nil {
		return contract, err
	}
	priv, err := n.Wallet.MasterPrivateKey().ECPrivKey()
	if err != nil {
		return contract, err
	}
	hashed := sha256.Sum256(serializedRefund)
	bitcoinSig, err := priv.Sign(hashed[:])
	if err != nil {
		return contract, err
	}
	s.Guid = guidSig
	s.Bitcoin = bitcoinSig.Serialize()
	contract.Signatures = append(contract.Signatures, s)
	return contract, nil
}

func (n *OpenBazaarNode) VerifySignaturesOnRefund(contract *pb.RicardianContract) error {
	guidPubkeyBytes := contract.VendorListings[0].VendorID.Pubkeys.Guid
	bitcoinPubkeyBytes := contract.VendorListings[0].VendorID.Pubkeys.Bitcoin
	guid := contract.VendorListings[0].VendorID.Guid
	ser, err := proto.Marshal(contract.Refund)
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
		if s.Section == pb.Signatures_REFUND {
			sig = s
			sigExists = true
			break
		}
	}
	if !sigExists {
		return errors.New("Contract does not contain a signature for the refund")
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
		return errors.New("Vendor's GUID signature on contact failed to verify")
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

	return nil
}
