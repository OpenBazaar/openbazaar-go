package migrations

import (
	"database/sql"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/schema"
)

func TestMigration007(t *testing.T) {

	// Setup
	basePath := schema.GenerateTempPath()
	testRepoPath, err := schema.OpenbazaarPathTransform(basePath, true)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := schema.NewCustomSchemaManager(schema.SchemaContext{DataPath: testRepoPath, TestModeEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = paths.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer paths.DestroySchemaDirectories()
	var (
		databasePath = paths.DatastorePath()
		schemaPath   = paths.DataPathJoin("repover")

		schemaSql         = "pragma key = 'foobarbaz';"
		insertCaseSQL     = "insert into cases (caseID, state, read, timestamp, buyerOpened, claim, buyerPayoutAddress, vendorPayoutAddress) values (?,?,?,?,?,?,?,?);"
		insertPurchaseSQL = "insert into purchases (orderID, contract, state, read, timestamp, total, thumbnail, vendorID, vendorHandle, title, shippingName, shippingAddress, paymentAddr, funded) values (?,?,?,?,?,?,?,?,?,?,?,?,?,?);"
		selectCaseSQL     = "select caseID, lastNotifiedAt from cases where caseID = ?;"
		selectPurchaseSQL = "select orderID, lastNotifiedAt from purchases where orderID = ?;"

		caseID     = "caseID"
		purchaseID = "purchaseID"
		executedAt = time.Now()
	)

	// Setup datastore
	dbSetupSql := strings.Join([]string{
		schemaSql,
		migration007_casesCreateSQL,
		migration007_purchasesCreateSQL,
		insertCaseSQL,
		insertPurchaseSQL,
	}, " ")
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(dbSetupSql,
		caseID, // dispute case id
		int(0), // dispute OrderState
		0,      // dispute read bool
		int(executedAt.Unix()), // dispute timestamp
		0,           // dispute buyerOpened bool
		"claimtext", // dispute claim text
		"",          // dispute buyerPayoutAddres
		"",          // dispute vendorPayoutAddres

		purchaseID, // purchase order id
		"",         // purchase contract blob
		1,          // purchase state
		0,          // purchase read bool
		int(executedAt.Unix()), // purchase timestamp
		int(0),                 // purchase total int
		"thumbnailHash",        // purchase thumbnail text
		"QmVendorPeerID",       // purchase vendorID text
		"vendor handle",        // purchase vendor handle text
		"An Item Title",        // purchase item title
		"shipping name",        // purchase shippingName text
		"shippingAddress",      // purchase shippingAddress text
		"paymentAddress",       // purchase paymentAddr text
		0,                      // purchase funded bool
	)
	if err != nil {
		t.Fatal(err)
	}

	// Create schema version file
	if err = ioutil.WriteFile(schemaPath, []byte("7"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// Execute Migration Up
	migration := Migration007{}
	if err := migration.Up(testRepoPath, "foobarbaz", true); err != nil {
		t.Fatal(err)
	}

	// Assert repo version updated
	if err = paths.VerifySchemaVersion("8"); err != nil {
		t.Fatal(err)
	}

	caseRows, err := db.Query(selectCaseSQL, caseID)
	if err != nil {
		t.Fatal(err)
	}

	// Assert lastNotifiedAt column on cases
	var lastNotifiedAtColumnOnCasesExists bool
	columns, err := caseRows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range columns {
		if c.Name() == "lastNotifiedAt" {
			lastNotifiedAtColumnOnCasesExists = true
		}
	}
	if lastNotifiedAtColumnOnCasesExists == false {
		t.Error("Expected lastNotifiedAt column to exist on cases and not be nullable")
	}

	// Assert lastNotifiedAt column on cases is set to (approx) the same
	// time the migration was executed
	var actualCase struct {
		CaseId         string
		LastNotifiedAt int64
	}

	for caseRows.Next() {
		err := caseRows.Scan(&actualCase.CaseId, &actualCase.LastNotifiedAt)
		if err != nil {
			t.Error(err)
		}
		if actualCase.CaseId != caseID {
			t.Error("Unexpected case ID returned")
		}
		timeSinceMigration := time.Now().Sub(time.Unix(actualCase.LastNotifiedAt, 0))
		if timeSinceMigration > (time.Duration(2) * time.Second) {
			t.Errorf("Expected lastNotifiedAt on case to be set within the last 2 seconds, but was set %s ago", timeSinceMigration)
		}
	}
	caseRows.Close()

	// Assert lastNotifiedColumn on purchases
	purchaseRows, err := db.Query(selectPurchaseSQL, purchaseID)
	if err != nil {
		t.Fatal(err)
	}
	var lastNotifiedColumnOnPurchasesExists bool
	columns, err = purchaseRows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range columns {
		if c.Name() == "lastNotifiedAt" {
			lastNotifiedColumnOnPurchasesExists = true
		}
	}
	if lastNotifiedColumnOnPurchasesExists == false {
		t.Error("Expected lastNotifiedAt column on purchases to exist on purchases and not be nullable")
	}

	// Assert lastNotifiedAt column on purchases is set to (approx) the
	// same time the migration was executed
	var actualPurchase struct {
		OrderId        string
		LastNotifiedAt int64
	}

	for purchaseRows.Next() {
		err := purchaseRows.Scan(&actualPurchase.OrderId, &actualPurchase.LastNotifiedAt)
		if err != nil {
			t.Error(err)
		}
		if actualPurchase.OrderId != purchaseID {
			t.Error("Unexpected orderID returned")
		}
		timeSinceMigration := time.Now().Sub(time.Unix(actualPurchase.LastNotifiedAt, 0))
		if timeSinceMigration > (time.Duration(2) * time.Second) {
			t.Errorf("Expected lastNotifiedAt on purchase to be set within the last 2 seconds, but was set %s ago", timeSinceMigration)
		}
	}
	purchaseRows.Close()
	db.Close()

	// Execute Migration Down
	if err = migration.Down(testRepoPath, "foobarbaz", true); err != nil {
		t.Fatal(err)
	}

	db, err = sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(schemaSql); err != nil {
		t.Fatal(err)
	}

	// Assert lastNotifiedAt column on cases
	_, err = db.Query("update cases set lastNotifiedAt = ? where caseID = ?;", 0, caseID)
	if err == nil {
		t.Error("Expected lastNotifiedAt update on cases to fail")
	}
	if err != nil && !strings.Contains(err.Error(), "no such column: lastNotifiedAt") {
		t.Error("Expected error to be 'no such column', was:", err.Error())
	}

	// Assert lastNotifiedAt column on purchases
	_, err = db.Query("update purchases set lastNotifiedAt = ? where orderID = ?;", 0, purchaseID)
	if err == nil {
		t.Error("Expected lastNotifiedAt update on purchases to fail")
	}
	if err != nil && !strings.Contains(err.Error(), "no such column: lastNotifiedAt") {
		t.Error("Expected error to be 'no such column', was:", err.Error())
	}

	// Assert repover was reverted
	if err = paths.VerifySchemaVersion("7"); err != nil {
		t.Error(err)
	}
	db.Close()
}
