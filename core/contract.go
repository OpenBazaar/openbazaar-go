package core

import (
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/btcec"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/wallet-interface"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/golang/protobuf/proto"
)

type Contract struct{}

func GetDeserializedContract(serialized []byte) (*pb.RicardianContract, error) {
	contract := new(pb.RicardianContract)
	err := proto.Unmarshal(serialized, contract)
	if err != nil {
		return nil, err
	}
	return contract, nil
}

func CheckContractFormatting(contract *pb.RicardianContract) error {
	if len(contract.VendorListings) == 0 || contract.BuyerOrder == nil || contract.BuyerOrder.Payment == nil {
		return errors.New("serialized contract is malformatted")
	}
	return nil
}

func GetThumbnails(contract *pb.RicardianContract) map[string]string {
	var thumbnailTiny string
	var thumbnailSmall string

	if len(contract.VendorListings) > 0 && contract.VendorListings[0].Item != nil && len(contract.VendorListings[0].Item.Images) > 0 {
		thumbnailTiny = contract.VendorListings[0].Item.Images[0].Tiny
		thumbnailSmall = contract.VendorListings[0].Item.Images[0].Small
		return map[string]string{
			"small": thumbnailSmall,
			"tiny":  thumbnailTiny,
		}
	}
	return nil
}

func GetContractOrderType(contract *pb.RicardianContract, peerID string) string {
	if GetModerator(contract) == peerID {
		return "case"
	} else if GetBuyer(contract) == peerID {
		return "purchase"
	} else if GetVendor(contract) == peerID {
		return "sale"
	}
	return ""
}

func GetModerator(contract *pb.RicardianContract) string {
	if contract.BuyerOrder.Payment.Moderator != "" {
		return contract.BuyerOrder.Payment.Moderator
	}
	return ""
}

func GetBuyer(contract *pb.RicardianContract) string {
	if contract.BuyerOrder != nil && contract.BuyerOrder.BuyerID != nil {
		return contract.BuyerOrder.BuyerID.PeerID
	}
	return ""
}

func GetBuyerHandle(contract *pb.RicardianContract) string {
	return contract.BuyerOrder.BuyerID.Handle
}

func GetBuyerPubkey(contract *pb.RicardianContract) []byte {
	return contract.BuyerOrder.BuyerID.Pubkeys.Identity
}

func GetRole(contract *pb.RicardianContract, peerID string) string {
	if contract.BuyerOrder.Payment.Moderator == peerID {
		return "moderator"
	} else if contract.VendorListings[0].VendorID.PeerID == peerID {
		return "vendor"
	} else if contract.BuyerOrder.BuyerID.PeerID == peerID {
		return "buyer"
	}
	return ""
}

func GetVendor(contract *pb.RicardianContract) string {
	return contract.VendorListings[0].VendorID.PeerID
}

func GetVendorHandle(contract *pb.RicardianContract) string {
	return contract.VendorListings[0].VendorID.Handle
}

func GetVendorPubkey(contract *pb.RicardianContract) []byte {
	return contract.VendorListings[0].VendorID.Pubkeys.Identity
}

func IsOrderStateDisputableForVendor(orderState pb.OrderState) bool {
	if orderState == pb.OrderState_COMPLETED || orderState == pb.OrderState_DISPUTED ||
		orderState == pb.OrderState_DECIDED || orderState == pb.OrderState_RESOLVED ||
		orderState == pb.OrderState_REFUNDED || orderState == pb.OrderState_CANCELED ||
		orderState == pb.OrderState_DECLINED || orderState == pb.OrderState_PROCESSING_ERROR {
		return true
	}
	return false
}

func IsOrderStateDisputableForBuyer(orderState pb.OrderState) bool {
	if orderState == pb.OrderState_COMPLETED || orderState == pb.OrderState_DISPUTED ||
		orderState == pb.OrderState_DECIDED || orderState == pb.OrderState_RESOLVED ||
		orderState == pb.OrderState_REFUNDED || orderState == pb.OrderState_CANCELED ||
		orderState == pb.OrderState_DECLINED {
		return true
	}
	return false
}

