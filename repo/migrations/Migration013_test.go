package migrations_test

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func TestMigration013Helpers(t *testing.T) {
	examples := map[string]string{
		"BTC":  "16Wgj9XcffBHc9JsKV7kbxnCBjFp16TMv1",
		"BCH":  "qrcuqadqrzp2uztjl9wn5sthepkg22majyxw4gmv6p",
		"ZEC":  "t1LQuBYziGBUAUeZezyatzSmJJtBUeGnqe9",
		"TBTC": "mmPbteRPfahVL2YK4SNpBDd6XXV3xysTNZ",
		"TBCH": "qzwa250qsz0768n6lkmzap2e2clmu8kwwgsx64qw20",
		"TZEC": "tmEiCTY5Ukmx9X2A6mu6orNrE9oQ3PiLAM3",
	}

	for coin, addr := range examples {
		var testmodeEnabled bool
		if strings.Index(coin, "T") == 0 {
			testmodeEnabled = true
		}

		// Convert source addr into script
		expectedScript, err := migrations.Migration013_AddressToScript(coin, addr, testmodeEnabled)
		if err != nil {
			t.Error(coin, err)
		}

		// Convert artifact script back to addr for symmetry verification
		actualAddr, err := migrations.Migration013_ScriptToAddress(coin, expectedScript, testmodeEnabled)
		if err != nil {
			t.Error(coin, err)
		}
		if actualAddr != addr {
			t.Errorf("Expected script to address convertion to be symmetric, but was not in the %s example", coin)
			t.Error("Actual:  ", actualAddr)
			t.Error("Expected:", addr)
		}

		// Convert symmetric addr back to script to verify reverse operation
		actualScript, err := migrations.Migration013_AddressToScript(coin, actualAddr, testmodeEnabled)
		if err != nil {
			t.Error(coin, err)
		}
		if !bytes.Equal(actualScript, expectedScript) {
			t.Errorf("Expected address to script convertion to be symmetric, but was not in the %s example", coin)
			t.Error("Actual:  ", actualScript)
			t.Error("Expected:", expectedScript)
		}
	}
}

