package migrations_test

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

const testMigration009Password = "letmein"

var (
	testMigration009SchemaStmts = []string{
		"DROP TABLE IF EXISTS cases;",
		"DROP TABLE IF EXISTS sales;",
		"DROP TABLE IF EXISTS purchases;",
		migrations.Migration009CreatePreviousCasesTable,
		migrations.Migration009CreatePreviousSalesTable,
		migrations.Migration009CreatePreviousPurchasesTable,
	}

	testMigration009FixtureStmts = []string{
		"INSERT INTO cases(caseID, buyerContract) VALUES('1', ?);",
		"INSERT INTO sales(orderID, contract) VALUES('1', ?);",
		"INSERT INTO purchases(orderID, contract) VALUES('1', ?);",
	}
)

func testMigration009SetupFixtures(t *testing.T, db *sql.DB) func() {
	for _, stmt := range testMigration009SchemaStmts {
		_, err := db.Exec(stmt)
		if err != nil {
			t.Fatal(err)
		}
	}

	marshaler := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	contract := factory.NewDisputedContract()
	contract.VendorListings[0] = factory.NewCryptoListing("TETH")
	marshaledContract, err := marshaler.MarshalToString(contract)
	if err != nil {
		t.Fatal(err)
	}

	for _, stmt := range testMigration009FixtureStmts {
		_, err = db.Exec(stmt, marshaledContract)
		if err != nil {
			t.Fatal(err)
		}
	}

	os.Mkdir(path.Join(".", "root"), os.ModePerm)
	os.Mkdir(path.Join(".", "root", "listings"), os.ModePerm)

	var (
		listingsIndexPath = path.Join(".", "root", "listings.json")
		listing1Path      = path.Join(".", "root", "listings", "slug-1.json")
		listing2Path      = path.Join(".", "root", "listings", "slug-2.json")
		listing1Fixture   = migrations.Migration009_listing{
			Listing: migrations.Migration009_listing_listing{
				Metadata: migrations.Migration009_listing_listing_metadata{
					AcceptedCurrencies: []string{"TBTC"},
				},
			},
		}
		listing2Fixture = migrations.Migration009_listing{
			Listing: migrations.Migration009_listing_listing{
				Metadata: migrations.Migration009_listing_listing_metadata{
					AcceptedCurrencies: []string{"TBCH"},
				},
			},
		}
	)

	if err = ioutil.WriteFile(listingsIndexPath, []byte(testMigration009ExpectedListingIndexBeforeMigration), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	listingJSON, err := json.Marshal(listing1Fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err = ioutil.WriteFile(listing1Path, listingJSON, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	listingJSON, err = json.Marshal(listing2Fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err = ioutil.WriteFile(listing2Path, listingJSON, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	return func() {
		os.RemoveAll("./root")
	}
}

func TestMigration009(t *testing.T) {
	os.Mkdir("./datastore", os.ModePerm)
	defer os.RemoveAll("./datastore")

	db, err := migrations.OpenDB(".", testMigration009Password, true)
	if err != nil {
		t.Fatal(err)
	}
	defer testMigration009SetupFixtures(t, db)()

	listingsIndexPath := path.Join(".", "root", "listings.json")

	// Test migration up
	var m migrations.Migration009
	err = m.Up(".", testMigration009Password, true)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll("./repover")
	assertCorrectRepoVer(t, "10")
	assertCorrectFileContents(t, listingsIndexPath, testMigration009ExpectedListingIndexAfterMigration)

	for _, table := range []string{"cases", "sales", "purchases"} {
		results := db.QueryRow("SELECT coinType, paymentCoin FROM " + table + " LIMIT 1;")
		if err != nil {
			t.Fatal(err)
		}
		var coinType, paymentCoin string
		err = results.Scan(&coinType, &paymentCoin)
		if err != nil {
			t.Fatal(err)
		}
		if coinType != "TETH" {
			t.Fatal("Incorrect coinType for table", table+":", coinType)
		}
		if paymentCoin != "TBTC" {
			t.Fatal("Incorrect paymentCoin for table", table+":", paymentCoin)
		}
	}

	// Test migration down
	err = m.Down(".", testMigration009Password, true)
	if err != nil {
		t.Fatal(err)
	}
	assertCorrectRepoVer(t, "9")
	assertCorrectFileContents(t, listingsIndexPath, testMigration009ExpectedListingIndexBeforeMigration)

	for _, table := range []string{"cases", "sales", "purchases"} {
		for _, column := range []string{"coinType", "paymentCoin"} {
			errStr := db.
				QueryRow("SELECT " + column + " FROM " + table + ";").
				Scan().
				Error()
			expectedErr := "no such column: " + column
			if errStr != expectedErr {
				t.Fatal("expected '" + expectedErr + "'")
			}
		}
	}
}

var (
	testMigration009ExpectedListingIndexAfterMigration = `[
    {
        "hash": "Listing1",
        "slug": "slug-1",
        "title": "Listing 1",
        "categories": null,
        "nsfw": false,
        "coinType": "",
        "contractType": "",
        "description": "",
        "thumbnail": {
            "tiny": "",
            "small": "",
            "medium": ""
        },
        "price": {
            "currencyCode": "",
            "amount": 0
        },
        "shipsTo": null,
        "freeShipping": null,
        "language": "",
        "averageRating": 0,
        "ratingCount": 0,
        "moderators": null,
        "acceptedCurrencies": [
            "TBTC"
        ]
    },
    {
        "hash": "Listing2",
        "slug": "slug-2",
        "title": "Listing 2",
        "categories": null,
        "nsfw": false,
        "coinType": "",
        "contractType": "",
        "description": "",
        "thumbnail": {
            "tiny": "",
            "small": "",
            "medium": ""
        },
        "price": {
            "currencyCode": "",
            "amount": 0
        },
        "shipsTo": null,
        "freeShipping": null,
        "language": "",
        "averageRating": 0,
        "ratingCount": 0,
        "moderators": null,
        "acceptedCurrencies": [
            "TBTC"
        ]
    }
]`

	testMigration009ExpectedListingIndexBeforeMigration = `[
    {
        "hash": "Listing1",
        "slug": "slug-1",
        "title": "Listing 1",
        "categories": null,
        "nsfw": false,
        "coinType": "",
        "contractType": "",
        "description": "",
        "thumbnail": {
            "tiny": "",
            "small": "",
            "medium": ""
        },
        "price": {
            "currencyCode": "",
            "amount": 0
        },
        "shipsTo": null,
        "freeShipping": null,
        "language": "",
        "averageRating": 0,
        "ratingCount": 0,
        "moderators": null
    },
    {
        "hash": "Listing2",
        "slug": "slug-2",
        "title": "Listing 2",
        "categories": null,
        "nsfw": false,
        "coinType": "",
        "contractType": "",
        "description": "",
        "thumbnail": {
            "tiny": "",
            "small": "",
            "medium": ""
        },
        "price": {
            "currencyCode": "",
            "amount": 0
        },
        "shipsTo": null,
        "freeShipping": null,
        "language": "",
        "averageRating": 0,
        "ratingCount": 0,
        "moderators": null
    }
]`
)
