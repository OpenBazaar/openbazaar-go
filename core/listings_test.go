package core_test

import (
	// "reflect"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestFactoryCryptoListingCoinDivisibilityMatchesConst(t *testing.T) {
	if factory.NewCryptoListing("blu").Metadata.CoinDivisibility != core.DefaultCoinDivisibility {
		t.Fatal("DefaultCoinDivisibility constant has changed. Please update factory value.")
	}
}

func TestValidShippingRegion(t *testing.T) {
	check := map[int32]error{
		0: core.ErrShippingRegionMustBeSet,

		1:   nil,
		247: nil,
		248: core.ErrShippingRegionUndefined,
		500: nil,

		501: core.ErrShippingRegionMustNotBeContinent,
		502: core.ErrShippingRegionMustNotBeContinent,
		503: core.ErrShippingRegionMustNotBeContinent,
		504: core.ErrShippingRegionMustNotBeContinent,
		505: core.ErrShippingRegionMustNotBeContinent,
		506: core.ErrShippingRegionMustNotBeContinent,
		507: core.ErrShippingRegionMustNotBeContinent,
		508: core.ErrShippingRegionMustNotBeContinent,

		509: core.ErrShippingRegionUndefined,
		510: core.ErrShippingRegionUndefined,
		511: core.ErrShippingRegionUndefined,

		5678:   core.ErrShippingRegionUndefined,
		123456: core.ErrShippingRegionUndefined,
	}
	// check error map
	m1 := make(map[int32]error)
	for v := range check {
		cc := pb.CountryCode(v)
		listing := factory.NewShippingRegionListing("asdfasdf", cc)
		for _, shippingOption := range listing.ShippingOptions {
			if err := core.ValidShippingRegion(shippingOption); err != nil {
				m1[v] = err
			} else {
				m1[v] = nil
			}
		}
	}

	// check the countrycodes.proto
	m2 := make(map[int32]error)
	for v := range pb.CountryCode_name {
		cc := pb.CountryCode(v)
		listing := factory.NewShippingRegionListing("asdfasdf", cc)
		for _, shippingOption := range listing.ShippingOptions {
			if err := core.ValidShippingRegion(shippingOption); err != nil {
				m2[v] = err
			} else {
				m2[v] = nil
			}
		}
	}

	for v, errtype := range m2 {
		if check[v] != errtype {
			t.Fatalf("( cc: %d, '%v' != '%v' ) : CountryCode does not match tests error checking map.\n", v, errtype, check[v])
		}
	}

	check[247] = core.ErrShippingRegionUndefined
	for v, errtype := range m1 {
		if check[v] != errtype {
			t.Logf("Should fail: ( cc: %d, '%v' != '%v' ) : CountryCode does not match tests error checking map.\n", v, errtype, check[v])
		}
	}
	for v, errtype := range m2 {
		if check[v] != errtype {
			t.Logf("Should fail: ( cc: %d, '%v' != '%v' ) : CountryCode does not match tests error checking map.\n", v, errtype, check[v])
		}
	}
}
