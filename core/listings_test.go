package core_test

import (
	"testing"
	"reflect"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestFactoryCryptoListingCoinDivisibilityMatchesConst(t *testing.T) {
	if factory.NewCryptoListing("blu").Metadata.CoinDivisibility != core.DefaultCoinDivisibility {
		t.Fatal("DefaultCoinDivisibility constant has changed. Please update factory value.")
	}
}

func TestValidShippingRegion(t *testing.T) {
	check := map[int32]bool {
		// NA
		0: true,
		// continents
		501: true,
		502: true,
		503: true,
		504: true,
		505: true,
		506: true,
		507: true,
		508: true,
		// !exist
		509: true,
		510: true,
		511: true,
		// some random numbers
		5678: true,
		123456: true,
	}
	// skip NA, continents, a few random numbers
	for _, v := range pb.CountryCode_value {
		if !check[v] {
			cc := pb.CountryCode(v)
			listing := factory.NewShippingRegionListing("asdfasdf", cc)
			for _, shippingOption := range listing.ShippingOptions {
				if ok := core.ValidShippingRegion(shippingOption); ok > 0 {
					t.Fatalf("Something has changed with valid shipping regions: %d %d", ok, v)
				}
			}
		}
	}
	// DONT skip NA, continents, a few random numbers
	for _, v := range pb.CountryCode_value {
		if check[v] {
			cc := pb.CountryCode(v)
			listing := factory.NewShippingRegionListing("asdfasdf", cc)
			for _, shippingOption := range listing.ShippingOptions {
				if ok := core.ValidShippingRegion(shippingOption); ok > 0 {
					t.Logf("Should error: %d %d", ok, v)
				}
			}
		}
	}
	count := 0
	m := make(map[int]bool)
	for n, _ := range check {
		cc := pb.CountryCode(n)
		listing := factory.NewShippingRegionListing("asdfasdf", cc)
		for _, shippingOption := range listing.ShippingOptions {
			if ok := core.ValidShippingRegion(shippingOption); ok > 0 {
				if ok == 2 {
					t.Logf("Should error2: %d %d", ok, n)
				}
				m[ok] = true
				count++
			}
		}
	}
	if count != 14 {
		t.Fatalf("Something has changed with valid shipping regions: counted %d", count)
	}
	errorCodes := map[int]bool{
		1: true, // NA
		2: true, // continent
		3: true, // !Exist
	}
	same := reflect.DeepEqual(m, errorCodes)
	if !same {
		t.Errorf("New/Unseen Shipping Region Error Code %v", same)
	}
}
