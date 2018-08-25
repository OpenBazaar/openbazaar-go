package migrations_test

import (
	"database/sql"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestMigration008(t *testing.T) {
	// Setup
	basePath := schema.GenerateTempPath()
	testRepoPath, err := schema.OpenbazaarPathTransform(basePath, true)
	if err != nil {
		t.Fatal(err)
	}
	appSchema, err := schema.NewCustomSchemaManager(schema.SchemaContext{DataPath: testRepoPath, TestModeEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = appSchema.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer appSchema.DestroySchemaDirectories()
	var (
		databasePath = appSchema.DatabasePath()
		schemaPath   = appSchema.DataPathJoin("repover")

		schemaSql         = "pragma key = 'foobarbaz';"
		insertCaseSQL     = "insert into cases (caseID, state, read, timestamp, buyerOpened, claim, buyerPayoutAddress, vendorPayoutAddress, lastNotifiedAt) values (?,?,?,?,?,?,?,?,?);"
		insertPurchaseSQL = "insert into purchases (orderID, contract, state, read, timestamp, total, thumbnail, vendorID, vendorHandle, title, shippingName, shippingAddress, paymentAddr, funded, lastNotifiedAt) values (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);"
		insertSaleSQL     = "insert into sales (orderID, contract, state, read, timestamp, total, thumbnail, buyerID, buyerHandle, title, shippingName, shippingAddress, paymentAddr, funded, lastNotifiedAt) values (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);"
		selectCaseSQL     = "select caseID, lastDisputeExpiryNotifiedAt from cases where caseID = ?;"
		selectPurchaseSQL = "select orderID, state, lastDisputeTimeoutNotifiedAt, lastDisputeExpiryNotifiedAt, disputedAt from purchases where orderID in (?,?);"
		selectSaleSQL     = "select orderID, lastDisputeTimeoutNotifiedAt from sales where orderID = ?;"

		caseID                   = "caseID"
		purchaseID               = "purchaseID"
		disputedPurchaseID       = "disputedPurchaseID"
		disputedPurchaseContract = factory.NewDisputedContract()
		saleID                   = "saleID"
		executedAt               = time.Now()
	)

	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	disputedPurchaseContractData, err := m.MarshalToString(disputedPurchaseContract)
	if err != nil {
		t.Fatal(err)
	}

	// Setup datastore
	dbSetupSql := strings.Join([]string{
		schemaSql,
		migrations.Migration008_casesCreateSQL,
		migrations.Migration008_purchasesCreateSQL,
		migrations.Migration008_salesCreateSQL,
		insertCaseSQL,
		insertPurchaseSQL, // non disputed purchase insert
		insertPurchaseSQL, // disputed purchase insert
		insertSaleSQL,
	}, " ")
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(dbSetupSql,
		caseID, // dispute case id
		migrations.Migration008_OrderState_PENDING, // order state int
		0, // dispute read bool
		int(executedAt.Unix()), // dispute timestamp
		0,           // dispute buyerOpened bool
		"claimtext", // dispute claim text
		"",          // dispute buyerPayoutAddres
		"",          // dispute vendorPayoutAddres
		0,           // lastNotifiedAt unix timestamp

		purchaseID, // purchase order id
		"",         // purchase contract blob
		migrations.Migration008_OrderState_AWAITING_PAYMENT, // order state int
		0, // purchase read bool
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
		0,                      // lastNotifiedAt unix timestamp

		disputedPurchaseID,                          // purchase order id
		disputedPurchaseContractData,                // purchase contract blob
		migrations.Migration008_OrderState_DISPUTED, // order state int
		0, // purchase read bool
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
		0,                      // lastNotifiedAt unix timestamp

		saleID, // sale order id
		"",     // sale contract blob
		migrations.Migration008_OrderState_AWAITING_PAYMENT, // order state int
		0, // purchase read bool
		0, // sale read bool
		int(executedAt.Unix()), // sale timestamp
		int(0),                 // sale total int
		"thumbnailHash",        // sale thumbnail text
		"QmBuyerPeerID",        // sale buyerID text
		"buyer handle",         // sale buyer handle text
		"An Item Title",        // sale item title
		"shipping name",        // sale shippingName text
		"shippingAddress",      // sale shippingAddress text
		"paymentAddress",       // sale paymentAddr text
		0,                      // sale funded bool
		0,                      // sale needsSync bool
		0,                      // lastNotifiedAt unix timestamp
	)
	if err != nil {
		t.Fatal(err)
	}

	// Create schema version file
	if err = ioutil.WriteFile(schemaPath, []byte("8"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// Execute Migration Up
	migration := migrations.Migration008{}
	if err := migration.Up(testRepoPath, "foobarbaz", true); err != nil {
		t.Fatal(err)
	}

	// Assert repo version updated
	if err = appSchema.VerifySchemaVersion("9"); err != nil {
		t.Fatal(err)
	}

	caseRows, err := db.Query(selectCaseSQL, caseID)
	if err != nil {
		t.Fatal(err)
	}

	// Assert lastDisputeExpiryNotifiedAt column on cases
	var (
		lastDisputeExpiryNotifiedAtColumnOnCasesExists bool
	)
	columns, err := caseRows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range columns {
		if c.Name() == "lastDisputeExpiryNotifiedAt" {
			lastDisputeExpiryNotifiedAtColumnOnCasesExists = true
		}
	}
	if lastDisputeExpiryNotifiedAtColumnOnCasesExists == false {
		t.Error("Expected lastDisputeExpiryNotifiedAt column to exist on cases")
	}

	// Assert lastDisputeExpiryNotifiedAt column on cases is set to (approx) the same
	// time the migration was executed
	var actualCase struct {
		CaseId                      string
		LastDisputeExpiryNotifiedAt int64
	}

	for caseRows.Next() {
		err := caseRows.Scan(&actualCase.CaseId, &actualCase.LastDisputeExpiryNotifiedAt)
		if err != nil {
			t.Error(err)
		}
		if actualCase.CaseId != caseID {
			t.Error("Unexpected case ID returned")
		}
		timeSinceMigration := time.Now().Sub(time.Unix(actualCase.LastDisputeExpiryNotifiedAt, 0))
		if timeSinceMigration > (time.Duration(2) * time.Second) {
			t.Errorf("Expected lastDisputeExpiryNotifiedAt on case to be set within the last 2 seconds, but was set %s ago", timeSinceMigration)
		}
	}
	caseRows.Close()

	// Assert lastDisputeTimeoutNotifiedColumn on purchases
	purchaseRows, err := db.Query(selectPurchaseSQL, purchaseID, disputedPurchaseID)
	if err != nil {
		t.Fatal(err)
	}
	var (
		disputedAtOnPurchaseExists                        bool
		lastDisputeTimeoutNotifiedColumnOnPurchasesExists bool
		lastDisputeExpiryNotifiedColumnOnPurchasesExists  bool
	)
	columns, err = purchaseRows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range columns {
		if c.Name() == "lastDisputeTimeoutNotifiedAt" {
			lastDisputeTimeoutNotifiedColumnOnPurchasesExists = true
		}
		if c.Name() == "lastDisputeExpiryNotifiedAt" {
			lastDisputeExpiryNotifiedColumnOnPurchasesExists = true
		}
		if c.Name() == "disputedAt" {
			disputedAtOnPurchaseExists = true
		}
	}
	if lastDisputeTimeoutNotifiedColumnOnPurchasesExists == false {
		t.Error("Expected lastDisputeTimeoutNotifiedAt column to exist on purchases")
	}
	if lastDisputeExpiryNotifiedColumnOnPurchasesExists == false {
		t.Error("Expected lastDisputeExpiryNotifiedAt column to exist on purchases")
	}
	if disputedAtOnPurchaseExists == false {
		t.Error("Expected disputedAt column to exist on purchases")
	}

	// Assert lastDisputeTimeoutNotifiedAt column on purchases is set to (approx) the
	// same time the migration was executed
	var actualPurchase struct {
		DisputedAt                   int64
		OrderId                      string
		OrderState                   int
		LastDisputeTimeoutNotifiedAt int64
		LastDisputeExpiryNotifiedAt  int64
	}

	for purchaseRows.Next() {
		err := purchaseRows.Scan(&actualPurchase.OrderId, &actualPurchase.OrderState, &actualPurchase.LastDisputeTimeoutNotifiedAt, &actualPurchase.LastDisputeExpiryNotifiedAt, &actualPurchase.DisputedAt)
		if err != nil {
			t.Error(err)
		}
		timeSinceMigration := time.Now().Sub(time.Unix(actualPurchase.LastDisputeTimeoutNotifiedAt, 0))
		if timeSinceMigration > (time.Duration(2) * time.Second) {
			t.Errorf("Expected lastDisputeTimeoutNotifiedAt on purchase to be set within the last 2 seconds, but was set %s ago", timeSinceMigration)
		}
		if actualPurchase.OrderState == migrations.Migration008_OrderState_DISPUTED {
			timeSinceMigration := time.Now().Sub(time.Unix(actualPurchase.LastDisputeExpiryNotifiedAt, 0))
			if timeSinceMigration > (time.Duration(2) * time.Second) {
				t.Errorf("Expected lastDisputeExpiryNotifiedAt on purchase to be set within the last 2 seconds, but was set %s ago", timeSinceMigration)
			}
			if actualPurchase.DisputedAt != disputedPurchaseContract.Dispute.Timestamp.Seconds {
				t.Errorf("Expected disputedAt on purchase to be set equivalent to when the dispute was opened")
			}
		} else {
			if actualPurchase.LastDisputeExpiryNotifiedAt != 0 {
				t.Errorf("Expected lastDisputeExpiryNotifiedAt on purchase to be 0 if not a disputed purchase")
			}
			if actualPurchase.DisputedAt != 0 {
				t.Errorf("Expected disputedAt on purchase to be 0 if not a disputed purchase")
			}
		}
	}
	purchaseRows.Close()

	// Assert lastNotifiedColumn on sales
	saleRows, err := db.Query(selectSaleSQL, saleID)
	if err != nil {
		t.Fatal(err)
	}
	var (
		lastDisputeTimeoutNotifierColumnOnSalesExists bool
	)
	columns, err = saleRows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range columns {
		if c.Name() == "lastDisputeTimeoutNotifiedAt" {
			lastDisputeTimeoutNotifierColumnOnSalesExists = true
		}
	}
	if lastDisputeTimeoutNotifierColumnOnSalesExists == false {
		t.Error("Expected lastDisputeTimeoutNotifiedAt column on sales to exist on sales and not be nullable")
	}

	// Assert lastDisputeTimeoutNotifiedAt column on sales is set to (approx) the
	// same time the migration was executed
	var actualSale struct {
		OrderId                      string
		LastDisputeTimeoutNotifiedAt int64
	}

	for saleRows.Next() {
		err := saleRows.Scan(&actualSale.OrderId, &actualSale.LastDisputeTimeoutNotifiedAt)
		if err != nil {
			t.Error(err)
		}
		if actualSale.OrderId != saleID {
			t.Error("Unexpected orderID returned")
		}
		timeSinceMigration := time.Now().Sub(time.Unix(actualSale.LastDisputeTimeoutNotifiedAt, 0))
		if timeSinceMigration > (time.Duration(2) * time.Second) {
			t.Errorf("Expected lastDisputeTimeoutNotifiedAt on sale to be set within the last 2 seconds, but was set %s ago", timeSinceMigration)
		}
	}
	saleRows.Close()
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

	// Assert lastDisputeTimeoutNotifiedAt column on cases migrated down
	_, err = db.Query("update cases set lastDisputeTimeoutNotifiedAt = ? where caseID = ?;", 0, caseID)
	if err == nil {
		t.Error("Expected lastDisputeTimeoutNotifiedAt update on cases to fail")
	}
	if err != nil && !strings.Contains(err.Error(), "no such column: lastDisputeTimeoutNotifiedAt") {
		t.Error("Expected error to be 'no such column', was:", err.Error())
	}

	// Assert lastDisputeTimeoutNotifiedAt column on purchases migrated down
	_, err = db.Query("update purchases set lastDisputeTimeoutNotifiedAt = ? where orderID = ?;", 0, purchaseID)
	if err == nil {
		t.Error("Expected lastDisputeTimeoutNotifiedAt update on purchases to fail")
	}
	if err != nil && !strings.Contains(err.Error(), "no such column: lastDisputeTimeoutNotifiedAt") {
		t.Error("Expected error to be 'no such column', was:", err.Error())
	}

	// Assert lastDisputeExpiryNotifiedAt column on purchases migrated down
	_, err = db.Query("update purchases set lastDisputeExpiryNotifiedAt = ? where orderID = ?;", 0, purchaseID)
	if err == nil {
		t.Error("Expected lastDisputeExpiryNotifiedAt update on purchases to fail")
	}
	if err != nil && !strings.Contains(err.Error(), "no such column: lastDisputeExpiryNotifiedAt") {
		t.Error("Expected error to be 'no such column', was:", err.Error())
	}

	// Assert disputedAt column on purchases migrated down
	_, err = db.Query("update purchases set disputedAt = ? where orderID = ?;", 0, purchaseID)
	if err == nil {
		t.Error("Expected disputedAt update on purchases to fail")
	}
	if err != nil && !strings.Contains(err.Error(), "no such column: disputedAt") {
		t.Error("Expected error to be 'no such column', was:", err.Error())
	}

	// Assert lastDisputeExpiryNotifiedAt column on purchases migrated down
	_, err = db.Query("update purchases set lastDisputeExpiryNotifiedAt = ? where orderID = ?;", 0, purchaseID)
	if err == nil {
		t.Error("Expected lastDisputeExpiryNotifiedAt update on purchases to fail")
	}
	if err != nil && !strings.Contains(err.Error(), "no such column: lastDisputeExpiryNotifiedAt") {
		t.Error("Expected error to be 'no such column', was:", err.Error())
	}

	// Assert lastDisputeTimeoutNotifiedAt column on sales migrated down
	_, err = db.Query("update sales set lastDisputeTimeoutNotifiedAt = ? where orderID = ?;", 0, saleID)
	if err == nil {
		t.Error("Expected lastDisputeTimeoutNotifiedAt update on sales to fail")
	}
	if err != nil && !strings.Contains(err.Error(), "no such column: lastDisputeTimeoutNotifiedAt") {
		t.Error("Expected error to be 'no such column', was:", err.Error())
	}

	// Assert repover was reverted
	if err = appSchema.VerifySchemaVersion("8"); err != nil {
		t.Error(err)
	}
	db.Close()
}
