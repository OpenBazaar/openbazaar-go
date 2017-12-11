package db

import (
	"bytes"
	"database/sql"
	"testing"
	"sync"
)

var odb OfflineMessagesDB

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	odb = OfflineMessagesDB{
		db: conn,
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

func TestOfflineMessagesSetMessage(t *testing.T) {
	err := odb.Put("abccc")
	if err != nil {
		t.Error(err)
	}
	err = odb.SetMessage("abccc", []byte("helloworld"))
	if err != nil {
		t.Error(err)
	}
	messages, err := odb.GetMessages()
	if err != nil {
		t.Error(err)
	}
	m, ok := messages["abccc"]
	if !ok || !bytes.Equal(m, []byte("helloworld")) {
		t.Error("Returned incorrect value")
	}

	err = odb.DeleteMessage("abccc")
	if err != nil {
		t.Error(err)
	}
	messages, err = odb.GetMessages()
	if err != nil {
		t.Error(err)
	}
	m, ok = messages["abccc"]
	if ok {
		t.Error("Failed to delete")
	}
}
