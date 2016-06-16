package db

import (
	"database/sql"
	"sync"
	"strconv"
	"encoding/hex"
	"errors"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	b32 "github.com/tyler-smith/go-bip32"
	"fmt"
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
		return err
	}
	tx.Commit()
	return nil
}

func (k *KeysDB) GetLastKey(purpose bitcoin.KeyPurpose) (*b32.Key, bool, error) {
	k.lock.Lock()
	defer k.lock.Unlock()
	stm := "select key, used from keys where purpose=" + strconv.Itoa(int(purpose)) + " order by rowid desc limit 1"
	rows, err := k.db.Query(stm)
	if err != nil {
		log.Error(err)
		return nil, false, err
	}
	var key string
	var usedInt int
	for rows.Next() {
		if err := rows.Scan(&key, &usedInt); err != nil {
			log.Error(err)
		}
		break

	}
	b32key := &b32.Key{}
	var used bool
	if key == "" {
		return nil, false, nil
	} else {
		if usedInt == 0 {
			used = false
		} else {
			used = true
		}
		// FIXME: b32key := b32.Deserialize(key)
	}
	return b32key, used, nil
}

func (k *KeysDB) GetKeyForScript(scriptPubKey []byte) (*b32.Key, error) {
	k.lock.Lock()
	defer k.lock.Unlock()
	stm := `select key from keys where scriptPubKey="` + hex.EncodeToString(scriptPubKey) + `"`
	rows, err := k.db.Query(stm)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	var key string
	for rows.Next() {
		if err := rows.Scan(&key); err != nil {
			log.Error(err)
		}
		break

	}
	fmt.Println(key)
	b32key := &b32.Key{}
	if key == "" {
		return nil, errors.New("Key not found")
	} else {
		// FIXME: b32key := b32.Deserialize(key)
	}
	return b32key, nil
}

