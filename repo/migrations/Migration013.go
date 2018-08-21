package migrations

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/OpenBazaar/zcashd-wallet"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
	"github.com/cpacia/bchutil"
)

type Migration013 struct{}

type Migration013_TransactionRecord_beforeMigration struct {
	Txid         string
	Index        uint32
	Value        int64
	ScriptPubKey string
	Spent        bool
	Timestamp    time.Time
}

type Migration013_TransactionRecord_afterMigration struct {
	Txid      string
	Index     uint32
	Value     int64
	Address   string
	Spent     bool
	Timestamp time.Time
}

type migration013_record struct {
	orderID                string
	coin                   string
	unmigratedTransactions []Migration013_TransactionRecord_beforeMigration
	migratedTransactions   []Migration013_TransactionRecord_afterMigration
}

func (Migration013) Up(repoPath string, dbPassword string, testnet bool) (err error) {
	db, err := OpenDB(repoPath, dbPassword, testnet)
	if err != nil {
		return fmt.Errorf("opening db: %s", err.Error())
	}
	saleMigrationRecords, err := migration013_extractRecords(db, "select orderID, transactions, paymentCoin from sales;", false)
	if err != nil {
		return fmt.Errorf("get sales rows: %s", err.Error())
	}
	purchaseMigrationRecords, err := migration013_extractRecords(db, "select orderID, transactions, paymentCoin from purchases;", false)
	if err != nil {
		return fmt.Errorf("get purchase rows: %s", err.Error())
	}

	if err := withTransaction(db, func(tx *sql.Tx) error {
		err := migration013_updateRecords(tx, saleMigrationRecords, "update sales set transactions = ? where orderID = ?", testnet, false)
		if err != nil {
			return fmt.Errorf("update sales: %s", err.Error())
		}
		err = migration013_updateRecords(tx, purchaseMigrationRecords, "update purchases set transactions = ? where orderID = ?", testnet, false)
		if err != nil {
			return fmt.Errorf("update purchases: %s", err.Error())
		}
		return nil
	}); err != nil {
		return fmt.Errorf("migrating up: %s", err.Error())
	}

	return writeRepoVer(repoPath, 14)
}

func (Migration013) Down(repoPath string, dbPassword string, testnet bool) (err error) {
	db, err := OpenDB(repoPath, dbPassword, testnet)
	if err != nil {
		return fmt.Errorf("opening db: %s", err.Error())
	}
	saleMigrationRecords, err := migration013_extractRecords(db, "select orderID, transactions, paymentCoin from sales;", true)
	if err != nil {
		return fmt.Errorf("get sales rows: %s", err.Error())
	}
	purchaseMigrationRecords, err := migration013_extractRecords(db, "select orderID, transactions, paymentCoin from purchases;", true)
	if err != nil {
		return fmt.Errorf("get purchase rows: %s", err.Error())
	}

	if err := withTransaction(db, func(tx *sql.Tx) error {
		err := migration013_updateRecords(tx, saleMigrationRecords, "update sales set transactions = ? where orderID = ?", testnet, true)
		if err != nil {
			return fmt.Errorf("update sales: %s", err.Error())
		}
		err = migration013_updateRecords(tx, purchaseMigrationRecords, "update purchases set transactions = ? where orderID = ?", testnet, true)
		if err != nil {
			return fmt.Errorf("update purchases: %s", err.Error())
		}
		return nil
	}); err != nil {
		return fmt.Errorf("migrating down: %s", err.Error())
	}

	return writeRepoVer(repoPath, 13)
}

