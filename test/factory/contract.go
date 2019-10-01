package factory

import (
	crypto "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"

	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/ptypes"
)

func NewContract() *pb.RicardianContract {
	var (
		now        = time.Unix(time.Now().Unix(), 0)
		nowData, _ = ptypes.TimestampProto(now)
		order      = &pb.Order{
			BuyerID: &pb.ID{
				PeerID: "buyerID",
				Handle: "@buyerID",
			},
			Shipping: &pb.Order_Shipping{
				Address: "1234 Test Ave",
				ShipTo:  "Buyer Name",
			},
			Payment: &pb.Order_Payment{
				BigAmount:      "10",
				AmountCurrency: &pb.CurrencyDefinition{Code: "BTC", Divisibility: 8},
				Method:         pb.Order_Payment_DIRECT,
				Address:        "3BDbGsH5h5ctDiFtWMmZawcf3E7iWirVms",
			},
			Timestamp: nowData,
		}
		images = []*pb.Listing_Item_Image{{Tiny: "tinyimagehashOne", Small: "smallimagehashOne"}}
	)
	return &pb.RicardianContract{
		VendorListings: []*pb.Listing{
			{
				VendorID: &pb.ID{
					PeerID: "vendorID",
					Handle: "@vendorID",
				},
				Item: &pb.Listing_Item{Images: images},
				Metadata: &pb.Listing_Metadata{
					AcceptedCurrencies: []string{"BTC"},
				},
			},
		},
		BuyerOrder: order,
	}
}

func NewDisputeableContract() *pb.RicardianContract {
	c := NewContract()
	c.BuyerOrder.Payment.Moderator = "somemoderatorid"       // Moderator PeerID must be set
	c.BuyerOrder.Payment.Method = pb.Order_Payment_MODERATED // Method must be Moderated
	_, key, _ := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	keyBytes, _ := crypto.MarshalPublicKey(key)
	c.BuyerOrder.BuyerID.Pubkeys = &pb.ID_Pubkeys{Identity: keyBytes}
	c.VendorListings[0].VendorID = &pb.ID{
		PeerID:  "buyerID",
		Handle:  "@buyerID",
		Pubkeys: &pb.ID_Pubkeys{Identity: keyBytes},
	}
	return c
}

func NewUndisputeableContract() *pb.RicardianContract {
	c := NewContract()
	c.BuyerOrder.Payment.Moderator = ""                   // Unmoderated contracts may not be disputed
	c.BuyerOrder.Payment.Method = pb.Order_Payment_DIRECT // Direct payments may not be disputed
	return c
}

func NewDisputedContract() *pb.RicardianContract {
	var (
		c              = NewDisputeableContract()
		started        = time.Unix(time.Now().Add(time.Duration(-10)*time.Hour).Unix(), 0)
		startedData, _ = ptypes.TimestampProto(started)
	)
	c.Dispute = &pb.Dispute{
		Timestamp:     startedData,
		Claim:         "Mine?",
		PayoutAddress: "disputepayoutaddress",
	}
	return c
}
