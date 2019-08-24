package migrations

import (
	"fmt"
	"strings"
)

type Migration025 struct{}

func (Migration025) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	db, err := OpenDB(repoPath, databasePassword, testnetEnabled)
	if err != nil {
		return fmt.Errorf("opening db: %s", err.Error())
	}

	const (
		alterSalesSQL     = "alter table sales rename to sales_old;"
		createNewSalesSQL = "create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, lastDisputeTimeoutNotifiedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');"
		insertSalesSQL    = "insert into sales select orderID, contract, state, read, timestamp, total, thumbnail, buyerID, buyerHandle, title, shippingName, shippingAddress, paymentAddr, funded, transactions, lastDisputeTimeoutNotifiedAt, coinType, paymentCoin from sales_old;"
		dropSalesTableSQL = "drop table sales_old;"
	)

	migration := strings.Join([]string{
		alterSalesSQL,
		createNewSalesSQL,
		insertSalesSQL,
		dropSalesTableSQL,
	}, " ")

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(migration); err != nil {
		tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if err := writeRepoVer(repoPath, 26); err != nil {
		return fmt.Errorf("bumping repover to 26: %s", err.Error())
	}
	return nil
}

func (Migration025) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	db, err := OpenDB(repoPath, databasePassword, testnetEnabled)
	if err != nil {
		return fmt.Errorf("opening db: %s", err.Error())
	}

	const (
		alterSalesSQL = "alter table sales add needsSync integer;"
	)

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(alterSalesSQL); err != nil {
		tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if err := writeRepoVer(repoPath, 25); err != nil {
		return fmt.Errorf("dropping repover to 25: %s", err.Error())
	}
	return nil
}
