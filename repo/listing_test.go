package repo_test

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestListingUnmarshalJSON(t *testing.T) {
	var examples = []string{
		"v3-physical-good",
		"v4-physical-good",
		"v4-digital-good",
		"v4-service",
		"v4-cryptocurrency",
		"v5-physical-good",
	}

	for _, e := range examples {
		var (
			fixtureBytes = factory.MustLoadListingFixture(e)
			_, err       = repo.UnmarshalJSONListing(fixtureBytes)
		)
		if err != nil {
			t.Errorf("exmaple (%s): %s", e, err)
		}
	}
}

// nolint:dupl
func TestListingAttributes(t *testing.T) {
	var examples = []struct {
		fixtureName                string
		expectedVersion            uint32
		expectedTitle              string
		expectedSlug               string
		expectedPrice              *repo.CurrencyValue
		expectedAcceptedCurrencies []string
		expectedCryptoDivisibility uint32
		expectedCryptoCurrencyCode string
	}{
		{
			fixtureName:     "v3-physical-good",
			expectedVersion: 3,
			expectedTitle:   "Physical Listing",
			expectedSlug:    "physical-listing",
			expectedPrice: &repo.CurrencyValue{
				Amount: big.NewInt(1235000000),
				Currency: repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("BCH"),
					Divisibility: 8,
					CurrencyType: "crypto",
				},
			},
			expectedAcceptedCurrencies: []string{"BCH"},
			expectedCryptoDivisibility: 0,
			expectedCryptoCurrencyCode: "",
		},
		{
			fixtureName:     "v4-physical-good",
			expectedVersion: 4,
			expectedTitle:   "Physical Good Listing",
			expectedSlug:    "physical-good-listing",
			expectedPrice: &repo.CurrencyValue{
				Amount: big.NewInt(12345678000),
				Currency: repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("BCH"),
					Divisibility: 8,
					CurrencyType: "crypto",
				},
			},
			expectedAcceptedCurrencies: []string{"ZEC", "LTC", "BTC", "BCH"},
			expectedCryptoDivisibility: 0,
			expectedCryptoCurrencyCode: "",
		},
		{
			fixtureName:     "v4-digital-good",
			expectedVersion: 4,
			expectedTitle:   "Digital Good Listing",
			expectedSlug:    "digital-good-listing",
			expectedPrice: &repo.CurrencyValue{
				Amount: big.NewInt(1320),
				Currency: repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("USD"),
					Divisibility: 2,
					CurrencyType: "fiat",
				},
			},
			expectedAcceptedCurrencies: []string{"ZEC"},
			expectedCryptoDivisibility: 0,
			expectedCryptoCurrencyCode: "",
		},
		{
			fixtureName:     "v4-service",
			expectedVersion: 4,
			expectedTitle:   "Service Listing",
			expectedSlug:    "service-listing",
			expectedPrice: &repo.CurrencyValue{
				Amount: big.NewInt(9877000000),
				Currency: repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("BTC"),
					Divisibility: 8,
					CurrencyType: "crypto",
				},
			},
			expectedAcceptedCurrencies: []string{"ZEC", "LTC", "BCH", "BTC"},
			expectedCryptoDivisibility: 0,
			expectedCryptoCurrencyCode: "",
		},
		{
			fixtureName:     "v4-cryptocurrency",
			expectedVersion: 4,
			expectedTitle:   "LTC-XMR",
			expectedSlug:    "ltc-xmr",
			expectedPrice: &repo.CurrencyValue{
				Amount:   big.NewInt(0),
				Currency: repo.NewUnknownCryptoDefinition("XMR", 0),
			},
			expectedAcceptedCurrencies: []string{"LTC"},
			expectedCryptoDivisibility: 8,
			expectedCryptoCurrencyCode: "XMR",
		},
		{
			fixtureName:     "v5-physical-good",
			expectedVersion: 5,
			expectedTitle:   "ETH - $1",
			expectedSlug:    "eth-1",
			expectedPrice: &repo.CurrencyValue{
				Amount: big.NewInt(100),
				Currency: repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("USD"),
					Divisibility: 2,
					CurrencyType: "fiat",
				},
			},
			expectedAcceptedCurrencies: []string{"BTC", "BCH", "ZEC", "LTC", "ETH"},
			expectedCryptoDivisibility: 0,
			expectedCryptoCurrencyCode: "",
		},
	}

	for _, e := range examples {
		t.Logf("example listing (%s)", e.fixtureName)
		var (
			fixtureBytes = factory.MustLoadListingFixture(e.fixtureName)
			l, err       = repo.UnmarshalJSONListing(fixtureBytes)
		)
		if err != nil {
			t.Errorf("unable to unmarshal example (%s)", e.fixtureName)
			continue
		}
		if l.GetVersion() != e.expectedVersion {
			t.Errorf("expected to have version response (%+v), but instead was (%+v)", e.expectedVersion, l.GetVersion())
		}
		if title := l.GetTitle(); title != e.expectedTitle {
			t.Errorf("expected to have title response (%+v), but instead was (%+v)", e.expectedTitle, title)
		}
		if slug := l.GetSlug(); slug != e.expectedSlug {
			t.Errorf("expected to have slug response (%+v), but instead was (%+v)", e.expectedSlug, slug)
		}
		if price, err := l.GetPrice(); err == nil {
			if !price.Equal(e.expectedPrice) {
				t.Errorf("expected to have price response (%+v), but instead was (%+v)", e.expectedPrice, price)
			}
		} else {
			t.Errorf("get price: %s", err.Error())
		}
		if acceptedCurrencies := l.GetAcceptedCurrencies(); len(acceptedCurrencies) != len(e.expectedAcceptedCurrencies) {
			t.Errorf("expected to have acceptedCurrencies response (%+v), but instead was (%+v)", e.expectedAcceptedCurrencies, acceptedCurrencies)
		}
		if actual := l.GetCryptoDivisibility(); actual != e.expectedCryptoDivisibility {
			t.Errorf("expected to have divisibility (%d), but was (%d)", e.expectedCryptoDivisibility, actual)
		}
		if actual := l.GetCryptoCurrencyCode(); actual != e.expectedCryptoCurrencyCode {
			t.Errorf("expected to have currency code (%s), but was (%s)", e.expectedCryptoCurrencyCode, actual)
		}
	}
}

