package db

import (
	"database/sql"
	"sync"
	"strconv"
	"encoding/hex"
	"errors"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	b32 "github.com/tyler-smith/go-bip32"
)

type KeysDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (k *KeysDB) Put(key *b32.Key, scriptPubKey []byte, purpose bitcoin.KeyPurpose) error {
	k.lock.Lock()
	defer k.lock.Unlock()
	tx, _ := k.db.Begin()
	stmt, _ := tx.Prepare("insert into keys(key, scriptPubKey, purpose, used) values(?,?,?,?)")
	defer stmt.Close()
	_, err := stmt.Exec(key.String(), hex.EncodeToString(scriptPubKey), int(purpose), 0)
	if err != nil {
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

func (k *KeysDB) GetLastKey(purpose bitcoin.KeyPurpose) (*b32.Key, bool, error) {
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

func (k *KeysDB) GetAllExternal() ([]*b32.Key, error) {
	k.lock.Lock()
	defer k.lock.Unlock()
	var ret []*b32.Key
	stm := "select key from keys where purpose=0 or purpose=2"
	rows, err := k.db.Query(stm)
	if err != nil {
		log.Error(err)
		return ret, err
	}
	for rows.Next() {
		var serializedKey string
		if err := rows.Scan(&serializedKey); err != nil {
			log.Error(err)
		}
		b32key, err := b32.B58Deserialize(serializedKey)
		if err != nil {
			return ret, err
		}
		ret = append(ret, b32key)
	}
	return ret, nil
}
