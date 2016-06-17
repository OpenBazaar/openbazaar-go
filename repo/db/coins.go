package db

import (
	"database/sql"
	"sync"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"encoding/hex"
	"strconv"
	"strings"
)

type CoinsDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (c *CoinsDB) Put(utxo bitcoin.Utxo) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("insert into coins(outpoint, value, scriptPubKey) values(?,?,?)")
	if err != nil {
		return err
	}
	outpoint := hex.EncodeToString(utxo.Txid) + ":" + strconv.Itoa(utxo.Index)
	defer stmt.Close()
	_, err = stmt.Exec(
		outpoint,
		utxo.Value,
		hex.EncodeToString(utxo.ScriptPubKey),
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (c *CoinsDB) Delete(txid []byte, index int) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	outpoint := hex.EncodeToString(txid) + ":" + strconv.Itoa(index)
	_, err := c.db.Exec("delete from coins where outpoint=?", outpoint)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (c *CoinsDB) GetAll() []bitcoin.Utxo {
	c.lock.Lock()
	defer c.lock.Unlock()
	var ret []bitcoin.Utxo
	stm := `select * from coins`
	rows, err := c.db.Query(stm)
	if err != nil {
		log.Error(err)
		return ret
	}
	for rows.Next() {
		var outpoint string
		var value int
		var scriptPubKey string
		if err := rows.Scan(&outpoint, &value, &scriptPubKey); err != nil {
			log.Error(err)
			return ret
		}
		s := strings.Split(outpoint, ":")
		txid, err := hex.DecodeString(s[0])
		if err != nil {
			log.Error(err)
			return ret
		}
		index, err := strconv.Atoi(s[1])
		if err != nil {
			log.Error(err)
			return ret
		}
		scriptBytes, err := hex.DecodeString(scriptPubKey)
		if err != nil {
			log.Error(err)
			return ret
		}
		ret = append(ret, bitcoin.Utxo{
			Txid: txid,
			Index: index,
			Value: value,
			ScriptPubKey: scriptBytes,

		})
	}
	return ret
}