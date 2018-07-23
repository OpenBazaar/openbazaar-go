package migrations_test

import (
	"bytes"
	"database/sql"
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/wallet-interface"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestMigration011(t *testing.T) {
	// Setup
	migrations.WalletCoinType = wallet.BitcoinCash
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

		schemaSql = "pragma key = 'foobarbaz';"

		CreateTableKeysSQL                      = "create table keys (scriptAddress text primary key not null, purpose integer, keyIndex integer, used integer, key text);"
		CreateTableUnspentTransactionOutputsSQL = "create table utxos (outpoint text primary key not null, value integer, height integer, scriptPubKey text, watchOnly integer);"
		CreateTableSpentTransactionOutputsSQL   = "create table stxos (outpoint text primary key not null, value integer, height integer, scriptPubKey text, watchOnly integer, spendHeight integer, spendTxid text);"
		CreateTableTransactionsSQL              = "create table txns (txid text primary key not null, value integer, height integer, timestamp integer, watchOnly integer, tx blob);"
		CreatedTableWatchedScriptsSQL           = "create table watchedscripts (scriptPubKey text primary key not null);"

		insertKeysSQL         = "insert into keys(scriptAddress, purpose, keyIndex, used, key) values(?,?,?,?,?)"
		insertUtxosSQL        = "insert or replace into utxos(outpoint, value, height, scriptPubKey, watchOnly) values(?,?,?,?,?)"
		insertStxosSQL        = "insert or replace into stxos(outpoint, value, height, scriptPubKey, watchOnly, spendHeight, spendTxid) values(?,?,?,?,?,?,?)"
		insertTxnsSQL         = "insert or replace into txns(txid, value, height, timestamp, watchOnly, tx) values(?,?,?,?,?,?)"
		insertWatchScriptsSQL = "insert or replace into watchedscripts(scriptPubKey) values(?)"

		selectKeysSQL           = "select coin, scriptAddress, purpose, keyIndex, used, key from keys where scriptAddress=?"
		selectUtxosSQL          = "select coin, outpoint, value, height, scriptPubKey, watchOnly from utxos where outpoint=?"
		selectStxosSQL          = "select coin, outpoint, value, height, scriptPubKey, watchOnly, spendHeight, spendTxid from stxos where outpoint=?"
		selectTxnsSQL           = "select coin, txid, value, height, timestamp, watchOnly, tx from txns where txid=?"
		selectWatchedScriptsSQL = "select coin, scriptPubKey from watchedscripts where scriptPubKey=?"
	)

	// Setup datastore
	dbSetupSql := strings.Join([]string{
		schemaSql,
		CreateTableKeysSQL,
		CreateTableUnspentTransactionOutputsSQL,
		CreateTableSpentTransactionOutputsSQL,
		CreateTableTransactionsSQL,
		CreatedTableWatchedScriptsSQL,
	}, " ")
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(dbSetupSql)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(insertKeysSQL, "key1", 0, 5, 0, "keyhex")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(insertUtxosSQL, "abc:0", 3000, 150000, "def", 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(insertStxosSQL, "abc:0", 3000, 150000, "def", 0, 150001, "1234")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(insertTxnsSQL, "abc", 3000, 150000, 12345, 0, "1234")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(insertWatchScriptsSQL, "abc")
	if err != nil {
		t.Fatal(err)
	}

	// Create schema version file
	if err = ioutil.WriteFile(schemaPath, []byte("9"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// Execute Migration Up
	migration := migrations.Migration011{}
	if err := migration.Up(testRepoPath, "foobarbaz", true); err != nil {
		t.Fatal(err)
	}

	// Assert repo version updated
	if err = appSchema.VerifySchemaVersion("10"); err != nil {
		t.Fatal(err)
	}

	// Verify keys data
	keysRows, err := db.Query(selectKeysSQL, "key1")
	if err != nil {
		t.Fatal(err)
	}
	coinColumnExists := false
	columns, err := keysRows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range columns {
		if c.Name() == "coin" {
			coinColumnExists = true
		}
	}
	if coinColumnExists == false {
		t.Error("Expected coin column to exist on keys")
	}
	for keysRows.Next() {
		var coin, scriptAddres, key string
		var purpose, keyIndex, used int
		err := keysRows.Scan(&coin, &scriptAddres, &purpose, &keyIndex, &used, &key)
		if err != nil {
			t.Error(err)
		}
		if coin != "BCH" {
			t.Error("Failed to return correct coin type")
		}
		if scriptAddres != "key1" {
			t.Error("Failed to return correct scriptAddress")
		}
		if key != "keyhex" {
			t.Error("Failed to return correct key")
		}
		if purpose != 0 {
			t.Error("Failed to return correct purpose")
		}
		if used != 0 {
			t.Error("Failed to return correct used")
		}
		if keyIndex != 5 {
			t.Error("Failed to return correct keyIndex")
		}
	}

	// Verify utxos data
	utxosRows, err := db.Query(selectUtxosSQL, "abc:0")
	if err != nil {
		t.Fatal(err)
	}
	coinColumnExists = false
	columns, err = utxosRows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range columns {
		if c.Name() == "coin" {
			coinColumnExists = true
		}
	}
	if coinColumnExists == false {
		t.Error("Expected coin column to exist on utxos")
	}
	for utxosRows.Next() {
		var coin, outpoint, scriptPubkey string
		var value, height, watchOnly int
		err := utxosRows.Scan(&coin, &outpoint, &value, &height, &scriptPubkey, &watchOnly)
		if err != nil {
			t.Error(err)
		}
		if coin != "BCH" {
			t.Error("Failed to return correct coin type")
		}
		if outpoint != "abc:0" {
			t.Error("Failed to return correct outpoint")
		}
		if value != 3000 {
			t.Error("Failed to return correct value")
		}
		if height != 150000 {
			t.Error("Failed to return correct height")
		}
		if watchOnly != 0 {
			t.Error("Failed to return correct watchOnly")
		}
	}

	// Verify stxos data
	stxosRows, err := db.Query(selectStxosSQL, "abc:0")
	if err != nil {
		t.Fatal(err)
	}
	coinColumnExists = false
	columns, err = stxosRows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range columns {
		if c.Name() == "coin" {
			coinColumnExists = true
		}
	}
	if coinColumnExists == false {
		t.Error("Expected coin column to exist on stxos")
	}
	for stxosRows.Next() {
		var coin, outpoint, scriptPubkey, spendTxid string
		var value, height, watchOnly, spendHeight int
		err := stxosRows.Scan(&coin, &outpoint, &value, &height, &scriptPubkey, &watchOnly, &spendHeight, &spendTxid)
		if err != nil {
			t.Error(err)
		}
		if coin != "BCH" {
			t.Error("Failed to return correct coin type")
		}
		if outpoint != "abc:0" {
			t.Error("Failed to return correct outpoint")
		}
		if value != 3000 {
			t.Error("Failed to return correct value")
		}
		if height != 150000 {
			t.Error("Failed to return correct height")
		}
		if watchOnly != 0 {
			t.Error("Failed to return correct watchOnly")
		}
		if spendHeight != 150001 {
			t.Error("Failed to return correct spendHeight")
		}
		if spendTxid != "1234" {
			t.Error("Failed to return correct spendTxid")
		}
	}

	// Verify txns data
	txnsRows, err := db.Query(selectTxnsSQL, "abc")
	if err != nil {
		t.Fatal(err)
	}
	coinColumnExists = false
	columns, err = txnsRows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range columns {
		if c.Name() == "coin" {
			coinColumnExists = true
		}
	}
	if coinColumnExists == false {
		t.Error("Expected coin column to exist on txns")
	}
	for txnsRows.Next() {
		var coin, txid string
		var tx []byte
		var value, height, watchOnly, timestamp int
		err := txnsRows.Scan(&coin, &txid, &value, &height, &timestamp, &watchOnly, &tx)
		if err != nil {
			t.Error(err)
		}
		if coin != "BCH" {
			t.Error("Failed to return correct coin type")
		}
		if txid != "abc" {
			t.Error("Failed to return correct txid")
		}
		if value != 3000 {
			t.Error("Failed to return correct value")
		}
		if height != 150000 {
			t.Error("Failed to return correct height")
		}
		if watchOnly != 0 {
			t.Error("Failed to return correct watchOnly")
		}
		if timestamp != 12345 {
			t.Error("Failed to return correct timestamp")
		}
		if !bytes.Equal(tx, []byte("1234")) {
			t.Error("Failed to return correct tx")
		}
	}

	// Verify txns data
	watchedScriptRows, err := db.Query(selectWatchedScriptsSQL, "abc")
	if err != nil {
		t.Fatal(err)
	}
	coinColumnExists = false
	columns, err = watchedScriptRows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range columns {
		if c.Name() == "coin" {
			coinColumnExists = true
		}
	}
	if coinColumnExists == false {
		t.Error("Expected coin column to exist on watchedScripts")
	}
	for watchedScriptRows.Next() {
		var coin, scriptPubkey string
		err := watchedScriptRows.Scan(&coin, &scriptPubkey)
		if err != nil {
			t.Error(err)
		}
		if coin != "BCH" {
			t.Error("Failed to return correct coin type")
		}
		if scriptPubkey != "abc" {
			t.Error("Failed to return correct txid")
		}
	}

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

	// Assert coin column on cases migrated down
	_, err = db.Query("update keys set coin = ? where scriptAddress = ?;", "BTC", "key1")
	if err == nil {
		t.Error("Expected coin update on keys to fail")
	}
	if err != nil && !strings.Contains(err.Error(), "no such column: coin") {
		t.Error("Expected error to be 'no such column', was:", err.Error())
	}
	_, err = db.Query("update utxos set coin = ? where outpoint = ?;", "BTC", "abc:0")
	if err == nil {
		t.Error("Expected coin update on utxos to fail")
	}
	if err != nil && !strings.Contains(err.Error(), "no such column: coin") {
		t.Error("Expected error to be 'no such column', was:", err.Error())
	}
	_, err = db.Query("update stxos set coin = ? where outpoint = ?;", "BTC", "abc:0")
	if err == nil {
		t.Error("Expected coin update on stxos to fail")
	}
	if err != nil && !strings.Contains(err.Error(), "no such column: coin") {
		t.Error("Expected error to be 'no such column', was:", err.Error())
	}
	_, err = db.Query("update txns set coin = ? where outpoint = ?;", "BTC", "abc")
	if err == nil {
		t.Error("Expected coin update on txns to fail")
	}
	if err != nil && !strings.Contains(err.Error(), "no such column: coin") {
		t.Error("Expected error to be 'no such column', was:", err.Error())
	}
	_, err = db.Query("update watchedscripts set coin = ? where outpoint = ?;", "BTC", "abc")
	if err == nil {
		t.Error("Expected coin update on watchedscripts to fail")
	}
	if err != nil && !strings.Contains(err.Error(), "no such column: coin") {
		t.Error("Expected error to be 'no such column', was:", err.Error())
	}
	db.Close()
}