func GetDisputeUpdateMessage(contract *pb.RicardianContract, orderID string, payoutAddress string, txRecords []*wallet.TransactionRecord) (*pb.DisputeUpdate, error) {
	update := new(pb.DisputeUpdate)
	ser, err := proto.Marshal(contract)
	if err != nil {
		return nil, err
	}
	update.SerializedContract = ser
	update.OrderId = orderID
	update.PayoutAddress = payoutAddress

	var outpoints []*pb.Outpoint
	for _, r := range txRecords {
		o := new(pb.Outpoint)
		o.Hash = r.Txid
		o.Index = r.Index
		o.Value = uint64(r.Value)
		outpoints = append(outpoints, o)
	}
	update.Outpoints = outpoints

	return update, nil
}

func GetListingSignatures(contract *pb.RicardianContract) []*pb.Signature {
	var listingSigs []*pb.Signature
	for _, sig := range contract.Signatures {
		if sig.Section == pb.Signature_LISTING {
			listingSigs = append(listingSigs, sig)
		}
	}
	return listingSigs
}

func HasListings(contract *pb.RicardianContract) bool {
	// Contract should have a listing and order to make it to this point
	return len(contract.VendorListings) > 0
}

func ContractHasBuyerOrder(contract *pb.RicardianContract) bool {
	return contract.BuyerOrder != nil
}

func ContractIsMissingVendorID(contract *pb.RicardianContract) bool {
	if len(contract.VendorListings) == 0 {
		return true
	}
	return contract.VendorListings[0].VendorID == nil || contract.VendorListings[0].VendorID.Pubkeys == nil
}

func ContractIsMissingBuyerID(contract *pb.RicardianContract) bool {
	return contract.BuyerOrder.BuyerID == nil || contract.BuyerOrder.BuyerID.Pubkeys == nil
}

func ContractIsMissingPayment(contract *pb.RicardianContract) bool {
	return contract.BuyerOrder.Payment == nil
}

func GetUnmatchedListingHashes(contract *pb.RicardianContract) []string {
	var listingHashes []string
	for _, item := range contract.BuyerOrder.Items {
		listingHashes = append(listingHashes, item.ListingHash)
	}
	for _, listing := range contract.VendorListings {
		ser, err := proto.Marshal(listing)
		if err != nil {
			continue
		}
		listingMH, err := EncodeCID(ser)
		if err != nil {
			continue
		}
		for i, l := range listingHashes {
			if l == listingMH.String() {
				// Delete from listingHashes
				listingHashes = append(listingHashes[:i], listingHashes[i+1:]...)
				break
			}
		}
	}
	return listingHashes
}

func CheckListingSignatures(contract *pb.RicardianContract) []string {
	var validationErrors []string
	listingSigs := GetListingSignatures(contract)
	vendorPubkey := GetVendorPubkey(contract)
	vendorID := GetVendor(contract)

	if len(listingSigs) < len(contract.VendorListings) {
		return []string{"Not all listings are signed by the vendor"}
	} else {
		// Verify the listing signatures only if there are enough signatures to validate
		for i, listing := range contract.VendorListings {
			if err := verifyMessageSignature(listing, vendorPubkey, []*pb.Signature{listingSigs[i]}, pb.Signature_LISTING, vendorID); err != nil {
				validationErrors = append(validationErrors, "Invalid vendor signature on listing "+strconv.Itoa(i)+err.Error())
			}
			if i == len(listingSigs)-1 {
				break
			}
		}
	}
	return validationErrors
}

