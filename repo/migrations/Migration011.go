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

func (Migration011) Up(repoPath string, dbPassword string, testnet bool) (err error) {
	db, err := OpenDB(repoPath, dbPassword, testnet)
	if err != nil {
		return fmt.Errorf("opening db: %s", err.Error())
	}

	type migrationRecord struct {
		orderID      string
		coin         string
		transactions []Migration011_TransactionRecord_beforeMigration
	}
	saleRows, err := db.Query("select orderID, transactions, paymentCoin from sales;")
	if err != nil {
		return fmt.Errorf("selecting sales: %s", err.Error())
	}
	var saleMigrationRecords = make([]migrationRecord, 0)
	for saleRows.Next() {
		var (
			serializedTransactions []byte
			r                      = migrationRecord{
				transactions: make([]Migration011_TransactionRecord_beforeMigration, 0),
			}
		)
		if err := saleRows.Scan(&r.orderID, &serializedTransactions, &r.coin); err != nil {
			return fmt.Errorf("scanning sale: %s", err.Error())
		}
		if err := json.Unmarshal(serializedTransactions, &r.transactions); err != nil {
			return fmt.Errorf("unmarshal transactions: %s", err.Error())
		}
		saleMigrationRecords = append(saleMigrationRecords, r)
	}
	if err := saleRows.Err(); err != nil {
		return fmt.Errorf("iterating over sales: %s", err.Error())
	}
	saleRows.Close()

	purchaseRows, err := db.Query("select orderID, transactions, paymentCoin from purchases;")
	if err != nil {
		return fmt.Errorf("selecting purchases: %s", err.Error())
	}
	var purchaseMigrationRecords = make([]migrationRecord, 0)
	for purchaseRows.Next() {
		var (
			serializedTransactions []byte
			r                      = migrationRecord{
				transactions: make([]Migration011_TransactionRecord_beforeMigration, 0),
			}
		)
		if err := purchaseRows.Scan(&r.orderID, &serializedTransactions, &r.coin); err != nil {
			return fmt.Errorf("scanning purchase: %s", err.Error())
		}
		if err := json.Unmarshal(serializedTransactions, &r.transactions); err != nil {
			return fmt.Errorf("unmarshal transactions: %s", err.Error())
		}
		purchaseMigrationRecords = append(purchaseMigrationRecords, r)
	}
	if err := purchaseRows.Err(); err != nil {
		return fmt.Errorf("iterating over purchases: %s", err.Error())
	}
	purchaseRows.Close()

	withTransaction(db, func(tx *sql.Tx) error {
		updateSale, err := tx.Prepare("update sales set transactions = ? where orderID = ?")
		if err != nil {
			return fmt.Errorf("prepare update sale statement: %s", err.Error())
		}
		for _, beforeRecord := range saleMigrationRecords {
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
				addr, err := Migration011_ScriptToAddress(beforeRecord.coin, script, testnet)
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
			if _, err := updateSale.Exec(string(serializedTransactionRecords), beforeRecord.orderID); err != nil {
				return fmt.Errorf("updating sale: %s", err.Error())
			}
		}
		updateSale.Close()

		updatePurchase, err := tx.Prepare("update purchases set transactions = ? where orderID = ?")
		if err != nil {
			return fmt.Errorf("prepare update purchase statement: %s", err.Error())
		}
		for _, beforeRecord := range purchaseMigrationRecords {
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
				addr, err := Migration011_ScriptToAddress(beforeRecord.coin, script, testnet)
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
			if _, err := updatePurchase.Exec(string(serializedTransactionRecords), beforeRecord.orderID); err != nil {
				return fmt.Errorf("updating purchase: %s", err.Error())
			}
		}
		updatePurchase.Close()
		return nil
	})

	return nil
}

func (Migration011) Down(repoPath string, dbPassword string, testnet bool) (err error) { return nil }

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
