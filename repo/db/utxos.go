package db

import (
	"database/sql"
	"encoding/hex"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"strconv"
	"strings"
	"sync"
)

type UtxoDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (u *UtxoDB) Put(utxo spvwallet.Utxo) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	tx, _ := u.db.Begin()
	stmt, err := tx.Prepare("insert or replace into utxos(outpoint, value, height, scriptPubKey, freeze) values(?,?,?,?,?)")
	defer stmt.Close()
	if err != nil {
		tx.Rollback()
		return err
	}
	freezeInt := 0
	if utxo.Freeze {
		freezeInt = 1
	}
	outpoint := utxo.Op.Hash.String() + ":" + strconv.Itoa(int(utxo.Op.Index))
	_, err = stmt.Exec(outpoint, int(utxo.Value), int(utxo.AtHeight), hex.EncodeToString(utxo.ScriptPubkey), freezeInt)
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
	stm := "select outpoint, value, height, scriptPubKey, freeze from utxos"
	rows, err := u.db.Query(stm)
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var outpoint string
		var value int
		var height int
		var scriptPubKey string
		var freezeInt int
		if err := rows.Scan(&outpoint, &value, &height, &scriptPubKey, &freezeInt); err != nil {
			continue
		}
		s := strings.Split(outpoint, ":")
		shaHash, err := chainhash.NewHashFromStr(s[0])
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
		freeze := false
		if freezeInt == 1 {
			freeze = true
		}
		ret = append(ret, spvwallet.Utxo{
			Op:           *wire.NewOutPoint(shaHash, uint32(index)),
			AtHeight:     int32(height),
			Value:        int64(value),
			ScriptPubkey: scriptBytes,
			Freeze:       freeze,
		})
	}
	return ret, nil
}

func (u *UtxoDB) Freeze(utxo spvwallet.Utxo) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	outpoint := utxo.Op.Hash.String() + ":" + strconv.Itoa(int(utxo.Op.Index))
	_, err := u.db.Exec("update utxos set freeze=? where outpoint=?", 1, outpoint)
	if err != nil {
		return err
	}
	return nil
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
