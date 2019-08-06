package migrations_test

import (
	"encoding/json"
	"io/ioutil"
	"math/big"
	"os"
	"testing"

	"github.com/OpenBazaar/jsonpb"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestMigration025(t *testing.T) {
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
		testListingSlug  = "Migration025_test_listing"
		testListingPath  = testRepo.DataPathJoin("root", "listings", testListingSlug+".json")

		// This listing hash is generated using the default IPFS hashing algorithm as of v0.4.19
		// If the default hashing algorithm changes at any point in the future you can expect this
		// test to fail and it will need to be updated to maintain the functionality of this migration.
		expectedListingHash = "QmeEhL5jcnuCimemQ9A5XATGTtDSEkMwveChaxiepoUBQF" //"QmfEr6qqLxRsjJhk1XPq2FBP6aiwG6w6Dwr1XepU1Rg1Wx"

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

	index := []*migrations.Migration025_ListingData{extractListingData(listing)}
	indexJSON, err := json.MarshalIndent(&index, "", "    ")
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(listingIndexPath, indexJSON, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	var migration migrations.Migration025
	if err := migration.Up(testRepo.DataPath(), "", true); err != nil {
		t.Fatal(err)
	}

	var listingIndex []migrations.Migration025_ListingData
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

	assertCorrectRepoVer(t, repoverPath, "26")

	if err := migration.Down(testRepo.DataPath(), "", true); err != nil {
		t.Fatal(err)
	}

	assertCorrectRepoVer(t, repoverPath, "25")
}

func extractListingData(listing *pb.Listing) *migrations.Migration025_ListingData {
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
				serviceValue, _ := new(big.Int).SetString(service.PriceValue.Amount, 10)
				if serviceValue.Int64() == 0 && !contains(freeShipping, region.String()) {
					freeShipping = append(freeShipping, region.String())
				}
			}
		}
	}

	priceValue, _ := new(big.Int).SetString(listing.Item.PriceValue.Amount, 10)
	ld := &migrations.Migration025_ListingData{
		Hash:         "aabbcc",
		Slug:         listing.Slug,
		Title:        listing.Item.Title,
		Categories:   listing.Item.Categories,
		NSFW:         listing.Item.Nsfw,
		CoinType:     listing.Metadata.PricingCurrencyDefn.Code,
		ContractType: listing.Metadata.ContractType.String(),
		Description:  listing.Item.Description[:descriptionLength],
		Thumbnail:    migrations.Migration029_Thumbnail{listing.Item.Images[0].Tiny, listing.Item.Images[0].Small, listing.Item.Images[0].Medium},
		Price: migrations.Migration029_Price{
			CurrencyCode: listing.Metadata.PricingCurrencyDefn.Code,
			Amount:       priceValue.Uint64(),
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
