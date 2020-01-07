package db

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

type OfflineMessagesDB struct {
	modelStore
}

func NewOfflineMessageStore(db *sql.DB, lock *sync.Mutex) repo.OfflineMessageStore {
	return &OfflineMessagesDB{modelStore{db, lock}}
}

func (o *OfflineMessagesDB) Put(url string) error {
	o.lock.Lock()
	defer o.lock.Unlock()
	stmt, err := o.PrepareQuery("insert into offlinemessages(url, timestamp) values(?,?)")
	if err != nil {
		return fmt.Errorf("prepare offline message sql: %s", err.Error())
	}
	defer stmt.Close()

	_, err = stmt.Exec(url, int(time.Now().Unix()))
	if err != nil {
		return fmt.Errorf("commit offline message: %s", err.Error())
	}
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
	return err == nil
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
		err = rows.Scan(&url, &message)
		if err != nil {
			log.Error(err)
		}
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
