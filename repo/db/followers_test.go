package db

import (
	"sync"
	"database/sql"
	"testing"
	"strconv"
)

var fdb FollowerDB

func init(){
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	fdb = FollowerDB{
		db: conn,
		lock: new(sync.Mutex),
	}
}

func TestPutFollower(t *testing.T) {
	err := fdb.Put("abc")
	if err != nil {
		t.Error(err)
	}
	stmt, err := fdb.db.Prepare("select peerID from followers where peerID=?")
	defer stmt.Close()
	var follower string
	err = stmt.QueryRow("abc").Scan(&follower)
	if err != nil {
		t.Error(err)
	}
	if follower != "abc" {
		t.Errorf(`Expected "abc" got %s`, follower)
	}
}

func TestPutDuplicateFollower(t *testing.T) {
	fdb.Put("abc")
	err := fdb.Put("abc")
	if err == nil {
		t.Error("Expected unquire constriant error to be thrown")
	}
}

func TestCountFollowers(t *testing.T){
	fdb.Put("abc")
	fdb.Put("123")
	fdb.Put("xyz")
	x := fdb.Count()
	if x != 3 {
		t.Errorf("Expected 3 got %d", x)
	}
	fdb.Delete("abc")
	fdb.Delete("123")
	fdb.Delete("xyz")
}

func TestDeleteFollower(t *testing.T){
	fdb.Put("abc")
	err := fdb.Delete("abc")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := fdb.db.Prepare("select peerID from followers where peerID=?")
	defer stmt.Close()
	var follower string
	stmt.QueryRow("abc").Scan(&follower)
	if follower != "" {
		t.Error("Failed to delete follower")
	}
}

func TestGetFollowers(t *testing.T){
	for i:=0; i<100; i++ {
		fdb.Put(strconv.Itoa(i))
	}
	followers, err := fdb.Get(0, 100)
	if err != nil {
		t.Error(err)
	}
	for i:=0; i < 100; i++ {
		f, _ := strconv.Atoi(followers[i])
		if f != 99- i {
			t.Errorf("Returned %d expected %d", f, 99 - i)
		}
	}

	followers, err = fdb.Get(30, 100)
	if err != nil {
		t.Error(err)
	}
	for i:=0; i < 70; i++ {
		f, _ := strconv.Atoi(followers[i])
		if f != 69- i {
			t.Errorf("Returned %d expected %d", f, 69 - i)
		}
	}
	if len(followers) != 70 {
		t.Error("Incorrect number of followers returned")
	}

	followers, err = fdb.Get(30, 5)
	if err != nil {
		t.Error(err)
	}
	if len(followers) != 5 {
		t.Error("Incorrect number of followers returned")
	}
	for i:=0; i < 5; i++ {
		f, _ := strconv.Atoi(followers[i])
		if f != 69- i {
			t.Errorf("Returned %d expected %d", f, 69 - i)
		}
	}

}
