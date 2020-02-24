package factory

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/ptypes/timestamp"

	"github.com/OpenBazaar/jsonpb"
)

// MustLoadListingFixture - load listing json from fixtures
func MustLoadListingFixture(fixtureName string) []byte {
	filename := filepath.Join(fixtureLoadPath(), "listings", fmt.Sprintf("%s.json", fixtureName))
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(fmt.Errorf("cannot find fixture (%s): %s", fixtureName, err))
	}
	return b
}

// NewListing - return new pb.Listing
func NewListing(slug string) *pb.Listing {
	var (
		idJSON = `{
            "peerID": "QmVisrQ9apmvTLnq9FSNKbP8dYvBvkP4AeeysHZg89oB9q",
            "pubkeys": {
                "identity": "CAESIBHz9BLX+9JlUN7cfPdaoh1QFN/a4gjJBzmVOZfSFD5G",
                "bitcoin": "Ai4YTSiFiBLqNxjV/iLcKilp4iaJCIvnatSf15EV25M2"
            },
            "bitcoinSig": "MEUCIQC7jvfG23aHIpPjvQjT1unn23PuKNSykh9v/Hc7v3vmoQIgMFI8BBtju7tAgpI66jKAL6PKWGb7jImVBo1DcDoNbpI="
        }`
		vendorID = new(pb.ID)
	)
	if err := jsonpb.UnmarshalString(idJSON, vendorID); err != nil {
		panic(err)
	}

	return &pb.Listing{
		Slug:               slug,
		TermsAndConditions: "Sample Terms and Conditions",
		RefundPolicy:       "Sample Refund policy",
		VendorID:           vendorID,
		Metadata: &pb.Listing_Metadata{
			Version:            5,
			AcceptedCurrencies: []string{"TBTC"},
			Expiry:             &timestamp.Timestamp{Seconds: 2147483647},
			Format:             pb.Listing_Metadata_FIXED_PRICE,
			ContractType:       pb.Listing_Metadata_PHYSICAL_GOOD,
		},
		Item: &pb.Listing_Item{
			Skus: []*pb.Listing_Item_Sku{
				{
					BigSurcharge: "0",
					BigQuantity:  "12",
					ProductID:    "1",
					VariantCombo: []uint32{0, 0},
				},
				{
					BigSurcharge: "0",
					BigQuantity:  "44",
					ProductID:    "2",
					VariantCombo: []uint32{0, 1},
				},
			},
			Title: "Ron Swanson Tshirt",
			Tags:  []string{"tshirts"},
			Options: []*pb.Listing_Item_Option{
				{
					Name:        "Size",
					Description: "What size do you want your shirt?",
					Variants: []*pb.Listing_Item_Option_Variant{
						{Name: "Small", Image: NewImage()},
						{Name: "Large", Image: NewImage()},
					},
				},
				{
					Name:        "Color",
					Description: "What color do you want your shirt?",
					Variants: []*pb.Listing_Item_Option_Variant{
						{Name: "Red", Image: NewImage()},
						{Name: "Green", Image: NewImage()},
					},
				},
			},
			Nsfw:           false,
			Description:    "Example item",
			BigPrice:       "2000",
			PriceCurrency:  &pb.CurrencyDefinition{Code: "TBTC", Divisibility: 8},
			ProcessingTime: "3 days",
			Categories:     []string{"tshirts"},
			Grams:          14,
			Condition:      "new",
			Images:         []*pb.Listing_Item_Image{NewImage(), NewImage()},
		},
		Taxes: []*pb.Listing_Tax{
			{
				Percentage:  7,
				TaxShipping: true,
				TaxType:     "Sales tax",
				TaxRegions:  []pb.CountryCode{pb.CountryCode_UNITED_STATES},
			},
		},
		ShippingOptions: []*pb.Listing_ShippingOption{
			{
				Name:    "usps",
				Type:    pb.Listing_ShippingOption_FIXED_PRICE,
				Regions: []pb.CountryCode{pb.CountryCode_ALL},
				Services: []*pb.Listing_ShippingOption_Service{
					{
						Name:              "standard",
						BigPrice:          "20",
						EstimatedDelivery: "3 days",
					},
				},
			},
		},
		Coupons: []*pb.Listing_Coupon{
			{
				Title:            "Insider's Discount",
				Code:             &pb.Listing_Coupon_DiscountCode{DiscountCode: "insider"},
				BigPriceDiscount: "5",
			},
		},
	}
}

// NewCryptoListing - return new crypto listing
func NewCryptoListing(slug string) *pb.Listing {
	listing := NewListing(slug)
	listing.Metadata.CryptoCurrencyCode = "TETH"
	listing.Metadata.CryptoDivisibility = 18
	listing.Metadata.ContractType = pb.Listing_Metadata_CRYPTOCURRENCY
	listing.Item.Skus = []*pb.Listing_Item_Sku{{BigQuantity: "100000000"}}
	listing.Metadata.PricingCurrency = ""
	listing.ShippingOptions = nil
	listing.Item.Condition = ""
	listing.Item.Options = nil
	listing.Item.BigPrice = "0"
	listing.Item.PriceCurrency = nil
	listing.Coupons = nil
	return listing
}

// NewListingWithShippingRegions - return new listing with shipping region
func NewListingWithShippingRegions(slug string) *pb.Listing {
	listing := NewListing(slug)
	listing.ShippingOptions = []*pb.Listing_ShippingOption{
		{
			Name:    "usps",
			Type:    pb.Listing_ShippingOption_FIXED_PRICE,
			Regions: []pb.CountryCode{pb.CountryCode_UNITED_KINGDOM},
			Services: []*pb.Listing_ShippingOption_Service{
				{
					Name:              "standard",
					BigPrice:          "20",
					EstimatedDelivery: "3 days",
				},
			},
		},
	}
	return listing
}
