package db_test

import (
	"sync"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func buildNewChatStore() (repo.ChatStore, func(), error) {
	appSchema := schema.MustNewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err := appSchema.BuildSchemaDirectories(); err != nil {
		return nil, nil, err
	}
	if err := appSchema.InitializeDatabase(); err != nil {
		return nil, nil, err
	}
	database, err := appSchema.OpenDatabase()
	if err != nil {
		return nil, nil, err
	}
	return db.NewChatStore(database, new(sync.Mutex)), appSchema.DestroySchemaDirectories, nil
}

func TestChatDB_Put(t *testing.T) {
	var chdb, teardown, err = buildNewChatStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = chdb.Put("12345", "abc", "", "mess", time.Now(), true, true)
	if err != nil {
		t.Error(err)
	}
	stmt, err := chdb.PrepareQuery("select messageID, peerID, subject, message, read, timestamp, outgoing from chat where peerID=?")
	if err != nil {
		t.Error(err)
	}
	defer stmt.Close()
	var msgId string
	var peerId string
	var subject string
	var message string
	var read int
	var timestamp int
	var outgoing int
	err = stmt.QueryRow("abc").Scan(&msgId, &peerId, &subject, &message, &read, &timestamp, &outgoing)
	if err != nil {
		t.Error(err)
	}
	if msgId != "12345" {
		t.Errorf(`Expected "abc" got %s`, peerId)
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
	var chdb, teardown, err = buildNewChatStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = chdb.Put("11111", "abc", "", "mess", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	err = chdb.Put("22222", "xyz", "", "mess", time.Now(), false, false)
	if err != nil {
		t.Error(err)
	}
	time.Sleep(time.Second)
	err = chdb.Put("33333", "xyz", "", "mess2", time.Now(), false, false)
	if err != nil {
		t.Error(err)
	}
	convos := chdb.GetConversations()
	if len(convos) != 2 {
		t.Error("Returned incorrect number of conversations")
	}
	if convos[0].PeerId == "abc" {
		if convos[0].Last != "mess" {
			t.Error("Returned incorrect last message")
		}
	}
	if convos[1].PeerId == "xyz" {
		if convos[1].Unread != 2 {
			t.Error("Returned incorrect unread count")
		}
		if convos[1].Last != "mess2" {
			t.Error("Returned incorrect last message")
		}
	}
}

func TestChatDB_GetMessages(t *testing.T) {
	var chdb, teardown, err = buildNewChatStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = chdb.Put("11111", "abc", "", "mess", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	time.Sleep(time.Second * 1)
	err = chdb.Put("22222", "abc", "", "mess2", time.Now(), true, true)
	if err != nil {
		t.Error(err)
	}
	time.Sleep(time.Second * 1)
	err = chdb.Put("33333", "xyz", "", "mess1", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	err = chdb.Put("4444", "xyz", "sub", "mess1", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	messages := chdb.GetMessages("abc", "", "", -1)
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

	limtedMessages := chdb.GetMessages("abc", "", "", 2)
	if len(limtedMessages) != 2 {
		t.Error("Returned incorrect number of messages")
		return
	}

	offsetMessages := chdb.GetMessages("abc", "", messages[0].MessageId, -1)
	if len(offsetMessages) != 1 {
		t.Error("Returned incorrect number of messages")
		return
	}
	messages = chdb.GetMessages("xyz", "sub", "", -1)
	if len(messages) != 1 {
		t.Error("Returned incorrect number of messages")
		return
	}
}

func TestChatDB_MarkAsRead_ReturnsHighestReadID(t *testing.T) {
	var chdb, teardown, err = buildNewChatStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = chdb.Put("33333", "xyz", "", "mess", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	err = chdb.Put("44444", "xyz", "", "mess", time.Now().Add(time.Second), false, true)
	if err != nil {
		t.Error(err)
	}
	err = chdb.Put("55555", "xyz", "", "mess", time.Now().Add(time.Second*2), false, true)
	if err != nil {
		t.Error(err)
	}

	last, _, err := chdb.MarkAsRead("xyz", "", true, "")
	if err != nil {
		t.Error(err)
	}
	if last != "55555" {
		t.Fatal("Expected last ID to be 55555, but was not")
	}
}

func TestChatDB_MarkAsRead(t *testing.T) {
	var chdb, teardown, err = buildNewChatStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = chdb.Put("11111", "abc", "", "mess", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	err = chdb.Put("22222", "abc", "", "mess", time.Now().Add(time.Second), false, false)
	if err != nil {
		t.Error(err)
	}
	err = chdb.Put("33333", "xyz", "", "mess", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	err = chdb.Put("44444", "xyz", "", "mess", time.Now().Add(time.Second), false, true)
	if err != nil {
		t.Error(err)
	}
	err = chdb.Put("55555", "xyz", "", "mess", time.Now().Add(time.Second*2), false, true)
	if err != nil {
		t.Error(err)
	}
	messages := chdb.GetMessages("abc", "", "", -1)
	if len(messages) == 0 {
		t.Error("Returned incorrect number of messages")
		return
	}
	last, updated, err := chdb.MarkAsRead("abc", "", true, "")
	if err != nil {
		t.Error(err)
	}
	if !updated {
		t.Error("Updated bool returned incorrectly")
	}
	stmt, err := chdb.PrepareQuery("select read from chat where messageID=?")
	if err != nil {
		t.Error(err)
	}
	defer stmt.Close()
	var read int
	err = stmt.QueryRow("11111").Scan(&read)
	if err != nil {
		t.Error(err)
	}
	if read != 1 {
		t.Error("Failed to mark message as read")
	}
	if last != "11111" {
		t.Error("Returned incorrect last message Id")
	}
	stmt2, err := chdb.PrepareQuery("select read from chat where messageID=?")
	if err != nil {
		t.Error(err)
	}
	defer stmt2.Close()
	err = stmt2.QueryRow("22222").Scan(&read)
	if err != nil {
		t.Error(err)
	}
	if read != 0 {
		t.Error("Failed to mark message as read")
	}
	last, updated, err = chdb.MarkAsRead("abc", "", false, "")
	if err != nil {
		t.Error(err)
	}
	if !updated {
		t.Error("Updated bool returned incorrectly")
	}
	stmt3, err := chdb.PrepareQuery("select read from chat where messageID=?")
	if err != nil {
		t.Error(err)
	}
	defer stmt3.Close()
	err = stmt3.QueryRow("22222").Scan(&read)
	if err != nil {
		t.Error(err)
	}
	if read != 1 {
		t.Error("Failed to mark message as read")
	}
	if last != "22222" {
		t.Error("Returned incorrect last message Id")
	}
	_, updated, err = chdb.MarkAsRead("xyz", "", true, "44444")
	if err != nil {
		t.Error(err)
	}
	if !updated {
		t.Error("Updated bool returned incorrectly")
	}
	stm := `select read, messageID from chat where peerID="xyz"`
	rows, err := chdb.PrepareAndExecuteQuery(stm)
	if err != nil {
		t.Error(err.Error())
	}

	defer rows.Close()
	for rows.Next() {
		var msgID string
		var read int
		rows.Scan(&read, &msgID)
		if msgID == "33333" && read == 0 {
			t.Error("Failed to set message as read")
		}
		if msgID == "44444" && read == 0 {
			t.Error("Failed to set message as read")
		}
		if msgID == "55555" && read == 1 {
			t.Error("Incorrectly set message as read")
		}
	}
}

// https://github.com/OpenBazaar/openbazaar-go/issues/1041
func TestChatDB_MarkAsRead_Issue1041(t *testing.T) {
	var chdb, teardown, err = buildNewChatStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	_, _, err = chdb.MarkAsRead("nonexistantpeerid", "", false, "")
	if err != nil {
		t.Error(err)
	}
}

func TestChatDB_GetUnreadCount(t *testing.T) {
	var chdb, teardown, err = buildNewChatStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = chdb.Put("11111", "abc", "sub", "mess", time.Now(), false, false)
	if err != nil {
		t.Error(err)
	}
	err = chdb.Put("22222", "abc", "sub", "mess", time.Now().Add(time.Second), false, false)
	if err != nil {
		t.Error(err)
	}
	count, err := chdb.GetUnreadCount("sub")
	if err != nil {
		t.Error(err)
	}
	if count != 2 {
		t.Error("GetUnreadCount returned incorrect count")
	}
}

func TestChatDB_DeleteMessage(t *testing.T) {
	var chdb, teardown, err = buildNewChatStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = chdb.Put("11111", "abc", "", "mess", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	messages := chdb.GetMessages("abc", "", "", -1)
	if len(messages) == 0 {
		t.Error("Returned incorrect number of messages")
		return
	}
	err = chdb.DeleteMessage(messages[0].MessageId)
	if err != nil {
		t.Error(err)
	}
	stmt, err := chdb.PrepareQuery("select messageID from chat where messageID=?")
	if err != nil {
		t.Error(err)
	}
	defer stmt.Close()
	var msgId int
	err = stmt.QueryRow(messages[0].MessageId).Scan(&msgId)
	if err == nil {
		t.Error("Delete failed")
	}
}

func TestChatDB_DeleteConversation(t *testing.T) {
	var chdb, teardown, err = buildNewChatStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = chdb.Put("11111", "abc", "", "mess", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	err = chdb.Put("22222", "abc", "", "mess2", time.Now(), false, true)
	if err != nil {
		t.Error(err)
	}
	messages := chdb.GetMessages("abc", "", "", -1)
	if len(messages) != 2 {
		t.Error("Returned incorrect number of messages")
		return
	}
	err = chdb.DeleteConversation("abc")
	if err != nil {
		t.Error(err)
	}
	stmt, err := chdb.PrepareQuery("select messageID from chat where messageID=?")
	if err != nil {
		t.Error(err)
	}
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
