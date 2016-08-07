package db

import (
	"database/sql"
	"sync"
	"testing"
)

var stdb StateDB

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	stdb = StateDB{
		db:   conn,
		lock: new(sync.Mutex),
	}
}

func TestStatePut(t *testing.T) {
	err := stdb.Put("hello", "world")
	if err != nil {
		t.Error(err)
	}
	stmt, err := stdb.db.Prepare("select value from state where key=?")
	defer stmt.Close()
	var ret string
	err = stmt.QueryRow("hello").Scan(&ret)
	if err != nil {
		t.Error(err)
	}
	if ret != "world" {
		t.Error("State db put wrong value")
	}
}

func TestStateGet(t *testing.T) {
	err := stdb.Put("hello", "world")
	if err != nil {
		t.Error(err)
	}
	val, err := stdb.Get("hello")
	if err != nil {
		t.Error(err)
	}
	if val != "world" {
		t.Error("State db get returned incorrect value")
	}
}

func TestStateGetInvalid(t *testing.T) {
	val, err := stdb.Get("xyz")
	if err == nil {
		t.Error(err)
	}
	if val != "" {
		t.Error("State db get returned incorrect value")
	}
}