func TestMigration013(t *testing.T) {
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
		buildSalesSchemaSQL    = "create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, needsSync integer, lastDisputeTimeoutNotifiedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');"

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

	examples := map[string]migrations.Migration013_TransactionRecord_beforeMigration{
		"BTC":  {Txid: "BTC", Index: 0, ScriptPubKey: "76a9143c75da9d9a5e8019c966f0d2f527da2256bdfd1a88ac"},
		"BCH":  {Txid: "BCH", Index: 0, ScriptPubKey: "76a914f1c075a01882ae0972f95d3a4177c86c852b7d9188ac"},
		"ZEC":  {Txid: "ZEC", Index: 0, ScriptPubKey: "76a9141bdb7bfbf8ce043c4edaebc88d53dacffb56630788ac"},
		"TBTC": {Txid: "TBTC", Index: 0, ScriptPubKey: "76a914406ccf980b91476c89dbf129b028b8cbc81a52d688ac"},
		"TBCH": {Txid: "TBCH", Index: 0, ScriptPubKey: "76a9149dd551e0809fed1e7afdb62e8559563fbe1ece7288ac"},
		"TZEC": {Txid: "TZEC", Index: 0, ScriptPubKey: "76a91436d138db609a730bf67cf5ed2bd0989c65f627b488ac"},
	}
	insertPurchaseStatement, err := db.Prepare("insert into purchases(orderID, transactions, paymentCoin) values(?, ?, ?);")
	if err != nil {
		t.Fatal(err)
	}
	insertSalesStatement, err := db.Prepare("insert into sales(orderID, transactions, paymentCoin) values(?, ?, ?);")
	if err != nil {
		t.Fatal(err)
	}
	for _, tx := range examples {
		transactions, err := json.Marshal([]migrations.Migration013_TransactionRecord_beforeMigration{tx})
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
	}
	insertPurchaseStatement.Close()
	insertSalesStatement.Close()

	// Create schema version file
	if err = ioutil.WriteFile(repoVerPath, []byte("13"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// Test migration up
	if err := (migrations.Migration013{}).Up(appSchema.DataPath(), dbPassword, true); err != nil {
		t.Fatal(err)
	}

	// Validate purchases are converted
	purchaseRows, err := db.Query("select orderID, transactions, paymentCoin from purchases")
	if err != nil {
		t.Fatal(err)
	}
	for purchaseRows.Next() {
		var (
			orderID, marshaledTransactions, paymentCoin string
			actualTransactions                          []migrations.Migration013_TransactionRecord_afterMigration
		)
		if err := purchaseRows.Scan(&orderID, &marshaledTransactions, &paymentCoin); err != nil {
			t.Error(err)
			continue
		}
		if err := json.Unmarshal([]byte(marshaledTransactions), &actualTransactions); err != nil {
			t.Error(err)
			continue
		}

		migration013_assertTransaction(t, examples, actualTransactions)
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
			orderID, marshaledTransactions, paymentCoin string
			actualTransactions                          []migrations.Migration013_TransactionRecord_afterMigration
		)
		if err := saleRows.Scan(&orderID, &marshaledTransactions, &paymentCoin); err != nil {
			t.Error(err)
			continue
		}
		if err := json.Unmarshal([]byte(marshaledTransactions), &actualTransactions); err != nil {
			t.Error(err)
			continue
		}

		migration013_assertTransaction(t, examples, actualTransactions)

	}
	if err := saleRows.Err(); err != nil {
		t.Error(err)
	}
	if err := saleRows.Close(); err != nil {
		t.Error(err)
	}

	assertCorrectRepoVer(t, repoVerPath, "14")

	// Test migration down
	if err := (migrations.Migration013{}).Down(appSchema.DataPath(), dbPassword, true); err != nil {
		t.Fatal(err)
	}

	// Validate purchases are reverted
	purchaseRows, err = db.Query("select orderID, transactions, paymentCoin from purchases")
	if err != nil {
		t.Fatal(err)
	}
	for purchaseRows.Next() {
		var (
			orderID, marshaledTransactions, paymentCoin string
			actualTransactions                          []migrations.Migration013_TransactionRecord_beforeMigration
		)
		if err := purchaseRows.Scan(&orderID, &marshaledTransactions, &paymentCoin); err != nil {
			t.Error(err)
			continue
		}
		if err := json.Unmarshal([]byte(marshaledTransactions), &actualTransactions); err != nil {
			t.Error(err)
			continue
		}

		for _, actualTx := range actualTransactions {
			if examples[actualTx.Txid].ScriptPubKey != actualTx.ScriptPubKey {
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
			orderID, marshaledTransactions, paymentCoin string
			actualTransactions                          []migrations.Migration013_TransactionRecord_beforeMigration
		)
		if err := saleRows.Scan(&orderID, &marshaledTransactions, &paymentCoin); err != nil {
			t.Error(err)
			continue
		}
		if err := json.Unmarshal([]byte(marshaledTransactions), &actualTransactions); err != nil {
			t.Error(err)
			continue
		}

		for _, actualTx := range actualTransactions {
			if examples[actualTx.Txid].ScriptPubKey != actualTx.ScriptPubKey {
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

	assertCorrectRepoVer(t, repoVerPath, "13")
}

func migration013_assertTransaction(t *testing.T, examples map[string]migrations.Migration013_TransactionRecord_beforeMigration, actual []migrations.Migration013_TransactionRecord_afterMigration) {
	for _, actualTx := range actual {
		var exampleName = actualTx.Txid
		decodedScript, err := hex.DecodeString(examples[exampleName].ScriptPubKey)
		if err != nil {
			t.Errorf("decoding script for example %s failed: %s", exampleName, err.Error())
			continue
		}
		expectedAddress, err := migrations.Migration013_ScriptToAddress(actualTx.Txid, decodedScript, true)
		if err != nil {
			t.Errorf("getting expected address for example %s failed: %s", exampleName, err.Error())
			continue
		}
		if expectedAddress != actualTx.Address {
			t.Errorf("Expected address to be converted for example %s, but did not match", exampleName)
			t.Errorf("Example: %+v", examples[actualTx.Txid])
			t.Errorf("Actual: %+v", actualTx)
		}
	}
}
