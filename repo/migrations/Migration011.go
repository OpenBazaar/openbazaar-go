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

type Migration011 struct{}

type Migration011_TransactionRecord_beforeMigration struct {
	Txid         string
	Index        uint32
	Value        int64
	ScriptPubKey string
	Spent        bool
	Timestamp    time.Time
}

type Migration011_TransactionRecord_afterMigration struct {
	Txid      string
	Index     uint32
	Value     int64
	Address   string
	Spent     bool
	Timestamp time.Time
}

type migration011_record struct {
	orderID      string
	coin         string
	transactions []Migration011_TransactionRecord_beforeMigration
}

func (Migration011) Up(repoPath string, dbPassword string, testnet bool) (err error) {
	db, err := OpenDB(repoPath, dbPassword, testnet)
	if err != nil {
		return fmt.Errorf("opening db: %s", err.Error())
	}
	saleMigrationRecords, err := migration011_extractRecords(db, "select orderID, transactions, paymentCoin from sales;")
	if err != nil {
		return fmt.Errorf("get sales rows: %s", err.Error())
	}
	purchaseMigrationRecords, err := migration011_extractRecords(db, "select orderID, transactions, paymentCoin from purchases;")
	if err != nil {
		return fmt.Errorf("get purchase rows: %s", err.Error())
	}

	withTransaction(db, func(tx *sql.Tx) error {
		err := migration011_updateRecords(tx, saleMigrationRecords, "update sales set transactions = ? where orderID = ?", testnet)
		if err != nil {
			return fmt.Errorf("update sales: %s", err.Error())
		}
		err = migration011_updateRecords(tx, purchaseMigrationRecords, "update purchases set transactions = ? where orderID = ?", testnet)
		if err != nil {
			return fmt.Errorf("update purchases: %s", err.Error())
		}
		return nil
	})

	return writeRepoVer(repoPath, 12)
}

func (Migration011) Down(repoPath string, dbPassword string, testnet bool) (err error) { return nil }

func migration011_extractRecords(db *sql.DB, query string) ([]migration011_record, error) {
	var (
		results   = make([]migration011_record, 0)
		rows, err = db.Query(query)
	)
	if err != nil {
		return nil, fmt.Errorf("selecting rows: %s", err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var (
			serializedTransactions []byte
			r                      = migration011_record{
				transactions: make([]Migration011_TransactionRecord_beforeMigration, 0),
			}
		)
		if err := rows.Scan(&r.orderID, &serializedTransactions, &r.coin); err != nil {
			return nil, fmt.Errorf("scanning rows: %s", err.Error())
		}
		if err := json.Unmarshal(serializedTransactions, &r.transactions); err != nil {
			return nil, fmt.Errorf("unmarshal transactions: %s", err.Error())
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %s", err.Error())
	}
	return results, nil
}

func migration011_updateRecords(tx *sql.Tx, records []migration011_record, query string, testMode bool) error {
	var update, err = tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("prepare update statement: %s", err.Error())
	}
	defer update.Close()
	for _, beforeRecord := range records {
		var migratedTransactionRecords = make([]Migration011_TransactionRecord_afterMigration, 0)
		for _, beforeTx := range beforeRecord.transactions {
			var migratedRecord = Migration011_TransactionRecord_afterMigration{
				Txid:      beforeTx.Txid,
				Index:     beforeTx.Index,
				Value:     beforeTx.Value,
				Spent:     beforeTx.Spent,
				Timestamp: beforeTx.Timestamp,
			}
			script, err := hex.DecodeString(beforeTx.ScriptPubKey)
			if err != nil {
				return fmt.Errorf("decode script: %s", err.Error())
			}
			addr, err := Migration011_ScriptToAddress(beforeRecord.coin, script, testMode)
			if err != nil {
				return fmt.Errorf("script to address: %s", err.Error())
			}
			migratedRecord.Address = addr
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
	return nil
}

func Migration011_ChainConfigParams(testnet bool) *chaincfg.Params {
	if testnet {
		return &chaincfg.TestNet3Params
	}
	return &chaincfg.MainNetParams
}

func Migration011_ScriptToAddress(coinType string, script []byte, testmodeEnanabled bool) (string, error) {
	var params = Migration011_ChainConfigParams(testmodeEnanabled)

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

func migration011_DecodeBCHAddress(addr string, params *chaincfg.Params) (*btcutil.Address, error) {
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

func Migration011_AddressToScript(coinType string, addr string, testmodeEnanabled bool) ([]byte, error) {
	var params = Migration011_ChainConfigParams(testmodeEnanabled)

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
		addr, err := migration011_DecodeBCHAddress(addr, params)
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
