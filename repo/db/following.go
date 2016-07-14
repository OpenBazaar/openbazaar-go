package db

import (
	"database/sql"
	"strconv"
	"sync"
)

type FollowingDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (f *FollowingDB) Put(follower string) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	tx, _ := f.db.Begin()
	stmt, _ := tx.Prepare("insert into following(peerID) values(?)")
	defer stmt.Close()
	_, err := stmt.Exec(follower)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (f *FollowingDB) Get(offset int, limit int) ([]string, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	stm := "select peerID from following order by rowid desc limit " + strconv.Itoa(limit) + " offset " + strconv.Itoa(offset)
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

func (f *FollowingDB) Delete(follower string) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.db.Exec("delete from following where peerID=?", follower)
	return nil
}

func (f *FollowingDB) Count() int {
	f.lock.Lock()
	defer f.lock.Unlock()
	row := f.db.QueryRow("select Count(*) from following")
	var count int
	row.Scan(&count)
	return count
}
