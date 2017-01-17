package db

import (
	"database/sql"
	"sync"
	"time"
)

type OfflineMessagesDB struct {
	db   *sql.DB
	lock sync.RWMutex
}

func (o *OfflineMessagesDB) Put(url string) error {
	o.lock.Lock()
	defer o.lock.Unlock()
	tx, err := o.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("insert into offlinemessages(url, timestamp) values(?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(url, int(time.Now().Unix()))
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (o *OfflineMessagesDB) Has(url string) bool {
	o.lock.RLock()
	defer o.lock.RUnlock()
	stmt, err := o.db.Prepare("select url from offlinemessages where url=?")
	if err != nil {
		return false
	}
	defer stmt.Close()
	var ret string
	err = stmt.QueryRow(url).Scan(&ret)
	if err != nil {
		return false
	}
	return true
}
