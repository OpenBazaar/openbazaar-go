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
	tx, err := k.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("insert into keys(key, scriptPubKey, purpose, used) values(?,?,?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	if err != nil {
		return err
	}
	_, err = stmt.Exec(key.String(), hex.EncodeToString(scriptPubKey), int(purpose), 0)
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
	stmt, err := tx.Prepare("update keys set used = 1 where key=?")
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
	b32key := &b32.Key{}
	var used bool
	if usedInt == 0 {
		used = false
	} else {
		used = true
	}
	// FIXME: b32key := b32.Deserialize(key)
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
	b32key := &b32.Key{}
	// FIXME: b32key := b32.Deserialize(key)
	return b32key, nil
}

