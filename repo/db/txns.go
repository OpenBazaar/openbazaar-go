package db

import (
	"bytes"
	"database/sql"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"sync"
	"time"
)

type TxnsDB struct {
	db   *sql.DB
	lock sync.RWMutex
}

func (t *TxnsDB) Put(txn *wire.MsgTx, value, height int, timestamp time.Time, watchOnly bool) error {
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
	var buf bytes.Buffer
	txn.Serialize(&buf)
	_, err = stmt.Exec(txn.TxHash().String(), value, height, int(timestamp.Unix()), watchOnlyInt, buf.Bytes())
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (t *TxnsDB) Get(txid chainhash.Hash) (*wire.MsgTx, spvwallet.Txn, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	var txn spvwallet.Txn
	stmt, err := t.db.Prepare("select tx, value, height, timestamp, watchOnly from txns where txid=?")
	if err != nil {
		return nil, txn, err
	}
	defer stmt.Close()
	var ret []byte
	var height int
	var timestamp int
	var value int
	var watchOnlyInt int
	err = stmt.QueryRow(txid.String()).Scan(&ret, &value, &height, &timestamp, &watchOnlyInt)
	if err != nil {
		return nil, txn, err
	}
	watchOnly := false
	if watchOnlyInt > 0 {
		watchOnly = true
	}
	r := bytes.NewReader(ret)
	msgTx := wire.NewMsgTx(1)
	msgTx.BtcDecode(r, 1)
	txn = spvwallet.Txn{
		Txid:      msgTx.TxHash().String(),
		Value:     int64(value),
		Height:    int32(height),
		Timestamp: time.Unix(int64(timestamp), 0),
		WatchOnly: watchOnly,
	}
	return msgTx, txn, nil
}

func (t *TxnsDB) GetAll(includeWatchOnly bool) ([]spvwallet.Txn, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	var ret []spvwallet.Txn
	stm := "select tx, value, height, timestamp, watchOnly from txns"
	rows, err := t.db.Query(stm)
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var tx []byte
		var value int
		var height int
		var timestamp int
		var watchOnlyInt int
		if err := rows.Scan(&tx, &value, &height, &timestamp, &watchOnlyInt); err != nil {
			continue
		}
		r := bytes.NewReader(tx)
		msgTx := wire.NewMsgTx(1)
		msgTx.BtcDecode(r, 1)

		watchOnly := false
		if watchOnlyInt > 0 {
			if !includeWatchOnly {
				continue
			}
			watchOnly = true
		}

		txn := spvwallet.Txn{msgTx.TxHash().String(), int64(value), int32(height), time.Unix(int64(timestamp), 0), watchOnly, tx}
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
