package core

import (
	"database/sql"
	"encoding/json"
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
		broadcastChannel = make(chan repo.Notifier, 0)
		timeStart        = time.Now().Add(time.Duration(-50*24) * time.Hour)
		twelveHours      = time.Duration(12) * time.Hour
		fifteenDays      = time.Duration(15*24) * time.Hour
		fourtyDays       = time.Duration(40*24) * time.Hour
		fourtyFourDays   = time.Duration(44*24) * time.Hour
		fourtyFiveDays   = time.Duration(45*24) * time.Hour

		// Produces notification for 0, 15, 40, 44 and 45 days
		neverNotified = &repo.DisputeCaseRecord{
			CaseID:         "neverNotified",
			Timestamp:      timeStart,
			LastNotifiedAt: time.Unix(0, 0),
		}
		// Produces notification for 15, 40, 44 and 45 days
		notifiedJustZeroDay = &repo.DisputeCaseRecord{
			CaseID:         "notifiedJustZeroDay",
			Timestamp:      timeStart,
			LastNotifiedAt: timeStart.Add(twelveHours),
		}
		// Produces notification for 40, 44 and 45 days
		notifiedUpToFifteenDay = &repo.DisputeCaseRecord{
			CaseID:         "notifiedUpToFifteenDay",
			Timestamp:      timeStart,
			LastNotifiedAt: timeStart.Add(fifteenDays + twelveHours),
		}
		// Produces notification for 44 and 45 days
		notifiedUpToFourtyDays = &repo.DisputeCaseRecord{
			CaseID:         "notifiedUpToFourtyDay",
			Timestamp:      timeStart,
			LastNotifiedAt: timeStart.Add(fourtyDays + twelveHours),
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
			notifiedUpToFourtyDays,
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
	s, err := database.Prepare("insert into cases (caseID, timestamp, lastNotifiedAt) values (?, ?, ?)")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range existingRecords {
		_, err := s.Exec(r.CaseID, int(r.Timestamp.Unix()), int(r.LastNotifiedAt.Unix()))
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
				t.Logf("notification received: %s", notifier.GetType())
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

	worker.PerformTask()

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
		case neverNotified.CaseID, notifiedJustZeroDay.CaseID, notifiedUpToFifteenDay.CaseID, notifiedUpToFourtyDays.CaseID, notifiedUpToFourtyFourDays.CaseID:
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
		checkNeverNotifiedDispute_ZeroDay       bool
		checkNeverNotifiedDispute_FifteenDay    bool
		checkNeverNotifiedDispute_FourtyDay     bool
		checkNeverNotifiedDispute_FourtyFourDay bool
		checkNeverNotifiedDispute_FourtyFiveDay bool
		checkZeroDayDispute_FifteenDay          bool
		checkZeroDayDispute_FourtyDay           bool
		checkZeroDayDispute_FourtyFourDay       bool
		checkZeroDayDispute_FourtyFiveDay       bool
		checkFifteenDayDispute_FourtyDay        bool
		checkFifteenDayDispute_FourtyFourDay    bool
		checkFifteenDayDispute_FourtyFiveDay    bool
		checkFourtyDayDispute_FourtyFourDay     bool
		checkFourtyDayDispute_FourtyFiveDay     bool
		checkFourtyFourDayDispute_FourtyFiveDay bool
	)
	for rows.Next() {
		var (
			nID, nJSON string
			nTimestamp sql.NullInt64
			n          *repo.Notification
		)
		if err = rows.Scan(&nID, &nJSON, &nTimestamp); err != nil {
			t.Error(err)
			continue
		}
		if err := json.Unmarshal([]byte(nJSON), &n); err != nil {
			t.Error("Failed unmarshalling notification:", err.Error())
			continue
		}
		var (
			refID = n.NotifierData.(repo.DisputeAgingNotification).CaseID
		)
		t.Log("looking for notification:", refID)
		if refID == neverNotified.CaseID {
			if n.NotifierType == repo.NotifierTypeDisputeAgedZeroDays {
				checkNeverNotifiedDispute_ZeroDay = true
				continue
			}
			if n.NotifierType == repo.NotifierTypeDisputeAgedFifteenDays {
				checkNeverNotifiedDispute_FifteenDay = true
				continue
			}
			if n.NotifierType == repo.NotifierTypeDisputeAgedFourtyDays {
				checkNeverNotifiedDispute_FourtyDay = true
				continue
			}
			if n.NotifierType == repo.NotifierTypeDisputeAgedFourtyFourDays {
				checkNeverNotifiedDispute_FourtyFourDay = true
				continue
			}
			if n.NotifierType == repo.NotifierTypeDisputeAgedFourtyFiveDays {
				checkNeverNotifiedDispute_FourtyFiveDay = true
				continue
			}
		}
		if refID == notifiedJustZeroDay.CaseID {
			if n.NotifierType == repo.NotifierTypeDisputeAgedFifteenDays {
				checkZeroDayDispute_FifteenDay = true
				continue
			}
			if n.NotifierType == repo.NotifierTypeDisputeAgedFourtyDays {
				checkZeroDayDispute_FourtyDay = true
				continue
			}
			if n.NotifierType == repo.NotifierTypeDisputeAgedFourtyFourDays {
				checkZeroDayDispute_FourtyFourDay = true
				continue
			}
			if n.NotifierType == repo.NotifierTypeDisputeAgedFourtyFiveDays {
				checkZeroDayDispute_FourtyFiveDay = true
				continue
			}
		}
		if refID == notifiedUpToFifteenDay.CaseID {
			if n.NotifierType == repo.NotifierTypeDisputeAgedFourtyDays {
				checkFifteenDayDispute_FourtyDay = true
				continue
			}
			if n.NotifierType == repo.NotifierTypeDisputeAgedFourtyFourDays {
				checkFifteenDayDispute_FourtyFourDay = true
				continue
			}
			if n.NotifierType == repo.NotifierTypeDisputeAgedFourtyFiveDays {
				checkFifteenDayDispute_FourtyFiveDay = true
				continue
			}
		}
		if refID == notifiedUpToFourtyDays.CaseID {
			if n.NotifierType == repo.NotifierTypeDisputeAgedFourtyFourDays {
				checkFourtyDayDispute_FourtyFourDay = true
				continue
			}
			if n.NotifierType == repo.NotifierTypeDisputeAgedFourtyFiveDays {
				checkFourtyDayDispute_FourtyFiveDay = true
				continue
			}
		}
		if refID == notifiedUpToFourtyFourDays.CaseID && n.NotifierType == repo.NotifierTypeDisputeAgedFourtyFiveDays {
			checkFourtyFourDayDispute_FourtyFiveDay = true
		}
	}

	if checkNeverNotifiedDispute_ZeroDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkNeverNotifiedDispute_ZeroDay")
	}
	if checkNeverNotifiedDispute_FifteenDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkNeverNotifiedDispute_FifteenDay")
	}
	if checkNeverNotifiedDispute_FourtyDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkNeverNotifiedDispute_FourtyDay")
	}
	if checkNeverNotifiedDispute_FourtyFourDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkNeverNotifiedDispute_FourtyFourDay")
	}
	if checkNeverNotifiedDispute_FourtyFiveDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkNeverNotifiedDispute_FourtyFiveDay")
	}
	if checkZeroDayDispute_FifteenDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkZeroDayDispute_FifteenDay")
	}
	if checkZeroDayDispute_FourtyDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkZeroDayDispute_FourtyDay")
	}
	if checkZeroDayDispute_FourtyFourDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkZeroDayDispute_FourtyFourDay")
	}
	if checkZeroDayDispute_FourtyFiveDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkZeroDayDispute_FourtyFiveDay")
	}
	if checkFifteenDayDispute_FourtyDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkFifteenDayDispute_FourtyDay")
	}
	if checkFifteenDayDispute_FourtyFourDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkFifteenDayDispute_FourtyFourDay")
	}
	if checkFifteenDayDispute_FourtyFiveDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkFifteenDayDispute_FourtyFiveDay")
	}
	if checkFourtyDayDispute_FourtyFourDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkFourtyDayDispute_FourtyFourDay")
	}
	if checkFourtyDayDispute_FourtyFiveDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkFourtyDayDispute_FourtyFiveDay")
	}
	if checkFourtyFourDayDispute_FourtyFiveDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkFourtyFourDayDispute_FourtyFiveDay")
	}
}

