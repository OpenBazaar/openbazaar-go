package db

import (
	"database/sql"
	"sync"
	"time"
)

type OfflineMessagesDB struct {
	db   *sql.DB
	lock *sync.Mutex
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
		return err
	}
	tx.Commit()
	return nil
}

func (o *OfflineMessagesDB) Exists(url string) bool {
	o.lock.Lock()
	defer o.lock.Unlock()
	stm := `select url from offlinemessages where url="` + url + `"`
	rows, err := o.db.Query(stm)
	if err != nil {
		log.Error(err)
		return false
	}
	var ret []string
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			log.Error(err)
		}
		ret = append(ret, url)
	}
	if len(ret) == 0 {
		return false
	} else {
		return true
	}
}
