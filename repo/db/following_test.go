package db

import (
	"database/sql"
	"strconv"
	"testing"
	"sync"
)

var fldb FollowingDB

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	fldb = FollowingDB{
		db: conn,
		lock: new(sync.Mutex),
	}
}

func TestPutFollowing(t *testing.T) {
	err := fldb.Put("abc")
	if err != nil {
		t.Error(err)
	}
	stmt, err := fldb.db.Prepare("select peerID from following where peerID=?")
	defer stmt.Close()
	var following string
	err = stmt.QueryRow("abc").Scan(&following)
	if err != nil {
		t.Error(err)
	}
	if following != "abc" {
		t.Errorf(`Expected "abc" got %s`, following)
	}
}

func TestPutDuplicateFollowing(t *testing.T) {
	fldb.Put("abc")
	err := fldb.Put("abc")
	if err == nil {
		t.Error("Expected unquire constriant error to be thrown")
	}
}

func TestCountFollowing(t *testing.T) {
	fldb.Put("abc")
	fldb.Put("123")
	fldb.Put("xyz")
	x := fldb.Count()
	if x != 3 {
		t.Errorf("Expected 3 got %d", x)
	}
	fldb.Delete("abc")
	fldb.Delete("123")
	fldb.Delete("xyz")
}

func TestDeleteFollowing(t *testing.T) {
	fldb.Put("abc")
	err := fldb.Delete("abc")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := fldb.db.Prepare("select peerID from followers where peerID=?")
	defer stmt.Close()
	var follower string
	stmt.QueryRow("abc").Scan(&follower)
	if follower != "" {
		t.Error("Failed to delete follower")
	}
}

func TestGetFollowing(t *testing.T) {
	for i := 0; i < 100; i++ {
		fldb.Put(strconv.Itoa(i))
	}
	followers, err := fldb.Get("", 100)
	if err != nil {
		t.Error(err)
	}
	for i := 0; i < 100; i++ {
		f, _ := strconv.Atoi(followers[i])
		if f != 99-i {
			t.Errorf("Returned %d expected %d", f, 99-i)
		}
	}

	followers, err = fldb.Get(strconv.Itoa(30), 100)
	if err != nil {
		t.Error(err)
	}
	for i := 0; i < 30; i++ {
		f, _ := strconv.Atoi(followers[i])
		if f != 29-i {
			t.Errorf("Returned %d expected %d", f, 29-i)
		}
	}
	if len(followers) != 30 {
		t.Error("Incorrect number of followers returned")
	}

	followers, err = fldb.Get(strconv.Itoa(30), 5)
	if err != nil {
		t.Error(err)
	}
	if len(followers) != 5 {
		t.Error("Incorrect number of followers returned")
	}
	for i := 0; i < 5; i++ {
		f, _ := strconv.Atoi(followers[i])
		if f != 29-i {
			t.Errorf("Returned %d expected %d", f, 29-i)
		}
	}
}

func TestIFollow(t *testing.T) {
	fldb.Put("abc")
	if !fldb.IsFollowing("abc") {
		t.Error("I follow failed to return correctly")
	}
}
