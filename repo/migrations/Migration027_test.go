package migrations_test

import (
	"encoding/base64"
	"encoding/json"
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
	"github.com/golang/protobuf/proto"
	coremock "github.com/ipfs/go-ipfs/core/mock"
	crypto "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
)

const (
	testMigration027_IdentityPrivateKeyBase64 = "CAESYHwrVuRp5s2u0w5ykibsR77aHWBmvpcaDq+vU9pv8lOqae31NJYJbdDsOlxVRqQZS/eDfssdd7N/rJmoVbQvPytp7fU0lglt0Ow6XFVGpBlL94N+yx13s3+smahVtC8/Kw=="
)

var (
	preMigration027ListingJSON = `{
    "listing": {
        "slug": "Migration027_test_listing",
        "metadata": {
            "contractType": "PHYSICAL_GOOD",
            "format": "FIXED_PRICE",
            "pricingCurrency": "BTC",
            "priceModifier": 1
        },
        "item": {
            "price": 100,
            "skus": [
                {
                    "surcharge": 9,
                    "quantity": 10
                }
            ]
        },
        "shippingOptions": [
            {
                "type": "LOCAL_PICKUP",
                "services": [
                    {
                        "price": 10,
                        "additionalItemPrice": 5
                    }
                ]
            }
        ],
        "coupons": [
            {
                "priceDiscount": 10
            }
        ]
    },
    "signature": "2+M/+M866ZaPyqHpBdsqJ9gCgLPNOQoKKtaOrruZmDu6YXhc3RiKQtoZs1BTfC02k8TwfIU5LeFPhxagwC6UAg=="
}`

	postMigration027ListingJSON = `{
    "listing": {
        "slug": "Migration027_test_listing",
        "metadata": {
            "contractType": "PHYSICAL_GOOD",
            "format": "FIXED_PRICE"
        },
        "item": {
            "skus": [
                {
                    "bigSurcharge": "9",
                    "bigQuantity": "10"
                }
            ],
            "priceModifier": 1,
            "bigPrice": "100",
            "priceCurrency": {
                "code": "BTC",
                "divisibility": 8
            }
        },
        "shippingOptions": [
            {
                "type": "LOCAL_PICKUP",
                "services": [
                    {
                        "bigPrice": "10",
                        "bigAdditionalItemPrice": "5"
                    }
                ]
            }
        ],
        "coupons": [
            {
                "bigPriceDiscount": "10"
            }
        ]
    },
    "signature": "r7j6YM3ePiTRyIKmd9ILMvjvhv5jaAB5m1gMlPJ9ug05KZIfAPXXa+EtuF+ExGvtEOpdyK3BEFpU2rm65CyvDw=="
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
	if err := testRepo.InitializeDatabase(); err != nil {
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

	db, err := migrations.OpenDB(testRepo.DataPath(), "", true)
	if err != nil {
		t.Error(err)
	}

	identityKey, err := base64.StdEncoding.DecodeString(testMigration027_IdentityPrivateKeyBase64)
	if err != nil {
		t.Fatal(err)
	}

	sk, err := crypto.UnmarshalPrivateKey(identityKey)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec("INSERT INTO config(key,value) VALUES('identityKey', ?)", identityKey)
	if err != nil {
		t.Fatal(err)
	}

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

	sl := new(pb.SignedListing)
	if err := jsonpb.UnmarshalString(string(upMigratedListing), sl); err != nil {
		t.Fatal(err)
	}

	ser, err := proto.Marshal(sl.Listing)
	if err != nil {
		t.Fatal(err)
	}

	valid, err := sk.GetPublic().Verify(ser, sl.Signature)
	if err != nil {
		t.Fatal(err)
	}

	if !valid {
		t.Errorf("Failed to validate up migrated listing signature")
	}

	nd, err := coremock.NewMockNode()
	if err != nil {
		t.Fatal(err)
	}

	listingHash, err := ipfs.GetHashOfFile(nd, testListingPath)
	if err != nil {
		t.Fatal(err)
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

	if listingIndex[0].Hash != listingHash {
		t.Errorf("Incorrect hash set")
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
		t.Fatal("Failed to migrate listing down")
	}

	sl = new(pb.SignedListing)
	if err := jsonpb.UnmarshalString(string(downMigratedListing), sl); err != nil {
		t.Fatal(err)
	}

	ser, err = proto.Marshal(sl.Listing)
	if err != nil {
		t.Fatal(err)
	}

	valid, err = sk.GetPublic().Verify(ser, sl.Signature)
	if err != nil {
		t.Fatal(err)
	}

	if !valid {
		t.Errorf("Failed to validate down migrated listing signature")
	}

	listingHash, err = ipfs.GetHashOfFile(nd, testListingPath)
	if err != nil {
		t.Fatal(err)
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

	if listingIndex2[0].Hash != listingHash {
		t.Errorf("Incorrect hash set")
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
