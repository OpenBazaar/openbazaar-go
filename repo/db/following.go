package db

import (
	"database/sql"
	"strconv"
	"sync"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

type FollowingDB struct {
	modelStore
}

func NewFollowingStore(db *sql.DB, lock *sync.Mutex) repo.FollowingStore {
	return &FollowingDB{modelStore{db, lock}}
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

func (f *FollowingDB) Get(offsetId string, limit int) ([]string, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	var stm string
	if offsetId != "" {
		stm = "select peerID from following order by rowid desc limit " + strconv.Itoa(limit) + " offset ((select coalesce(max(rowid)+1, 0) from following)-(select rowid from following where peerID='" + offsetId + "'))"
	} else {
		stm = "select peerID from following order by rowid desc limit " + strconv.Itoa(limit) + " offset 0"
	}
	var ret []string
	rows, err := f.db.Query(stm)
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

func (f *FollowingDB) Delete(follower string) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	_, err := f.db.Exec("delete from following where peerID=?", follower)
	if err != nil {
		return err
	}
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

func (f *FollowingDB) IsFollowing(peerId string) bool {
	f.lock.Lock()
	defer f.lock.Unlock()
	stmt, err := f.db.Prepare("select peerID from following where peerID=?")
	if err != nil {
		return false
	}
	defer stmt.Close()
	var follower string
	err = stmt.QueryRow(peerId).Scan(&follower)
	if err != nil {
		return false
	}
	return true
}
