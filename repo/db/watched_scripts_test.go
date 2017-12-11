package db

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"testing"
	"sync"
)

var wsdb WatchedScriptsDB

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	wsdb = WatchedScriptsDB{
		db: conn,
		lock: new(sync.Mutex),
	}
}

func TestWatchedScriptsDB_Put(t *testing.T) {
	err := wsdb.Put([]byte("test"))
	if err != nil {
		t.Error(err)
	}
	stmt, _ := wsdb.db.Prepare("select * from watchedscripts")
	defer stmt.Close()

	var out string
	err = stmt.QueryRow().Scan(&out)
	if err != nil {
		t.Error(err)
	}
	if hex.EncodeToString([]byte("test")) != out {
		t.Error("Failed to inserted watched script into db")
	}
}

func TestWatchedScriptsDB_GetAll(t *testing.T) {
	err := wsdb.Put([]byte("test"))
	if err != nil {
		t.Error(err)
	}
	err = wsdb.Put([]byte("test2"))
	if err != nil {
		t.Error(err)
	}
	scripts, err := wsdb.GetAll()
	if err != nil {
		t.Error(err)
	}
	if len(scripts) != 2 {
		t.Error("Returned incorrect number of watched scripts")
	}
	if !bytes.Equal(scripts[0], []byte("test")) {
		t.Error("Returned incorrect watched script")
	}
	if !bytes.Equal(scripts[1], []byte("test2")) {
		t.Error("Returned incorrect watched script")
	}
}

func TestWatchedScriptsDB_Delete(t *testing.T) {
	err := wsdb.Put([]byte("test"))
	if err != nil {
		t.Error(err)
	}
	err = wsdb.Delete([]byte("test"))
	if err != nil {
		t.Error(err)
	}
	scripts, err := wsdb.GetAll()
	if err != nil {
		t.Error(err)
	}
	for _, script := range scripts {
		if bytes.Equal(script, []byte("test")) {
			t.Error("Failed to delete watched script")
		}
	}
}