func TestPerformTaskCreatesBuyerDisputeTimeoutNotifications(t *testing.T) {
	// Start each purchase 50 days ago and have the lastNotifiedAt at a day after
	// each notification is suppose to be sent. With no notifications already queued,
	// it should produce all the old notifications up to the most recent one expected
	var (
		broadcastChannel = make(chan repo.Notifier, 0)
		timeStart        = time.Now().Add(time.Duration(-50*24) * time.Hour)
		twelveHours      = time.Duration(12) * time.Hour
		fifteenDays      = time.Duration(15*24) * time.Hour
		fourtyDays       = time.Duration(40*24) * time.Hour
		fourtyFourDays   = time.Duration(44*24) * time.Hour
		fourtyFiveDays   = time.Duration(45*24) * time.Hour

		// Produces notification for 0, 15, 40, 44 and 45 days
		neverNotified = &repo.PurchaseRecord{
			OrderID:        "neverNotified",
			Timestamp:      timeStart,
			LastNotifiedAt: time.Unix(0, 0),
		}
		// Produces notification for 15, 40, 44 and 45 days
		notifiedJustZeroDay = &repo.PurchaseRecord{
			OrderID:        "notifiedJustZeroDay",
			Timestamp:      timeStart,
			LastNotifiedAt: timeStart.Add(twelveHours),
		}
		// Produces notification for 40, 44 and 45 days
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
				t.Logf("notification received: %s", notifier.GetType())
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

	worker.PerformTask()

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
		checkNeverNotifiedPurchase_ZeroDay       bool
		checkNeverNotifiedPurchase_FifteenDay    bool
		checkNeverNotifiedPurchase_FourtyDay     bool
		checkNeverNotifiedPurchase_FourtyFourDay bool
		checkNeverNotifiedPurchase_FourtyFiveDay bool
		checkZeroDayPurchase_FifteenDay          bool
		checkZeroDayPurchase_FourtyDay           bool
		checkZeroDayPurchase_FourtyFourDay       bool
		checkZeroDayPurchase_FourtyFiveDay       bool
		checkFifteenDayPurchase_FourtyDay        bool
		checkFifteenDayPurchase_FourtyFourDay    bool
		checkFifteenDayPurchase_FourtyFiveDay    bool
		checkFourtyDayPurchase_FourtyFourDay     bool
		checkFourtyDayPurchase_FourtyFiveDay     bool
		checkFourtyFourDayPurchase_FourtyFiveDay bool

		firstInterval_ExpectedExpiresIn  = uint((repo.BuyerDisputeTimeout_lastInterval - repo.BuyerDisputeTimeout_firstInterval).Seconds())
		secondInterval_ExpectedExpiresIn = uint((repo.BuyerDisputeTimeout_lastInterval - repo.BuyerDisputeTimeout_secondInterval).Seconds())
		thirdInterval_ExpectedExpiresIn  = uint((repo.BuyerDisputeTimeout_lastInterval - repo.BuyerDisputeTimeout_thirdInterval).Seconds())
		fourthInterval_ExpectedExpiresIn = uint((repo.BuyerDisputeTimeout_lastInterval - repo.BuyerDisputeTimeout_fourthInterval).Seconds())
		lastInterval_ExpectedExpiresIn   = uint(0)
	)
	for rows.Next() {
		var (
			nID, nJSON string
			nTimestamp sql.NullInt64
			n          *repo.Notification
		)
		if err = rows.Scan(&nID, &nJSON, &nTimestamp); err != nil {
			t.Error(err)
			continue
		}
		if err := json.Unmarshal([]byte(nJSON), &n); err != nil {
			t.Error("Failed unmarshalling notification:", err.Error())
			continue
		}
		var (
			refID     = n.NotifierData.(repo.BuyerDisputeTimeout).OrderID
			expiresIn = n.NotifierData.(repo.BuyerDisputeTimeout).ExpiresIn
		)
		if refID == neverNotified.OrderID {
			if expiresIn == firstInterval_ExpectedExpiresIn {
				checkNeverNotifiedPurchase_ZeroDay = true
				continue
			}
			if expiresIn == secondInterval_ExpectedExpiresIn {
				checkNeverNotifiedPurchase_FifteenDay = true
				continue
			}
			if expiresIn == thirdInterval_ExpectedExpiresIn {
				checkNeverNotifiedPurchase_FourtyDay = true
				continue
			}
			if expiresIn == fourthInterval_ExpectedExpiresIn {
				checkNeverNotifiedPurchase_FourtyFourDay = true
				continue
			}
			if expiresIn == lastInterval_ExpectedExpiresIn {
				checkNeverNotifiedPurchase_FourtyFiveDay = true
				continue
			}
		}
		if refID == notifiedJustZeroDay.OrderID {
			if expiresIn == secondInterval_ExpectedExpiresIn {
				checkZeroDayPurchase_FifteenDay = true
				continue
			}
			if expiresIn == thirdInterval_ExpectedExpiresIn {
				checkZeroDayPurchase_FourtyDay = true
				continue
			}
			if expiresIn == fourthInterval_ExpectedExpiresIn {
				checkZeroDayPurchase_FourtyFourDay = true
				continue
			}
			if expiresIn == lastInterval_ExpectedExpiresIn {
				checkZeroDayPurchase_FourtyFiveDay = true
				continue
			}
		}
		if refID == notifiedUpToFifteenDay.OrderID {
			if expiresIn == thirdInterval_ExpectedExpiresIn {
				checkFifteenDayPurchase_FourtyDay = true
				continue
			}
			if expiresIn == fourthInterval_ExpectedExpiresIn {
				checkFifteenDayPurchase_FourtyFourDay = true
				continue
			}
			if expiresIn == lastInterval_ExpectedExpiresIn {
				checkFifteenDayPurchase_FourtyFiveDay = true
				continue
			}
		}
		if refID == notifiedUpToFourtyDay.OrderID {
			if expiresIn == fourthInterval_ExpectedExpiresIn {
				checkFourtyDayPurchase_FourtyFourDay = true
				continue
			}
			if expiresIn == lastInterval_ExpectedExpiresIn {
				checkFourtyDayPurchase_FourtyFiveDay = true
				continue
			}
		}
		if refID == notifiedUpToFourtyFourDays.OrderID && expiresIn == lastInterval_ExpectedExpiresIn {
			checkFourtyFourDayPurchase_FourtyFiveDay = true
		}
	}

	if checkNeverNotifiedPurchase_ZeroDay != true {
		t.Errorf("Expected notification missing: checkNeverNotifiedPurchase_ZeroDay")
	}
	if checkNeverNotifiedPurchase_FifteenDay != true {
		t.Errorf("Expected notification missing: checkNeverNotifiedPurchase_FifteenDay")
	}
	if checkNeverNotifiedPurchase_FourtyDay != true {
		t.Errorf("Expected notification missing: checkNeverNotifiedPurchase_FourtyDay")
	}
	if checkNeverNotifiedPurchase_FourtyFourDay != true {
		t.Errorf("Expected notification missing: checkNeverNotifiedPurchase_FourtyFourDay")
	}
	if checkNeverNotifiedPurchase_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkNeverNotifiedPurchase_FourtyFiveDay")
	}
	if checkZeroDayPurchase_FifteenDay != true {
		t.Errorf("Expected notification missing: checkZeroDayPurchase_FifteenDay")
	}
	if checkZeroDayPurchase_FourtyDay != true {
		t.Errorf("Expected notification missing: checkZeroDayPurchase_FourtyDay")
	}
	if checkZeroDayPurchase_FourtyFourDay != true {
		t.Errorf("Expected notification missing: checkZeroDayPurchase_FourtyFourDay")
	}
	if checkZeroDayPurchase_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkZeroDayPurchase_FourtyFiveDay")
	}
	if checkFifteenDayPurchase_FourtyDay != true {
		t.Errorf("Expected notification missing: checkFifteenDayPurchase_FourtyDay")
	}
	if checkFifteenDayPurchase_FourtyFourDay != true {
		t.Errorf("Expected notification missing: checkFifteenDayPurchase_FourtyFourDay")
	}
	if checkFifteenDayPurchase_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkFifteenDayPurchase_FourtyFiveDay")
	}
	if checkFourtyDayPurchase_FourtyFourDay != true {
		t.Errorf("Expected notification missing: checkFourtyDayPurchase_FourtyFourDay")
	}
	if checkFourtyDayPurchase_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkFourtyDayPurchase_FourtyFiveDay")
	}
	if checkFourtyFourDayPurchase_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkFourtyFourDayPurchase_FourtyFiveDay")
	}
}

