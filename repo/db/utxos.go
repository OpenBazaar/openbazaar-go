package db

import (
	"database/sql"
	"sync"
	"github.com/OpenBazaar/spvwallet"
	"strconv"
	"encoding/hex"
	"strings"
	"github.com/btcsuite/btcd/wire"
)

type UtxoDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (u *UtxoDB) Put(utxo spvwallet.Utxo) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	tx, _ := u.db.Begin()
	stmt, _ := tx.Prepare("insert or replace into utxos(outpoint, value, height, scriptPubKey) values(?,?,?,?)")
	defer stmt.Close()

	outpoint := utxo.Op.Hash.String() + ":" + strconv.Itoa(int(utxo.Op.Index))
	_, err := stmt.Exec(outpoint, int(utxo.Value), int(utxo.AtHeight), hex.EncodeToString(utxo.ScriptPubkey))
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (u *UtxoDB) GetAll() ([]spvwallet.Utxo, error) {
	u.lock.Lock()
	defer u.lock.Unlock()
	var ret []spvwallet.Utxo
	stm := "select outpoint, value, height, scriptPubKey from utxos"
	rows, err := u.db.Query(stm)
	if err != nil {
		return ret, err
	}
	for rows.Next() {
		var outpoint string
		var value int
		var height int
		var scriptPubKey string
		if err := rows.Scan(&outpoint, &value, &height, &scriptPubKey); err != nil {
			continue
		}
		s := strings.Split(outpoint, ":")
		if err != nil {
			continue
		}
		shaHash, err := wire.NewShaHashFromStr(s[0])
		if err != nil {
			continue
		}
		index, err := strconv.Atoi(s[1])
		if err != nil {
			continue
		}
		scriptBytes, err := hex.DecodeString(scriptPubKey)
		if err != nil {
			continue
		}
		ret = append(ret, spvwallet.Utxo{
			Op: *wire.NewOutPoint(shaHash, uint32(index)),
			AtHeight: int32(height),
			Value: int64(value),
			ScriptPubkey: scriptBytes,
		})
	}
	return ret, nil
}

func (u *UtxoDB) Delete(utxo spvwallet.Utxo) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	outpoint := utxo.Op.Hash.String() + ":" + strconv.Itoa(int(utxo.Op.Index))
	_, err := u.db.Exec("delete from utxos where outpoint=?", outpoint)
	if err != nil {
		return err
	}
	return nil
}