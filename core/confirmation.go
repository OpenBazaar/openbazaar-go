package core

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"

	crypto "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"

	"github.com/OpenBazaar/wallet-interface"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

// NewOrderConfirmation - add order confirmation to the contract
func (n *OpenBazaarNode) NewOrderConfirmation(contract *pb.RicardianContract, addressRequest bool) (*pb.RicardianContract, error) {
	oc := new(pb.OrderConfirmation)
	// Calculate order ID
	orderID, err := n.CalcOrderID(contract.BuyerOrder)
	if err != nil {
		return nil, err
	}
	oc.OrderID = orderID

	order, err := repo.ToV5Order(contract.BuyerOrder, n.LookupCurrency)
	if err != nil {
		return nil, err
	}

	wal, err := n.Multiwallet.WalletForCurrencyCode(order.Payment.AmountCurrency.Code)
	if err != nil {
		return nil, err
	}

	if addressRequest {
		addr := wal.NewAddress(wallet.EXTERNAL)
		oc.PaymentAddress = addr.String()
	}

	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return nil, err
	}
	oc.Timestamp = ts

	oc.RatingSignatures = []*pb.RatingSignature{}
	if order.Payment.Method == pb.Order_Payment_MODERATED {
		for _, listing := range contract.VendorListings {
			metadata := new(pb.RatingSignature_TransactionMetadata)
			metadata.ListingSlug = listing.Slug
			metadata.ModeratorKey = order.Payment.ModeratorKey

			if order.Version > 0 {
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
		oc.PaymentAddress = order.Payment.Address
	}

	oc.BigRequestedAmount = order.Payment.BigAmount
	contract.VendorOrderConfirmation = oc
	contract, err = n.SignOrderConfirmation(contract)
	if err != nil {
		return nil, err
	}
	return contract, nil
}

// ConfirmOfflineOrder - confirm offline order
func (n *OpenBazaarNode) ConfirmOfflineOrder(oldState pb.OrderState, contract *pb.RicardianContract, records []*wallet.TransactionRecord) error {
	confirmedContract, err := n.NewOrderConfirmation(contract, false)
	if err != nil {
		return err
	}

	order, err := repo.ToV5Order(contract.BuyerOrder, n.LookupCurrency)
	if err != nil {
		return err
	}

	wal, err := n.Multiwallet.WalletForCurrencyCode(order.Payment.AmountCurrency.Code)
	if err != nil {
		return err
	}
	err = n.Datastore.Sales().Put(confirmedContract.VendorOrderConfirmation.OrderID, *confirmedContract, pb.OrderState_AWAITING_FULFILLMENT, false)
	if err != nil {
		return err
	}

	recoverState := func() {
		err = n.Datastore.Sales().Put(confirmedContract.VendorOrderConfirmation.OrderID, *contract, oldState, false)
		if err != nil {
			log.Errorf("failed to recover state on order (%s): %s", confirmedContract.VendorOrderConfirmation.OrderID, err.Error())
		}
	}

	if confirmedContract.BuyerOrder.Payment.Method != pb.Order_Payment_MODERATED {
		escrowWallet, ok := wal.(wallet.EscrowWallet)
		if !ok {
			return errors.New("wallet does not support escrow")
		}
		// Sweep the temp address into our wallet
		var txInputs []wallet.TransactionInput
		for _, r := range records {
			if !r.Spent && r.Value.Cmp(big.NewInt(0)) > 0 {
				addr, err := wal.DecodeAddress(r.Address)
				if err != nil {
					recoverState()
					return err
				}
				outpointHash, err := hex.DecodeString(strings.TrimPrefix(r.Txid, "0x"))
				if err != nil {
					recoverState()
					return fmt.Errorf("decoding transaction hash: %s", err.Error())
				}
				txInput := wallet.TransactionInput{
					LinkedAddress: addr,
					OutpointIndex: r.Index,
					OutpointHash:  outpointHash,
					Value:         r.Value,
				}
				txInputs = append(txInputs, txInput)
			}
		}

		if len(txInputs) == 0 {
			recoverState()
			return errors.New("no unspent transactions found to fund order")
		}

		chaincode, err := hex.DecodeString(confirmedContract.BuyerOrder.Payment.Chaincode)
		if err != nil {
			recoverState()
			return err
		}
		mECKey, err := n.MasterPrivateKey.ECPrivKey()
		if err != nil {
			recoverState()
			return err
		}
		vendorKey, err := wal.ChildKey(mECKey.Serialize(), chaincode, true)
		if err != nil {
			recoverState()
			return err
		}
		redeemScript, err := hex.DecodeString(confirmedContract.BuyerOrder.Payment.RedeemScript)
		if err != nil {
			recoverState()
			return err
		}
		_, err = escrowWallet.SweepAddress(txInputs, nil, vendorKey, &redeemScript, wallet.NORMAL)
		if err != nil {
			recoverState()
			return err
		}
	}
	err = n.SendOrderConfirmation(confirmedContract.BuyerOrder.BuyerID.PeerID, confirmedContract)
	if err != nil {
		// TODO: local order state is accurate, remote order state needs to be retransmitted
		log.Errorf("failed sending confirmation for order (%s): %s", confirmedContract.VendorOrderConfirmation.OrderID, err.Error())
		return nil
	}
	return nil
}

// RejectOfflineOrder - reject offline order
func (n *OpenBazaarNode) RejectOfflineOrder(contract *pb.RicardianContract, records []*wallet.TransactionRecord) error {
	orderID, err := n.CalcOrderID(contract.BuyerOrder)
	if err != nil {
		return fmt.Errorf("generate order id: %s", err.Error())
	}
	rejectMsg := new(pb.OrderReject)
	rejectMsg.OrderID = orderID
	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return fmt.Errorf("marshal timestamp: %s", err.Error())
	}
	order, err := repo.ToV5Order(contract.BuyerOrder, n.LookupCurrency)
	if err != nil {
		return err
	}
	wal, err := n.Multiwallet.WalletForCurrencyCode(order.Payment.AmountCurrency.Code)
	if err != nil {
		return err
	}
	rejectMsg.Timestamp = ts
	if order.Payment.Method == pb.Order_Payment_MODERATED {
		escrowWallet, ok := wal.(wallet.EscrowWallet)
		if !ok {
			return errors.New("wallet does not support escrow")
		}
		var ins []wallet.TransactionInput
		outValue := *big.NewInt(0)
		for _, r := range records {
			if !r.Spent && r.Value.Cmp(big.NewInt(0)) > 0 {
				addr, err := wal.DecodeAddress(r.Address)
				if err != nil {
					return fmt.Errorf("decode prior transactions address: %s", err.Error())
				}
				outpointHash, err := hex.DecodeString(strings.TrimPrefix(r.Txid, "0x"))
				if err != nil {
					return fmt.Errorf("decoding transaction hash: %s", err.Error())
				}
				in := wallet.TransactionInput{
					LinkedAddress: addr,
					OutpointIndex: r.Index,
					OutpointHash:  outpointHash,
					Value:         r.Value,
				}
				ins = append(ins, in)
				outValue = *new(big.Int).Add(&outValue, &r.Value)
			}
		}

		refundAddress, err := wal.DecodeAddress(order.RefundAddress)
		if err != nil {
			return fmt.Errorf("decode refund address: %s", err.Error())
		}
		var output = wallet.TransactionOutput{
			Address: refundAddress,
			Value:   outValue,
		}

		chaincode, err := hex.DecodeString(order.Payment.Chaincode)
		if err != nil {
			return fmt.Errorf("decode buyer chaincode: %s", err.Error())
		}
		mECKey, err := n.MasterPrivateKey.ECPrivKey()
		if err != nil {
			return fmt.Errorf("generate ec private key: %s", err.Error())
		}
		vendorKey, err := wal.ChildKey(mECKey.Serialize(), chaincode, true)
		if err != nil {
			return fmt.Errorf("generate child key: %s", err.Error())
		}
		redeemScript, err := hex.DecodeString(order.Payment.RedeemScript)
		if err != nil {
			return fmt.Errorf("generate child key: %s", err.Error())
		}
		fee, ok := new(big.Int).SetString(order.BigRefundFee, 10)
		if !ok {
			return errors.New("invalid refund fee value")
		}
		signatures, err := escrowWallet.CreateMultisigSignature(ins, []wallet.TransactionOutput{output}, vendorKey, redeemScript, *fee)
		if err != nil {
			return fmt.Errorf("generate multisig: %s", err.Error())
		}
		var sigs []*pb.BitcoinSignature
		for _, s := range signatures {
			pbSig := &pb.BitcoinSignature{Signature: s.Signature, InputIndex: s.InputIndex}
			sigs = append(sigs, pbSig)
		}
		rejectMsg.Sigs = sigs
	}
	err = n.SendReject(order.BuyerID.PeerID, rejectMsg)
	if err != nil {
		return fmt.Errorf("sending rejection: %s", err.Error())
	}
	if err := n.Datastore.Sales().Put(orderID, *contract, pb.OrderState_DECLINED, true); err != nil {
		return fmt.Errorf("updating sale state: %s", err.Error())
	}
	return nil
}

