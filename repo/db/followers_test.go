package db

import (
	"bytes"
	"database/sql"
	"strconv"
	"testing"
	"sync"
)

var fdb FollowerDB

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	fdb = FollowerDB{
		db: conn,
		lock: new(sync.Mutex),
	}
}

func TestPutFollower(t *testing.T) {
	err := fdb.Put("abc", []byte("proof"))
	if err != nil {
		t.Error(err)
	}
	stmt, err := fdb.db.Prepare("select peerID, proof from followers where peerID=?")
	defer stmt.Close()
	var follower string
	var proof []byte
	err = stmt.QueryRow("abc").Scan(&follower, &proof)
	if err != nil {
		t.Error(err)
	}
	if follower != "abc" {
		t.Errorf(`Expected "abc" got %s`, follower)
	}
	if !bytes.Equal(proof, []byte("proof")) {
		t.Error("Returned incorrect proof")
	}
}

func TestPutDuplicateFollower(t *testing.T) {
	fdb.Put("abc", []byte("proof"))
	err := fdb.Put("abc", []byte("asdf"))
	if err == nil {
		t.Error("Expected unquire constriant error to be thrown")
	}
}

func TestCountFollowers(t *testing.T) {
	fdb.Put("abc", []byte("proof"))
	fdb.Put("123", []byte("proof"))
	fdb.Put("xyz", []byte("proof"))
	x := fdb.Count()
	if x != 3 {
		t.Errorf("Expected 3 got %d", x)
	}
	fdb.Delete("abc")
	fdb.Delete("123")
	fdb.Delete("xyz")
}

func TestDeleteFollower(t *testing.T) {
	fdb.Put("abc", []byte("proof"))
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

func TestGetFollowers(t *testing.T) {
	for i := 0; i < 100; i++ {
		fdb.Put(strconv.Itoa(i), []byte("proof"))
	}
	followers, err := fdb.Get("", 100)
	if err != nil {
		t.Error(err)
	}
	for i := 0; i < 100; i++ {
		f, _ := strconv.Atoi(followers[i].PeerId)
		if f != 99-i {
			t.Errorf("Returned %d expected %d", f, 99-i)
		}
	}

	followers, err = fdb.Get(strconv.Itoa(30), 100)
	if err != nil {
		t.Error(err)
	}
	for i := 0; i < 30; i++ {
		f, _ := strconv.Atoi(followers[i].PeerId)
		if f != 29-i {
			t.Errorf("Returned %d expected %d", f, 29-i)
		}
	}
	if len(followers) != 30 {
		t.Error("Incorrect number of followers returned")
	}

	followers, err = fdb.Get(strconv.Itoa(30), 5)
	if err != nil {
		t.Error(err)
	}
	if len(followers) != 5 {
		t.Error("Incorrect number of followers returned")
	}
	for i := 0; i < 5; i++ {
		f, _ := strconv.Atoi(followers[i].PeerId)
		if f != 29-i {
			t.Errorf("Returned %d expected %d", f, 29-i)
		}
	}
}

func TestFollowsMe(t *testing.T) {
	fdb.Put("abc", []byte("proof"))
	if !fdb.FollowsMe("abc") {
		t.Error("Follows Me failed to return correctly")
	}
}
