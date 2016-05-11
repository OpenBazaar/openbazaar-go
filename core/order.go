package core

import (
	"time"
	"github.com/OpenBazaar/openbazaar-go/pb"
)

type option struct {
	name  string
	value string
}

type item struct {
	listingHash  string
	quantity     int
	options      []option
}

type PurchaseData struct{
	shipTo      string
	address     string
	city        string
	state       string
	postalCode  string
	countryCode string
	moderator   string
	items       []item
}

func (n *OpenBazaarNode) Purchase(data *PurchaseData) error {
	// TODO: validate the purchase data is formatted properly
	order := new(pb.Order)
	order.RefundAddress = n.Wallet.GetNextRefundAddress().EncodeAddress()

	order.Shipping.ShipTo = data.shipTo
	order.Shipping.Address = data.address
	order.Shipping.City = data.city
	order.Shipping.State = data.state
	order.Shipping.PostalCode = data.postalCode
	order.Shipping.Country = pb.CountryCode(pb.CountryCode_value[data.countryCode])

	// TODO: Add blockchain ID to order
	order.BuyerID.Guid = n.IpfsNode.Identity.Pretty()
	pubkey, err := n.IpfsNode.PrivateKey.GetPublic().Bytes()
	if err != nil {
		return err
	}
	order.BuyerID.Pubkeys.Guid = pubkey
	order.BuyerID.Pubkeys.Bitcoin = n.Wallet.GetMasterPublicKey().Key

	order.Timestamp = uint64(time.Now().Unix())

	// TODO: need to finish this
	return nil
}