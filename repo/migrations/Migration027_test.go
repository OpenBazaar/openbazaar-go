package migrations_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strconv"
	"testing"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestUpdateListingHash(t *testing.T) {
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
		testListingSlug  = "UpdateListingHash_test_listing"
		testListingPath  = testRepo.DataPathJoin("root", "listings", testListingSlug+".json")

		// This listing hash is generated using the default IPFS hashing algorithm as of v0.4.19
		// If the default hashing algorithm changes at any point in the future you can expect this
		// test to fail and it will need to be updated to maintain the functionality of this migration.
		expectedListingHash = "Qmdv3oZmVtRuN4tWewsUcBqFYKGD8kaQCq46Yx6rwzEDvH"

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

	index := []*migrations.UpdateListingHash_ListingData{extractListingData26(listing)}
	indexJSON, err := json.MarshalIndent(&index, "", "    ")
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(listingIndexPath, indexJSON, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	var migration migrations.UpdateListingHash
	if err := migration.Up(testRepo.DataPath(), "", true); err != nil {
		t.Fatal(err)
	}

	var listingIndex []migrations.UpdateListingHash_ListingData
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

	assertCorrectRepoVer(t, repoverPath, "27")

	if err := migration.Down(testRepo.DataPath(), "", true); err != nil {
		t.Fatal(err)
	}

	assertCorrectRepoVer(t, repoverPath, "26")
}

func extractListingData26(listing *pb.Listing) *migrations.UpdateListingHash_ListingData {
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

	ld := &migrations.UpdateListingHash_ListingData{
		Hash:         "aabbcc",
		Slug:         listing.Slug,
		Title:        listing.Item.Title,
		Categories:   listing.Item.Categories,
		NSFW:         listing.Item.Nsfw,
		ContractType: listing.Metadata.ContractType.String(),
		Description:  listing.Item.Description[:descriptionLength],
		Thumbnail:    migrations.UpdateListingHash_Thumbnail{listing.Item.Images[0].Tiny, listing.Item.Images[0].Small, listing.Item.Images[0].Medium},
		Price: migrations.UpdateListingHash_Price{
			CurrencyCode: listing.Item.PriceCurrency.Code,
			Amount:       amt,
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