func migration013_extractRecords(db *sql.DB, query string, migrateDown bool) ([]migration013_record, error) {
	var (
		results   = make([]migration013_record, 0)
		rows, err = db.Query(query)
	)
	if err != nil {
		return nil, fmt.Errorf("selecting rows: %s", err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var (
			serializedTransactions sql.NullString
			r                      = migration013_record{}
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

func migration013_updateRecords(tx *sql.Tx, records []migration013_record, query string, testMode bool, migrateDown bool) error {
	var update, err = tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("prepare update statement: %s", err.Error())
	}
	defer update.Close()
	for _, beforeRecord := range records {

		if migrateDown {
			var migratedTransactionRecords = make([]Migration013_TransactionRecord_beforeMigration, 0)
			for _, beforeTx := range beforeRecord.migratedTransactions {
				script, err := Migration013_AddressToScript(beforeRecord.coin, beforeTx.Address, testMode)
				if err != nil {
					return fmt.Errorf("address to script: %s", err.Error())
				}
				var migratedRecord = Migration013_TransactionRecord_beforeMigration{
					Txid:         beforeTx.Txid,
					Index:        beforeTx.Index,
					Value:        beforeTx.Value,
					Spent:        beforeTx.Spent,
					Timestamp:    beforeTx.Timestamp,
					ScriptPubKey: hex.EncodeToString(script),
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
		} else {
			var migratedTransactionRecords = make([]Migration013_TransactionRecord_afterMigration, 0)
			for _, beforeTx := range beforeRecord.unmigratedTransactions {
				script, err := hex.DecodeString(beforeTx.ScriptPubKey)
				if err != nil {
					return fmt.Errorf("decode script: %s", err.Error())
				}
				addr, err := Migration013_ScriptToAddress(beforeRecord.coin, script, testMode)
				if err != nil {
					return fmt.Errorf("script to address: %s", err.Error())
				}
				var migratedRecord = Migration013_TransactionRecord_afterMigration{
					Txid:      beforeTx.Txid,
					Index:     beforeTx.Index,
					Value:     beforeTx.Value,
					Spent:     beforeTx.Spent,
					Timestamp: beforeTx.Timestamp,
					Address:   addr,
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

func Migration013_ChainConfigParams(testnet bool) *chaincfg.Params {
	if testnet {
		return &chaincfg.TestNet3Params
	}
	return &chaincfg.MainNetParams
}

func Migration013_ScriptToAddress(coinType string, script []byte, testmodeEnanabled bool) (string, error) {
	var params = Migration013_ChainConfigParams(testmodeEnanabled)

	switch strings.ToLower(coinType) {
	case "btc", "tbtc":
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(script, params)
		if err != nil {
			return "", fmt.Errorf("converting %s script to address: %s", coinType, err.Error())
		}
		if len(addrs) == 0 {
			return "", fmt.Errorf("unable to convert %s address to script", coinType)
		}
		return addrs[0].String(), nil
	case "bch", "tbch":
		addr, err := bchutil.ExtractPkScriptAddrs(script, params)
		if err != nil {
			return "", fmt.Errorf("converting %s script to address: %s", coinType, err.Error())
		}
		return btcutil.Address(addr).String(), nil
	case "zec", "tzec":
		addr, err := zcashd.ExtractPkScriptAddrs(script, params)
		if err != nil {
			return "", fmt.Errorf("converting %s script to address: %s", coinType, err.Error())
		}
		return addr.String(), nil
	}
	return "", fmt.Errorf("unable to migrate coinType %s", coinType)
}

func migration013_DecodeBCHAddress(addr string, params *chaincfg.Params) (*btcutil.Address, error) {
	// Legacy
	decoded, err := btcutil.DecodeAddress(addr, params)
	if err == nil {
		return &decoded, nil
	}
	// Cashaddr
	decoded, err = bchutil.DecodeAddress(addr, params)
	if err == nil {
		return &decoded, nil
	}
	// Bitpay
	decoded, err = bchutil.DecodeBitpay(addr, params)
	if err == nil {
		return &decoded, nil
	}
	return nil, fmt.Errorf("unable to decode BCH address")
}

func Migration013_AddressToScript(coinType string, addr string, testmodeEnanabled bool) ([]byte, error) {
	var params = Migration013_ChainConfigParams(testmodeEnanabled)

	switch strings.ToLower(coinType) {
	case "btc", "tbtc":
		addr, err := btcutil.DecodeAddress(addr, params)
		if err != nil {
			return nil, fmt.Errorf("decoding %s address: %s", coinType, err.Error())
		}
		script, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return nil, fmt.Errorf("converting %s address to script: %s", coinType, err.Error())
		}
		return script, nil
	case "bch", "tbch":
		addr, err := migration013_DecodeBCHAddress(addr, params)
		if err != nil {
			return nil, fmt.Errorf("decoding %s address: %s", coinType, err.Error())
		}
		script, err := bchutil.PayToAddrScript(*addr)
		if err != nil {
			return nil, fmt.Errorf("converting %s address to script: %s", coinType, err.Error())
		}
		return script, nil
	case "zec", "tzec":
		addr, err := zcashd.DecodeAddress(addr, params)
		if err != nil {
			return nil, fmt.Errorf("decoding %s address: %s", coinType, err.Error())
		}
		script, err := zcashd.PayToAddrScript(addr)
		if err != nil {
			return nil, fmt.Errorf("converting %s address to script: %s", coinType, err.Error())
		}
		return script, nil
	}
	return nil, fmt.Errorf("Unable to migrate coinType %s", coinType)
}