func VerifyRedeemScript(contract *pb.RicardianContract, wal wallet.Wallet, masterECPubkey *btcec.PublicKey) []string {
	var validationErrors []string

	if contract.BuyerOrder.Payment != nil {
		chaincode, err := hex.DecodeString(contract.BuyerOrder.Payment.Chaincode)
		if err != nil {
			validationErrors = append(validationErrors, "Error validating bitcoin address and redeem script")
			return validationErrors
		}

		moderatorKey, err := wal.ChildKey(masterECPubkey.SerializeCompressed(), chaincode, false)
		if err != nil {
			validationErrors = append(validationErrors, "Error validating bitcoin address and redeem script")
			return validationErrors
		}
		buyerKey, err := wal.ChildKey(contract.BuyerOrder.BuyerID.Pubkeys.Bitcoin, chaincode, false)
		if err != nil {
			validationErrors = append(validationErrors, "Error validating bitcoin address and redeem script")
			return validationErrors
		}
		vendorKey, err := wal.ChildKey(contract.VendorListings[0].VendorID.Pubkeys.Bitcoin, chaincode, false)
		if err != nil {
			validationErrors = append(validationErrors, "Error validating bitcoin address and redeem script")
			return validationErrors
		}
		timeout, _ := time.ParseDuration(strconv.Itoa(int(contract.VendorListings[0].Metadata.EscrowTimeoutHours)) + "h")
		addr, redeemScript, err := wal.GenerateMultisigScript([]hd.ExtendedKey{*buyerKey, *vendorKey, *moderatorKey}, 2, timeout, vendorKey)
		if err != nil {
			validationErrors = append(validationErrors, "Error generating multisig script")
			return validationErrors
		}

		if contract.BuyerOrder.Payment.Address != addr.EncodeAddress() {
			validationErrors = append(validationErrors, "The calculated bitcoin address doesn't match the address in the order")
		}

		if hex.EncodeToString(redeemScript) != contract.BuyerOrder.Payment.RedeemScript {
			validationErrors = append(validationErrors, "The calculated redeem script doesn't match the redeem script in the order")
		}
	}
	return validationErrors
}

func ValidBuyerOrderSignature(contract *pb.RicardianContract) bool {
	buyerPubkey := GetBuyerPubkey(contract)
	buyerGUID := GetBuyer(contract)
	err := verifyMessageSignature(contract.BuyerOrder, buyerPubkey, contract.Signatures, pb.Signature_ORDER, buyerGUID)
	if err != nil {
		log.Error(err)
		return false
	}
	return true
}

func ValidOrderConfirmationSignature(contract *pb.RicardianContract) bool {
	vendorPubkey := GetVendorPubkey(contract)
	vendorGUID := GetVendor(contract)
	if contract.VendorOrderConfirmation != nil {
		err := verifyMessageSignature(contract.VendorOrderConfirmation, vendorPubkey, contract.Signatures, pb.Signature_ORDER_CONFIRMATION, vendorGUID)
		if err != nil {
			log.Error(err)
			return false
		}
		return true
	}
	return false
}

func GetFulfillmentSignatures(contract *pb.RicardianContract) []*pb.Signature {
	var fulfillmentSigs []*pb.Signature
	for _, sig := range contract.Signatures {
		if sig.Section == pb.Signature_ORDER_FULFILLMENT {
			fulfillmentSigs = append(fulfillmentSigs, sig)
		}
	}
	return fulfillmentSigs
}

func GetFulfillments(contract *pb.RicardianContract) []*pb.OrderFulfillment {
	return contract.VendorOrderFulfillment
}

func CheckFulfillmentSignatures(contract *pb.RicardianContract) []string {
	var validationErrors []string

	vendorPubkey := GetVendorPubkey(contract)
	vendorGUID := GetVendor(contract)
	fulfillmentSigs := GetFulfillmentSignatures(contract)

	for i, f := range contract.VendorOrderFulfillment {
		if err := verifyMessageSignature(f, vendorPubkey, []*pb.Signature{fulfillmentSigs[i]}, pb.Signature_ORDER_FULFILLMENT, vendorGUID); err != nil {
			validationErrors = append(validationErrors, "Invalid vendor signature on fulfilment "+strconv.Itoa(i))
		}
		if i == len(fulfillmentSigs)-1 {
			break
		}
	}
	return validationErrors
}
