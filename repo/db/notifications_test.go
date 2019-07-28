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
		notificationDb, teardown, err = newNotificationStore()
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
		if err := notificationDb.PutRecord(subject); err != nil {
			t.Fatal(err)
		}
		allNotifications, _, err := notificationDb.GetAll("", -1, []string{})
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
			if !reflect.DeepEqual(subject, actual) {
				t.Error("Expected found notification to equal each other")
				t.Errorf("Expected: %+v\n", subject)
				t.Errorf("Actual: %+v\n", actual)

			}
		}

		if !foundNotification {
			t.Errorf("Expected to find notification, but was not found\nExpected type: (%s) Expected ID: (%s)", subject.GetType(), subject.GetID())
			t.Errorf("Found records: %+v", allNotifications)
		}
	}
}

func TestNotficationsDB_Delete(t *testing.T) {
	notificationDb, teardown, err := newNotificationStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	n := repo.FollowNotification{ID: "1", Type: repo.NotifierTypeFollowNotification, PeerId: "abc"}
	err = notificationDb.PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		t.Error(err)
	}
	err = notificationDb.Delete("1")
	if err != nil {
		t.Error(err)
	}
	stmt, err := notificationDb.PrepareQuery("select notifID from notifications where notifID='1'")
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
	notificationDb, teardown, err := newNotificationStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	f := repo.FollowNotification{ID: "1", Type: repo.NotifierTypeFollowNotification, PeerId: "abc"}
	err = notificationDb.PutRecord(repo.NewNotification(f, time.Now(), false))
	if err != nil {
		t.Error(err)
	}
	u := repo.UnfollowNotification{ID: "2", Type: repo.NotifierTypeUnfollowNotification, PeerId: "123"}
	err = notificationDb.PutRecord(repo.NewNotification(u, time.Now().Add(time.Second), false))
	if err != nil {
		t.Error(err)
	}
	u = repo.UnfollowNotification{ID: "3", Type: repo.NotifierTypeUnfollowNotification, PeerId: "56778"}
	err = notificationDb.PutRecord(repo.NewNotification(u, time.Now().Add(time.Second*2), false))
	if err != nil {
		t.Error(err)
	}
	notifs, _, err := notificationDb.GetAll("", -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(notifs) != 3 {
		t.Error("Returned incorrect number of messages")
		return
	}

	limtedMessages, _, err := notificationDb.GetAll("", 2, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(limtedMessages) != 2 {
		t.Error("Returned incorrect number of messages")
		return
	}

	offsetMessages, _, err := notificationDb.GetAll("3", -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(offsetMessages) != 2 {
		t.Errorf("Returned incorrect number of messages %d", len(offsetMessages))
		return
	}

	filteredMessages, _, err := notificationDb.GetAll("", -1, []string{"unfollow"})
	if err != nil {
		t.Error(err)
	}
	if len(filteredMessages) != 2 {
		t.Errorf("Returned incorrect number of messages %d", len(filteredMessages))
		return
	}
}

func TestNotficationsDB_MarkAsRead(t *testing.T) {
	notificationDb, teardown, err := newNotificationStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	n := repo.FollowNotification{ID: "5", Type: repo.NotifierTypeFollowNotification, PeerId: "abc"}
	err = notificationDb.PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		t.Error(err)
	}
	err = notificationDb.MarkAsRead("5")
	if err != nil {
		t.Error(err)
	}
	stmt, err := notificationDb.PrepareQuery("select read from notifications where notifID='5'")
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
	notificationDb, teardown, err := newNotificationStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	n := repo.FollowNotification{ID: "6", Type: repo.NotifierTypeFollowNotification, PeerId: "abc"}
	err = notificationDb.PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		t.Error(err)
	}
	n = repo.FollowNotification{ID: "7", Type: repo.NotifierTypeFollowNotification, PeerId: "123"}
	err = notificationDb.PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		t.Error(err)
	}
	err = notificationDb.MarkAllAsRead()
	if err != nil {
		t.Error(err)
	}
	rows, err := notificationDb.PrepareAndExecuteQuery("select * from notifications where read=0")
	if err != nil {
		t.Error(err)
	}
	if rows.Next() {
		t.Error("Failed to mark all as read")
	}
}

func TestNotificationDB_GetUnreadCount(t *testing.T) {
	notificationDb, teardown, err := newNotificationStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	n := repo.FollowNotification{ID: "8", Type: repo.NotifierTypeFollowNotification, PeerId: "abc"}
	err = notificationDb.PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		t.Error(err)
	}
	err = notificationDb.MarkAsRead("8")
	if err != nil {
		t.Error(err)
	}
	n = repo.FollowNotification{ID: "9", Type: repo.NotifierTypeFollowNotification, PeerId: "xyz"}
	err = notificationDb.PutRecord(repo.NewNotification(n, time.Now(), false))
	if err != nil {
		t.Error(err)
	}
	all, _, err := notificationDb.GetAll("", -1, []string{})
	if err != nil {
		t.Error(err)
	}
	var c int
	for _, a := range all {
		if !a.IsRead {
			c++
		}
	}
	count, err := notificationDb.GetUnreadCount()
	if err != nil {
		t.Error(err)
	}
	if count != c {
		t.Error("GetUnreadCount returned incorrect count")
	}
}
