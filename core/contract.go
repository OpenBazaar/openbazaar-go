package core

import (
	"errors"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/wallet-interface"
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
