package migrations

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/OpenBazaar/openbazaar-go/schema"
)

const migration007_casesCreateSQL = "create table cases (caseID text primary key not null, buyerContract blob, vendorContract blob, buyerValidationErrors blob, vendorValidationErrors blob, buyerPayoutAddress text, vendorPayoutAddress text, buyerOutpoints blob, vendorOutpoints blob, state integer, read integer, timestamp integer, buyerOpened integer, claim text, disputeResolution blob);"
const migration007_purchasesCreateSQL = "create table purchases (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, vendorID text, vendorHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob);"

type Migration007 struct{}

func (Migration007) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	var executedAt = time.Now()
	paths, err := schema.NewCustomSchemaManager(schema.SchemaContext{
		DataPath:        repoPath,
		TestModeEnabled: testnetEnabled,
	})
	if err != nil {
		return err
	}

	// Add lastNotifiedAt column
	db, err := sql.Open("sqlite3", paths.DatastorePath())
	if err != nil {
		return err
	}
	defer db.Close()
	if databasePassword != "" {
		p := fmt.Sprintf("pragma key = '%s';", databasePassword)
		_, err := db.Exec(p)
		if err != nil {
			return err
		}
	}
	_, err = db.Exec("alter table cases add column lastNotifiedAt integer not null default 0")
	if err != nil {
		return err
	}
	_, err = db.Exec("alter table purchases add column lastNotifiedAt integer not null default 0")
	if err != nil {
		return err
	}

	_, err = db.Exec("update cases set lastNotifiedAt = ?", executedAt.Unix())
	if err != nil {
		return err
	}
	_, err = db.Exec("update purchases set lastNotifiedAt = ?", executedAt.Unix())
	if err != nil {
		return err
	}

	// Bump schema version
	err = ioutil.WriteFile(paths.DataPathJoin("repover"), []byte("8"), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func (Migration007) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	paths, err := schema.NewCustomSchemaManager(schema.SchemaContext{
		DataPath:        repoPath,
		TestModeEnabled: testnetEnabled,
	})
	if err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", paths.DatastorePath())
	if err != nil {
		return err
	}
	defer db.Close()
	if databasePassword != "" {
		p := fmt.Sprintf("pragma key = '%s';", databasePassword)
		_, err := db.Exec(p)
		if err != nil {
			return err
		}
	}

	const (
		alterCasesSQL         = "alter table cases rename to cases_old;"
		alterPurchasesSQL     = "alter table purchases rename to purchases_old;"
		insertCasesSQL        = "insert into cases select caseID, buyerContract, vendorContract, buyerValidationErrors, vendorValidationErrors, buyerPayoutAddress, vendorPayoutAddress, buyerOutpoints, vendorOutpoints, state, read, timestamp, buyerOpened, claim, disputeResolution from cases_old;"
		insertPurchasesSQL    = "insert into purchases select orderID, contract, state, read, timestamp, total, thumbnail, vendorID, vendorHandle, title, shippingName, shippingAddress, paymentAddr, funded, transactions from purchases_old;"
		dropCasesTableSQL     = "drop table cases_old;"
		dropPurchasesTableSQL = "drop table purchases_old;"
	)

	dropColumnOperation := strings.Join([]string{
		alterCasesSQL,
		migration007_casesCreateSQL,
		insertCasesSQL,
		dropCasesTableSQL,
		alterPurchasesSQL,
		migration007_purchasesCreateSQL,
		insertPurchasesSQL,
		dropPurchasesTableSQL,
	}, " ")
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(dropColumnOperation); err != nil {
		tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	// Revert schema version
	err = ioutil.WriteFile(paths.DataPathJoin("repover"), []byte("7"), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}
