package db

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
)

type WatchedScriptsDB struct {
	modelStore
	coinType wallet.CoinType
}

func NewWatchedScriptStore(db *sql.DB, lock *sync.Mutex, coinType wallet.CoinType) repo.WatchedScriptStore {
	return &WatchedScriptsDB{modelStore{db, lock}, coinType}
}

func (w *WatchedScriptsDB) PutAll(scriptPubKeys [][]byte) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	tx, err := w.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("insert or replace into watchedscripts(coin, scriptPubKey) values(?,?)")
	if err != nil {
		if rErr := tx.Rollback(); rErr != nil {
			return fmt.Errorf("put AND rollback failed: %s (rollback error: %s)", err.Error(), rErr.Error())
		}
		return err
	}
	defer stmt.Close()

	for _, scriptPubKey := range scriptPubKeys {
		_, err = stmt.Exec(w.coinType.CurrencyCode(), hex.EncodeToString(scriptPubKey))
		if err != nil {
			if rErr := tx.Rollback(); rErr != nil {
				return fmt.Errorf("put AND rollback failed: %s (rollback error: %s)", err.Error(), rErr.Error())
			}
			return err
		}
	}

	return tx.Commit()
}

func (w *WatchedScriptsDB) Put(scriptPubKey []byte) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	tx, err := w.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("insert or replace into watchedscripts(coin, scriptPubKey) values(?,?)")
	if err != nil {
		if rErr := tx.Rollback(); rErr != nil {
			return fmt.Errorf("put AND rollback failed: %s (rollback error: %s)", err.Error(), rErr.Error())
		}
		return err
	}
	defer stmt.Close()

	for _, scriptPubKey := range scriptPubKeys {
		_, err = stmt.Exec(w.coinType.CurrencyCode(), hex.EncodeToString(scriptPubKey))
		if err != nil {
			if rErr := tx.Rollback(); rErr != nil {
				return fmt.Errorf("put AND rollback failed: %s (rollback error: %s)", err.Error(), rErr.Error())
			}
			return err
		}
	}

	return tx.Commit()
}

func (w *WatchedScriptsDB) Put(scriptPubKey []byte) error {
	w.lock.Lock()
	defer w.lock.Unlock()
	stmt, err := w.PrepareQuery("insert or replace into watchedscripts(coin, scriptPubKey) values(?,?)")
	if err != nil {
		return fmt.Errorf("prepare watch script sql: %s", err.Error())
	}
	defer stmt.Close()
	_, err = stmt.Exec(w.coinType.CurrencyCode(), hex.EncodeToString(scriptPubKey))
	if err != nil {
		return fmt.Errorf("commit watch script: %s", err.Error())
	}
	return nil
}

func (w *WatchedScriptsDB) GetAll() ([][]byte, error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	var ret [][]byte
	stm := "select scriptPubKey from watchedscripts where coin=?"
	rows, err := w.db.Query(stm, w.coinType.CurrencyCode())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var scriptHex string
		if err := rows.Scan(&scriptHex); err != nil {
			log.Errorf("scan watch script key: %s", err.Error())
			continue
		}
		scriptPubKey, err := hex.DecodeString(scriptHex)
		if err != nil {
			log.Errorf("decode watch script key: %s", err.Error())
			continue
		}
		ret = append(ret, scriptPubKey)
	}
	return ret, nil
}

func (w *WatchedScriptsDB) Delete(scriptPubKey []byte) error {
	w.lock.Lock()
	defer w.lock.Unlock()
	_, err := w.db.Exec("delete from watchedscripts where scriptPubKey=? and coin=?", hex.EncodeToString(scriptPubKey), w.coinType.CurrencyCode())
	if err != nil {
		return fmt.Errorf("delete watch script key: %s", err.Error())
	}
	return nil
}
