package db

import (
	"bytes"
	"database/sql"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"sync"
)

type TxnsDB struct {
	db   *sql.DB
	lock sync.RWMutex
}

func (t *TxnsDB) Put(txn *wire.MsgTx, value, height int) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	tx, err := t.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("insert or replace into txns(txid, value, height, tx) values(?,?,?,?)")
	defer stmt.Close()
	if err != nil {
		tx.Rollback()
		return err
	}
	var buf bytes.Buffer
	txn.Serialize(&buf)
	_, err = stmt.Exec(txn.TxHash().String(), value, height, buf.Bytes())
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (t *TxnsDB) Get(txid chainhash.Hash) (*wire.MsgTx, uint32, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	stmt, err := t.db.Prepare("select tx, height from txns where txid=?")
	defer stmt.Close()
	var ret []byte
	var height int
	err = stmt.QueryRow(txid.String()).Scan(&ret, &height)
	if err != nil {
		return nil, uint32(0), err
	}
	r := bytes.NewReader(ret)
	msgTx := wire.NewMsgTx(1)
	msgTx.BtcDecode(r, 1)
	return msgTx, uint32(height), nil
}

func (t *TxnsDB) GetAll() ([]spvwallet.Txn, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	var ret []spvwallet.Txn
	stm := "select tx, value, height from txns"
	rows, err := t.db.Query(stm)
	defer rows.Close()
	if err != nil {
		return ret, err
	}
	for rows.Next() {
		var tx []byte
		var value int
		var height int
		if err := rows.Scan(&tx, &value, &height); err != nil {
			continue
		}
		r := bytes.NewReader(tx)
		msgTx := wire.NewMsgTx(1)
		msgTx.BtcDecode(r, 1)

		txn := spvwallet.Txn{msgTx.TxHash().String(), int64(value), uint32(height)}
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
