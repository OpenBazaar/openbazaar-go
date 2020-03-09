package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"path"
	"strings"

	_ "github.com/mutecomm/go-sqlcipher"
)

var (
	am03_up_create_utxos   = "create table utxos (outpoint text primary key not null, value text, height integer, scriptPubKey text, watchOnly integer, coin text);"
	am03_down_create_utxos = "create table utxos (outpoint text primary key not null, value integer, height integer, scriptPubKey text, watchOnly integer, coin text);"
	am03_temp_utxos        = "ALTER TABLE utxos RENAME TO temp_utxos;"
	am03_insert_utxos      = "INSERT INTO utxos SELECT outpoint, value, height, scriptPubKey, watchOnly, coin FROM temp_utxos;"
	am03_drop_temp_utxos   = "DROP TABLE temp_utxos;"

	am03_up_create_stxos   = "create table stxos (outpoint text primary key not null, value text, height integer, scriptPubKey text, watchOnly integer, spendHeight integer, spendTxid text, coin text);"
	am03_down_create_stxos = "create table stxos (outpoint text primary key not null, value integer, height integer, scriptPubKey text, watchOnly integer, spendHeight integer, spendTxid text, coin text);"
	am03_temp_stxos        = "ALTER TABLE stxos RENAME TO temp_stxos;"
	am03_insert_stxos      = "INSERT INTO stxos SELECT outpoint, value, height, scriptPubKey, watchOnly, spendHeight, spendTxid, coin FROM temp_stxos;"
	am03_drop_temp_stxos   = "DROP TABLE temp_stxos;"

	am03_up_create_txns   = "create table txns (txid text primary key not null, value text, height integer, timestamp integer, watchOnly integer, tx blob, coin text);"
	am03_down_create_txns = "create table txns (txid text primary key not null, value integer, height integer, timestamp integer, watchOnly integer, tx blob, coin text);"
	am03_temp_txns        = "ALTER TABLE txns RENAME TO temp_txns;"
	am03_insert_txns      = "INSERT INTO txns SELECT txid, value, height, timestamp, watchOnly, tx, coin FROM temp_txns;"
	am03_drop_temp_txns   = "DROP TABLE temp_txns;"
)

type Migration030 struct{ AM03 }

type AM03 struct{}

func (AM03) Up(repoPath string, dbPassword string, testnet bool) error {
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
		if _, err := db.Exec(p); err != nil {
			return err
		}
	}

	upSequence := strings.Join([]string{
		am03_temp_utxos,
		am03_up_create_utxos,
		am03_insert_utxos,
		am03_drop_temp_utxos,
		am03_temp_stxos,
		am03_up_create_stxos,
		am03_insert_stxos,
		am03_drop_temp_stxos,
		am03_temp_txns,
		am03_up_create_txns,
		am03_insert_txns,
		am03_drop_temp_txns,
	}, " ")

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(upSequence); err != nil {
		if rErr := tx.Rollback(); rErr != nil {
			return fmt.Errorf("rollback failed: (%s) due to (%s)", rErr.Error(), err.Error())
		}
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	f1, err := os.Create(path.Join(repoPath, "repover"))
	if err != nil {
		return err
	}
	_, err = f1.Write([]byte("31"))
	if err != nil {
		return err
	}
	f1.Close()
	return nil
}

func (AM03) Down(repoPath string, dbPassword string, testnet bool) error {
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
		if _, err := db.Exec(p); err != nil {
			return err
		}
	}
	downSequence := strings.Join([]string{
		am03_temp_utxos,
		am03_down_create_utxos,
		am03_insert_utxos,
		am03_drop_temp_utxos,
		am03_temp_stxos,
		am03_down_create_stxos,
		am03_insert_stxos,
		am03_drop_temp_stxos,
		am03_temp_txns,
		am03_down_create_txns,
		am03_insert_txns,
		am03_drop_temp_txns,
	}, " ")

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(downSequence); err != nil {
		if rErr := tx.Rollback(); rErr != nil {
			return fmt.Errorf("rollback failed: (%s) due to (%s)", rErr.Error(), err.Error())
		}
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	f1, err := os.Create(path.Join(repoPath, "repover"))
	if err != nil {
		return err
	}
	_, err = f1.Write([]byte("30"))
	if err != nil {
		return err
	}
	f1.Close()
	return nil
}
