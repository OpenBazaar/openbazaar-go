package migrations_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func TestMigration032(t *testing.T) {
	// Setup
	appSchema := schema.MustNewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err := appSchema.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer appSchema.DestroySchemaDirectories()

	var (
		dbPassword  = "foobarbaz"
		repoVerPath = appSchema.DataPathJoin("repover")

		encryptDB              = fmt.Sprintf("pragma key = '%s';", dbPassword)
		buildPurchaseSchemaSQL = "create table purchases (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, vendorID text, vendorHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, lastDisputeTimeoutNotifiedAt integer not null default 0, lastDisputeExpiryNotifiedAt integer not null default 0, disputedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');"
		buildSalesSchemaSQL    = "create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, lastDisputeTimeoutNotifiedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');"

		db, err = sql.Open("sqlite3", appSchema.DatabasePath())
	)

	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(strings.Join([]string{
		encryptDB,
		buildPurchaseSchemaSQL,
		buildSalesSchemaSQL,
	}, " "))
	if err != nil {
		t.Fatal(err)
	}

	examples := map[string]migrations.AM01_TransactionRecord_beforeMigration{
		"BTC":  {Txid: "BTC", Index: 0, Address: "76a9143c75da9d9a5e8019c966f0d2f527da2256bdfd1a88ac", Value: 12345678},
		"BCH":  {Txid: "BCH", Index: 0, Address: "76a914f1c075a01882ae0972f95d3a4177c86c852b7d9188ac", Value: 12345678},
		"ZEC":  {Txid: "ZEC", Index: 0, Address: "76a9141bdb7bfbf8ce043c4edaebc88d53dacffb56630788ac", Value: 12345678},
		"TBTC": {Txid: "TBTC", Index: 0, Address: "76a914406ccf980b91476c89dbf129b028b8cbc81a52d688ac", Value: 12345678},
		"TBCH": {Txid: "TBCH", Index: 0, Address: "76a9149dd551e0809fed1e7afdb62e8559563fbe1ece7288ac", Value: 12345678},
		"TZEC": {Txid: "TZEC", Index: 0, Address: "76a91436d138db609a730bf67cf5ed2bd0989c65f627b488ac", Value: 12345678},
	}
	insertPurchaseStatement, err := db.Prepare("insert into purchases(orderID, transactions, paymentCoin) values(?, ?, ?);")
	if err != nil {
		t.Fatal(err)
	}
	insertSalesStatement, err := db.Prepare("insert into sales(orderID, transactions, paymentCoin) values(?, ?, ?);")
	if err != nil {
		t.Fatal(err)
	}
	insertNullTransactionsSaleStatement, err := db.Prepare("insert into sales(orderID, transactions, paymentCoin) values(?, NULL, ?);")
	if err != nil {
		t.Fatal(err)
	}
	insertNullTransactionsPurchaseStatement, err := db.Prepare("insert into purchases(orderID, transactions, paymentCoin) values(?, NULL, ?);")
	if err != nil {
		t.Fatal(err)
	}
	for _, tx := range examples {
		transactions, err := json.Marshal([]migrations.AM01_TransactionRecord_beforeMigration{tx})
		if err != nil {
			t.Fatal(err)
		}
		_, err = insertPurchaseStatement.Exec(tx.Txid, string(transactions), tx.Txid)
		if err != nil {
			t.Fatal(err)
		}
		_, err = insertSalesStatement.Exec(tx.Txid, string(transactions), tx.Txid)
		if err != nil {
			t.Fatal(err)
		}
		_, err = insertNullTransactionsSaleStatement.Exec(fmt.Sprintf("NULLED%s", tx.Txid), tx.Txid)
		if err != nil {
			t.Fatal(err)
		}
		_, err = insertNullTransactionsPurchaseStatement.Exec(fmt.Sprintf("NULLED%s", tx.Txid), tx.Txid)
		if err != nil {
			t.Fatal(err)
		}
	}
	insertPurchaseStatement.Close()
	insertSalesStatement.Close()

	// Create schema version file
	if err = ioutil.WriteFile(repoVerPath, []byte("31"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// Test migration up
	if err := (migrations.AM01{}).Up(appSchema.DataPath(), dbPassword, true); err != nil {
		t.Fatal(err)
	}

	// Validate purchases are converted
	purchaseRows, err := db.Query("select orderID, transactions, paymentCoin from purchases")
	if err != nil {
		t.Fatal(err)
	}
	for purchaseRows.Next() {
		var (
			marshaledTransactions sql.NullString
			orderID, paymentCoin  string
			actualTransactions    []migrations.AM01_TransactionRecord_afterMigration
		)
		if err := purchaseRows.Scan(&orderID, &marshaledTransactions, &paymentCoin); err != nil {
			t.Error(err)
			continue
		}
		if !marshaledTransactions.Valid {
			continue
		}
		if err := json.Unmarshal([]byte(marshaledTransactions.String), &actualTransactions); err != nil {
			t.Error(err)
			continue
		}

		AM01_assertTransaction(t, examples, actualTransactions)
	}
	if err := purchaseRows.Err(); err != nil {
		t.Error(err)
	}
	purchaseRows.Close()

	// Validate sales are converted
	saleRows, err := db.Query("select orderID, transactions, paymentCoin from sales")
	if err != nil {
		t.Fatal(err)
	}
	for saleRows.Next() {
		var (
			orderID, paymentCoin  string
			marshaledTransactions sql.NullString
			actualTransactions    []migrations.AM01_TransactionRecord_afterMigration
		)
		if err := saleRows.Scan(&orderID, &marshaledTransactions, &paymentCoin); err != nil {
			t.Error(err)
			continue
		}
		if !marshaledTransactions.Valid {
			continue
		}
		if err := json.Unmarshal([]byte(marshaledTransactions.String), &actualTransactions); err != nil {
			t.Error(err)
			continue
		}

		AM01_assertTransaction(t, examples, actualTransactions)

	}
	if err := saleRows.Err(); err != nil {
		t.Error(err)
	}
	if err := saleRows.Close(); err != nil {
		t.Error(err)
	}

	assertCorrectRepoVer(t, repoVerPath, "33")

	// Test migration down
	if err := (migrations.AM01{}).Down(appSchema.DataPath(), dbPassword, true); err != nil {
		t.Fatal(err)
	}

	// Validate purchases are reverted
	purchaseRows, err = db.Query("select orderID, transactions, paymentCoin from purchases")
	if err != nil {
		t.Fatal(err)
	}
	for purchaseRows.Next() {
		var (
			marshaledTransactions sql.NullString
			orderID, paymentCoin  string
			actualTransactions    []migrations.AM01_TransactionRecord_beforeMigration
		)
		if err := purchaseRows.Scan(&orderID, &marshaledTransactions, &paymentCoin); err != nil {
			t.Error(err)
			continue
		}
		if !marshaledTransactions.Valid {
			continue
		}
		if err := json.Unmarshal([]byte(marshaledTransactions.String), &actualTransactions); err != nil {
			t.Error(err)
			continue
		}

		for _, actualTx := range actualTransactions {
			if examples[actualTx.Txid].Address != actualTx.Address {
				t.Errorf("Expected address to be converted for example %s, but did not match", actualTx.Txid)
				t.Errorf("Example: %+v", examples[actualTx.Txid])
				t.Errorf("Actual: %+v", actualTx)
			}
		}
	}
	if err := purchaseRows.Err(); err != nil {
		t.Error(err)
	}
	purchaseRows.Close()

	// Validate sales are reverted
	saleRows, err = db.Query("select orderID, transactions, paymentCoin from sales")
	if err != nil {
		t.Fatal(err)
	}
	for saleRows.Next() {
		var (
			marshaledTransactions sql.NullString
			orderID, paymentCoin  string
			actualTransactions    []migrations.AM01_TransactionRecord_beforeMigration
		)
		if err := saleRows.Scan(&orderID, &marshaledTransactions, &paymentCoin); err != nil {
			t.Error(err)
			continue
		}
		if !marshaledTransactions.Valid {
			continue
		}
		if err := json.Unmarshal([]byte(marshaledTransactions.String), &actualTransactions); err != nil {
			t.Error(err)
			continue
		}

		for _, actualTx := range actualTransactions {
			if examples[actualTx.Txid].Address != actualTx.Address {
				t.Errorf("Expected address to be converted for example %s, but did not match", actualTx.Txid)
				t.Errorf("Example: %+v", examples[actualTx.Txid])
				t.Errorf("Actual: %+v", actualTx)
			}
		}

	}
	if err := saleRows.Err(); err != nil {
		t.Error(err)
	}
	if err := saleRows.Close(); err != nil {
		t.Error(err)
	}

	assertCorrectRepoVer(t, repoVerPath, "32")
}

func AM01_assertTransaction(t *testing.T, examples map[string]migrations.AM01_TransactionRecord_beforeMigration, actual []migrations.AM01_TransactionRecord_afterMigration) {
	for _, actualTx := range actual {
		var exampleName = actualTx.Txid
		originalValue := examples[exampleName].Value
		expectedValue := actualTx.Value.Int64()
		if originalValue != expectedValue {
			t.Errorf("Expected value to be converted for example %s, but did not match", exampleName)
			t.Errorf("Example: %+v", examples[actualTx.Txid])
			t.Errorf("Actual: %+v", actualTx)
		}
	}
}
