package repo_test

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestListingIndexUnmarshalJSON(t *testing.T) {
	var examples = map[string][]repo.ListingIndexData{
		"v4-index": []repo.ListingIndexData{
			{
				Hash:         "QmbAhieQdN7WzDktpkZ3ZDuv3AKF3DxG3SMFyEcVg3HGcP",
				Slug:         "test-service",
				Title:        "Test Service",
				Categories:   []string{"test"},
				NSFW:         true,
				ContractType: "SERVICE",
				Description:  "Test service listing",
				Thumbnail: repo.ListingThumbnail{
					Tiny:   "zb2rhfN4RQyNP6eZszvEBfwBRMZxaysoqF72MYWPKoofV5AQr",
					Small:  "zb2rhfMZFaaWZxZGvkqAPCMUbmdxNHhAaCby5XCkRrV13bew8",
					Medium: "zb2rhnppMGkZYp6Zg7Qf2irDH9z1ZM5jc2VcAXfy6mEnifoEy",
				},
				Price: repo.Price{
					CurrencyCode: "USD",
					Amount: repo.CurrencyValue{
						Amount: big.NewInt(25),
						Currency: &repo.CurrencyDefinition{
							Name:         "United States Dollar",
							Code:         "USD",
							Divisibility: 2,
							CurrencyType: "fiat",
						},
					},
					Modifier: 0,
				},
				ShipsTo:       []string{},
				FreeShipping:  []string{},
				Language:      "English",
				AverageRating: 0,
				RatingCount:   0,
				ModeratorIDs: []string{
					"QmQifVhzhnHRu9bGT9WNcbbuc5EF2bXRF6iJpSzNj7yRtQ",
				},
				AcceptedCurrencies: []string{
					"BTC",
					"BCH",
					"ZEC",
					"LTC",
				},
				CoinType: "",
			},
		},
	}

	for fixtureName, expected := range examples {
		var (
			fixtureBytes = factory.MustLoadListingFixture(fixtureName)
			l, err       = repo.UnmarshalJSONSignedListingIndex(fixtureBytes)
		)
		if err != nil {
			t.Errorf("error parsing fixture (%s): %s", fixtureName, err)
		}
		if !reflect.DeepEqual(expected, l) {
			t.Errorf("failed to parse (%s)", fixtureName)
			t.Logf("\texpected: %+v", expected)
			t.Logf("\tactual: %+v", l)
		}
	}
}
