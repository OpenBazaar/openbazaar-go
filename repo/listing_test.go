package repo_test

import (
	"bytes"
	"math/big"
	"testing"

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

func TestListingAttributes(t *testing.T) {
	var examples = []struct {
		fixtureName                string
		expectedResponse           uint
		expectedTitle              string
		expectedSlug               string
		expectedPrice              *repo.CurrencyValue
		expectedAcceptedCurrencies []string
		expectedCryptoDivisibility uint32
		expectedCryptoCurrencyCode string
	}{
		{
			fixtureName:      "v3-physical-good",
			expectedResponse: 3,
			expectedTitle:    "Physical Listing",
			expectedSlug:     "physical-listing",
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
			fixtureName:      "v4-physical-good",
			expectedResponse: 4,
			expectedTitle:    "Physical Good Listing",
			expectedSlug:     "physical-good-listing",
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
			fixtureName:      "v4-digital-good",
			expectedResponse: 4,
			expectedTitle:    "Digital Good Listing",
			expectedSlug:     "digital-good-listing",
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
			fixtureName:      "v4-service",
			expectedResponse: 4,
			expectedTitle:    "Service Listing",
			expectedSlug:     "service-listing",
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
			fixtureName:                "v4-cryptocurrency",
			expectedResponse:           4,
			expectedTitle:              "LTC-XMR",
			expectedSlug:               "ltc-xmr",
			expectedPrice:              nil,
			expectedAcceptedCurrencies: []string{"LTC"},
			expectedCryptoDivisibility: 8,
			expectedCryptoCurrencyCode: "XMR",
		},
		{
			fixtureName:      "v5-physical-good",
			expectedResponse: 5,
			expectedTitle:    "ETH - $1",
			expectedSlug:     "eth-1",
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
		if l.Metadata.Version != e.expectedResponse {
			t.Errorf("expected to have version response (%+v), but instead was (%+v)", e.expectedResponse, l.Metadata.Version)
		}
		if title, _ := l.GetTitle(); title != e.expectedTitle {
			t.Errorf("expected to have title response (%+v), but instead was (%+v)", e.expectedTitle, title)
		}
		if slug, _ := l.GetSlug(); slug != e.expectedSlug {
			t.Errorf("expected to have slug response (%+v), but instead was (%+v)", e.expectedSlug, slug)
		}
		if price, _ := l.GetPrice(); !price.Equal(e.expectedPrice) {
			t.Errorf("expected to have price response (%+v), but instead was (%+v)", e.expectedPrice, price)
		}
		if acceptedCurrencies, _ := l.GetAcceptedCurrencies(); len(acceptedCurrencies) != len(e.expectedAcceptedCurrencies) {
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

	if subject.Slug != actual.Slug {
		t.Errorf("expected slug to be (%s), but was (%s)", subject.Slug, actual.Slug)
	}
	if subject.TermsAndConditions != actual.TermsAndConditions {
		t.Errorf("expected terms/conditions to be (%s), but was (%s)", subject.TermsAndConditions, actual.TermsAndConditions)
	}
	if subject.RefundPolicy != actual.RefundPolicy {
		t.Errorf("expected refund policy to be (%s), but was (%s)", subject.RefundPolicy, actual.RefundPolicy)
	}
	if subject.Metadata.Version != actual.ListingVersion {
		t.Errorf("expected vesion to be (%d), but was (%d)", subject.Metadata.Version, actual.ListingVersion)
	}
	if hash, err := actual.Vendor.Hash(); err != nil && subject.VendorID.PeerID != hash {
		t.Errorf("expected hash to be (%s), but was (%s)", subject.VendorID.PeerID, hash)
		t.Logf("hash had an error: %s", err)

	}
	if !bytes.Equal(subject.VendorID.BitcoinSig, actual.Vendor.BitcoinSignature()) {
		t.Errorf("expected refund policy to be (%s), but was (%s)", subject.VendorID.BitcoinSig, actual.Vendor.BitcoinSignature())
	}

}
