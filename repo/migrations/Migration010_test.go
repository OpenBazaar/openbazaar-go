package migrations_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

const testmigration010Password = "letmein"

var (
	caseIdsToCoinSet = map[string]struct {
		paymentCoion string
		coinType     string
	}{
		"1": {"TBC", "TETH"},
		"2": {"TBC", "TETH"},
	}

	testmigration010SchemaStmts = []string{
		"DROP TABLE IF EXISTS cases;",
		"DROP TABLE IF EXISTS sales;",
		"DROP TABLE IF EXISTS purchases;",
		"create table cases (caseID text primary key not null, buyerContract blob, vendorContract blob, buyerValidationErrors blob, vendorValidationErrors blob, buyerPayoutAddress text, vendorPayoutAddress text, buyerOutpoints blob, vendorOutpoints blob, state integer, read integer, timestamp integer, buyerOpened integer, claim text, disputeResolution blob, lastDisputeExpiryNotifiedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');",
		"create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, needsSync integer, lastDisputeTimeoutNotifiedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');",
		"create table purchases (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, vendorID text, vendorHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, lastDisputeTimeoutNotifiedAt integer not null default 0, lastDisputeExpiryNotifiedAt integer not null default 0, disputedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');",
	}

	testmigration010FixtureStmts = []string{
		"INSERT INTO cases(caseID, buyerContract, paymentCoin, coinType) VALUES('1', ?, 'TBTC', 'TETH');",
		"INSERT INTO sales(orderID, contract, paymentCoin, coinType) VALUES('1', ?, 'TBTC', 'TETH');",
		"INSERT INTO purchases(orderID, contract, paymentCoin, coinType) VALUES('1', ?, 'TBTC', 'TETH');",

		"INSERT INTO cases(caseID, buyerContract) VALUES('2', ?);",
		"INSERT INTO sales(orderID, contract) VALUES('2', ?);",
		"INSERT INTO purchases(orderID, contract) VALUES('2', ?);",
	}
)

func testmigration010SetupFixtures(t *testing.T, db *sql.DB) {
	for _, stmt := range testmigration010SchemaStmts {
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
}

func TestMigration010(t *testing.T) {
	os.Mkdir("./datastore", os.ModePerm)
	defer os.RemoveAll("./datastore")

	db, err := migrations.OpenDB(".", testmigration010Password, true)
	if err != nil {
		t.Fatal(err)
	}
	testmigration010SetupFixtures(t, db)

	// Test migration up
	var m migrations.Migration010
	err = m.Up(".", testmigration010Password, true)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll("./repover")
	assertCorrectRepoVer(t, "./repover", "11")

	for _, table := range []string{"cases", "sales", "purchases"} {
		results := db.QueryRow("SELECT coinType, paymentCoin FROM " + table + " LIMIT 1;")
		if err != nil {
			t.Fatal(err)
		}

		var coinType, paymentCoin string
		err = results.Scan(&coinType, &paymentCoin)
		if err != nil {
			t.Fatal("table:", table, "err:", err)
		}

		if coinType != "TETH" {
			t.Fatal("Incorrect coinType for table", table+":", coinType)
		}
		if paymentCoin != "TBTC" {
			t.Fatal("Incorrect paymentCoin for table", table+":", paymentCoin)
		}
	}

	// Test migration down
	err = m.Down(".", testmigration010Password, true)
	if err != nil {
		t.Fatal(err)
	}
	assertCorrectRepoVer(t, "./repover", "10")
}
