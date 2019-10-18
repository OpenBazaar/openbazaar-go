package migrations_test

import (
	"encoding/json"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
)

var (
	preMigration027ListingJSON = `{
    "coupons": [
        {
            "priceDiscount": 10
        }
    ],
    "item": {
        "price": 100,
        "skus": [
            {
                "quantity": 10,
                "surcharge": 9
            }
        ]
    },
    "metadata": {
        "coinDivisibility": 8,
        "coinType": "BCH",
        "priceModifier": 1,
        "pricingCurrency": "BTC"
    },
    "shippingOptions": [
        {
            "services": [
                {
                    "additionalItemPrice": 5,
                    "price": 10
                }
            ]
        }
    ]
}`

	postMigration027ListingJSON = `{
    "coupons": [
        {
            "bigPriceDiscount": "10"
        }
    ],
    "item": {
        "bigPrice": "100",
        "priceCurrency": {
            "code": "BTC",
            "divisibility": 8
        },
        "priceModifier": 1,
        "skus": [
            {
                "bigQuantity": "10",
                "bigSurcharge": "9"
            }
        ]
    },
    "metadata": {
        "cryptoCurrencyCode": "BCH",
        "cryptoDivisibility": 8
    },
    "shippingOptions": [
        {
            "services": [
                {
                    "bigAdditionalItemPrice": "5",
                    "bigPrice": "10"
                }
            ]
        }
    ]
}`
)

func TestMigration027(t *testing.T) {
	var testRepo, err = schema.NewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err = testRepo.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer testRepo.DestroySchemaDirectories()

	var (
		repoverPath      = testRepo.DataPathJoin("repover")
		listingIndexPath = testRepo.DataPathJoin("root", "listings.json")
		testListingSlug  = "Migration027_test_listing"
		testListingPath  = testRepo.DataPathJoin("root", "listings", testListingSlug+".json")

		listing = factory.NewListing(testListingSlug)
	)

	f, err := os.Create(testListingPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(preMigration027ListingJSON)); err != nil {
		t.Fatal(err)
	}

	index := []*migrations.Migration027V4ListingIndexData{extractListingData27(listing)}
	indexJSON, err := json.MarshalIndent(&index, "", "    ")
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(listingIndexPath, indexJSON, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	var migration migrations.Migration027
	if err := migration.Up(testRepo.DataPath(), "", true); err != nil {
		t.Fatal(err)
	}

	upMigratedListing, err := ioutil.ReadFile(testListingPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(upMigratedListing) != postMigration027ListingJSON {
		t.Fatal("Failed to migrate listing up")
	}

	var listingIndex []migrations.Migration027V5ListingIndexData
	listingsJSON, err := ioutil.ReadFile(listingIndexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(listingsJSON, &listingIndex); err != nil {
		t.Fatal(err)
	}

	if listingIndex[0].Price.Amount.String() != strconv.Itoa(int(index[0].Price.Amount)) {
		t.Errorf("Incorrect price set")
	}

	if string(listingIndex[0].Price.Currency.Code) != index[0].Price.CurrencyCode {
		t.Errorf("Incorrect currency code set")
	}

	assertCorrectRepoVer(t, repoverPath, "28")

	if err := migration.Down(testRepo.DataPath(), "", true); err != nil {
		t.Fatal(err)
	}

	downMigratedListing, err := ioutil.ReadFile(testListingPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(downMigratedListing) != preMigration027ListingJSON {
		t.Fatal("Failed to migrate listing up")
	}

	var listingIndex2 []migrations.Migration027V4ListingIndexData
	listingsJSON, err = ioutil.ReadFile(listingIndexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(listingsJSON, &listingIndex2); err != nil {
		t.Fatal(err)
	}

	if listingIndex[0].Price.Amount.String() != strconv.Itoa(int(listingIndex2[0].Price.Amount)) {
		t.Errorf("Incorrect price set")
	}

	if string(listingIndex[0].Price.Currency.Code) != listingIndex2[0].Price.CurrencyCode {
		t.Errorf("Incorrect currency code set")
	}

	assertCorrectRepoVer(t, repoverPath, "27")
}

func extractListingData27(listing *pb.Listing) *migrations.Migration027V4ListingIndexData {
	descriptionLength := len(listing.Item.Description)

	contains := func(s []string, e string) bool {
		for _, a := range s {
			if a == e {
				return true
			}
		}
		return false
	}

	var shipsTo []string
	var freeShipping []string
	for _, shippingOption := range listing.ShippingOptions {
		for _, region := range shippingOption.Regions {
			if !contains(shipsTo, region.String()) {
				shipsTo = append(shipsTo, region.String())
			}
			for _, service := range shippingOption.Services {
				if service.BigPrice == "0" && !contains(freeShipping, region.String()) {
					freeShipping = append(freeShipping, region.String())
				}
			}
		}
	}

	amt, _ := strconv.ParseUint(listing.Item.BigPrice, 10, 64)

	ld := &migrations.Migration027V4ListingIndexData{
		Hash:         "aabbcc",
		Slug:         listing.Slug,
		Title:        listing.Item.Title,
		Categories:   listing.Item.Categories,
		NSFW:         listing.Item.Nsfw,
		ContractType: listing.Metadata.ContractType.String(),
		Description:  listing.Item.Description[:descriptionLength],
		Thumbnail:    migrations.Migration027ListingThumbnail{listing.Item.Images[0].Tiny, listing.Item.Images[0].Small, listing.Item.Images[0].Medium},
		Price: migrations.Migration027V4price{
			CurrencyCode: listing.Item.PriceCurrency.Code,
			Amount:       uint(amt),
			Modifier:     listing.Metadata.PriceModifier,
		},
		ShipsTo:            shipsTo,
		FreeShipping:       freeShipping,
		Language:           listing.Metadata.Language,
		ModeratorIDs:       listing.Moderators,
		AcceptedCurrencies: listing.Metadata.AcceptedCurrencies,
	}
	return ld
}
