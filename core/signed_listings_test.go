package core_test

import (
	"path/filepath"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"

	"github.com/OpenBazaar/openbazaar-go/core"
)

func TestOpenBazaarSignedListings_GetSignedListingFromPath(t *testing.T) {

	absPathInvalid, _ := filepath.Abs("../test/contracts/signed_listings_1_invalid.json")

	// Check for non-existent file
	_, err := core.GetSignedListingFromPath("nonsense.file")
	if err == nil {
		t.Error(err)
	}

	// Check for invalid listing
	_, err = core.GetSignedListingFromPath(absPathInvalid)
	if err == nil {
		t.Error(err)
	}
}

func TestOpenBazaarSignedListings_SetAcceptedCurrencies(t *testing.T) {
	absPath, _ := filepath.Abs("../test/contracts/signed_listings_1.json")
	currencies := []string{"TEST"}

	listing, err := core.GetSignedListingFromPath(absPath)
	if err != nil {
		t.Error(err)
	}
	oldCurrencies := listing.Listing.Metadata.AcceptedCurrencies

	core.SetAcceptedCurrencies(listing, currencies)

	if EqualStringSlices(listing.Listing.Metadata.AcceptedCurrencies, oldCurrencies) {
		t.Error("Accepted currencies were not updated")
	}

	if !EqualStringSlices(listing.Listing.Metadata.AcceptedCurrencies, currencies) {
		t.Error("Accepted currencies changed but not correctly")
	}
}

func TestOpenBazaarSignedListings_AssignMatchingCoupons(t *testing.T) {
	absPath, _ := filepath.Abs("../test/contracts/signed_listings_1.json")
	coupons := []repo.Coupon{
		{"signed_listings_1", "test", "QmQ5vueeX64fsSo6fU9Z1dDFMR9rky5FjowEr7m7cSiGd8"},
		{"signed_listings_1", "bad", "BADHASH"},
	}

	listing, err := core.GetSignedListingFromPath(absPath)
	if err != nil {
		t.Error(err)
	}
	//old_coupons := listing.Listing.Coupons

	core.AssignMatchingCoupons(coupons, listing)

	if listing.Listing.Coupons[0].GetDiscountCode() != "test" {
		t.Error("Coupons were not assigned")
	}

	if listing.Listing.Coupons[0].GetDiscountCode() == "bad" || listing.Listing.Coupons[1].GetDiscountCode() == "bad" {
		t.Error("Coupons were assigned improperly")
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