// SALES
func TestPerformTaskCreatesSaleAgingNotifications(t *testing.T) {
	// Start each sale 50 days ago and have the lastNotifiedAt at a day after
	// each notification is suppose to be sent. With no notifications already queued,
	// it should produce all the old notifications up to the most recent one expected
	var (
		broadcastChannel = make(chan repo.Notifier, 0)
		timeStart        = time.Now().Add(time.Duration(-50*24) * time.Hour)
		twelveHours      = time.Duration(12) * time.Hour
		fourtyFiveDays   = time.Duration(45*24) * time.Hour

		// Produces notification for 45 days
		neverNotified = &repo.SaleRecord{
			OrderID:        "neverNotified",
			Timestamp:      timeStart,
			LastNotifiedAt: time.Unix(0, 0),
		}
		// Produces no notifications as all have already been created
		notifiedUpToFourtyFiveDays = &repo.SaleRecord{
			OrderID:        "notifiedUpToFourtyFiveDays",
			Timestamp:      timeStart,
			LastNotifiedAt: timeStart.Add(fourtyFiveDays + twelveHours),
		}
		existingRecords = []*repo.SaleRecord{
			neverNotified,
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
		_, err := database.Exec("insert into sales (orderID, timestamp, lastNotifiedAt) values (?, ?, ?)", r.OrderID, int(r.Timestamp.Unix()), int(r.LastNotifiedAt.Unix()))
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
				t.Logf("notification received: %s", notifier.GetType())
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

	worker.PerformTask()

	// Verify NotificationRecords in datastore
	rows, err := database.Query("select orderID, lastNotifiedAt from sales")
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
		case neverNotified.OrderID:
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
	if count != 1 {
		t.Errorf("Expected 1 notification to be produced, but found %d", count)
	}

	rows, err = database.Query("select notifID, serializedNotification, timestamp from notifications")
	if err != nil {
		t.Fatal(err)
	}

	var (
		checkNeverNotifiedSale_FourtyFiveDay bool
	)
	for rows.Next() {
		var (
			nID, nJSON string
			nTimestamp sql.NullInt64
			n          *repo.Notification
		)
		if err = rows.Scan(&nID, &nJSON, &nTimestamp); err != nil {
			t.Error(err)
			continue
		}
		if err := json.Unmarshal([]byte(nJSON), &n); err != nil {
			t.Error("Failed unmarshalling notification:", err.Error())
			continue
		}
		var (
			refID = n.NotifierData.(repo.SaleAgingNotification).OrderID
		)
		if refID == neverNotified.OrderID {
			if n.NotifierType == repo.NotifierTypeSaleAgedFourtyFiveDays {
				checkNeverNotifiedSale_FourtyFiveDay = true
				continue
			}
		}
	}

	if checkNeverNotifiedSale_FourtyFiveDay != true {
		t.Errorf("Expected notification missing: checkNeverNotifiedSale_FourtyFiveDay")
	}
}
