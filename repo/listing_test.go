package repo_test

import (
	//"fmt"
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
	}

	for _, e := range examples {
		var (
			fixtureBytes = factory.MustLoadListingFixture(e)
			_, err       = repo.UnmarshalJSONListing(fixtureBytes)
		)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestListingAttributes(t *testing.T) {
	var examples = []struct {
		fixtureName      string
		expectedResponse uint
		expectedTitle    string
		expectedSlug     string
		expectedPrice    repo.CurrencyValue
	}{
		{
			fixtureName:      "v3-physical-good",
			expectedResponse: 3,
			expectedTitle:    "Physical Listing",
			expectedSlug:     "physical-listing",
			expectedPrice: repo.CurrencyValue{
				Amount: big.NewInt(1235000000),
				Currency: &repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("BCH"),
					Divisibility: 8,
				},
			},
		},
		{
			fixtureName:      "v4-physical-good",
			expectedResponse: 4,
			expectedTitle:    "Physical Good Listing",
			expectedSlug:     "physical-good-listing",
			expectedPrice: repo.CurrencyValue{
				Amount: big.NewInt(12345678000),
				Currency: &repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("BCH"),
					Divisibility: 8,
				},
			},
		},
		{
			fixtureName:      "v4-digital-good",
			expectedResponse: 4,
			expectedTitle:    "Digital Good Listing",
			expectedSlug:     "digital-good-listing",
			expectedPrice: repo.CurrencyValue{
				Amount: big.NewInt(1320),
				Currency: &repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("USD"),
					Divisibility: 8,
				},
			},
		},
		{
			fixtureName:      "v4-service",
			expectedResponse: 4,
			expectedTitle:    "Service Listing",
			expectedSlug:     "service-listing",
			expectedPrice: repo.CurrencyValue{
				Amount: big.NewInt(9877000000),
				Currency: &repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("BTC"),
					Divisibility: 8,
				},
			},
		},
		{
			fixtureName:      "v4-cryptocurrency",
			expectedResponse: 4,
			expectedTitle:    "LTC-XMR",
			expectedSlug:     "ltc-xmr",
			expectedPrice: repo.CurrencyValue{
				Amount: big.NewInt(0),
				Currency: &repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("XMR"),
					Divisibility: 8,
				},
			},
		},
		{
			fixtureName:      "v5-physical-good",
			expectedResponse: 5,
			expectedTitle:    "ETH - $1",
			expectedSlug:     "eth-1",
			expectedPrice: repo.CurrencyValue{
				Amount: big.NewInt(100),
				Currency: &repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("USD"),
					Divisibility: 8,
				},
			},
		},
	}

	for _, e := range examples {
		var (
			fixtureBytes = factory.MustLoadListingFixture(e.fixtureName)
			l, err       = repo.UnmarshalJSONListing(fixtureBytes)
		)
		if err != nil {
			t.Errorf("unable to unmarshal example (%s)", e.fixtureName)
			continue
		}
		//fmt.Println("l version : ", l.Metadata.Version, "  exp ver : ", e.expectedResponse, "   extr ver : ", l.ListingVersion)
		if l.Metadata.Version != e.expectedResponse {
			t.Errorf("expected example (%s) to have version response (%+v), but instead was (%+v)", e.fixtureName, e.expectedResponse, l.Metadata.Version)
		}
		if title, _ := l.GetTitle(); title != e.expectedTitle {
			t.Errorf("expected example (%s) to have title response (%+v), but instead was (%+v)", e.fixtureName, e.expectedTitle, title)
		}
		if slug, _ := l.GetSlug(); slug != e.expectedSlug {
			t.Errorf("expected example (%s) to have slug response (%+v), but instead was (%+v)", e.fixtureName, e.expectedSlug, slug)
		}
		if price, _ := l.GetPrice(); !price.Equal(&e.expectedPrice) {
			t.Errorf("expected example (%s) to have price response (%+v), but instead was (%+v)", e.fixtureName, e.expectedPrice, price)
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
	//fmt.Println(actual.ListingVersion)
	//fmt.Println(actual.ListingBytes)
	if subject.Metadata.Version != actual.ListingVersion {
		t.Errorf("expected vesion to be (%d), but was (%d)", subject.Metadata.Version, actual.ListingVersion)
	}
	//fmt.Println(actual.GetTitle())
	//if hash, err := actual.Vendor.Hash(); err != nil && subject.VendorID.PeerID != hash {
	//	t.Errorf("expected hash to be (%s), but was (%s)", subject.VendorID.PeerID, hash)
	//	if err != nil {
	//		t.Logf("hash had an error: %s", err)
	//	}
	//}
	//if !bytes.Equal(subject.VendorID.BitcoinSig, actual.Vendor.BitcoinSignature()) {
	//	t.Errorf("expected refund policy to be (%s), but was (%s)", subject.VendorID.BitcoinSig, actual.Vendor.BitcoinSignature())
	//}
}
