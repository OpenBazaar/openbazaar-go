package db

import (
	"database/sql"
	"sync"
	"strconv"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	b32 "github.com/tyler-smith/go-bip32"
)

type KeysDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (k *KeysDB) Put(key *b32.Key, purpose bitcoin.KeyPurpose) error {
	k.lock.Lock()
	defer k.lock.Unlock()
	tx, err := k.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("insert into keys(key, purpose, used) values(?,?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(key.String(), int(purpose), 0)
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

func (k *KeysDB) GetCurrentKey(purpose bitcoin.KeyPurpose) (*b32.Key, error) {
	k.lock.Lock()
	defer k.lock.Unlock()
	stm := "select key from keys where purpose=" + strconv.Itoa(int(purpose)) + " and used=0"
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
	b32key := &b32.Key{}
	if key == "" {
		return nil, nil
	} else {
		// FIXME: b32key := b32.Deserialize(key)
	}
	return b32key, nil
}