package db

import (
	"database/sql"
	"sync"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"encoding/hex"
	"time"
)

type TransactionsDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (t *TransactionsDB) Put(txinfo bitcoin.TransactionInfo) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	tx, err := t.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("insert into transactions(txid, tx, height, state, timestamp, value, exchangeRate, exchangeCurrency) values(?,?,?,?,?,?,?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(
		hex.EncodeToString(txinfo.Txid),
		txinfo.Tx,
		txinfo.Height,
		int(txinfo.State),
		int(txinfo.Timestamp.Unix()),
		txinfo.Value,
		txinfo.ExchangeRate,
		txinfo.ExchangCurrency,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (t *TransactionsDB) Has(txid []byte) bool {
	t.lock.Lock()
	defer t.lock.Unlock()
	stmt, err := t.db.Prepare("select txid from transactions where txid=?")
	defer stmt.Close()
	var ret string
	err = stmt.QueryRow(hex.EncodeToString(txid)).Scan(&ret)
	if err != nil {
		return false
	}
	return true
}

func (t *TransactionsDB) GetAll() []bitcoin.TransactionInfo {
	t.lock.Lock()
	defer t.lock.Unlock()
	var ret []bitcoin.TransactionInfo
	stm := `select * from transactions`
	rows, err := t.db.Query(stm)
	if err != nil {
		log.Error(err)
		return ret
	}
	for rows.Next() {
		var txidhex string
		var tx []byte
		var height int
		var state int
		var timestamp int
		var value int
		var exchangeRate float64
		var exchangeCurrency string
		if err := rows.Scan(&txidhex, &tx, &height, &state, &timestamp, &value, &exchangeRate, &exchangeCurrency); err != nil {
			log.Error(err)
			return ret
		}
		txid, err := hex.DecodeString(txidhex)
		if err != nil {
			log.Error(err)
			return ret
		}
		ret = append(ret, bitcoin.TransactionInfo{
			Txid: txid,
			Tx: tx,
			Height: height,
			State: bitcoin.TransactionState(state),
			Timestamp: time.Unix(int64(timestamp), 0),
			Value: value,
			ExchangeRate: exchangeRate,
			ExchangCurrency: exchangeCurrency,

		})
	}
	return ret
}

func (t *TransactionsDB) UpdateState(txid []byte, state bitcoin.TransactionState) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	tx, err := t.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("update transactions set state=? where txid=?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(int(state), hex.EncodeToString(txid))
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (t *TransactionsDB) UpdateHeight(txid []byte, height int) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	tx, err := t.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("update transactions set height=? where txid=?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(height, hex.EncodeToString(txid))
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

