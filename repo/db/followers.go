package db

import (
	"database/sql"
	"strconv"
	"sync"
)

type FollowerDB struct {
	db   *sql.DB
	lock *sync.Mutex
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

func (f *FollowerDB) Get(offset int, limit int) ([]string, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	stm := "select peerID from followers order by rowid desc limit " + strconv.Itoa(limit) + " offset " + strconv.Itoa(offset)
	rows, _ := f.db.Query(stm)

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
	f.lock.Lock()
	defer f.lock.Unlock()
	row := f.db.QueryRow("select Count(*) from followers")
	var count int
	row.Scan(&count)
	return count
}
