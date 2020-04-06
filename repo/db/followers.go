package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"sync"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

type FollowerDB struct {
	modelStore
}

func NewFollowerStore(db *sql.DB, lock *sync.Mutex) repo.FollowerStore {
	return &FollowerDB{modelStore{db, lock}}
}

func (f *FollowerDB) Put(follower string, proof []byte) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	stmt, err := f.PrepareQuery("insert into followers(peerID, proof) values(?,?)")
	if err != nil {
		return fmt.Errorf("prepare followers sql: %s", err.Error())
	}

	defer stmt.Close()
	_, err = stmt.Exec(follower, proof)
	if err != nil {
		return fmt.Errorf("update followers: %s", err.Error())
	}
	return nil
}

func (f *FollowerDB) Get(offsetId string, limit int) ([]repo.Follower, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	var stm string
	if offsetId != "" {
		stm = "select peerID, proof from followers order by rowid desc limit " + strconv.Itoa(limit) + " offset ((select coalesce(max(rowid)+1, 0) from followers)-(select rowid from followers where peerID='" + offsetId + "'))"
	} else {
		stm = "select peerID, proof from followers order by rowid desc limit " + strconv.Itoa(limit) + " offset 0"
	}
	var ret []repo.Follower
	rows, err := f.db.Query(stm)
	if err != nil {
		return ret, err
	}
	defer rows.Close()

	for rows.Next() {
		var peerID string
		var proof []byte
		err = rows.Scan(&peerID, &proof)
		if err != nil {
			log.Error(err)
		}
		ret = append(ret, repo.Follower{PeerId: peerID, Proof: proof})
	}
	return ret, nil
}

func (f *FollowerDB) Delete(follower string) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	_, err := f.db.Exec("delete from followers where peerID=?", follower)
	if err != nil {
		return err
	}
	return nil
}

func (f *FollowerDB) Count() int {
	f.lock.Lock()
	defer f.lock.Unlock()
	row := f.db.QueryRow("select Count(*) from followers")
	var count int
	err := row.Scan(&count)
	if err != nil {
		log.Error(err)
	}
	return count
}

func (f *FollowerDB) FollowsMe(peerId string) bool {
	f.lock.Lock()
	defer f.lock.Unlock()
	stmt, err := f.db.Prepare("select peerID from followers where peerID=?")
	if err != nil {
		return false
	}
	defer stmt.Close()
	var follower string
	err = stmt.QueryRow(peerId).Scan(&follower)
	return err == nil
}
