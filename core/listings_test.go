package core_test

import (
	"testing"

	"github.com/golang/protobuf/proto"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestFactoryCryptoListingCoinDivisibilityMatchesConst(t *testing.T) {
	if factory.NewCryptoListing("blu").Metadata.CoinDivisibility != core.DefaultCoinDivisibility {
		t.Fatal("DefaultCoinDivisibility constant has changed. Please update factory value.")
	}
}

var expectedErrorStatesForValidShippingRegion = map[int32]error{
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

func TestValidShippingRegionErrorCases(t *testing.T) {
	for example, expectedResult := range expectedErrorStatesForValidShippingRegion {
		listing := factory.NewListingWithShippingRegions("asdfasdf")
		listing.ShippingOptions[0].Regions = []pb.CountryCode{pb.CountryCode(example)}
		for _, shippingOption := range listing.ShippingOptions {
			if result := core.ValidShippingRegion(shippingOption); result != expectedResult {
				t.Errorf("unexpected result using CountryCode (%d): %s", example, result)
			}
		}
	}
}

func TestValidShippingRegionUsingDefinedCountryCodes(t *testing.T) {
	for countryCode := range pb.CountryCode_name {
		listing := factory.NewListingWithShippingRegions("asdfasdf")
		listing.ShippingOptions[0].Regions = []pb.CountryCode{pb.CountryCode(countryCode)}
		for _, shippingOption := range listing.ShippingOptions {
			result := core.ValidShippingRegion(shippingOption)
			if result != expectedErrorStatesForValidShippingRegion[countryCode] {
				t.Errorf("unexpected result using CountryCode (%d): %s", countryCode, result)
			}
		}
	}
}

func TestListingProtobufAlias(t *testing.T) {
	countrycodes := []pb.CountryCode{
		pb.CountryCode(212),
		pb.CountryCode_SWAZILAND,
		pb.CountryCode_ESWATINI,
	}
	for _, cc := range countrycodes {
		var (
			listing             = factory.NewListingWithShippingRegions("asdfasdf")
			unmarshalledListing = &pb.Listing{}
		)
		listing.ShippingOptions[0].Regions = []pb.CountryCode{cc}

		marshalled, err := proto.Marshal(listing)
		if err != nil {
			t.Fatal(err)
		}
		err = proto.Unmarshal(marshalled, unmarshalledListing)
		if err != nil {
			t.Fatal(err)
		}
		for _, region := range unmarshalledListing.ShippingOptions[0].Regions {
			if region != pb.CountryCode_ESWATINI {
				t.Fatal("expected aliased CountryCode to always unmarshal as pb.CountryCode_ESWATINI but didn't")
			}
		}
	}
}
