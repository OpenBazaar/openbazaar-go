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
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (o *OfflineMessagesDB) Has(url string) bool {
	o.lock.Lock()
	defer o.lock.Unlock()
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

func (o *OfflineMessagesDB) SetMessage(url string, message []byte) error {
	o.lock.Lock()
	defer o.lock.Unlock()
	_, err := o.db.Exec("update offlinemessages set message=? where url=?", message, url)
	if err != nil {
		return err
	}
	return nil
}

func (o *OfflineMessagesDB) GetMessages() (map[string][]byte, error) {
	o.lock.Lock()
	defer o.lock.Unlock()
	stm := "select url, message from offlinemessages where message is not null"

	ret := make(map[string][]byte)
	rows, err := o.db.Query(stm)
	if err != nil {
		return ret, err
	}
	defer rows.Close()

	for rows.Next() {
		var url string
		var message []byte
		rows.Scan(&url, &message)
		ret[url] = message
	}
	return ret, nil
}

func (o *OfflineMessagesDB) DeleteMessage(url string) error {
	o.lock.Lock()
	defer o.lock.Unlock()
	_, err := o.db.Exec("update offlinemessages set message=null where url=?", url)
	if err != nil {
		return err
	}
	return nil
}
