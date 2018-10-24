package db

import (
	"database/sql"
	"strconv"
	"sync"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

type ModeratedDB struct {
	modelStore
}

func NewModeratedStore(db *sql.DB, lock *sync.Mutex) repo.ModeratedStore {
	return &ModeratedDB{modelStore{db, lock}}
}

func (m *ModeratedDB) Put(peerId string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	tx, _ := m.db.Begin()
	stmt, _ := tx.Prepare("insert into moderatedstores(peerID) values(?)")

	defer stmt.Close()
	_, err := stmt.Exec(peerId)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (m *ModeratedDB) Get(offsetId string, limit int) ([]string, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	var stm string
	if offsetId != "" {
		stm = "select peerID from moderatedstores order by rowid desc limit " + strconv.Itoa(limit) + " offset ((select coalesce(max(rowid)+1, 0) from moderatedstores)-(select rowid from moderatedstores where peerID='" + offsetId + "'))"
	} else {
		stm = "select peerID from moderatedstores order by rowid desc limit " + strconv.Itoa(limit) + " offset 0"
	}
	var ret []string
	rows, err := m.db.Query(stm)
	if err != nil {
		return ret, err
	}
	defer rows.Close()

	for rows.Next() {
		var peerID string
		rows.Scan(&peerID)
		ret = append(ret, peerID)
	}
	return ret, nil
}

func (m *ModeratedDB) Delete(follower string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.db.Exec("delete from moderatedstores where peerID=?", follower)
	return nil
}
