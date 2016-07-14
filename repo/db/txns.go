package db

import (
	"database/sql"
	"sync"
	"github.com/btcsuite/btcd/wire"
	"bytes"
)

type TxnsDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (t *TxnsDB) Put(txn *wire.MsgTx) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	tx, err := t.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("insert into txns(txid, tx) values(?,?)")
	defer stmt.Close()
	if err != nil {
		tx.Rollback()
		return err
	}
	var buf bytes.Buffer
	txn.Serialize(&buf)
	_, err = stmt.Exec(txn.TxSha().String(), buf.Bytes())
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (t *TxnsDB) Get(txid wire.ShaHash) (*wire.MsgTx, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	stmt, err := t.db.Prepare("select tx from txns where txid=?")
	defer stmt.Close()
	var ret []byte
	err = stmt.QueryRow(txid.String()).Scan(&ret)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(ret)
	msgTx := wire.NewMsgTx()
	msgTx.BtcDecode(r, 1)
	return msgTx, nil
}

func (t *TxnsDB) GetAll() ([]*wire.MsgTx, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	var ret []*wire.MsgTx
	stm := "select tx from txns"
	rows, err := t.db.Query(stm)
	defer rows.Close()
	if err != nil {
		return ret, err
	}
	for rows.Next() {
		var tx []byte
		if err := rows.Scan(&tx); err != nil {
			continue
		}
		r := bytes.NewReader(tx)
		msgTx := wire.NewMsgTx()
		msgTx.BtcDecode(r, 1)
		ret = append(ret, msgTx)
	}
	return ret, nil
}

func (t *TxnsDB) Delete(txid *wire.ShaHash) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	_, err := t.db.Exec("delete from txns where txid=?", txid.String())
	if err != nil {
		return err
	}
	return nil
}
