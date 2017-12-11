package db

import (
	"database/sql"
	"strconv"
	"sync"
	"testing"
)

var modDB ModeratedDB

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	modDB = ModeratedDB{
		db:   conn,
		lock: new(sync.Mutex),
	}
}

func TestModeratedDB_Put(t *testing.T) {
	err := modDB.Put("abc")
	if err != nil {
		t.Error(err)
	}
	stmt, err := modDB.db.Prepare("select peerID from moderatedstores where peerID=?")
	defer stmt.Close()
	var peerId string
	err = stmt.QueryRow("abc").Scan(&peerId)
	if err != nil {
		t.Error(err)
	}
	if peerId != "abc" {
		t.Errorf(`Expected "abc" got %s`, peerId)
	}
}

func TestModeratedDB_Put_Duplicate(t *testing.T) {
	modDB.Put("abc")
	err := modDB.Put("abc")
	if err == nil {
		t.Error("Expected unquire constriant error to be thrown")
	}
}

func TestModeratedDB_Delete(t *testing.T) {
	modDB.Put("abc")
	err := modDB.Delete("abc")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := modDB.db.Prepare("select peerID from moderatedstores where peerID=?")
	defer stmt.Close()
	var peerId string
	stmt.QueryRow("abc").Scan(&peerId)
	if peerId != "" {
		t.Error("Failed to delete moderated store")
	}
}

func TestModeratedDB_Get(t *testing.T) {
	for i := 0; i < 100; i++ {
		modDB.Put(strconv.Itoa(i))
	}
	stores, err := modDB.Get("", 100)
	if err != nil {
		t.Error(err)
	}
	for i := 0; i < 100; i++ {
		f, _ := strconv.Atoi(stores[i])
		if f != 99-i {
			t.Errorf("Returned %d expected %d", f, 99-i)
		}
	}

	stores, err = modDB.Get(strconv.Itoa(30), 100)
	if err != nil {
		t.Error(err)
	}
	for i := 0; i < 30; i++ {
		f, _ := strconv.Atoi(stores[i])
		if f != 29-i {
			t.Errorf("Returned %d expected %d", f, 29-i)
		}
	}
	if len(stores) != 30 {
		t.Error("Incorrect number of moderated stores returned")
	}

	stores, err = modDB.Get(strconv.Itoa(30), 5)
	if err != nil {
		t.Error(err)
	}
	if len(stores) != 5 {
		t.Error("Incorrect number of moderated stores returned")
	}
	for i := 0; i < 5; i++ {
		f, _ := strconv.Atoi(stores[i])
		if f != 29-i {
			t.Errorf("Returned %d expected %d", f, 29-i)
		}
	}
}
