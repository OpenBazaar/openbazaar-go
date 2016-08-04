package db

import (
	"database/sql"
	"sync"
	"testing"
)

var odb OfflineMessagesDB

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	odb = OfflineMessagesDB{
		db:   conn,
		lock: new(sync.Mutex),
	}
}

func TestOfflineMessagesPut(t *testing.T) {
	err := odb.Put("abc")
	if err != nil {
		t.Error(err)
	}

	stmt, _ := odb.db.Prepare("select url, timestamp from offlinemessages where url=?")
	defer stmt.Close()

	var url string
	var timestamp int
	err = stmt.QueryRow("abc").Scan(&url, &timestamp)
	if err != nil {
		t.Error(err)
	}
	if url != "abc" || timestamp <= 0 {
		t.Error("Offline messages put failed")
	}
}

func TestOfflineMessagesPutDuplicate(t *testing.T) {
	err := odb.Put("123")
	if err != nil {
		t.Error(err)
	}
	err = odb.Put("123")
	if err == nil {
		t.Error("Put offline messages duplicate returned no error")
	}
}

func TestOfflineMessagesHas(t *testing.T) {
	err := odb.Put("abcc")
	if err != nil {
		t.Error(err)
	}
	has := odb.Has("abcc")
	if !has {
		t.Error("Failed to find offline message url in db")
	}
	has = odb.Has("xyz")
	if has {
		t.Error("Offline messages has returned incorrect")
	}

}
