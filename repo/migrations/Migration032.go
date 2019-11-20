package migrations

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"time"
)

type Migration032 struct {
	AM01
}

// AM01 migrates the listing and order data to use higher precision.
type AM01 struct{}

var AM01UpVer = 33
var AM01DownVer = 32

type AM01_TransactionRecord_beforeMigration struct {
	Txid      string
	Index     uint32
	Value     int64
	Address   string
	Spent     bool
	Timestamp time.Time
}

type AM01_TransactionRecord_afterMigration struct {
	Txid      string
	Index     uint32
	Value     big.Int
	Address   string
	Spent     bool
	Timestamp time.Time
}

type AM01_record struct {
	orderID                string
	coin                   string
	unmigratedTransactions []AM01_TransactionRecord_beforeMigration
	migratedTransactions   []AM01_TransactionRecord_afterMigration
}

func (AM01) Up(repoPath string, dbPassword string, testnet bool) (err error) {
	db, err := OpenDB(repoPath, dbPassword, testnet)
	if err != nil {
		return fmt.Errorf("opening db: %s", err.Error())
	}
	saleMigrationRecords, err := AM01_extractRecords(db, "select orderID, transactions, paymentCoin from sales;", false)
	if err != nil {
		return fmt.Errorf("get sales rows: %s", err.Error())
	}
	purchaseMigrationRecords, err := AM01_extractRecords(db, "select orderID, transactions, paymentCoin from purchases;", false)
	if err != nil {
		return fmt.Errorf("get purchase rows: %s", err.Error())
	}

	if err := withTransaction(db, func(tx *sql.Tx) error {
		err := AM01_updateRecords(tx, saleMigrationRecords, "update sales set transactions = ? where orderID = ?", testnet, false)
		if err != nil {
			return fmt.Errorf("update sales: %s", err.Error())
		}
		err = AM01_updateRecords(tx, purchaseMigrationRecords, "update purchases set transactions = ? where orderID = ?", testnet, false)
		if err != nil {
			return fmt.Errorf("update purchases: %s", err.Error())
		}
		return nil
	}); err != nil {
		return fmt.Errorf("migrating up: %s", err.Error())
	}

	return writeRepoVer(repoPath, AM01UpVer)
}

func (AM01) Down(repoPath string, dbPassword string, testnet bool) (err error) {
	db, err := OpenDB(repoPath, dbPassword, testnet)
	if err != nil {
		return fmt.Errorf("opening db: %s", err.Error())
	}
	saleMigrationRecords, err := AM01_extractRecords(db, "select orderID, transactions, paymentCoin from sales;", true)
	if err != nil {
		return fmt.Errorf("get sales rows: %s", err.Error())
	}
	purchaseMigrationRecords, err := AM01_extractRecords(db, "select orderID, transactions, paymentCoin from purchases;", true)
	if err != nil {
		return fmt.Errorf("get purchase rows: %s", err.Error())
	}

	if err := withTransaction(db, func(tx *sql.Tx) error {
		err := AM01_updateRecords(tx, saleMigrationRecords, "update sales set transactions = ? where orderID = ?", testnet, true)
		if err != nil {
			return fmt.Errorf("update sales: %s", err.Error())
		}
		err = AM01_updateRecords(tx, purchaseMigrationRecords, "update purchases set transactions = ? where orderID = ?", testnet, true)
		if err != nil {
			return fmt.Errorf("update purchases: %s", err.Error())
		}
		return nil
	}); err != nil {
		return fmt.Errorf("migrating down: %s", err.Error())
	}

	return writeRepoVer(repoPath, AM01DownVer)
}

func AM01_extractRecords(db *sql.DB, query string, migrateDown bool) ([]AM01_record, error) {
	var (
		results   = make([]AM01_record, 0)
		rows, err = db.Query(query)
	)
	if err != nil {
		return nil, fmt.Errorf("selecting rows: %s", err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var (
			serializedTransactions sql.NullString
			r                      = AM01_record{}
		)
		if err := rows.Scan(&r.orderID, &serializedTransactions, &r.coin); err != nil {
			return nil, fmt.Errorf("scanning rows: %s", err.Error())
		}
		if !serializedTransactions.Valid {
			continue
		}
		if migrateDown {
			if err := json.Unmarshal([]byte(serializedTransactions.String), &r.migratedTransactions); err != nil {
				return nil, fmt.Errorf("unmarshal migrated transactions: %s", err.Error())
			}
		} else {
			if err := json.Unmarshal([]byte(serializedTransactions.String), &r.unmigratedTransactions); err != nil {
				return nil, fmt.Errorf("unmarshal unmigrated transactions: %s", err.Error())
			}
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %s", err.Error())
	}
	return results, nil
}

func AM01_updateRecords(tx *sql.Tx, records []AM01_record, query string, testMode bool, migrateDown bool) error {
	var update, err = tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("prepare update statement: %s", err.Error())
	}
	defer update.Close()
	for _, beforeRecord := range records {

		if migrateDown {
			var migratedTransactionRecords = make([]AM01_TransactionRecord_beforeMigration, 0)
			for _, beforeTx := range beforeRecord.migratedTransactions {
				var migratedRecord = AM01_TransactionRecord_beforeMigration{
					Txid:      beforeTx.Txid,
					Index:     beforeTx.Index,
					Value:     beforeTx.Value.Int64(),
					Spent:     beforeTx.Spent,
					Timestamp: beforeTx.Timestamp,
					Address:   beforeTx.Address,
				}
				migratedTransactionRecords = append(migratedTransactionRecords, migratedRecord)
			}
			serializedTransactionRecords, err := json.Marshal(migratedTransactionRecords)
			if err != nil {
				return fmt.Errorf("marshal transactions: %s", err.Error())
			}
			if _, err := update.Exec(string(serializedTransactionRecords), beforeRecord.orderID); err != nil {
				return fmt.Errorf("updating record: %s", err.Error())
			}
		} else {
			var migratedTransactionRecords = make([]AM01_TransactionRecord_afterMigration, 0)
			for _, beforeTx := range beforeRecord.unmigratedTransactions {
				n := big.NewInt(beforeTx.Value)
				var migratedRecord = AM01_TransactionRecord_afterMigration{
					Txid:      beforeTx.Txid,
					Index:     beforeTx.Index,
					Value:     *n,
					Spent:     beforeTx.Spent,
					Timestamp: beforeTx.Timestamp,
					Address:   beforeTx.Address,
				}
				migratedTransactionRecords = append(migratedTransactionRecords, migratedRecord)
			}
			serializedTransactionRecords, err := json.Marshal(migratedTransactionRecords)
			if err != nil {
				return fmt.Errorf("marhsal transactions: %s", err.Error())
			}
			if _, err := update.Exec(string(serializedTransactionRecords), beforeRecord.orderID); err != nil {
				return fmt.Errorf("updating record: %s", err.Error())
			}
		}

	}
	return nil
}
