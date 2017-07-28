package db

import (
	"database/sql"
	notif "github.com/OpenBazaar/openbazaar-go/api/notifications"
	"testing"
	"time"
)

var notifDB NotficationsDB

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	notifDB = NotficationsDB{
		db: conn,
	}
}

func TestNotficationsDB_Put(t *testing.T) {
	n := notif.FollowNotification{"follow", "abc"}
	err := notifDB.Put(n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	stmt, err := notifDB.db.Prepare("select * from notifications")
	defer stmt.Close()
	var data []byte
	var timestamp int
	var notifType string
	var read int
	err = stmt.QueryRow().Scan(&data, &notifType, &timestamp, &read)
	if err != nil {
		t.Error(err)
	}
	if notifType != "follow" {
		t.Error("Returned incorrect type")
	}
	if string(data) != `{"type":"follow","peerId":"abc"}` {
		t.Error("Returned incorrect notification")
	}
	if read != 0 {
		t.Error("Returned incorrect read value")
	}
	if timestamp <= 0 {
		t.Error("Returned incorrect timestamp")
	}
	notifDB.Delete(1)
}

func TestNotficationsDB_Delete(t *testing.T) {
	n := notif.FollowNotification{"follow", "abc"}
	err := notifDB.Put(n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	err = notifDB.Delete(1)
	if err != nil {
		t.Error(err)
	}
	stmt, err := chdb.db.Prepare("select rowid from notifications where rowid=1")
	defer stmt.Close()
	var notifId int
	err = stmt.QueryRow().Scan(&notifId)
	if err == nil {
		t.Error("Delete failed")
	}
}

func TestNotficationsDB_GetAll(t *testing.T) {
	n := notif.FollowNotification{"follow", "abc"}
	err := notifDB.Put(n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	n = notif.FollowNotification{"order", "123"}
	err = notifDB.Put(n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	n = notif.FollowNotification{"order", "56778"}
	err = notifDB.Put(n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	notifs := notifDB.GetAll(0, -1, []string{})
	if len(notifs) != 3 {
		t.Error("Returned incorrect number of messages")
		return
	}

	limtedMessages := notifDB.GetAll(0, 2, []string{})
	if len(limtedMessages) != 2 {
		t.Error("Returned incorrect number of messages")
		return
	}

	offsetMessages := notifDB.GetAll(2, -1, []string{})
	if len(offsetMessages) != 1 {
		t.Errorf("Returned incorrect number of messages %d", len(offsetMessages))
		return
	}

	filteredMessages := notifDB.GetAll(0, -1, []string{"order"})
	if len(filteredMessages) != 2 {
		t.Errorf("Returned incorrect number of messages %d", len(filteredMessages))
		return
	}
}

func TestNotficationsDB_MarkAsRead(t *testing.T) {
	n := notif.FollowNotification{"follow", "abc"}
	err := notifDB.Put(n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	err = notifDB.MarkAsRead(1)
	if err != nil {
		t.Error(err)
	}
	stmt, err := notifDB.db.Prepare("select read from notifications where rowid=1")
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
	n := notif.FollowNotification{"follow", "abc"}
	err := notifDB.Put(n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	n = notif.FollowNotification{"follow", "123"}
	err = notifDB.Put(n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	err = notifDB.MarkAllAsRead()
	if err != nil {
		t.Error(err)
	}
	rows, err := notifDB.db.Query("select * from notifications where read=0")
	if err != nil {
		t.Error(err)
	}
	if rows.Next() {
		t.Error("Failed to mark all as read")
	}
}

func TestNotificationDB_GetUnreadCount(t *testing.T) {
	n := notif.FollowNotification{"follow", "abc"}
	err := notifDB.Put(n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	err = notifDB.MarkAsRead(1)
	if err != nil {
		t.Error(err)
	}
	n = notif.FollowNotification{"follow", "xyz"}
	err = notifDB.Put(n, n.Type, time.Now())
	if err != nil {
		t.Error(err)
	}
	all := notifDB.GetAll(0, -1, []string{})
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
