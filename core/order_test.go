package core_test

import (
	"fmt"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/test"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
	"github.com/golang/protobuf/proto"
)

func TestOpenBazaarNode_CalculateOrderTotal(t *testing.T) {
	node, err := test.NewNode()
	if err != nil {
		t.Error(err)
	}
	contract := &pb.RicardianContract{
		VendorListings: []*pb.Listing{
			{
				Metadata: &pb.Listing_Metadata{
					ContractType:       pb.Listing_Metadata_PHYSICAL_GOOD,
					Format:             pb.Listing_Metadata_FIXED_PRICE,
					AcceptedCurrencies: []string{"TBTC"},
					EscrowTimeoutHours: 1080,
					Version:            5,
				},
				Item: &pb.Listing_Item{
					BigPrice:      "100000",
					PriceCurrency: &pb.CurrencyDefinition{Code: "TBTC", Divisibility: 8},
				},
				ShippingOptions: []*pb.Listing_ShippingOption{
					{
						Name:    "UPS",
						Regions: []pb.CountryCode{pb.CountryCode_UNITED_STATES},
						Type:    pb.Listing_ShippingOption_FIXED_PRICE,
						Services: []*pb.Listing_ShippingOption_Service{
							{
								Name:                   "Standard shipping",
								BigPrice:               "25000",
								BigAdditionalItemPrice: "10000",
							},
						},
					},
				},
			},
		},
	}

	ser, err := proto.Marshal(contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err := ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	order := &pb.Order{
		Items: []*pb.Order_Item{
			{
				ListingHash: listingID.String(),
				BigQuantity: "1",
				ShippingOption: &pb.Order_Item_ShippingOption{
					Name:    "UPS",
					Service: "Standard shipping",
				},
			},
		},
		Shipping: &pb.Order_Shipping{
			Country: pb.CountryCode_UNITED_STATES,
		},
		Payment: &pb.Order_Payment{
			AmountCurrency: &pb.CurrencyDefinition{Code: "TBTC", Divisibility: 8},
		},
	}
	contract.BuyerOrder = order

	// Test standard contract
	total, err := node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 125000 {
		t.Errorf("Calculated wrong order total. Wanted 125000, got %d", total.Int64())
	}

	// Test higher quantity
	contract.BuyerOrder.Items[0].BigQuantity = "2"
	total, err = node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 235000 {
		t.Errorf("Calculated wrong order total. Wanted 235000, got %d", total.Int64())
	}

	// Test with options
	contract.BuyerOrder.Items[0].BigQuantity = "1"
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
			BigSurcharge: "50000",
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
	listingID, err = ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 175000 {
		t.Errorf("Calculated wrong order total. Wanted 175000, got %d", total.Int64())
	}

	// Test negative surcharge
	contract.VendorListings[0].Item.Skus = []*pb.Listing_Item_Sku{
		{
			BigSurcharge: "-50000",
			VariantCombo: []uint32{0},
		},
	}
	ser, err = proto.Marshal(contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 75000 {
		t.Errorf("Calculated wrong order total. Wanted 75000, got %d", total.Int64())
	}

	// Test with coupon percent discount
	couponHash, err := ipfs.EncodeMultihash([]byte("testcoupon"))
	if err != nil {
		t.Error(err)
	}
	contract.VendorListings[0].Coupons = []*pb.Listing_Coupon{
		{
			Code:            &pb.Listing_Coupon_Hash{Hash: couponHash.B58String()},
			Title:           "coup",
			PercentDiscount: 10,
		},
	}

	ser, err = proto.Marshal(contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	contract.BuyerOrder.Items[0].CouponCodes = []string{"testcoupon"}
	contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total1, err := node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total1.Int64() != 70000 {
		t.Errorf("failed calculating correct total, expected (%d), got (%d)", 70000, total1.Int64())
	}

	// Test with coupon percent discount
	couponHash, err = ipfs.EncodeMultihash([]byte("testcoupon2"))
	if err != nil {
		t.Error(err)
	}
	contract.VendorListings[0].Coupons = []*pb.Listing_Coupon{
		{
			Code:             &pb.Listing_Coupon_Hash{Hash: couponHash.B58String()},
			Title:            "coup",
			BigPriceDiscount: "6000",
		},
	}

	ser, err = proto.Marshal(contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	contract.BuyerOrder.Items[0].CouponCodes = []string{"testcoupon2"}
	contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 69000 {
		t.Errorf("Calculated wrong order total. Wanted 69000, got %d", total.Int64())
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
	listingID, err = ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 71200 {
		t.Errorf("Calculated wrong order total. Wanted 71200, got %d", total.Int64())
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
	listingID, err = ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 72450 {
		t.Fatalf("Calculated wrong order total. Wanted 72450, got %d", total.Int64())
	}

	// Test local pickup
	contract.VendorListings[0].ShippingOptions[0].Type = pb.Listing_ShippingOption_LOCAL_PICKUP

	ser, err = proto.Marshal(contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 46200 {
		t.Errorf("Calculated wrong order total. Wanted 46200, got %d", total.Int64())
	}
}

func TestOpenBazaarNode_CalculateOrderTotalWithV4Schema(t *testing.T) {
	node, err := test.NewNode()
	if err != nil {
		t.Error(err)
	}
	v4Contract := &pb.RicardianContract{
		VendorListings: []*pb.Listing{
			{
				Metadata: &pb.Listing_Metadata{
					ContractType:       pb.Listing_Metadata_PHYSICAL_GOOD,
					Format:             pb.Listing_Metadata_FIXED_PRICE,
					AcceptedCurrencies: []string{"TBTC"},
					EscrowTimeoutHours: 1080,
					PricingCurrency:    "TBTC",
					Version:            4,
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
			},
		},
	}

	ser, err := proto.Marshal(v4Contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err := ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	order := &pb.Order{
		Items: []*pb.Order_Item{
			{
				ListingHash: listingID.String(),
				Quantity64:  1,
				ShippingOption: &pb.Order_Item_ShippingOption{
					Name:    "UPS",
					Service: "Standard shipping",
				},
			},
		},
		Shipping: &pb.Order_Shipping{
			Country: pb.CountryCode_UNITED_STATES,
		},
		Payment: &pb.Order_Payment{
			Amount: 125000,
			Coin:   "TBTC",
		},
	}
	v4Contract.BuyerOrder = order

	// Basic contract
	total, err := node.CalculateOrderTotal(v4Contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 125000 {
		t.Errorf("Calculated wrong order total. Wanted 125000, got %d", total.Int64())
	}

	// Test higher quantity
	v4Contract.BuyerOrder.Items[0].Quantity64 = 2
	total, err = node.CalculateOrderTotal(v4Contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 235000 {
		t.Errorf("Calculated wrong order total. Wanted 235000, got %d", total.Int64())
	}

	// Test with options
	v4Contract.BuyerOrder.Items[0].Quantity64 = 1
	v4Contract.VendorListings[0].Item.Options = []*pb.Listing_Item_Option{
		{
			Name: "color",
			Variants: []*pb.Listing_Item_Option_Variant{
				{
					Name: "red",
				},
			},
		},
	}
	v4Contract.VendorListings[0].Item.Skus = []*pb.Listing_Item_Sku{
		{
			Surcharge:    50000,
			VariantCombo: []uint32{0},
		},
	}
	v4Contract.BuyerOrder.Items[0].Options = []*pb.Order_Item_Option{
		{
			Name:  "color",
			Value: "red",
		},
	}
	ser, err = proto.Marshal(v4Contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	v4Contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(v4Contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 175000 {
		t.Errorf("Calculated wrong order total. Wanted 175000, got %d", total.Int64())
	}

	// Test negative surcharge
	v4Contract.VendorListings[0].Item.Skus = []*pb.Listing_Item_Sku{
		{
			Surcharge:    -50000,
			VariantCombo: []uint32{0},
		},
	}
	ser, err = proto.Marshal(v4Contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	v4Contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(v4Contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 75000 {
		t.Errorf("Calculated wrong order total. Wanted 75000, got %d", total.Int64())
	}

	// Test with coupon percent discount
	couponHash, err := ipfs.EncodeMultihash([]byte("testcoupon"))
	if err != nil {
		t.Error(err)
	}
	v4Contract.VendorListings[0].Coupons = []*pb.Listing_Coupon{
		{
			Code:            &pb.Listing_Coupon_Hash{Hash: couponHash.B58String()},
			Title:           "coup",
			PercentDiscount: 10,
		},
	}

	ser, err = proto.Marshal(v4Contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	v4Contract.BuyerOrder.Items[0].CouponCodes = []string{"testcoupon"}
	v4Contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(v4Contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 70000 {
		t.Errorf("Calculated wrong order total. Wanted 70000, got %d", total.Int64())
	}

	// Test with coupon percent discount
	couponHash, err = ipfs.EncodeMultihash([]byte("testcoupon2"))
	if err != nil {
		t.Error(err)
	}
	v4Contract.VendorListings[0].Coupons = []*pb.Listing_Coupon{
		{
			Code:          &pb.Listing_Coupon_Hash{Hash: couponHash.B58String()},
			Title:         "coup",
			PriceDiscount: 6000,
		},
	}

	ser, err = proto.Marshal(v4Contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	v4Contract.BuyerOrder.Items[0].CouponCodes = []string{"testcoupon2"}
	v4Contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(v4Contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 69000 {
		t.Errorf("Calculated wrong order total. Wanted 69000, got %d", total.Int64())
	}

	// Test with tax no tax shipping
	v4Contract.VendorListings[0].Taxes = []*pb.Listing_Tax{
		{
			Percentage:  5,
			TaxShipping: false,
			TaxRegions:  []pb.CountryCode{pb.CountryCode_UNITED_STATES},
		},
	}

	ser, err = proto.Marshal(v4Contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	v4Contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(v4Contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 71200 {
		t.Errorf("Calculated wrong order total. Wanted 71200, got %d", total.Int64())
	}

	// Test with tax with tax shipping
	v4Contract.VendorListings[0].Taxes = []*pb.Listing_Tax{
		{
			Percentage:  5,
			TaxShipping: true,
			TaxRegions:  []pb.CountryCode{pb.CountryCode_UNITED_STATES},
		},
	}

	ser, err = proto.Marshal(v4Contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	v4Contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(v4Contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 72450 {
		t.Fatalf("Calculated wrong order total. Wanted 72450, got %d", total.Int64())
	}

	// Test local pickup
	v4Contract.VendorListings[0].ShippingOptions[0].Type = pb.Listing_ShippingOption_LOCAL_PICKUP

	ser, err = proto.Marshal(v4Contract.VendorListings[0])
	if err != nil {
		t.Error(err)
	}
	listingID, err = ipfs.EncodeCID(ser)
	if err != nil {
		t.Error(err)
	}
	v4Contract.BuyerOrder.Items[0].ListingHash = listingID.String()
	total, err = node.CalculateOrderTotal(v4Contract)
	if err != nil {
		t.Error(err)
	}
	if total.Int64() != 46200 {
		t.Errorf("Calculated wrong order total. Wanted 46200, got %d", total.Int64())
	}
}

func TestOpenBazaarNode_GetOrder(t *testing.T) {
	node, err := test.NewNode()
	if err != nil {
		t.Fatal(err)
	}

	contract := factory.NewContract()

	orderID, err := node.CalcOrderID(contract.BuyerOrder)
	if err != nil {
		t.Fatal(err)
	}

	state := pb.OrderState_AWAITING_PAYMENT
	err = node.Datastore.Purchases().Put(orderID, *contract, state, false)
	if err != nil {
		t.Fatal(err)
	}

	orderResponse, err := node.GetOrder(orderID)
	if err != nil {
		t.Fatal(err)
	}

	if orderResponse.State != state {
		t.Fatal(fmt.Errorf("expected order state to be %s, but was %s",
			pb.OrderState_name[int32(state)],
			pb.OrderState_name[int32(orderResponse.State)]))
	}
}
