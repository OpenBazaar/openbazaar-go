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

func (u *TxnsDB) Put(txn *wire.MsgTx) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	tx, _ := u.db.Begin()
	stmt, _ := tx.Prepare("insert into txns(txid, tx) values(?,?)")
	defer stmt.Close()

	var buf bytes.Buffer
	txn.Serialize(&buf)
	_, err := stmt.Exec(txn.TxSha().String(), buf.Bytes())
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (u *TxnsDB) Get(txid wire.ShaHash) (*wire.MsgTx, error) {
	u.lock.Lock()
	defer u.lock.Unlock()
	stmt, err := u.db.Prepare("select tx from txns where txid=?")
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

func (u *TxnsDB) GetAll() ([]*wire.MsgTx, error) {
	u.lock.Lock()
	defer u.lock.Unlock()
	var ret []*wire.MsgTx
	stm := "select tx from txns"
	rows, err := u.db.Query(stm)
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

func (u *TxnsDB) Delete(txid *wire.ShaHash) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	_, err := u.db.Exec("delete from txns where txid=?", txid.String())
	if err != nil {
		return err
	}
	return nil
}
