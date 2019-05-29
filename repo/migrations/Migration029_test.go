package migrations_test

import (
	"encoding/json"
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
	"io/ioutil"
	"os"
	"testing"
)

func TestMigration029(t *testing.T) {
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
		testListingSlug  = "Migration029_test_listing"
		testListingPath  = testRepo.DataPathJoin("root", "listings", testListingSlug+".json")

		// This listing hash is generated using the default IPFS hashing algorithm as of v0.4.19
		// If the default hashing algorithm changes at any point in the future you can expect this
		// test to fail and it will need to be updated to maintain the functionality of this migration.
		expectedListingHash = "QmfEr6qqLxRsjJhk1XPq2FBP6aiwG6w6Dwr1XepU1Rg1Wx"

		listing = factory.NewListing(testListingSlug)
		m       = jsonpb.Marshaler{
			Indent:       "    ",
			EmitDefaults: true,
		}
	)

	f, err := os.Create(testListingPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Marshal(f, listing); err != nil {
		t.Fatal(err)
	}

	index := []*migrations.Migration029_ListingData{extractListingData(listing)}
	indexJSON, err := json.MarshalIndent(&index, "", "    ")
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(listingIndexPath, indexJSON, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	var migration migrations.Migration029
	if err := migration.Up(testRepo.DataPath(), "", true); err != nil {
		t.Fatal(err)
	}

	var listingIndex []migrations.Migration029_ListingData
	listingsJSON, err := ioutil.ReadFile(listingIndexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(listingsJSON, &listingIndex); err != nil {
		t.Fatal(err)
	}

	// See comment above on expectedListingHash
	if listingIndex[0].Hash != expectedListingHash {
		t.Errorf("Expected listing hash %s got %s", expectedListingHash, listingIndex[0].Hash)
	}

	assertCorrectRepoVer(t, repoverPath, "30")

	if err := migration.Down(testRepo.DataPath(), "", true); err != nil {
		t.Fatal(err)
	}

	assertCorrectRepoVer(t, repoverPath, "29")
}

func extractListingData(listing *pb.Listing) *migrations.Migration029_ListingData {
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
				if service.Price == 0 && !contains(freeShipping, region.String()) {
					freeShipping = append(freeShipping, region.String())
				}
			}
		}
	}

	ld := &migrations.Migration029_ListingData{
		Hash:         "aabbcc",
		Slug:         listing.Slug,
		Title:        listing.Item.Title,
		Categories:   listing.Item.Categories,
		NSFW:         listing.Item.Nsfw,
		CoinType:     listing.Metadata.CoinType,
		ContractType: listing.Metadata.ContractType.String(),
		Description:  listing.Item.Description[:descriptionLength],
		Thumbnail:    migrations.Migration029_Thumbnail{listing.Item.Images[0].Tiny, listing.Item.Images[0].Small, listing.Item.Images[0].Medium},
		Price: migrations.Migration029_Price{
			CurrencyCode: listing.Metadata.PricingCurrency,
			Amount:       listing.Item.Price,
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
