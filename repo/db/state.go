package db

import (
	"database/sql"
	"sync"
)

type StateDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (u *StateDB) Put(key, value string) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	tx, _ := u.db.Begin()
	stmt, _ := tx.Prepare("insert or replace into state(key, value) values(?,?)")
	defer stmt.Close()
	_, err := stmt.Exec(key, value)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (u *StateDB) Get(key string) (string, error) {
	u.lock.Lock()
	defer u.lock.Unlock()
	stmt, err := u.db.Prepare("select value from state where key=?")
	defer stmt.Close()
	var ret string
	err = stmt.QueryRow(key).Scan(&ret)
	if err != nil {
		return "", err
	}
	return ret, nil
}