func TestListingFromProtobuf(t *testing.T) {
	var (
		subject     = factory.NewListing("slug")
		actual, err = repo.NewListingFromProtobuf(subject)
	)
	if err != nil {
		t.Fatal(err)
	}

	if subject.GetSlug() != actual.GetSlug() {
		t.Errorf("expected slug to be (%s), but was (%s)", subject.GetSlug(), actual.GetSlug())
	}
	if subject.GetTermsAndConditions() != actual.GetTermsAndConditions() {
		t.Errorf("expected terms/conditions to be (%s), but was (%s)", subject.GetTermsAndConditions(), actual.GetTermsAndConditions())
	}
	if subject.GetRefundPolicy() != actual.GetRefundPolicy() {
		t.Errorf("expected refund policy to be (%s), but was (%s)", subject.GetRefundPolicy(), actual.GetRefundPolicy())
	}
	if subject.Metadata.GetVersion() != actual.GetVersion() {
		t.Errorf("expected vesion to be (%d), but was (%d)", subject.Metadata.GetVersion(), actual.GetVersion())
	}
	if hash, err := actual.GetVendorID().Hash(); err != nil && subject.VendorID.PeerID != hash {
		t.Errorf("expected hash to be (%s), but was (%s)", subject.VendorID.PeerID, hash)
		t.Logf("hash had an error: %s", err)

	}
	if !bytes.Equal(subject.VendorID.BitcoinSig, actual.GetVendorID().BitcoinSignature()) {
		t.Errorf("expected refund policy to be (%s), but was (%s)", subject.VendorID.BitcoinSig, actual.GetVendorID().BitcoinSignature())
	}
}

