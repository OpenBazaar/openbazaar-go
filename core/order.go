package core

import (
	"crypto/sha256"
	"gx/ipfs/QmT6n4mspWYEya864BhCUJEgyxiRfmiSY9ruQwTUNpRKaM/protobuf/proto"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/jsonpb"
)

type option struct {
	name  string
	value string
}

type item struct {
	listingHash string
	quantity    int
	options     []option
}

type PurchaseData struct {
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
	contract := new(pb.RicardianContract)
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

	for _, item := range data.items {
		i := new(pb.Order_Item)
		b, err := ipfs.Cat(n.Context, item.listingHash)
		if err != nil {
			return err
		}
		rc := new(pb.RicardianContract)
		err = jsonpb.UnmarshalString(string(b), rc)
		if err != nil {
			return err
		}
		// TODO: validate signatures on this contract before we purchase it
		contract.VendorListings = append(contract.VendorListings, rc.VendorListings[0])
		contract.Signatures = append(contract.Signatures, rc.Signatures[0])
		ser, err := proto.Marshal(rc.VendorListings[0])
		if err != nil {
			return err
		}
		h := sha256.Sum256(ser)
		i.ListingHash = h[:]
		i.Quantity = uint32(item.quantity)

		for _, option := range item.options {
			o := new(pb.Order_Item_Option)
			o.Name = option.name
			o.Value = option.value
			i.Options = append(i.Options, o)
		}
		order.Items = append(order.Items, i)
	}

	// TODO: create payment obj
	// TODO: send to vendor
	return nil
}
