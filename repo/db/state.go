package db

import (
	"database/sql"
	"sync"
)

type StateDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (s *StateDB) Put(key, value string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	tx, _ := s.db.Begin()
	stmt, err := tx.Prepare("insert or replace into state(key, value) values(?,?)")
	defer stmt.Close()
	if err != nil {
		tx.Rollback()
		return err
	}
	_, err = stmt.Exec(key, value)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (s *StateDB) Get(key string) (string, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	stmt, err := s.db.Prepare("select value from state where key=?")
	defer stmt.Close()
	var ret string
	err = stmt.QueryRow(key).Scan(&ret)
	if err != nil {
		return "", err
	}
	return ret, nil
}
