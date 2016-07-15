package db

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/OpenBazaar/spvwallet"
	"strconv"
	"sync"
)

type KeysDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (k *KeysDB) Put(scriptPubKey []byte, keyPath spvwallet.KeyPath) error {
	k.lock.Lock()
	defer k.lock.Unlock()
	tx, err := k.db.Begin()
	if err != nil {
		return err
	}
	stmt, _ := tx.Prepare("insert into keys(scriptPubKey, purpose, keyIndex, used) values(?,?,?,?)")
	defer stmt.Close()
	_, err = stmt.Exec(hex.EncodeToString(scriptPubKey), int(keyPath.Purpose), keyPath.Index, 0)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (k *KeysDB) MarkKeyAsUsed(scriptPubKey []byte) error {
	k.lock.Lock()
	defer k.lock.Unlock()
	tx, err := k.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("update keys set used=1 where scriptPubKey=?")
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

func (k *KeysDB) GetLastKeyIndex(purpose spvwallet.KeyPurpose) (int, bool, error) {
	k.lock.Lock()
	defer k.lock.Unlock()

	stm := "select keyIndex, used from keys where purpose=" + strconv.Itoa(int(purpose)) + " order by rowid desc limit 1"
	stmt, err := k.db.Prepare(stm)
	defer stmt.Close()
	var index int
	var usedInt int
	err = stmt.QueryRow().Scan(&index, &usedInt)
	if err != nil {
		return 0, false, err
	}
	var used bool
	if usedInt == 0 {
		used = false
	} else {
		used = true
	}
	return index, used, nil
}

func (k *KeysDB) GetPathForScript(scriptPubKey []byte) (spvwallet.KeyPath, error) {
	k.lock.Lock()
	defer k.lock.Unlock()

	stmt, err := k.db.Prepare("select purpose, keyIndex from keys where scriptPubKey=?")
	defer stmt.Close()
	var purpose int
	var index int
	err = stmt.QueryRow(hex.EncodeToString(scriptPubKey)).Scan(&purpose, &index)
	if err != nil {
		return spvwallet.KeyPath{}, errors.New("Key not found")
	}
	p := spvwallet.KeyPath{
		Purpose: spvwallet.KeyPurpose(purpose),
		Index:   index,
	}
	return p, nil
}

func (k *KeysDB) GetUnused(purpose spvwallet.KeyPurpose) (int, error) {
	k.lock.Lock()
	defer k.lock.Unlock()

	stm := "select keyIndex from keys where purpose=" + strconv.Itoa(int(purpose)) + " and used=0 order by rowid asc limit 1"
	stmt, err := k.db.Prepare(stm)
	defer stmt.Close()
	var index int
	err = stmt.QueryRow().Scan(&index)
	if err != nil {
		return 0, err
	}
	return index, nil
}

func (k *KeysDB) GetAll() ([]spvwallet.KeyPath, error) {
	k.lock.Lock()
	defer k.lock.Unlock()
	var ret []spvwallet.KeyPath
	stm := "select purpose, keyIndex from keys"
	rows, err := k.db.Query(stm)
	defer rows.Close()
	if err != nil {
		fmt.Println(err)
		return ret, err
	}
	for rows.Next() {
		var purpose int
		var index int
		if err := rows.Scan(&purpose, &index); err != nil {
			fmt.Println(err)
		}
		p := spvwallet.KeyPath{
			Purpose: spvwallet.KeyPurpose(purpose),
			Index:   index,
		}
		ret = append(ret, p)
	}
	return ret, nil
}

func (k *KeysDB) GetLookaheadWindows() map[spvwallet.KeyPurpose]int {
	k.lock.Lock()
	defer k.lock.Unlock()
	windows := make(map[spvwallet.KeyPurpose]int)
	for i := 0; i < 2; i++ {
		stm := "select used from keys where purpose=" + strconv.Itoa(i) + " order by rowid desc"
		rows, err := k.db.Query(stm)
		if err != nil {
			continue
		}
		var unusedCount int
		for rows.Next() {
			var used int
			if err := rows.Scan(&used); err != nil {
				used = 1
			}
			if used == 0 {
				unusedCount++
			} else {
				break
			}
		}
		purpose := spvwallet.KeyPurpose(i)
		windows[purpose] = unusedCount
		rows.Close()
	}
	return windows
}
