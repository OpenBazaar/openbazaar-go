package db

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

type TxnsDB struct {
	modelStore
	coinType wallet.CoinType
}

func NewTransactionStore(db *sql.DB, lock *sync.Mutex, coinType wallet.CoinType) repo.TransactionStore {
	return &TxnsDB{modelStore{db, lock}, coinType}
}

func (t *TxnsDB) Put(raw []byte, txid, value string, height int, timestamp time.Time, watchOnly bool) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	stmt, err := t.PrepareQuery("insert or replace into txns(coin, txid, value, height, timestamp, watchOnly, tx) values(?,?,?,?,?,?,?)")
	if err != nil {
		return fmt.Errorf("prepare txn sql: %s", err.Error())
	}
	defer stmt.Close()

	watchOnlyInt := 0
	if watchOnly {
		watchOnlyInt = 1
	}
	_, err = stmt.Exec(t.coinType.CurrencyCode(), txid, value, height, int(timestamp.Unix()), watchOnlyInt, raw)
	if err != nil {
		return fmt.Errorf("update txn: %s", err.Error())
	}
	return nil
}

func (t *TxnsDB) Get(txid string) (wallet.Txn, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	var txn wallet.Txn
	stmt, err := t.db.Prepare("select tx, value, height, timestamp, watchOnly from txns where txid=? and coin=?")
	if err != nil {
		return txn, err
	}
	defer stmt.Close()
	var raw []byte
	var height int
	var timestamp int
	var value string
	var watchOnlyInt int
	err = stmt.QueryRow(txid, t.coinType.CurrencyCode()).Scan(&raw, &value, &height, &timestamp, &watchOnlyInt)
	if err != nil {
		return txn, err
	}
	watchOnly := false
	if watchOnlyInt > 0 {
		watchOnly = true
	}
	txn = wallet.Txn{
		Txid:      txid,
		Value:     value,
		Height:    int32(height),
		Timestamp: time.Unix(int64(timestamp), 0),
		WatchOnly: watchOnly,
		Bytes:     raw,
	}
	return txn, nil
}

func (t *TxnsDB) GetAll(includeWatchOnly bool) ([]wallet.Txn, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	var ret []wallet.Txn
	stm := "select tx, txid, value, height, timestamp, watchOnly from txns where coin=?"
	rows, err := t.db.Query(stm, t.coinType.CurrencyCode())
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var txid string
		var value string
		var height int
		var timestamp int
		var watchOnlyInt int
		if err := rows.Scan(&raw, &txid, &value, &height, &timestamp, &watchOnlyInt); err != nil {
			continue
		}

		watchOnly := false
		if watchOnlyInt > 0 {
			if !includeWatchOnly {
				continue
			}
			watchOnly = true
		}

		txn := wallet.Txn{
			Txid:      txid,
			Value:     value,
			Height:    int32(height),
			Timestamp: time.Unix(int64(timestamp), 0),
			WatchOnly: watchOnly,
			Bytes:     raw,
		}

		ret = append(ret, txn)
	}
	return ret, nil
}

func (t *TxnsDB) Delete(txid *chainhash.Hash) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	_, err := t.db.Exec("delete from txns where txid=? and coin=?", txid.String(), t.coinType.CurrencyCode())
	if err != nil {
		return err
	}
	return nil
}

func (t *TxnsDB) UpdateHeight(txid string, height int, timestamp time.Time) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	stmt, err := t.PrepareQuery("update txns set height=?, timestamp=? where txid=? and coin=?")
	if err != nil {
		return fmt.Errorf("prepare txn sql: %s", err.Error())
	}
	defer stmt.Close()
	_, err = stmt.Exec(height, int(timestamp.Unix()), txid, t.coinType.CurrencyCode())
	if err != nil {
		return fmt.Errorf("update txns: %s", err.Error())
	}
	return nil
}
