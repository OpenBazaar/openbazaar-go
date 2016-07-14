package db

import (
	"database/sql"
	"encoding/hex"
	"strconv"
	"sync"
	"errors"
	b32 "github.com/tyler-smith/go-bip32"
	"github.com/OpenBazaar/spvwallet"
	"fmt"
)

type KeysDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (k *KeysDB) Put(key *b32.Key, scriptPubKey []byte, purpose spvwallet.KeyPurpose) error {
	k.lock.Lock()
	defer k.lock.Unlock()
	tx, err := k.db.Begin()
	if err != nil {
		log.Error(err)
	}
	stmt, _ := tx.Prepare("insert into keys(key, scriptPubKey, purpose, used) values(?,?,?,?)")
	defer stmt.Close()
	_, err = stmt.Exec(key.String(), hex.EncodeToString(scriptPubKey), int(purpose), 0)
	if err != nil {
		log.Error("Error executing statement.")
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (k *KeysDB) MarkKeyAsUsed(key *b32.Key) error {
	k.lock.Lock()
	defer k.lock.Unlock()
	tx, err := k.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("update keys set used=1 where key=?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(key.String())
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (k *KeysDB) GetLastKey(purpose spvwallet.KeyPurpose) (*b32.Key, bool, error) {
	k.lock.Lock()
	defer k.lock.Unlock()

	stm := "select key, used from keys where purpose=" + strconv.Itoa(int(purpose)) + " order by rowid desc limit 1"
	stmt, err := k.db.Prepare(stm)
	defer stmt.Close()
	var key string
	var usedInt int
	err = stmt.QueryRow().Scan(&key, &usedInt)
	if err != nil {
		return nil, false, nil
	}
	var used bool
	if usedInt == 0 {
		used = false
	} else {
		used = true
	}
	b32key, err := b32.B58Deserialize(key)
	if err != nil {
		return nil, used, err
	}
	return b32key, used, nil
}

func (k *KeysDB) GetKeyForScript(scriptPubKey []byte) (*b32.Key, error) {
	k.lock.Lock()
	defer k.lock.Unlock()

	stmt, err := k.db.Prepare("select key from keys where scriptPubKey=?")
	defer stmt.Close()
	var key string
	err = stmt.QueryRow(hex.EncodeToString(scriptPubKey)).Scan(&key)
	if err != nil {
		return nil, errors.New("Key not found")
	}
	b32key, err := b32.B58Deserialize(key)
	if err != nil {
		return nil, err
	}
	return b32key, nil
}

func (k *KeysDB) GetUnused(purpose spvwallet.KeyPurpose) (*b32.Key, error) {
	k.lock.Lock()
	defer k.lock.Unlock()

	stm := "select key from keys where purpose=" + strconv.Itoa(int(purpose)) + " and used=0 order by rowid asc limit 1"
	stmt, err := k.db.Prepare(stm)
	defer stmt.Close()
	var key string
	err = stmt.QueryRow().Scan(&key)
	if err != nil {
		return nil, nil
	}
	b32key, err := b32.B58Deserialize(key)
	if err != nil {
		return nil, err
	}
	return b32key, nil
}

func (k *KeysDB) GetAll() ([]*b32.Key, error) {
	k.lock.Lock()
	defer k.lock.Unlock()
	var ret []*b32.Key
	stm := "select key from keys"
	rows, err := k.db.Query(stm)
	if err != nil {
		fmt.Println(err)
		return ret, err
	}
	for rows.Next() {
		var serializedKey string
		if err := rows.Scan(&serializedKey); err != nil {
			fmt.Println(err)
		}
		b32key, err := b32.B58Deserialize(serializedKey)
		if err != nil {
			return ret, err
		}
		ret = append(ret, b32key)
	}
	return ret, nil
}

func (k *KeysDB) GetLookaheadWindows() map[spvwallet.KeyPurpose] int {
	k.lock.Lock()
	defer k.lock.Unlock()
	windows := make(map[spvwallet.KeyPurpose] int)
	for i:=0; i<2; i++ {
		stm := "select used from keys where purpose=" + strconv.Itoa(i) +" order by rowid desc"
		rows, err := k.db.Query(stm)
		if err != nil {
			fmt.Println(err)
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
	}
	return windows
}