// ValidateOrderConfirmation - validate address and signatures for order confirmation
func (n *OpenBazaarNode) ValidateOrderConfirmation(contract *pb.RicardianContract, validateAddress bool) error {
	orderID, err := n.CalcOrderID(contract.BuyerOrder)
	if err != nil {
		return err
	}

	orderConfirmation := repo.ToV5OrderConfirmation(contract.VendorOrderConfirmation)

	order, err := repo.ToV5Order(contract.BuyerOrder, n.LookupCurrency)
	if err != nil {
		return err
	}
	if orderConfirmation.OrderID != orderID {
		return errors.New("vendor's response contained invalid order ID")
	}
	if orderConfirmation.BigRequestedAmount != order.Payment.BigAmount {
		return errors.New("vendor requested an amount different from what we calculated")
	}
	wal, err := n.Multiwallet.WalletForCurrencyCode(order.Payment.AmountCurrency.Code)
	if err != nil {
		return err
	}
	if order.Payment.Method == pb.Order_Payment_MODERATED {
		for _, sig := range contract.VendorOrderConfirmation.RatingSignatures {
			exists := false
			for _, listing := range contract.VendorListings {
				if sig.Metadata.ListingSlug == listing.Slug {
					exists = true
					break
				}
			}
			if !exists {
				return errors.New("rating signatures do not cover all purchased listings")
			}
			pubkey, err := crypto.UnmarshalPublicKey(contract.VendorListings[0].VendorID.Pubkeys.Identity)
			if err != nil {
				return err
			}

			if !bytes.Equal(sig.Metadata.ModeratorKey, order.Payment.ModeratorKey) {
				return errors.New("rating signature does not contain moderatory key")
			}
			ser, err := proto.Marshal(sig.Metadata)
			if err != nil {
				return err
			}
			valid, err := pubkey.Verify(ser, sig.Signature)
			if err != nil || !valid {
				return errors.New("failed to verify signature on rating keys")
			}
		}
	}
	if validateAddress {
		_, err = wal.DecodeAddress(contract.VendorOrderConfirmation.PaymentAddress)
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

// SignOrderConfirmation - sign the added order confirmation
func (n *OpenBazaarNode) SignOrderConfirmation(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedOrderConf, err := proto.Marshal(contract.VendorOrderConfirmation)
	if err != nil {
		return contract, err
	}
	s := new(pb.Signature)
	s.Section = pb.Signature_ORDER_CONFIRMATION
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
		contract.VendorListings[0].VendorID.Pubkeys.Identity,
		contract.Signatures,
		pb.Signature_ORDER_CONFIRMATION,
		contract.VendorListings[0].VendorID.PeerID,
	); err != nil {
		switch err.(type) {
		case noSigError:
			return errors.New("contract does not contain a signature for the order confirmation")
		case invalidSigError:
			return errors.New("vendor's guid signature on contact failed to verify")
		case matchKeyError:
			return errors.New("public key in order confirmation does not match reported vendor ID")
		default:
			return err
		}
	}
	return nil
}
