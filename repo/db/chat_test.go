package db

import (
	"database/sql"
	"sync"
	"testing"
	"time"
)

var chdb ChatDB

func init() {
	setupDB()
}

func setupDB() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	chdb = ChatDB{
		db:   conn,
		lock: new(sync.Mutex),
	}
}

func TestChatDB_Put(t *testing.T) {
	err := chdb.Put("abc", "", "mess", time.Now(), true, true)
	if err != nil {
		t.Error(err)
	}
	stmt, err := chdb.db.Prepare("select peerID, subject, message, read, timestamp, outgoing from chat where peerID=?")
	defer stmt.Close()
	var peerId string
	var subject string
	var message string
	var read int
	var timestamp int
	var outgoing int
	err = stmt.QueryRow("abc").Scan(&peerId, &subject, &message, &read, &timestamp, &outgoing)
	if err != nil {
		t.Error(err)
	}
	if peerId != "abc" {
		t.Errorf(`Expected "abc" got %s`, peerId)
	}
	if subject != "" {
		t.Errorf(`Expected "" got %s`, subject)
	}
	if message != "mess" {
		t.Errorf(`Expected "mess" got %s`, message)
	}
	if read != 1 {
		t.Errorf(`Expected 1 got %d`, read)
	}
	if outgoing != 1 {
		t.Errorf(`Expected 1 got %d`, outgoing)
	}
	if timestamp <= 0 {
		t.Error("Returned incorrect timestamp")
	}
}

func TestChatDB_GetConversations(t *testing.T) {
	err := chdb.Put("abc", "", "mess", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	err = chdb.Put("xyz", "", "mess", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	err = chdb.Put("xyz", "", "mess2", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	convos := chdb.GetConversations()
	if len(convos) != 2 {
		t.Error("Returned incorrect number of conversations")
	}
	if convos[0].PeerId == "abc" {
		if convos[0].Unread != 1 {
			t.Error("Returned incorrect unread count")
		}
	}
	if convos[1].PeerId == "xyz" {
		if convos[1].Unread != 2 {
			t.Error("Returned incorrect unread count")
		}
	}
}

func TestChatDB_GetMessages(t *testing.T) {
	setupDB()
	err := chdb.Put("abc", "", "mess", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	time.Sleep(time.Second * 1)
	err = chdb.Put("abc", "", "mess2", time.Now(), true, true)
	if err != nil {
		t.Error(err)
	}
	time.Sleep(time.Second * 1)
	err = chdb.Put("xyz", "", "mess1", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	messages := chdb.GetMessages("abc", "", 0, -1)
	if len(messages) != 2 {
		t.Error("Returned incorrect number of messages")
		return
	}
	if messages[0].Message == "mess" {
		if messages[0].PeerId != "abc" {
			t.Error("Returned incorrect peerID")
		}
		if messages[0].Subject != "" {
			t.Error("Returned incorrect subject")
		}
		if messages[0].Read != false {
			t.Error("Returned incorrect read bool")
		}
		if messages[0].Outgoing != true {
			t.Error("Returned incorrect read bool")
		}
		if messages[0].Timestamp.Second() <= 0 {
			t.Error("Returned incorrect timestamp")
		}
	}
	if messages[1].Message == "mess2" {
		if messages[1].PeerId != "abc" {
			t.Error("Returned incorrect peerID")
		}
		if messages[1].Subject != "" {
			t.Error("Returned incorrect subject")
		}
		if messages[1].Read != true {
			t.Error("Returned incorrect read bool")
		}
		if messages[1].Outgoing != true {
			t.Error("Returned incorrect read bool")
		}
		if messages[1].Timestamp.Second() <= 0 {
			t.Error("Returned incorrect timestamp")
		}
	}

	limtedMessages := chdb.GetMessages("abc", "", 0, 2)
	if len(limtedMessages) != 2 {
		t.Error("Returned incorrect number of messages")
		return
	}

	offsetMessages := chdb.GetMessages("abc", "", messages[0].MessageId, -1)
	if len(offsetMessages) != 1 {
		t.Error("Returned incorrect number of messages")
		return
	}
}

func TestChatDB_MarkAsRead(t *testing.T) {
	setupDB()
	err := chdb.Put("abc", "", "mess", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	messages := chdb.GetMessages("abc", "", 0, -1)
	if len(messages) == 0 {
		t.Error("Returned incorrect number of messages")
		return
	}
	err = chdb.MarkAsRead("abc")
	if err != nil {
		t.Error(err)
	}
	stmt, err := chdb.db.Prepare("select read from chat where rowid=?")
	defer stmt.Close()
	var read int
	err = stmt.QueryRow(messages[0].MessageId).Scan(&read)
	if err != nil {
		t.Error(err)
	}
	if read != 1 {
		t.Error("Failed to mark message as read")
	}
}

func TestChatDB_DeleteMessage(t *testing.T) {
	setupDB()
	err := chdb.Put("abc", "", "mess", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	messages := chdb.GetMessages("abc", "", 0, -1)
	if len(messages) == 0 {
		t.Error("Returned incorrect number of messages")
		return
	}
	err = chdb.DeleteMessage(messages[0].MessageId)
	if err != nil {
		t.Error(err)
	}
	stmt, err := chdb.db.Prepare("select rowid from chat where rowid=?")
	defer stmt.Close()
	var msgId int
	err = stmt.QueryRow(messages[0].MessageId).Scan(&msgId)
	if err == nil {
		t.Error("Delete failed")
	}
}

func TestChatDB_DeleteConversation(t *testing.T) {
	setupDB()
	err := chdb.Put("abc", "", "mess", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	err = chdb.Put("abc", "", "mess2", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	messages := chdb.GetMessages("abc", "", 0, -1)
	if len(messages) != 2 {
		t.Error("Returned incorrect number of messages")
		return
	}
	err = chdb.DeleteConversation("abc")
	if err != nil {
		t.Error(err)
	}
	stmt, err := chdb.db.Prepare("select rowid from chat where rowid=?")
	var msgId int
	err = stmt.QueryRow(messages[0].MessageId).Scan(&msgId)
	if err == nil {
		t.Error("Delete failed")
	}
	err = stmt.QueryRow(messages[1].MessageId).Scan(&msgId)
	if err == nil {
		t.Error("Delete failed")
	}
	stmt.Close()
}
