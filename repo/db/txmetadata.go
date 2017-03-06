package db

import (
	"database/sql"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"sync"
)

type TxMetadataDB struct {
	db   *sql.DB
	lock sync.RWMutex
}

func (t *TxMetadataDB) Put(m repo.Metadata) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	tx, _ := t.db.Begin()
	stmt, err := tx.Prepare("insert or replace into txmetadata(txid, address, memo, orderID, thumbnail) values(?,?,?,?,?)")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(m.Txid, m.Address, m.Memo, m.OrderId, m.Thumbnail)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (t *TxMetadataDB) Get(txid string) (repo.Metadata, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	var m repo.Metadata
	stmt, err := t.db.Prepare("select txid, address, memo, orderID, thumbnail from txmetadata where txid=?")
	defer stmt.Close()
	var id, address, memo, orderId, thumbnail string
	err = stmt.QueryRow(txid).Scan(&id, &address, &memo, &orderId, &thumbnail)
	if err != nil {
		return m, err
	}
	m = repo.Metadata{id, address, memo, orderId, thumbnail}
	return m, nil
}

func (t *TxMetadataDB) GetAll() (map[string]repo.Metadata, error) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	ret := make(map[string]repo.Metadata)
	stm := "select txid, address, memo, orderID, thumbnail from txmetadata"
	rows, err := t.db.Query(stm)
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var txid, address, memo, orderId, thumbnail string
		if err := rows.Scan(&txid, &address, &memo, &orderId, &thumbnail); err != nil {
			return ret, err
		}
		m := repo.Metadata{
			Txid:      txid,
			Address:   address,
			Memo:      memo,
			OrderId:   orderId,
			Thumbnail: thumbnail,
		}
		ret[txid] = m
	}
	return ret, nil
}

func (t *TxMetadataDB) Delete(txid string) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	_, err := t.db.Exec("delete from txmetadata where txid=?", txid)
	if err != nil {
		return err
	}
	return nil
}
