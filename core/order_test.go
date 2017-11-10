package core_test

import (
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/test"
	"github.com/golang/protobuf/proto"
	"testing"
)

func TestOpenBazaarNode_CalculateOrderTotal(t *testing.T) {
	node, err := test.NewNode()
	if err != nil {
		t.Error(err)
	}
	contract := &pb.RicardianContract{
		VendorListings: []*pb.Listing{{
			Metadata: &pb.Listing_Metadata{
				ContractType:       pb.Listing_Metadata_PHYSICAL_GOOD,
				Format:             pb.Listing_Metadata_FIXED_PRICE,
				AcceptedCurrencies: []string{"BTC"},
				PricingCurrency:    "BTC",
				Version:            2,
			},
			Item: &pb.Listing_Item{
				Price: 100000,
			},
			ShippingOptions: []*pb.Listing_ShippingOption{
				{
					Name:    "UPS",
					Regions: []pb.CountryCode{pb.CountryCode_UNITED_STATES},
					Type:    pb.Listing_ShippingOption_FIXED_PRICE,
					Services: []*pb.Listing_ShippingOption_Service{
						{
							Name:                "Standard shipping",
							Price:               25000,
							AdditionalItemPrice: 10000,
						},
					},
				},
			},
		}},
	}

	ser, err := proto.Marshal(contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err := core.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	order := &pb.Order{
		Items: []*pb.Order_Item{
			{
				ListingHash: listingID.String(),
				Quantity:    1,
				ShippingOption: &pb.Order_Item_ShippingOption{
					Name:    "UPS",
					Service: "Standard shipping",
				},
			},
		},
		Shipping: &pb.Order_Shipping{
			Country: pb.CountryCode_UNITED_STATES,
		},
	}
	contract.BuyerOrder = order

	// Test standard contract
	total, err := node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total != 125000 {
		t.Error("Calculated wrong order total")
		return
	}

	// Test higher quantity
	contract.BuyerOrder.Items[0].Quantity = 2
	total, err = node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total != 235000 {
		t.Error("Calculated wrong order total")
	}

	// Test with options
	contract.BuyerOrder.Items[0].Quantity = 1
	contract.VendorListings[0].Item.Options = []*pb.Listing_Item_Option{
		{
			Name: "color",
			Variants: []*pb.Listing_Item_Option_Variant{
				{
					Name: "red",
				},
			},
		},
	}
	contract.VendorListings[0].Item.Skus = []*pb.Listing_Item_Sku{
		{
			Surcharge:    50000,
			VariantCombo: []uint32{0},
		},
	}
	contract.BuyerOrder.Items[0].Options = []*pb.Order_Item_Option{
		{
			Name:  "color",
			Value: "red",
		},
	}
	ser, err = proto.Marshal(contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = core.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total != 175000 {
		t.Error("Calculated wrong order total")
	}

	// Test negative surcharge
	contract.VendorListings[0].Item.Skus = []*pb.Listing_Item_Sku{
		{
			Surcharge:    -50000,
			VariantCombo: []uint32{0},
		},
	}
	ser, err = proto.Marshal(contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = core.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total != 75000 {
		t.Error("Calculated wrong order total")
	}

	// Test with coupon percent discount
	couponHash, err := core.EncodeMultihash([]byte("testcoupon"))
	if err != nil {
		t.Error(err)
	}
	contract.VendorListings[0].Coupons = []*pb.Listing_Coupon{
		{
			Code:     &pb.Listing_Coupon_Hash{couponHash.B58String()},
			Title:    "coup",
			Discount: &pb.Listing_Coupon_PercentDiscount{10},
		},
	}

	ser, err = proto.Marshal(contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = core.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	contract.BuyerOrder.Items[0].CouponCodes = []string{"testcoupon"}
	contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total != 70000 {
		t.Error("Calculated wrong order total")
	}

	// Test with coupon percent discount
	couponHash, err = core.EncodeMultihash([]byte("testcoupon2"))
	if err != nil {
		t.Error(err)
	}
	contract.VendorListings[0].Coupons = []*pb.Listing_Coupon{
		{
			Code:     &pb.Listing_Coupon_Hash{couponHash.B58String()},
			Title:    "coup",
			Discount: &pb.Listing_Coupon_PriceDiscount{6000},
		},
	}

	ser, err = proto.Marshal(contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = core.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	contract.BuyerOrder.Items[0].CouponCodes = []string{"testcoupon2"}
	contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total != 69000 {
		t.Error("Calculated wrong order total")
	}

	// Test with tax no tax shipping
	contract.VendorListings[0].Taxes = []*pb.Listing_Tax{
		{
			Percentage:  5,
			TaxShipping: false,
			TaxRegions:  []pb.CountryCode{pb.CountryCode_UNITED_STATES},
		},
	}

	ser, err = proto.Marshal(contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = core.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total != 71200 {
		t.Error("Calculated wrong order total")
	}

	// Test with tax with tax shipping
	contract.VendorListings[0].Taxes = []*pb.Listing_Tax{
		{
			Percentage:  5,
			TaxShipping: true,
			TaxRegions:  []pb.CountryCode{pb.CountryCode_UNITED_STATES},
		},
	}

	ser, err = proto.Marshal(contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = core.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total != 72450 {
		t.Error("Calculated wrong order total")
		return
	}

	// Test local pickup
	contract.VendorListings[0].ShippingOptions[0].Type = pb.Listing_ShippingOption_LOCAL_PICKUP

	ser, err = proto.Marshal(contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = core.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total != 46200 {
		t.Error("Calculated wrong order total")
	}

	contract2 := &pb.RicardianContract{
		VendorListings: []*pb.Listing{{
			Metadata: &pb.Listing_Metadata{
				ContractType:       pb.Listing_Metadata_PHYSICAL_GOOD,
				Format:             pb.Listing_Metadata_FIXED_PRICE,
				AcceptedCurrencies: []string{"BTC"},
				PricingCurrency:    "BTC",
			},
			Item: &pb.Listing_Item{
				Price: 100000,
			},
			ShippingOptions: []*pb.Listing_ShippingOption{
				{
					Name:    "UPS",
					Regions: []pb.CountryCode{pb.CountryCode_UNITED_STATES},
					Type:    pb.Listing_ShippingOption_FIXED_PRICE,
					Services: []*pb.Listing_ShippingOption_Service{
						{
							Name:  "Standard shipping",
							Price: 25000,
						},
					},
				},
			},
		}},
	}

	ser, err = proto.Marshal(contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = core.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	order2 := &pb.Order{
		Items: []*pb.Order_Item{
			{
				ListingHash: listingID.String(),
				Quantity:    1,
				ShippingOption: &pb.Order_Item_ShippingOption{
					Name:    "UPS",
					Service: "Standard shipping",
				},
			},
		},
		Shipping: &pb.Order_Shipping{
			Country: pb.CountryCode_UNITED_STATES,
		},
	}
	contract2.BuyerOrder = order2
}
