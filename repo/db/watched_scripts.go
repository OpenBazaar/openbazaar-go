package db

import (
	"database/sql"
	"encoding/hex"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"sync"
)

type WatchedScriptsDB struct {
	modelStore
}

func NewWatchedScriptStore(db *sql.DB, lock *sync.Mutex) repo.WatchedScriptStore {
	return &WatchedScriptsDB{modelStore{db, lock}}
}

func (w *WatchedScriptsDB) Put(scriptPubKey []byte) error {
	w.lock.Lock()
	defer w.lock.Unlock()
	tx, _ := w.db.Begin()
	stmt, err := tx.Prepare("insert or replace into watchedscripts(scriptPubKey) values(?)")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(hex.EncodeToString(scriptPubKey))
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (w *WatchedScriptsDB) GetAll() ([][]byte, error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	var ret [][]byte
	stm := "select scriptPubKey from watchedscripts"
	rows, err := w.db.Query(stm)
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var scriptHex string
		if err := rows.Scan(&scriptHex); err != nil {
			continue
		}
		scriptPubKey, err := hex.DecodeString(scriptHex)
		if err != nil {
			continue
		}
		ret = append(ret, scriptPubKey)
	}
	return ret, nil
}

func (w *WatchedScriptsDB) Delete(scriptPubKey []byte) error {
	w.lock.Lock()
	defer w.lock.Unlock()
	_, err := w.db.Exec("delete from watchedscripts where scriptPubKey=?", hex.EncodeToString(scriptPubKey))
	if err != nil {
		return err
	}
	return nil
}
