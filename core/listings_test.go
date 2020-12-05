package core_test

import (
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestOpenBazaarNode_SetCurrencyOnListings(t *testing.T) {
	node, err := test.NewNode()
	if err != nil {
		t.Fatal(err)
	}

	var (
		regularListingSlug    = "test_regular_listing"
		cryptoListingSlug     = "test_crypto_listing"
		newAcceptedCurrencies = []string{"TBCH", "TLTC"}
		cryptoListingCurrency = "TBTC"
	)

	regularListing := factory.NewListing(regularListingSlug)
	regularListing.Metadata.AcceptedCurrencies = []string{"TBTC"}
	regularRepoListing, err := repo.NewListingFromProtobuf(regularListing)
	if err != nil {
		t.Fatal(err)
	}

	lb, err := regularRepoListing.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := node.CreateListing(lb); err != nil {
		t.Fatal(err)
	}

	cryptoListing := factory.NewCryptoListing(cryptoListingSlug)
	regularListing.Metadata.AcceptedCurrencies = []string{cryptoListingCurrency}
	cryptoRepoListing, err := repo.NewListingFromProtobuf(cryptoListing)
	if err != nil {
		t.Fatal(err)
	}

	clb, err := cryptoRepoListing.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := node.CreateListing(clb); err != nil {
		t.Fatal(err)
	}

	if err := node.SetCurrencyOnListings(newAcceptedCurrencies, true); err != nil {
		t.Fatal(err)
	}

	checkListing, err := node.GetListingFromSlug(regularListingSlug)
	if err != nil {
		t.Fatal(err)
	}
	if checkListing.Listing.Metadata.AcceptedCurrencies[0] != newAcceptedCurrencies[0] ||
		checkListing.Listing.Metadata.AcceptedCurrencies[1] != newAcceptedCurrencies[1] ||
		len(checkListing.Listing.Metadata.AcceptedCurrencies) != len(newAcceptedCurrencies) {

		t.Errorf("Listing %s expected accepted currency list %v, got %v", regularListingSlug, newAcceptedCurrencies, checkListing.Listing.Metadata.AcceptedCurrencies)
	}

	checkListing2, err := node.GetListingFromSlug(cryptoListingSlug)
	if err != nil {
		t.Fatal(err)
	}
	if len(checkListing2.Listing.Metadata.AcceptedCurrencies) != 1 || checkListing2.Listing.Metadata.AcceptedCurrencies[0] != cryptoListingCurrency {

		t.Errorf("Listing %s expected accepted currency list %v, got %v", cryptoListingSlug, []string{cryptoListingCurrency}, checkListing2.Listing.Metadata.AcceptedCurrencies)
	}

}
