package db_test

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func newNotificationStore() (repo.NotificationStore, func(), error) {
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
	return db.NewNotificationStore(database, new(sync.Mutex)), appSchema.DestroySchemaDirectories, nil
}

func TestNotficationsDB_PutRecord(t *testing.T) {
	var (
		db, teardown, err = newNotificationStore()
		// now as Unix() quantizes time to DB's resolution which makes reflect.DeepEqual pass below
		now               = time.Unix(time.Now().UTC().Unix(), 0)
		putRecordExamples = []*repo.Notification{
			repo.NewNotification(repo.OrderCancelNotification{
				ID:      "orderCancelNotif",
				Type:    repo.NotifierTypeOrderCancelNotification,
				OrderId: "orderCancelReferenceOrderID",
			}, now, true),
			repo.NewNotification(repo.ModeratorDisputeExpiry{
				ID:     "disputeAgingNotif",
				Type:   repo.NotifierTypeModeratorDisputeExpiry,
				CaseID: "disputAgingReferenceCaseID",
			}, now, false),
			repo.NewNotification(repo.BuyerDisputeTimeout{
				ID:      "purchaseAgingNotif",
				Type:    repo.NotifierTypeBuyerDisputeTimeout,
				OrderID: "purchaseAgingReferenceOrderID",
			}, now, true),
		}
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	for _, subject := range putRecordExamples {
		if err := db.PutRecord(subject); err != nil {
			t.Fatal(err)
		}
		allNotifications, _, err := db.GetAll("", -1, []string{})
		if err != nil {
			t.Fatal(err)
		}

		foundNotification := false
		for _, actual := range allNotifications {
			if actual.ID != subject.ID {
				t.Logf("Actual notification ID (%s) did not match subject, continuing...", actual.ID)
				continue
			}

			foundNotification = true
			if actual.GetType() != subject.GetType() {
				t.Error("Expected found notification to match types")
				t.Errorf("Expected: %s", subject.GetType())
				t.Errorf("Actual: %s", actual.GetType())
			}
			if reflect.DeepEqual(subject, actual) != true {
				t.Error("Expected found notification to equal each other")
				t.Errorf("Expected: %+v\n", subject)
				t.Errorf("Actual: %+v\n", actual)

			}
		}

		if foundNotification == false {
			t.Errorf("Expected to find notification, but was not found\nExpected type: (%s) Expected ID: (%s)", subject.GetType(), subject.GetID())
			t.Errorf("Found records: %+v", allNotifications)
		}
	}
}

func TestNotficationsDB_Delete(t *testing.T) {
	db, teardown, err := newNotificationStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	n := repo.FollowNotification{"1", repo.NotifierTypeFollowNotification, "abc"}
	err = db.PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		t.Error(err)
	}
	err = db.Delete("1")
	if err != nil {
		t.Error(err)
	}
	stmt, err := db.PrepareQuery("select notifID from notifications where notifID='1'")
	if err != nil {
		t.Error(err)
	}
	defer stmt.Close()
	var notifId int
	err = stmt.QueryRow().Scan(&notifId)
	if err == nil {
		t.Error("Delete failed")
	}
}

func TestNotficationsDB_GetAll(t *testing.T) {
	db, teardown, err := newNotificationStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	f := repo.FollowNotification{"1", repo.NotifierTypeFollowNotification, "abc"}
	err = db.PutRecord(repo.NewNotification(f, time.Now(), false))
	if err != nil {
		t.Error(err)
	}
	u := repo.UnfollowNotification{"2", repo.NotifierTypeUnfollowNotification, "123"}
	err = db.PutRecord(repo.NewNotification(u, time.Now().Add(time.Second), false))
	if err != nil {
		t.Error(err)
	}
	u = repo.UnfollowNotification{"3", repo.NotifierTypeUnfollowNotification, "56778"}
	err = db.PutRecord(repo.NewNotification(u, time.Now().Add(time.Second*2), false))
	if err != nil {
		t.Error(err)
	}
	notifs, _, err := db.GetAll("", -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(notifs) != 3 {
		t.Error("Returned incorrect number of messages")
		return
	}

	limtedMessages, _, err := db.GetAll("", 2, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(limtedMessages) != 2 {
		t.Error("Returned incorrect number of messages")
		return
	}

	offsetMessages, _, err := db.GetAll("3", -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(offsetMessages) != 2 {
		t.Errorf("Returned incorrect number of messages %d", len(offsetMessages))
		return
	}

	filteredMessages, _, err := db.GetAll("", -1, []string{"unfollow"})
	if err != nil {
		t.Error(err)
	}
	if len(filteredMessages) != 2 {
		t.Errorf("Returned incorrect number of messages %d", len(filteredMessages))
		return
	}
}

func TestNotficationsDB_MarkAsRead(t *testing.T) {
	db, teardown, err := newNotificationStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	n := repo.FollowNotification{"5", repo.NotifierTypeFollowNotification, "abc"}
	err = db.PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		t.Error(err)
	}
	err = db.MarkAsRead("5")
	if err != nil {
		t.Error(err)
	}
	stmt, err := db.PrepareQuery("select read from notifications where notifID='5'")
	if err != nil {
		t.Error(err)
	}
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
	db, teardown, err := newNotificationStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	n := repo.FollowNotification{"6", repo.NotifierTypeFollowNotification, "abc"}
	err = db.PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		t.Error(err)
	}
	n = repo.FollowNotification{"7", repo.NotifierTypeFollowNotification, "123"}
	err = db.PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		t.Error(err)
	}
	err = db.MarkAllAsRead()
	if err != nil {
		t.Error(err)
	}
	rows, err := db.PrepareAndExecuteQuery("select * from notifications where read=0")
	if err != nil {
		t.Error(err)
	}
	if rows.Next() {
		t.Error("Failed to mark all as read")
	}
}

func TestNotificationDB_GetUnreadCount(t *testing.T) {
	db, teardown, err := newNotificationStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	n := repo.FollowNotification{"8", repo.NotifierTypeFollowNotification, "abc"}
	err = db.PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		t.Error(err)
	}
	err = db.MarkAsRead("8")
	if err != nil {
		t.Error(err)
	}
	n = repo.FollowNotification{"9", repo.NotifierTypeFollowNotification, "xyz"}
	err = db.PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		t.Error(err)
	}
	all, _, err := db.GetAll("", -1, []string{})
	if err != nil {
		t.Error(err)
	}
	var c int
	for _, a := range all {
		if !a.IsRead {
			c++
		}
	}
	count, err := db.GetUnreadCount()
	if err != nil {
		t.Error(err)
	}
	if count != c {
		t.Error("GetUnreadCount returned incorrect count")
	}
}
