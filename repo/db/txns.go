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

func (t *TxnsDB) Put(txn *wire.MsgTx, value, height int, timestamp time.Time) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	tx, err := t.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("insert or replace into txns(txid, value, height, timestamp, tx) values(?,?,?,?,?)")
	defer stmt.Close()
	if err != nil {
		tx.Rollback()
		return err
	}
	var buf bytes.Buffer
	txn.Serialize(&buf)
	_, err = stmt.Exec(txn.TxHash().String(), value, height, int(timestamp.Unix()), buf.Bytes())
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
	stmt, err := t.db.Prepare("select tx, value, height, timestamp from txns where txid=?")
	if err != nil {
		return nil, txn, err
	}
	defer stmt.Close()
	var ret []byte
	var height int
	var timestamp int
	var value int
	err = stmt.QueryRow(txid.String()).Scan(&ret, &value, &height, &timestamp)
	if err != nil {
		return nil, txn, err
	}
	r := bytes.NewReader(ret)
	msgTx := wire.NewMsgTx(1)
	msgTx.BtcDecode(r, 1)
	txn = spvwallet.Txn{
		Txid:      msgTx.TxHash().String(),
		Value:     int64(value),
		Height:    int32(height),
		Timestamp: time.Unix(int64(timestamp), 0),
	}
	return msgTx, txn, nil
}

func (t *TxnsDB) GetAll() ([]spvwallet.Txn, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	var ret []spvwallet.Txn
	stm := "select tx, value, height, timestamp from txns"
	rows, err := t.db.Query(stm)
	defer rows.Close()
	if err != nil {
		return ret, err
	}
	for rows.Next() {
		var tx []byte
		var value int
		var height int
		var timestamp int
		if err := rows.Scan(&tx, &value, &height, &timestamp); err != nil {
			continue
		}
		r := bytes.NewReader(tx)
		msgTx := wire.NewMsgTx(1)
		msgTx.BtcDecode(r, 1)

		txn := spvwallet.Txn{msgTx.TxHash().String(), int64(value), int32(height), time.Unix(int64(timestamp), 0)}
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

func (t *TxnsDB) MarkAsDead(txid chainhash.Hash) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	tx, err := t.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("update txns set height=-1 where txid=?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(txid.String())
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}
