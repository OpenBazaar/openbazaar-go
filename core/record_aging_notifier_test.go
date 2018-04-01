package core

import (
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/op/go-logging"
)

func TestPerformTaskCreatesDisputeAgingNotifications(t *testing.T) {
	// Start each case 50 days ago and have the lastNotifiedAt at a day after
	// each notification is suppose to be sent. With no notifications already queued,
	// it should produce all the old notifications up to the most recent one expected
	var (
		broadcastChannel = make(chan interface{}, 0)
		timeStart        = time.Now().Add(time.Duration(-50*24) * time.Hour)
		twelveHours      = time.Duration(12) * time.Hour
		fifteenDays      = time.Duration(15*24) * time.Hour
		thirtyDays       = time.Duration(30*24) * time.Hour
		fourtyFourDays   = time.Duration(44*24) * time.Hour
		fourtyFiveDays   = time.Duration(45*24) * time.Hour

		// Produces notification for 0, 15, 30, 44 and 45 days
		neverNotified = &repo.DisputeCaseRecord{
			CaseID:         "neverNotified",
			Timestamp:      timeStart,
			LastNotifiedAt: time.Unix(0, 0),
		}
		// Produces notification for 15, 30, 44 and 45 days
		notifiedJustZeroDay = &repo.DisputeCaseRecord{
			CaseID:         "notifiedJustZeroDay",
			Timestamp:      timeStart,
			LastNotifiedAt: timeStart.Add(twelveHours),
		}
		// Produces notification for 30, 44 and 45 days
		notifiedUpToFifteenDay = &repo.DisputeCaseRecord{
			CaseID:         "notifiedUpToFifteenDay",
			Timestamp:      timeStart,
			LastNotifiedAt: timeStart.Add(fifteenDays + twelveHours),
		}
		// Produces notification for 44 and 45 days
		notifiedUpToThirtyDay = &repo.DisputeCaseRecord{
			CaseID:         "notifiedUpToThirtyDay",
			Timestamp:      timeStart,
			LastNotifiedAt: timeStart.Add(thirtyDays + twelveHours),
		}
		// Produces notification for 45 days
		notifiedUpToFourtyFourDays = &repo.DisputeCaseRecord{
			CaseID:         "notifiedUpToFourtyFourDays",
			Timestamp:      timeStart,
			LastNotifiedAt: timeStart.Add(fourtyFourDays + twelveHours),
		}
		// Produces no notifications as all have already been created
		notifiedUpToFourtyFiveDays = &repo.DisputeCaseRecord{
			CaseID:         "notifiedUpToFourtyFiveDays",
			Timestamp:      timeStart,
			LastNotifiedAt: timeStart.Add(fourtyFiveDays + twelveHours),
		}
		existingRecords = []*repo.DisputeCaseRecord{
			neverNotified,
			notifiedJustZeroDay,
			notifiedUpToFifteenDay,
			notifiedUpToThirtyDay,
			notifiedUpToFourtyFourDays,
			notifiedUpToFourtyFiveDays,
		}

		appSchema = schema.MustNewCustomSchemaManager(schema.SchemaContext{
			DataPath:        schema.GenerateTempPath(),
			TestModeEnabled: true,
		})
	)

	if err := appSchema.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	if err := appSchema.InitializeDatabase(); err != nil {
		t.Fatal(err)
	}

	database, err := appSchema.OpenDatabase()
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range existingRecords {
		_, err := database.Exec("insert into cases (caseID, timestamp, lastNotifiedAt) values (?, ?, ?)", r.CaseID, int(r.Timestamp.Unix()), int(r.LastNotifiedAt.Unix()))
		if err != nil {
			t.Fatal(err)
		}
	}

	var (
		closeAsyncChannelVerifier = make(chan bool, 0)
		broadcastCount            = 0
	)
	go func() {
		for {
			select {
			case n := <-broadcastChannel:
				notifier, ok := n.(repo.Notifier)
				if !ok {
					t.Errorf("unable to cast as Notifier: %+v", n)
				}
				t.Log("notification broadcast: %s", notifier.GetNotificationType())
				broadcastCount += 1
			case <-closeAsyncChannelVerifier:
				return
			}
		}
	}()

	datastore := db.NewSQLiteDatastore(database, new(sync.Mutex))
	worker := &recordAgingNotifier{
		datastore: datastore,
		broadcast: broadcastChannel,
		logger:    logging.MustGetLogger("testRecordAgingNotifier"),
	}
	if err := worker.PerformTask(); err != nil {
		t.Fatal(err)
	}

	// Verify Notifications received in channel
	closeAsyncChannelVerifier <- true
	if broadcastCount != 15 {
		t.Error("Expected 15 notifications to be broadcast, found", broadcastCount)
	}

	// Verify NotificationRecords in datastore
	rows, err := database.Query("select caseID, lastNotifiedAt from cases")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var (
			caseID         string
			lastNotifiedAt int64
		)
		if err := rows.Scan(&caseID, &lastNotifiedAt); err != nil {
			t.Fatal(err)
		}
		switch caseID {
		case neverNotified.CaseID, notifiedJustZeroDay.CaseID, notifiedUpToFifteenDay.CaseID, notifiedUpToThirtyDay.CaseID, notifiedUpToFourtyFourDays.CaseID:
			durationFromActual := time.Now().Sub(time.Unix(lastNotifiedAt, 0))
			if durationFromActual > (time.Duration(5) * time.Second) {
				t.Errorf("Expected %s to have lastNotifiedAt set when executed, was %s", caseID, time.Unix(lastNotifiedAt, 0).String())
			}
		case notifiedUpToFourtyFiveDays.CaseID:
			if lastNotifiedAt != notifiedUpToFourtyFiveDays.LastNotifiedAt.Unix() {
				t.Error("Expected notifiedUpToFourtyFiveDays to not update LastNotifiedAt")
			}
		default:
			t.Error("Unexpected dispute case")
		}
	}

	var count int64
	err = database.QueryRow("select count(*) from notifications").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 15 {
		t.Errorf("Expected 15 notifications to be produced, but found %d", count)
	}

	rows, err = database.Query("select notifID, serializedNotification, timestamp from notifications")
	if err != nil {
		t.Fatal(err)
	}

	var (
		checkNeverNotified_ZeroDay       bool
		checkNeverNotified_FifteenDay    bool
		checkNeverNotified_ThirtyDay     bool
		checkNeverNotified_FourtyFourDay bool
		checkNeverNotified_FourtyFiveDay bool
		checkZeroDay_FifteenDay          bool
		checkZeroDay_ThirtyDay           bool
		checkZeroDay_FourtyFourDay       bool
		checkZeroDay_FourtyFiveDay       bool
		checkFifteenDay_ThirtyDay        bool
		checkFifteenDay_FourtyFourDay    bool
		checkFifteenDay_FourtyFiveDay    bool
		checkThirtyDay_FourtyFourDay     bool
		checkThirtyDay_FourtyFiveDay     bool
		checkFourtyFourDay_FourtyFiveDay bool
	)
	for rows.Next() {
		var (
			nID, nJSON string
			nTimestamp sql.NullInt64
		)
		if err = rows.Scan(&nID, &nJSON, &nTimestamp); err != nil {
			t.Error(err)
			continue
		}
		n, err := repo.UnmarshalNotificationRecord(nJSON, nTimestamp.Int64)
		if err != nil {
			t.Error("Failed unmarshalling notification:", err.Error())
			continue
		}
		caseID, err := repo.GetDisputeCaseID(n.Notification)
		if err != nil {
			t.Error("getting dispute case id:", err.Error())
		}
		if caseID == neverNotified.CaseID {
			if n.GetType() == repo.NotifierTypeDisputeAgedZeroDaysOld {
				checkNeverNotified_ZeroDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypeDisputeAgedFifteenDaysOld {
				checkNeverNotified_FifteenDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypeDisputeAgedThirtyDaysOld {
				checkNeverNotified_ThirtyDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypeDisputeAgedFourtyFourDaysOld {
				checkNeverNotified_FourtyFourDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypeDisputeAgedFourtyFiveDaysOld {
				checkNeverNotified_FourtyFiveDay = true
				continue
			}
		}
		if caseID == notifiedJustZeroDay.CaseID {
			if n.GetType() == repo.NotifierTypeDisputeAgedFifteenDaysOld {
				checkZeroDay_FifteenDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypeDisputeAgedThirtyDaysOld {
				checkZeroDay_ThirtyDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypeDisputeAgedFourtyFourDaysOld {
				checkZeroDay_FourtyFourDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypeDisputeAgedFourtyFiveDaysOld {
				checkZeroDay_FourtyFiveDay = true
				continue
			}
		}
		if caseID == notifiedUpToFifteenDay.CaseID {
			if n.GetType() == repo.NotifierTypeDisputeAgedThirtyDaysOld {
				checkFifteenDay_ThirtyDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypeDisputeAgedFourtyFourDaysOld {
				checkFifteenDay_FourtyFourDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypeDisputeAgedFourtyFiveDaysOld {
				checkFifteenDay_FourtyFiveDay = true
				continue
			}
		}
		if caseID == notifiedUpToThirtyDay.CaseID {
			if n.GetType() == repo.NotifierTypeDisputeAgedFourtyFourDaysOld {
				checkThirtyDay_FourtyFourDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypeDisputeAgedFourtyFiveDaysOld {
				checkThirtyDay_FourtyFiveDay = true
				continue
			}
		}
		if caseID == notifiedUpToFourtyFourDays.CaseID && n.GetType() == repo.NotifierTypeDisputeAgedFourtyFiveDaysOld {
			checkFourtyFourDay_FourtyFiveDay = true
		}
	}

	if checkNeverNotified_ZeroDay != true {
		t.Errorf("Expected notification missing: checkNeverNotified_ZeroDay")
	}
	if checkNeverNotified_FifteenDay != true {
		t.Errorf("Expected notification missing: checkNeverNotified_FifteenDay")
	}
	if checkNeverNotified_ThirtyDay != true {
		t.Errorf("Expected notification missing: checkNeverNotified_ThirtyDay")
	}
	if checkNeverNotified_FourtyFourDay != true {
		t.Errorf("Expected notification missing: checkNeverNotified_FourtyFourDay")
	}
	if checkNeverNotified_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkNeverNotified_FourtyFiveDay")
	}
	if checkZeroDay_FifteenDay != true {
		t.Errorf("Expected notification missing: checkZeroDay_FifteenDay")
	}
	if checkZeroDay_ThirtyDay != true {
		t.Errorf("Expected notification missing: checkZeroDay_ThirtyDay")
	}
	if checkZeroDay_FourtyFourDay != true {
		t.Errorf("Expected notification missing: checkZeroDay_FourtyFourDay")
	}
	if checkZeroDay_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkZeroDay_FourtyFiveDay")
	}
	if checkFifteenDay_ThirtyDay != true {
		t.Errorf("Expected notification missing: checkFifteenDay_ThirtyDay")
	}
	if checkFifteenDay_FourtyFourDay != true {
		t.Errorf("Expected notification missing: checkFifteenDay_FourtyFourDay")
	}
	if checkFifteenDay_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkFifteenDay_FourtyFiveDay")
	}
	if checkThirtyDay_FourtyFourDay != true {
		t.Errorf("Expected notification missing: checkThirtyDay_FourtyFourDay")
	}
	if checkThirtyDay_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkThirtyDay_FourtyFiveDay")
	}
	if checkFourtyFourDay_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkFourtyFourDay_FourtyFiveDay")
	}
}

func TestPerformTaskCreatesPurchaseAgingNotifications(t *testing.T) {
	// Start each purchase 50 days ago and have the lastNotifiedAt at a day after
	// each notification is suppose to be sent. With no notifications already queued,
	// it should produce all the old notifications up to the most recent one expected
	var (
		broadcastChannel = make(chan interface{}, 0)
		timeStart        = time.Now().Add(time.Duration(-50*24) * time.Hour)
		twelveHours      = time.Duration(12) * time.Hour
		fifteenDays      = time.Duration(15*24) * time.Hour
		fourtyDays       = time.Duration(40*24) * time.Hour
		fourtyFourDays   = time.Duration(44*24) * time.Hour
		fourtyFiveDays   = time.Duration(45*24) * time.Hour

		// Produces notification for 0, 15, 30, 44 and 45 days
		neverNotified = &repo.PurchaseRecord{
			OrderID:        "neverNotified",
			Timestamp:      timeStart,
			LastNotifiedAt: time.Unix(0, 0),
		}
		// Produces notification for 15, 30, 44 and 45 days
		notifiedJustZeroDay = &repo.PurchaseRecord{
			OrderID:        "notifiedJustZeroDay",
			Timestamp:      timeStart,
			LastNotifiedAt: timeStart.Add(twelveHours),
		}
		// Produces notification for 30, 44 and 45 days
		notifiedUpToFifteenDay = &repo.PurchaseRecord{
			OrderID:        "notifiedUpToFifteenDay",
			Timestamp:      timeStart,
			LastNotifiedAt: timeStart.Add(fifteenDays + twelveHours),
		}
		// Produces notification for 44 and 45 days
		notifiedUpToFourtyDay = &repo.PurchaseRecord{
			OrderID:        "notifiedUpToFourtyDay",
			Timestamp:      timeStart,
			LastNotifiedAt: timeStart.Add(fourtyDays + twelveHours),
		}
		// Produces notification for 45 days
		notifiedUpToFourtyFourDays = &repo.PurchaseRecord{
			OrderID:        "notifiedUpToFourtyFourDays",
			Timestamp:      timeStart,
			LastNotifiedAt: timeStart.Add(fourtyFourDays + twelveHours),
		}
		// Produces no notifications as all have already been created
		notifiedUpToFourtyFiveDays = &repo.PurchaseRecord{
			OrderID:        "notifiedUpToFourtyFiveDays",
			Timestamp:      timeStart,
			LastNotifiedAt: timeStart.Add(fourtyFiveDays + twelveHours),
		}
		existingRecords = []*repo.PurchaseRecord{
			neverNotified,
			notifiedJustZeroDay,
			notifiedUpToFifteenDay,
			notifiedUpToFourtyDay,
			notifiedUpToFourtyFourDays,
			notifiedUpToFourtyFiveDays,
		}

		appSchema = schema.MustNewCustomSchemaManager(schema.SchemaContext{
			DataPath:        schema.GenerateTempPath(),
			TestModeEnabled: true,
		})
	)

	if err := appSchema.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	if err := appSchema.InitializeDatabase(); err != nil {
		t.Fatal(err)
	}
	database, err := appSchema.OpenDatabase()
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range existingRecords {
		_, err := database.Exec("insert into purchases (orderID, timestamp, lastNotifiedAt) values (?, ?, ?)", r.OrderID, int(r.Timestamp.Unix()), int(r.LastNotifiedAt.Unix()))
		if err != nil {
			t.Fatal(err)
		}
	}

	datastore := db.NewSQLiteDatastore(database, new(sync.Mutex))
	worker := &recordAgingNotifier{
		datastore: datastore,
		broadcast: broadcastChannel,
		logger:    logging.MustGetLogger("testRecordAgingNotifier"),
	}
	if err := worker.PerformTask(); err != nil {
		t.Fatal(err)
	}

	// Verify NotificationRecords in datastore
	rows, err := database.Query("select orderID, lastNotifiedAt from purchases")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var (
			orderID        string
			lastNotifiedAt int64
		)
		if err := rows.Scan(&orderID, &lastNotifiedAt); err != nil {
			t.Fatal(err)
		}
		switch orderID {
		case neverNotified.OrderID, notifiedJustZeroDay.OrderID, notifiedUpToFifteenDay.OrderID, notifiedUpToFourtyDay.OrderID, notifiedUpToFourtyFourDays.OrderID:
			durationFromActual := time.Now().Sub(time.Unix(lastNotifiedAt, 0))
			if durationFromActual > (time.Duration(5) * time.Second) {
				t.Errorf("Expected %s to have lastNotifiedAt set when executed, was %s", orderID, time.Unix(lastNotifiedAt, 0).String())
			}
		case notifiedUpToFourtyFiveDays.OrderID:
			if lastNotifiedAt != notifiedUpToFourtyFiveDays.LastNotifiedAt.Unix() {
				t.Error("Expected notifiedUpToFourtyFiveDays to not update LastNotifiedAt")
			}
		default:
			t.Error("Unexpected dispute case")
		}
	}

	var count int64
	err = database.QueryRow("select count(*) from notifications").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 15 {
		t.Errorf("Expected 15 notifications to be produced, but found %d", count)
	}

	rows, err = database.Query("select notifID, serializedNotification, timestamp from notifications")
	if err != nil {
		t.Fatal(err)
	}

	var (
		checkNeverNotified_ZeroDay       bool
		checkNeverNotified_FifteenDay    bool
		checkNeverNotified_FourtyDay     bool
		checkNeverNotified_FourtyFourDay bool
		checkNeverNotified_FourtyFiveDay bool
		checkZeroDay_FifteenDay          bool
		checkZeroDay_FourtyDay           bool
		checkZeroDay_FourtyFourDay       bool
		checkZeroDay_FourtyFiveDay       bool
		checkFifteenDay_FourtyDay        bool
		checkFifteenDay_FourtyFourDay    bool
		checkFifteenDay_FourtyFiveDay    bool
		checkFourtyDay_FourtyFourDay     bool
		checkFourtyDay_FourtyFiveDay     bool
		checkFourtyFourDay_FourtyFiveDay bool
	)
	for rows.Next() {
		var (
			nID, nJSON string
			nTimestamp sql.NullInt64
		)
		if err = rows.Scan(&nID, &nJSON, &nTimestamp); err != nil {
			t.Error(err)
			continue
		}
		n, err := repo.UnmarshalNotificationRecord(nJSON, nTimestamp.Int64)
		if err != nil {
			t.Error("Failed unmarshalling notification:", err.Error())
			continue
		}
		orderID, err := repo.GetPurchaseOrderID(n.Notification)
		if err != nil {
			t.Error("getting dispute case id:", err.Error())
		}
		if orderID == neverNotified.OrderID {
			if n.GetType() == repo.NotifierTypePurchaseAgedZeroDaysOld {
				checkNeverNotified_ZeroDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypePurchaseAgedFifteenDaysOld {
				checkNeverNotified_FifteenDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypePurchaseAgedFourtyDaysOld {
				checkNeverNotified_FourtyDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypePurchaseAgedFourtyFourDaysOld {
				checkNeverNotified_FourtyFourDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypePurchaseAgedFourtyFiveDaysOld {
				checkNeverNotified_FourtyFiveDay = true
				continue
			}
		}
		if orderID == notifiedJustZeroDay.OrderID {
			if n.GetType() == repo.NotifierTypePurchaseAgedFifteenDaysOld {
				checkZeroDay_FifteenDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypePurchaseAgedFourtyDaysOld {
				checkZeroDay_FourtyDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypePurchaseAgedFourtyFourDaysOld {
				checkZeroDay_FourtyFourDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypePurchaseAgedFourtyFiveDaysOld {
				checkZeroDay_FourtyFiveDay = true
				continue
			}
		}
		if orderID == notifiedUpToFifteenDay.OrderID {
			if n.GetType() == repo.NotifierTypePurchaseAgedFourtyDaysOld {
				checkFifteenDay_FourtyDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypePurchaseAgedFourtyFourDaysOld {
				checkFifteenDay_FourtyFourDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypePurchaseAgedFourtyFiveDaysOld {
				checkFifteenDay_FourtyFiveDay = true
				continue
			}
		}
		if orderID == notifiedUpToFourtyDay.OrderID {
			if n.GetType() == repo.NotifierTypePurchaseAgedFourtyFourDaysOld {
				checkFourtyDay_FourtyFourDay = true
				continue
			}
			if n.GetType() == repo.NotifierTypePurchaseAgedFourtyFiveDaysOld {
				checkFourtyDay_FourtyFiveDay = true
				continue
			}
		}
		if orderID == notifiedUpToFourtyFourDays.OrderID && n.GetType() == repo.NotifierTypePurchaseAgedFourtyFiveDaysOld {
			checkFourtyFourDay_FourtyFiveDay = true
		}
	}

	if checkNeverNotified_ZeroDay != true {
		t.Errorf("Expected notification missing: checkNeverNotified_ZeroDay")
	}
	if checkNeverNotified_FifteenDay != true {
		t.Errorf("Expected notification missing: checkNeverNotified_FifteenDay")
	}
	if checkNeverNotified_FourtyDay != true {
		t.Errorf("Expected notification missing: checkNeverNotified_FourtyDay")
	}
	if checkNeverNotified_FourtyFourDay != true {
		t.Errorf("Expected notification missing: checkNeverNotified_FourtyFourDay")
	}
	if checkNeverNotified_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkNeverNotified_FourtyFiveDay")
	}
	if checkZeroDay_FifteenDay != true {
		t.Errorf("Expected notification missing: checkZeroDay_FifteenDay")
	}
	if checkZeroDay_FourtyDay != true {
		t.Errorf("Expected notification missing: checkZeroDay_FourtyDay")
	}
	if checkZeroDay_FourtyFourDay != true {
		t.Errorf("Expected notification missing: checkZeroDay_FourtyFourDay")
	}
	if checkZeroDay_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkZeroDay_FourtyFiveDay")
	}
	if checkFifteenDay_FourtyDay != true {
		t.Errorf("Expected notification missing: checkFifteenDay_FourtyDay")
	}
	if checkFifteenDay_FourtyFourDay != true {
		t.Errorf("Expected notification missing: checkFifteenDay_FourtyFourDay")
	}
	if checkFifteenDay_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkFifteenDay_FourtyFiveDay")
	}
	if checkFourtyDay_FourtyFourDay != true {
		t.Errorf("Expected notification missing: checkFourtyDay_FourtyFourDay")
	}
	if checkFourtyDay_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkFourtyDay_FourtyFiveDay")
	}
	if checkFourtyFourDay_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkFourtyFourDay_FourtyFiveDay")
	}
}
