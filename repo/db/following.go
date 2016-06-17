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
	tx, err := f.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("insert into followers(peerID) values(?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(follower)
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
	stm := "select peerID from followers order by rowid desc limit " + strconv.Itoa(limit) + " offset " + strconv.Itoa(offset)
	rows, err := f.db.Query(stm)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	var ret []string
	for rows.Next() {
		var peerID string
		if err := rows.Scan(&peerID); err != nil {
			log.Error(err)
		}
		ret = append(ret, peerID)
	}
	return ret, nil
}

func (f *FollowingDB) Delete(follower string) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	_, err := f.db.Exec("delete from followers where peerID=?", follower)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (f *FollowingDB) Count() int {
	f.lock.Lock()
	defer f.lock.Unlock()
	row := f.db.QueryRow("select Count(*) from followers")
	var count int
	row.Scan(&count)
	return count
}
