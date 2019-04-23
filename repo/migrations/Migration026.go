package migrations

import (
	"database/sql"
	"os"
	"path"
	"strings"

	_ "github.com/mutecomm/go-sqlcipher"
)

var (
	m026_up_create_sales   = "create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total text, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, needsSync integer, lastDisputeTimeoutNotifiedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');"
	m026_down_create_sales = "create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, needsSync integer, lastDisputeTimeoutNotifiedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');"
	m026_temp_sales        = "ALTER TABLE sales RENAME TO temp_sales;"
	m026_insert_sales      = "INSERT INTO sales SELECT orderID, contract, state, read, timestamp, total, thumbnail, buyerID, buyerHandle, title, shippingName, shippingAddress, paymentAddr, funded, transactions, needsSync, lastDisputeTimeoutNotifiedAt, coinType, paymentCoin FROM temp_sales;"
	m026_drop_temp_sales   = "DROP TABLE temp_sales;"

	m026_up_create_purchases   = "create table purchases (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total text, thumbnail text, vendorID text, vendorHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, lastDisputeTimeoutNotifiedAt integer not null default 0, lastDisputeExpiryNotifiedAt integer not null default 0, disputedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');"
	m026_down_create_purchases = "create table purchases (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, vendorID text, vendorHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, lastDisputeTimeoutNotifiedAt integer not null default 0, lastDisputeExpiryNotifiedAt integer not null default 0, disputedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');"
	m026_temp_purchases        = "ALTER TABLE purchases RENAME TO temp_purchases;"
	m026_insert_purchases      = "INSERT INTO purchases SELECT orderID, contract, state, read, timestamp, total, thumbnail, vendorID, vendorHandle, title, shippingName, shippingAddress, paymentAddr, funded, transactions, lastDisputeTimeoutNotifiedAt, lastDisputeExpiryNotifiedAt, disputedAt, coinType, paymentCoin FROM temp_purchases;"
	m026_drop_temp_purchases   = "DROP TABLE temp_purchases;"
)

type Migration026 struct{}

func (Migration026) Up(repoPath string, dbPassword string, testnet bool) error {
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
		m026_temp_sales,
		m026_up_create_sales,
		m026_insert_sales,
		m026_drop_temp_sales,
		m026_temp_purchases,
		m026_up_create_purchases,
		m026_insert_purchases,
		m026_drop_temp_purchases,
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
	_, err = f1.Write([]byte("27"))
	if err != nil {
		return err
	}
	f1.Close()
	return nil
}

func (Migration026) Down(repoPath string, dbPassword string, testnet bool) error {
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
		m026_temp_sales,
		m026_down_create_sales,
		m026_insert_sales,
		m026_drop_temp_sales,
		m026_temp_purchases,
		m026_down_create_purchases,
		m026_insert_purchases,
		m026_drop_temp_purchases,
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
	_, err = f1.Write([]byte("26"))
	if err != nil {
		return err
	}
	f1.Close()
	return nil
}
