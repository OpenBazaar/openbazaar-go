package migrations

import (
	"database/sql"
	"os"
	"path"
	"strings"

	_ "github.com/mutecomm/go-sqlcipher"
)

var (
	AM02_up_create_sales   = "create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total text, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, needsSync integer, lastDisputeTimeoutNotifiedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');"
	AM02_down_create_sales = "create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, needsSync integer, lastDisputeTimeoutNotifiedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');"
	AM02_temp_sales        = "ALTER TABLE sales RENAME TO temp_sales;"
	AM02_insert_sales      = "INSERT INTO sales SELECT orderID, contract, state, read, timestamp, total, thumbnail, buyerID, buyerHandle, title, shippingName, shippingAddress, paymentAddr, funded, transactions, needsSync, lastDisputeTimeoutNotifiedAt, coinType, paymentCoin FROM temp_sales;"
	AM02_drop_temp_sales   = "DROP TABLE temp_sales;"

	AM02_up_create_purchases   = "create table purchases (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total text, thumbnail text, vendorID text, vendorHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, lastDisputeTimeoutNotifiedAt integer not null default 0, lastDisputeExpiryNotifiedAt integer not null default 0, disputedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');"
	AM02_down_create_purchases = "create table purchases (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, vendorID text, vendorHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, lastDisputeTimeoutNotifiedAt integer not null default 0, lastDisputeExpiryNotifiedAt integer not null default 0, disputedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');"
	AM02_temp_purchases        = "ALTER TABLE purchases RENAME TO temp_purchases;"
	AM02_insert_purchases      = "INSERT INTO purchases SELECT orderID, contract, state, read, timestamp, total, thumbnail, vendorID, vendorHandle, title, shippingName, shippingAddress, paymentAddr, funded, transactions, lastDisputeTimeoutNotifiedAt, lastDisputeExpiryNotifiedAt, disputedAt, coinType, paymentCoin FROM temp_purchases;"
	AM02_drop_temp_purchases   = "DROP TABLE temp_purchases;"
)

type Migration028 struct {
	AM02
}

type AM02 struct{}

var AM02UpVer = "29"
var AM02DownVer = "28"

func (AM02) Up(repoPath string, dbPassword string, testnet bool) error {
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
		AM02_temp_sales,
		AM02_up_create_sales,
		AM02_insert_sales,
		AM02_drop_temp_sales,
		AM02_temp_purchases,
		AM02_up_create_purchases,
		AM02_insert_purchases,
		AM02_drop_temp_purchases,
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
	_, err = f1.Write([]byte(AM02UpVer))
	if err != nil {
		return err
	}
	f1.Close()
	return nil
}

func (AM02) Down(repoPath string, dbPassword string, testnet bool) error {
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
		AM02_temp_sales,
		AM02_down_create_sales,
		AM02_insert_sales,
		AM02_drop_temp_sales,
		AM02_temp_purchases,
		AM02_down_create_purchases,
		AM02_insert_purchases,
		AM02_drop_temp_purchases,
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
	_, err = f1.Write([]byte(AM02DownVer))
	if err != nil {
		return err
	}
	f1.Close()
	return nil
}
