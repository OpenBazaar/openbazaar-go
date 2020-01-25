package core_test

import (
	"testing"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestOpenBazaarSignedListings_GetSignedListingFromPath(t *testing.T) {

	// Check for non-existent file
	_, err := core.GetSignedListingFromPath("nonsense.file")
	if err == nil {
		t.Error(err)
	}

	// Check for invalid listing
	invalidBytes := []byte(`{ "listings": }`)
	_, err = repo.UnmarshalJSONSignedListing(invalidBytes)
	if err == nil {
		t.Error(err)
	}
}

func TestOpenBazaarSignedListings_SetAcceptedCurrencies(t *testing.T) {
	currencies := []string{"ETH"}

	fixtureBytes := factory.MustLoadListingFixture("v5-signed-physical-good-2")
	slisting, err := repo.UnmarshalJSONSignedListing(fixtureBytes)
	if err != nil {
		t.Error(err)
	}
	listing := slisting.GetListing()

	oldCurrencies := listing.GetAcceptedCurrencies()

	if err := listing.SetAcceptedCurrencies(currencies...); err != nil {
		t.Fatal(err)
	}

	if EqualStringSlices(listing.GetAcceptedCurrencies(), oldCurrencies) {
		t.Error("Accepted currencies were not updated")
	}

	if !EqualStringSlices(listing.GetAcceptedCurrencies(), currencies) {
		t.Error("Accepted currencies changed but not correctly")
	}
}

func EqualStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {

		if v != b[i] {
			return false
		}
	}
	return true
}
