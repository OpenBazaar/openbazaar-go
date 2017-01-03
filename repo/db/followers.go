package db

import (
	"database/sql"
	"strconv"
	"sync"
)

type FollowerDB struct {
	db   *sql.DB
	lock sync.RWMutex
}

func (f *FollowerDB) Put(follower string) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	tx, _ := f.db.Begin()
	stmt, _ := tx.Prepare("insert into followers(peerID) values(?)")

	defer stmt.Close()
	_, err := stmt.Exec(follower)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (f *FollowerDB) Get(offsetId string, limit int) ([]string, error) {
	f.lock.RLock()
	defer f.lock.RUnlock()
	var stm string
	if offsetId != "" {
		stm = "select peerID from followers order by rowid desc limit " + strconv.Itoa(limit) + " offset ((select coalesce(max(rowid)+1, 0) from followers)-(select rowid from followers where peerID='" + offsetId + "'))"
	} else {
		stm = "select peerID from followers order by rowid desc limit " + strconv.Itoa(limit) + " offset 0"
	}
	rows, _ := f.db.Query(stm)
	defer rows.Close()
	var ret []string
	for rows.Next() {
		var peerID string
		rows.Scan(&peerID)
		ret = append(ret, peerID)
	}
	return ret, nil
}

func (f *FollowerDB) Delete(follower string) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.db.Exec("delete from followers where peerID=?", follower)
	return nil
}

func (f *FollowerDB) Count() int {
	f.lock.RLock()
	defer f.lock.RUnlock()
	row := f.db.QueryRow("select Count(*) from followers")
	var count int
	row.Scan(&count)
	return count
}

func (f *FollowerDB) FollowsMe(peerId string) bool {
	f.lock.RLock()
	defer f.lock.RUnlock()
	stmt, err := f.db.Prepare("select peerID from followers where peerID=?")
	defer stmt.Close()
	var follower string
	err = stmt.QueryRow(peerId).Scan(&follower)
	if err != nil {
		return false
	}
	return true
}
