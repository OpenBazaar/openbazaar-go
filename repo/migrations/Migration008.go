package migrations

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
)

const (
	Migration008_OrderState_PENDING          = 0
	Migration008_OrderState_AWAITING_PAYMENT = 1
	Migration008_OrderState_DISPUTED         = 10

	Migration008_casesCreateSQL     = "create table cases (caseID text primary key not null, buyerContract blob, vendorContract blob, buyerValidationErrors blob, vendorValidationErrors blob, buyerPayoutAddress text, vendorPayoutAddress text, buyerOutpoints blob, vendorOutpoints blob, state integer, read integer, timestamp integer, buyerOpened integer, claim text, disputeResolution blob, lastNotifiedAt integer not null default 0);"
	Migration008_purchasesCreateSQL = "create table purchases (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, vendorID text, vendorHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, lastNotifiedAt integer not null default 0);"
	Migration008_salesCreateSQL     = "create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, needsSync integer, lastNotifiedAt integer not null default 0);"
)

type Migration008 struct{}

func (Migration008) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	var (
		databaseFilePath    string
		executedAt          = time.Now()
		repoVersionFilePath = path.Join(repoPath, "repover")
	)
	if testnetEnabled {
		databaseFilePath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		databaseFilePath = path.Join(repoPath, "datastore", "mainnet.db")
	}

	const (
		alterCasesSQL             = "alter table cases rename to cases_old;"
		alterPurchasesSQL         = "alter table purchases rename to purchases_old;"
		alterSalesSQL             = "alter table sales rename to sales_old;"
		createNewDisputedCasesSQL = "create table cases (caseID text primary key not null, buyerContract blob, vendorContract blob, buyerValidationErrors blob, vendorValidationErrors blob, buyerPayoutAddress text, vendorPayoutAddress text, buyerOutpoints blob, vendorOutpoints blob, state integer, read integer, timestamp integer, buyerOpened integer, claim text, disputeResolution blob, lastDisputeExpiryNotifiedAt integer not null default 0);"
		createNewPurchasesSQL     = "create table purchases (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, vendorID text, vendorHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, lastDisputeTimeoutNotifiedAt integer not null default 0, lastDisputeExpiryNotifiedAt integer, disputedAt integer not null default 0);"
		createNewSalesSQL         = "create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, needsSync integer, lastDisputeTimeoutNotifiedAt integer not null default 0);"
		insertCasesSQL            = "insert into cases select caseID, buyerContract, vendorContract, buyerValidationErrors, vendorValidationErrors, buyerPayoutAddress, vendorPayoutAddress, buyerOutpoints, vendorOutpoints, state, read, timestamp, buyerOpened, claim, disputeResolution, lastNotifiedAt from cases_old;"
		insertPurchasesSQL        = "insert into purchases select orderID, contract, state, read, timestamp, total, thumbnail, vendorID, vendorHandle, title, shippingName, shippingAddress, paymentAddr, funded, transactions, lastNotifiedAt, 0, 0 from purchases_old;"
		insertSalesSQL            = "insert into sales select orderID, contract, state, read, timestamp, total, thumbnail, buyerID, buyerHandle, title, shippingName, shippingAddress, paymentAddr, funded, transactions, needsSync, lastNotifiedAt from sales_old;"
		dropCasesTableSQL         = "drop table cases_old;"
		dropPurchasesTableSQL     = "drop table purchases_old;"
		dropSalesTableSQL         = "drop table sales_old;"
	)

	db, err := sql.Open("sqlite3", databaseFilePath)
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

	migration := strings.Join([]string{
		alterCasesSQL,
		createNewDisputedCasesSQL,
		insertCasesSQL,
		dropCasesTableSQL,
		alterPurchasesSQL,
		createNewPurchasesSQL,
		insertPurchasesSQL,
		dropPurchasesTableSQL,
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

	_, err = db.Exec("update cases set lastDisputeExpiryNotifiedAt = ?", executedAt.Unix())
	if err != nil {
		return err
	}
	_, err = db.Exec("update purchases set lastDisputeTimeoutNotifiedAt = ?", executedAt.Unix())
	if err != nil {
		return err
	}
	_, err = db.Exec("update purchases set lastDisputeExpiryNotifiedAt = ? where state = ?", executedAt.Unix(), Migration008_OrderState_DISPUTED)
	if err != nil {
		return err
	}
	_, err = db.Exec("update sales set lastDisputeTimeoutNotifiedAt = ?", executedAt.Unix())
	if err != nil {
		return err
	}

	updateDisputedAtRows, err := db.Query("select orderId, contract from purchases where state = ?", Migration008_OrderState_DISPUTED)
	if err != nil {
		return err
	}
	var updateDisputedAtTimestamps = make(map[string]int64)
	for updateDisputedAtRows.Next() {
		var (
			orderID      string
			contractData string
			contract     = &pb.RicardianContract{}
		)
		if err := updateDisputedAtRows.Scan(&orderID, &contractData); err != nil {
			return err
		}
		if err := jsonpb.UnmarshalString(contractData, contract); err != nil {
			return err
		}
		if contract.Dispute != nil && contract.Dispute.Timestamp != nil {
			updateDisputedAtTimestamps[orderID] = contract.Dispute.Timestamp.Seconds
		}
	}
	if err = updateDisputedAtRows.Close(); err != nil {
		return err
	}

	for orderID, unixTimestamp := range updateDisputedAtTimestamps {
		_, err := db.Exec("update purchases set disputedAt = ? where orderId = ?", unixTimestamp, orderID)
		if err != nil {
			return err
		}
	}

	// Bump schema version
	err = ioutil.WriteFile(repoVersionFilePath, []byte("9"), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func (Migration008) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	var (
		databaseFilePath    string
		repoVersionFilePath = path.Join(repoPath, "repover")
	)
	if testnetEnabled {
		databaseFilePath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		databaseFilePath = path.Join(repoPath, "datastore", "mainnet.db")
	}

	db, err := sql.Open("sqlite3", databaseFilePath)
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
		alterSalesSQL         = "alter table sales rename to sales_old;"
		insertCasesSQL        = "insert into cases select caseID, buyerContract, vendorContract, buyerValidationErrors, vendorValidationErrors, buyerPayoutAddress, vendorPayoutAddress, buyerOutpoints, vendorOutpoints, state, read, timestamp, buyerOpened, claim, disputeResolution, lastDisputeExpiryNotifiedAt from cases_old;"
		insertPurchasesSQL    = "insert into purchases select orderID, contract, state, read, timestamp, total, thumbnail, vendorID, vendorHandle, title, shippingName, shippingAddress, paymentAddr, funded, transactions, lastDisputeTimeoutNotifiedAt from purchases_old;"
		insertSalesSQL        = "insert into sales select orderID, contract, state, read, timestamp, total, thumbnail, buyerID, buyerHandle, title, shippingName, shippingAddress, paymentAddr, funded, transactions, needsSync, lastDisputeTimeoutNotifiedAt from sales_old;"
		dropCasesTableSQL     = "drop table cases_old;"
		dropPurchasesTableSQL = "drop table purchases_old;"
		dropSalesTableSQL     = "drop table sales_old;"
	)

	migration := strings.Join([]string{
		alterCasesSQL,
		Migration008_casesCreateSQL,
		insertCasesSQL,
		dropCasesTableSQL,
		alterPurchasesSQL,
		Migration008_purchasesCreateSQL,
		insertPurchasesSQL,
		dropPurchasesTableSQL,
		alterSalesSQL,
		Migration008_salesCreateSQL,
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

	// Revert schema version
	err = ioutil.WriteFile(repoVersionFilePath, []byte("8"), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}
