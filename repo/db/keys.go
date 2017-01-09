package db

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/btcec"
	"strconv"
	"sync"
)

type KeysDB struct {
	db   *sql.DB
	lock sync.RWMutex
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

func (k *KeysDB) ImportKey(scriptPubKey []byte, key *btcec.PrivateKey) error {
	k.lock.Lock()
	defer k.lock.Unlock()
	tx, err := k.db.Begin()
	if err != nil {
		return err
	}
	stmt, _ := tx.Prepare("insert into keys(scriptPubKey, purpose, used, key) values(?,?,?,?)")
	defer stmt.Close()
	_, err = stmt.Exec(hex.EncodeToString(scriptPubKey), -1, 0, hex.EncodeToString(key.Serialize()))
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
	stmt, _ := tx.Prepare("update keys set used=1 where scriptPubKey=?")

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
	k.lock.RLock()
	defer k.lock.RUnlock()

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
	k.lock.RLock()
	defer k.lock.RUnlock()

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

func (k *KeysDB) GetKeyForScript(scriptPubKey []byte) (*btcec.PrivateKey, error) {
	k.lock.Lock()
	defer k.lock.Unlock()

	stmt, err := k.db.Prepare("select key from keys where scriptPubKey=? and purpose=-1")
	defer stmt.Close()
	var keyHex string
	err = stmt.QueryRow(hex.EncodeToString(scriptPubKey)).Scan(&keyHex)
	if err != nil {
		return nil, errors.New("Key not found")
	}
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, err
	}
	key, _ := btcec.PrivKeyFromBytes(btcec.S256(), keyBytes)
	return key, nil
}

func (k *KeysDB) GetUnused(purpose spvwallet.KeyPurpose) (int, error) {
	k.lock.RLock()
	defer k.lock.RUnlock()

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
	k.lock.RLock()
	defer k.lock.RUnlock()
	var ret []spvwallet.KeyPath
	stm := "select purpose, keyIndex from keys"
	rows, err := k.db.Query(stm)
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var purpose int
		var index int
		if err := rows.Scan(&purpose, &index); err != nil {
			continue
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
	k.lock.RLock()
	defer k.lock.RUnlock()
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