func TestV4PhysicalGoodDataNormalizesToLatestSchema(t *testing.T) {
	var (
		expectedPrice          uint64 = 100000
		expectedPriceCurrency         = "EUR"
		expectedSkuSurcharge   int64  = 200
		expectedSkuQuantity    int64  = 12
		expectedShippingPrice  uint64 = 30
		expectedCouponDiscount uint64 = 50
		v4Proto                       = &pb.Listing{
			Metadata: &pb.Listing_Metadata{
				Version:         4,
				ContractType:    pb.Listing_Metadata_PHYSICAL_GOOD,
				PricingCurrency: expectedPriceCurrency,
			},
			Item: &pb.Listing_Item{
				Price: expectedPrice,
				Skus: []*pb.Listing_Item_Sku{
					{
						Surcharge: expectedSkuSurcharge,
						Quantity:  expectedSkuQuantity,
					},
				},
			},
			ShippingOptions: []*pb.Listing_ShippingOption{
				{
					Services: []*pb.Listing_ShippingOption_Service{
						{
							Price:               expectedShippingPrice,
							AdditionalItemPrice: expectedShippingPrice + 1,
						},
					},
				},
			},
			Coupons: []*pb.Listing_Coupon{
				{
					Discount: &pb.Listing_Coupon_PriceDiscount{PriceDiscount: expectedCouponDiscount},
				},
			},
		}
	)

	l, err := repo.NewListingFromProtobuf(v4Proto)
	if err != nil {
		t.Fatal(err)
	}

	nl, err := l.Normalize()
	if err != nil {
		t.Fatal(err)
	}

	nlp := nl.GetProtobuf()
	if v := nlp.Metadata.GetVersion(); v != repo.ListingVersion {
		t.Errorf("expected version to be (%d), but was (%d)", repo.ListingVersion, v)
	}

	if p := nlp.Item.BigPrice; p != fmt.Sprintf("%d", expectedPrice) {
		t.Errorf("expected price to be (%d), but was (%s)", expectedPrice, p)
	}

	if pc := nlp.Item.PriceCurrency; pc == nil {
		t.Error("expected to have pricing currency set, but was nil")
	} else {
		if pcc := pc.Code; pcc != expectedPriceCurrency {
			t.Errorf("expected price currency to be (%s), but was (%s)", expectedPriceCurrency, pcc)
		}
	}

	if s := nlp.Item.Skus[0]; s == nil {
		t.Error("expected sku to be present, but was nil")
	} else {
		if ss := s.BigSurcharge; ss != fmt.Sprintf("%d", expectedSkuSurcharge) {
			t.Errorf("expected surcharge to be (%d), but was (%s)", expectedSkuSurcharge, ss)
		}
		if sq := s.BigQuantity; sq != fmt.Sprintf("%d", expectedSkuQuantity) {
			t.Errorf("expected quantity to be (%d), but was (%s)", expectedSkuQuantity, sq)
		}
	}

	if s := nlp.ShippingOptions[0]; s == nil {
		t.Error("expected shipping options to be present, but was nil")
	} else {
		if ss := s.Services[0]; ss == nil {
			t.Error("expected shipping option services to be present, but was nil")
		} else {
			if ssp := ss.BigPrice; ssp != fmt.Sprintf("%d", expectedShippingPrice) {
				t.Errorf("expected shipping option price to be (%d), but was (%s)", expectedShippingPrice, ssp)
			}
		}
	}

	if c := nlp.Coupons[0]; c == nil {
		t.Error("expected coupon to be present, but was nil")
	} else {
		if cd := c.GetBigPriceDiscount(); cd != fmt.Sprintf("%d", expectedCouponDiscount) {
			t.Errorf("expected coupon discount to be (%d), but was (%s)", expectedCouponDiscount, cd)
		}
	}
}
