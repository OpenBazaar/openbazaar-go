package db

import (
	"database/sql"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

type TxnsDB struct {
	modelStore
}

func NewTransactionStore(db *sql.DB, lock *sync.Mutex) repo.TransactionStore {
	return &TxnsDB{modelStore{db, lock}}
}

func (t *TxnsDB) Put(raw []byte, txid string, value, height int, timestamp time.Time, watchOnly bool) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	tx, err := t.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("insert or replace into txns(txid, value, height, timestamp, watchOnly, tx) values(?,?,?,?,?,?)")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	watchOnlyInt := 0
	if watchOnly {
		watchOnlyInt = 1
	}
	_, err = stmt.Exec(txid, value, height, int(timestamp.Unix()), watchOnlyInt, raw)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (t *TxnsDB) Get(txid chainhash.Hash) (wallet.Txn, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	var txn wallet.Txn
	stmt, err := t.db.Prepare("select tx, value, height, timestamp, watchOnly from txns where txid=?")
	if err != nil {
		return txn, err
	}
	defer stmt.Close()
	var raw []byte
	var height int
	var timestamp int
	var value int
	var watchOnlyInt int
	err = stmt.QueryRow(txid.String()).Scan(&raw, &value, &height, &timestamp, &watchOnlyInt)
	if err != nil {
		return txn, err
	}
	watchOnly := false
	if watchOnlyInt > 0 {
		watchOnly = true
	}
	txn = wallet.Txn{
		Txid:      txid.String(),
		Value:     int64(value),
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
	stm := "select tx, txid, value, height, timestamp, watchOnly from txns"
	rows, err := t.db.Query(stm)
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var txid string
		var value int
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

		txn := wallet.Txn{txid, int64(value), int32(height), time.Unix(int64(timestamp), 0), watchOnly, raw}
		ret = append(ret, txn)
	}
	return ret, nil
}

func (t *TxnsDB) Delete(txid *chainhash.Hash) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	_, err := t.db.Exec("delete from txns where txid=?", txid.String())
	if err != nil {
		return err
	}
	return nil
}

func (t *TxnsDB) UpdateHeight(txid chainhash.Hash, height int) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	tx, err := t.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("update txns set height=? where txid=?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(height, txid.String())
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}
