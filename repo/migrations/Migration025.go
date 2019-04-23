package migrations

import (
	"database/sql"
	"os"
	"path"
	"strings"

	_ "github.com/mutecomm/go-sqlcipher"
)

var (
	m025_up_create_utxos   = "create table utxos (outpoint text primary key not null, value text, height integer, scriptPubKey text, watchOnly integer, coin text);"
	m025_down_create_utxos = "create table utxos (outpoint text primary key not null, value integer, height integer, scriptPubKey text, watchOnly integer, coin text);"
	m025_temp_utxos        = "ALTER TABLE utxos RENAME TO temp_utxos;"
	m025_insert_utxos      = "INSERT INTO utxos SELECT outpoint, value, height, scriptPubKey, watchOnly, coin FROM temp_utxos;"
	m025_drop_temp_utxos   = "DROP TABLE temp_utxos;"

	m025_up_create_stxos   = "create table stxos (outpoint text primary key not null, value text, height integer, scriptPubKey text, watchOnly integer, spendHeight integer, spendTxid text, coin text);"
	m025_down_create_stxos = "create table stxos (outpoint text primary key not null, value integer, height integer, scriptPubKey text, watchOnly integer, spendHeight integer, spendTxid text, coin text);"
	m025_temp_stxos        = "ALTER TABLE stxos RENAME TO temp_stxos;"
	m025_insert_stxos      = "INSERT INTO stxos SELECT outpoint, value, height, scriptPubKey, watchOnly, spendHeight, spendTxid, coin FROM temp_stxos;"
	m025_drop_temp_stxos   = "DROP TABLE temp_stxos;"

	m025_up_create_txns   = "create table txns (txid text primary key not null, value text, height integer, timestamp integer, watchOnly integer, tx blob, coin text);"
	m025_down_create_txns = "create table txns (txid text primary key not null, value integer, height integer, timestamp integer, watchOnly integer, tx blob, coin text);"
	m025_temp_txns        = "ALTER TABLE txns RENAME TO temp_txns;"
	m025_insert_txns      = "INSERT INTO txns SELECT txid, value, height, timestamp, watchOnly, tx, coin FROM temp_txns;"
	m025_drop_temp_txns   = "DROP TABLE temp_txns;"
)

type Migration025 struct{}

func (Migration025) Up(repoPath string, dbPassword string, testnet bool) error {
	var dbPath string
	if testnet {
		dbPath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		dbPath = path.Join(repoPath, "datastore", "mainnet.db")
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	if dbPassword != "" {
		p := "pragma key='" + dbPassword + "';"
		db.Exec(p)
	}

	upSequence := strings.Join([]string{
		m025_temp_utxos,
		m025_up_create_utxos,
		m025_insert_utxos,
		m025_drop_temp_utxos,
		m025_temp_stxos,
		m025_up_create_stxos,
		m025_insert_stxos,
		m025_drop_temp_stxos,
		m025_temp_txns,
		m025_up_create_txns,
		m025_insert_txns,
		m025_drop_temp_txns,
	}, " ")

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(upSequence); err != nil {
		tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	f1, err := os.Create(path.Join(repoPath, "repover"))
	if err != nil {
		return err
	}
	_, err = f1.Write([]byte("26"))
	if err != nil {
		return err
	}
	f1.Close()
	return nil
}

func (Migration025) Down(repoPath string, dbPassword string, testnet bool) error {
	var dbPath string
	if testnet {
		dbPath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		dbPath = path.Join(repoPath, "datastore", "mainnet.db")
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	if dbPassword != "" {
		p := "pragma key='" + dbPassword + "';"
		db.Exec(p)
	}
	downSequence := strings.Join([]string{
		m025_temp_utxos,
		m025_down_create_utxos,
		m025_insert_utxos,
		m025_drop_temp_utxos,
		m025_temp_stxos,
		m025_down_create_stxos,
		m025_insert_stxos,
		m025_drop_temp_stxos,
		m025_temp_txns,
		m025_down_create_txns,
		m025_insert_txns,
		m025_drop_temp_txns,
	}, " ")

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(downSequence); err != nil {
		tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	f1, err := os.Create(path.Join(repoPath, "repover"))
	if err != nil {
		return err
	}
	_, err = f1.Write([]byte("25"))
	if err != nil {
		return err
	}
	f1.Close()
	return nil
}
