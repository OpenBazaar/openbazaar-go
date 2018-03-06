package db

import (
	"database/sql"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"sync"
	"testing"
	"time"
)

var notifDB repo.NotificationStore

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	notifDB = NewNotificationStore(conn, new(sync.Mutex))
}

func TestNotficationsDB_Put(t *testing.T) {
	n := repo.FollowNotification{"1", "follow", "abc"}
	err := notifDB.Put(n.ID, n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	stmt, err := notifDB.PrepareQuery("select * from notifications")
	defer stmt.Close()
	var notifID string
	var data []byte
	var timestamp int
	var notifType string
	var read int
	err = stmt.QueryRow().Scan(&notifID, &data, &notifType, &timestamp, &read)
	if err != nil {
		t.Error(err)
	}
	if notifID != "1" {
		t.Error("Returned incorrect ID")
	}
	if notifType != "follow" {
		t.Error("Returned incorrect type")
	}
	if string(data) != `{"notificationId":"1","type":"follow","peerId":"abc"}` {
		t.Error("Returned incorrect notification")
	}
	if read != 0 {
		t.Error("Returned incorrect read value")
	}
	if timestamp <= 0 {
		t.Error("Returned incorrect timestamp")
	}
	notifDB.Delete("1")
}

func TestNotficationsDB_Delete(t *testing.T) {
	n := repo.FollowNotification{"1", "follow", "abc"}
	err := notifDB.Put(n.ID, n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	err = notifDB.Delete("1")
	if err != nil {
		t.Error(err)
	}
	stmt, err := chdb.PrepareQuery("select notifID from notifications where notifID='1'")
	defer stmt.Close()
	var notifId int
	err = stmt.QueryRow().Scan(&notifId)
	if err == nil {
		t.Error("Delete failed")
	}
}

func TestNotficationsDB_GetAll(t *testing.T) {
	n := repo.FollowNotification{"1", "follow", "abc"}
	err := notifDB.Put(n.ID, n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	n = repo.FollowNotification{"2", "order", "123"}
	err = notifDB.Put(n.ID, n, n.Type, time.Now().Add(time.Second))
	if err != nil {
		t.Error(err)
	}
	n = repo.FollowNotification{"3", "order", "56778"}
	err = notifDB.Put(n.ID, n, n.Type, time.Now().Add(time.Second*2))
	if err != nil {
		t.Error(err)
	}
	notifs, _, err := notifDB.GetAll("", -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(notifs) != 3 {
		t.Error("Returned incorrect number of messages")
		return
	}

	limtedMessages, _, err := notifDB.GetAll("", 2, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(limtedMessages) != 2 {
		t.Error("Returned incorrect number of messages")
		return
	}

	offsetMessages, _, err := notifDB.GetAll("3", -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(offsetMessages) != 2 {
		t.Errorf("Returned incorrect number of messages %d", len(offsetMessages))
		return
	}

	filteredMessages, _, err := notifDB.GetAll("", -1, []string{"order"})
	if err != nil {
		t.Error(err)
	}
	if len(filteredMessages) != 2 {
		t.Errorf("Returned incorrect number of messages %d", len(filteredMessages))
		return
	}
}

func TestNotficationsDB_MarkAsRead(t *testing.T) {
	n := repo.FollowNotification{"5", "follow", "abc"}
	err := notifDB.Put(n.ID, n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	err = notifDB.MarkAsRead("5")
	if err != nil {
		t.Error(err)
	}
	stmt, err := notifDB.PrepareQuery("select read from notifications where notifID='5'")
	defer stmt.Close()
	var read int
	err = stmt.QueryRow().Scan(&read)
	if err != nil {
		t.Error(err)
	}
	if read != 1 {
		t.Error("Failed to mark message as read")
	}
}

func TestNotficationsDB_MarkAllAsRead(t *testing.T) {
	n := repo.FollowNotification{"6", "follow", "abc"}
	err := notifDB.Put(n.ID, n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	n = repo.FollowNotification{"7", "follow", "123"}
	err = notifDB.Put(n.ID, n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	err = notifDB.MarkAllAsRead()
	if err != nil {
		t.Error(err)
	}
	rows, err := notifDB.PrepareAndExecuteQuery("select * from notifications where read=0")
	if err != nil {
		t.Error(err)
	}
	if rows.Next() {
		t.Error("Failed to mark all as read")
	}
}

func TestNotificationDB_GetUnreadCount(t *testing.T) {
	n := repo.FollowNotification{"8", "follow", "abc"}
	err := notifDB.Put(n.ID, n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	err = notifDB.MarkAsRead("8")
	if err != nil {
		t.Error(err)
	}
	n = repo.FollowNotification{"9", "follow", "xyz"}
	err = notifDB.Put(n.ID, n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	all, _, err := notifDB.GetAll("", -1, []string{})
	if err != nil {
		t.Error(err)
	}
	var c int
	for _, a := range all {
		if !a.Read {
			c++
		}
	}
	count, err := notifDB.GetUnreadCount()
	if err != nil {
		t.Error(err)
	}
	if count != c {
		t.Error("GetUnreadCount returned incorrect count")
	}
}
