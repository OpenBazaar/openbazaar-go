package db

import (
	"database/sql"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

type UtxoDB struct {
	modelStore
	coinType wallet.CoinType
}

func NewUnspentTransactionStore(db *sql.DB, lock *sync.Mutex, coinType wallet.CoinType) repo.UnspentTransactionOutputStore {
	return &UtxoDB{modelStore{db, lock}, coinType}
}

func (u *UtxoDB) Put(utxo wallet.Utxo) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	tx, _ := u.db.Begin()
	stmt, err := tx.Prepare("insert or replace into utxos(coin, outpoint, value, height, scriptPubKey, watchOnly) values(?,?,?,?,?,?)")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	watchOnlyInt := 0
	if utxo.WatchOnly {
		watchOnlyInt = 1
	}
	outpoint := utxo.Op.Hash.String() + ":" + strconv.Itoa(int(utxo.Op.Index))
	_, err = stmt.Exec(u.coinType.CurrencyCode(), outpoint, int(utxo.Value), int(utxo.AtHeight), hex.EncodeToString(utxo.ScriptPubkey), watchOnlyInt)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (u *UtxoDB) GetAll() ([]wallet.Utxo, error) {
	u.lock.Lock()
	defer u.lock.Unlock()
	var ret []wallet.Utxo
	stm := "select outpoint, value, height, scriptPubKey, watchOnly from utxos where coin=?"
	rows, err := u.db.Query(stm, u.coinType.CurrencyCode())
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var outpoint string
		var value int
		var height int
		var scriptPubKey string
		var watchOnlyInt int
		if err := rows.Scan(&outpoint, &value, &height, &scriptPubKey, &watchOnlyInt); err != nil {
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
		watchOnly := false
		if watchOnlyInt == 1 {
			watchOnly = true
		}
		ret = append(ret, wallet.Utxo{
			Op:           *wire.NewOutPoint(shaHash, uint32(index)),
			AtHeight:     int32(height),
			Value:        int64(value),
			ScriptPubkey: scriptBytes,
			WatchOnly:    watchOnly,
		})
	}
	return ret, nil
}

func (u *UtxoDB) SetWatchOnly(utxo wallet.Utxo) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	outpoint := utxo.Op.Hash.String() + ":" + strconv.Itoa(int(utxo.Op.Index))
	_, err := u.db.Exec("update utxos set watchOnly=? where outpoint=? and coin=?", 1, outpoint, u.coinType.CurrencyCode())
	if err != nil {
		return err
	}
	return nil
}

func (u *UtxoDB) Delete(utxo wallet.Utxo) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	outpoint := utxo.Op.Hash.String() + ":" + strconv.Itoa(int(utxo.Op.Index))
	_, err := u.db.Exec("delete from utxos where outpoint=? and coin=?", outpoint, u.coinType.CurrencyCode())
	if err != nil {
		return err
	}
	return nil
}
