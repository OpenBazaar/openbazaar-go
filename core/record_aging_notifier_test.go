package core

import (
	"database/sql"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/op/go-logging"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
	wi "github.com/OpenBazaar/wallet-interface"
)

// DISPUTE CASES
func TestPerformTaskCreatesModeratorDisputeExpiryNotifications(t *testing.T) {
	// Start each case 50 days ago and have the lastDisputeExpiryNotifiedAt at a day after
	// each notification is suppose to be sent. With no notifications already queued,
	// it should produce all the old notifications up to the most recent one expected
	var (
		broadcastChannel = make(chan repo.Notifier, 0)
		timeStart        = time.Now().Add(time.Duration(-50*24) * time.Hour)
		twelveHours      = time.Duration(12) * time.Hour
		firstInterval    = repo.ModeratorDisputeExpiry_firstInterval
		secondInterval   = repo.ModeratorDisputeExpiry_secondInterval
		thirdInterval    = repo.ModeratorDisputeExpiry_thirdInterval
		lastInterval     = repo.ModeratorDisputeExpiry_lastInterval

		// Produces notification for 15, 40, 44 and 45 days
		neverNotified = &repo.DisputeCaseRecord{
			CaseID:                      "neverNotified",
			OrderState:                  pb.OrderState_DISPUTED,
			Timestamp:                   timeStart,
			LastDisputeExpiryNotifiedAt: timeStart.Add(twelveHours),
			BuyerContract: &pb.RicardianContract{
				VendorListings: []*pb.Listing{
					{Item: &pb.Listing_Item{Images: []*pb.Listing_Item_Image{{Tiny: "never-buyer-tinyimagehash", Small: "never-buyer-smallimagehash"}}}},
				},
			},
			VendorContract: &pb.RicardianContract{
				VendorListings: []*pb.Listing{
					{Item: &pb.Listing_Item{Images: []*pb.Listing_Item_Image{{}}}},
				},
			},
			IsBuyerInitiated: true,
		}
		// Produces notification for 40, 44 and 45 days
		notifiedUpToFifteenDay = &repo.DisputeCaseRecord{
			CaseID:                      "notifiedUpToFifteenDay",
			OrderState:                  pb.OrderState_DISPUTED,
			Timestamp:                   timeStart,
			LastDisputeExpiryNotifiedAt: timeStart.Add(firstInterval + twelveHours),
			BuyerContract: &pb.RicardianContract{
				VendorListings: []*pb.Listing{
					{Item: &pb.Listing_Item{Images: []*pb.Listing_Item_Image{{}}}},
				},
			},
			VendorContract: &pb.RicardianContract{
				VendorListings: []*pb.Listing{
					{Item: &pb.Listing_Item{Images: []*pb.Listing_Item_Image{{Tiny: "fifteen-vendor-tinyimagehash", Small: "fifteen-vendor-smallimagehash"}}}},
				},
			},
			IsBuyerInitiated: false,
		}
		// Produces notification for 44 and 45 days
		notifiedUpToFourtyDays = &repo.DisputeCaseRecord{
			CaseID:                      "notifiedUpToFourtyDay",
			OrderState:                  pb.OrderState_DISPUTED,
			Timestamp:                   timeStart,
			LastDisputeExpiryNotifiedAt: timeStart.Add(secondInterval + twelveHours),
			BuyerContract: &pb.RicardianContract{
				VendorListings: []*pb.Listing{
					{Item: &pb.Listing_Item{Images: []*pb.Listing_Item_Image{{Tiny: "fourty-buyer-tinyimagehash", Small: "fourty-buyer-smallimagehash"}}}},
				},
			},
			VendorContract: &pb.RicardianContract{
				VendorListings: []*pb.Listing{
					{Item: &pb.Listing_Item{Images: []*pb.Listing_Item_Image{{}}}},
				},
			},
			IsBuyerInitiated: true,
		}
		// Produces notification for 45 days
		notifiedUpToFourtyFourDays = &repo.DisputeCaseRecord{
			CaseID:                      "notifiedUpToFourtyFourDays",
			OrderState:                  pb.OrderState_DISPUTED,
			Timestamp:                   timeStart,
			LastDisputeExpiryNotifiedAt: timeStart.Add(thirdInterval + twelveHours),
			BuyerContract: &pb.RicardianContract{
				VendorListings: []*pb.Listing{
					{Item: &pb.Listing_Item{Images: []*pb.Listing_Item_Image{{}}}},
				},
			},
			VendorContract: &pb.RicardianContract{
				VendorListings: []*pb.Listing{
					{Item: &pb.Listing_Item{Images: []*pb.Listing_Item_Image{{Tiny: "fourtyfour-vendor-tinyimagehash", Small: "fourtyfour-vendor-smallimagehash"}}}},
				},
			},
			IsBuyerInitiated: false,
		}
		// Produces no notifications as all have already been created
		notifiedUpToFourtyFiveDays = &repo.DisputeCaseRecord{
			CaseID:                      "notifiedUpToFourtyFiveDays",
			OrderState:                  pb.OrderState_DISPUTED,
			Timestamp:                   timeStart,
			LastDisputeExpiryNotifiedAt: timeStart.Add(lastInterval + twelveHours),
			BuyerContract: &pb.RicardianContract{
				VendorListings: []*pb.Listing{
					{Item: &pb.Listing_Item{Images: []*pb.Listing_Item_Image{{}}}},
				},
			},
			VendorContract: &pb.RicardianContract{
				VendorListings: []*pb.Listing{
					{Item: &pb.Listing_Item{Images: []*pb.Listing_Item_Image{{}}}},
				},
			},
			IsBuyerInitiated: false,
		}
		existingRecords = []*repo.DisputeCaseRecord{
			neverNotified,
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
	defer appSchema.DestroySchemaDirectories()
	if err := appSchema.InitializeDatabase(); err != nil {
		t.Fatal(err)
	}

	database, err := appSchema.OpenDatabase()
	if err != nil {
		t.Fatal(err)
	}
	s, err := database.Prepare("insert into cases (caseID, state, buyerContract, vendorContract, timestamp, buyerOpened, lastDisputeExpiryNotifiedAt) values (?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		t.Fatal(err)
	}

	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	for _, r := range existingRecords {
		var isBuyerInitiated int // = 0
		if r.IsBuyerInitiated {
			isBuyerInitiated = 1
		}
		buyerContractData, err := m.MarshalToString(r.BuyerContract)
		if err != nil {
			t.Fatal(err)
		}
		vendorContractData, err := m.MarshalToString(r.VendorContract)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.Exec(r.CaseID, int(r.OrderState), buyerContractData, vendorContractData, int(r.Timestamp.Unix()), isBuyerInitiated, int(r.LastDisputeExpiryNotifiedAt.Unix()))
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
				t.Logf("notification received: %s", notifier)
				broadcastCount++ // += 1
			case <-closeAsyncChannelVerifier:
				return
			}
		}
	}()

	datastore := db.NewSQLiteDatastore(database, new(sync.Mutex), wi.Bitcoin)
	worker := &recordAgingNotifier{
		datastore: datastore,
		broadcast: broadcastChannel,
		logger:    logging.MustGetLogger("testRecordAgingNotifier"),
	}

	worker.PerformTask()

	// Verify Notifications received in channel
	closeAsyncChannelVerifier <- true
	if broadcastCount != 10 {
		t.Error("Expected 10 notifications to be broadcast, found", broadcastCount)
	}

	// Verify NotificationRecords in datastore
	rows, err := database.Query("select caseID, lastDisputeExpiryNotifiedAt from cases")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var (
			caseID                      string
			lastDisputeExpiryNotifiedAt int64
		)
		if err := rows.Scan(&caseID, &lastDisputeExpiryNotifiedAt); err != nil {
			t.Fatal(err)
		}
		switch caseID {
		case neverNotified.CaseID, notifiedUpToFifteenDay.CaseID, notifiedUpToFourtyDays.CaseID, notifiedUpToFourtyFourDays.CaseID:
			durationFromActual := time.Now().Sub(time.Unix(lastDisputeExpiryNotifiedAt, 0))
			if durationFromActual > (time.Duration(5) * time.Second) {
				t.Errorf("Expected %s to have lastDisputeExpiryNotifiedAt set when executed, was %s", caseID, time.Unix(lastDisputeExpiryNotifiedAt, 0).String())
			}
		case notifiedUpToFourtyFiveDays.CaseID:
			if lastDisputeExpiryNotifiedAt != notifiedUpToFourtyFiveDays.LastDisputeExpiryNotifiedAt.Unix() {
				t.Error("Expected notifiedUpToFourtyFiveDays to not update LastDisputeExpiryNotifiedAt")
			}
		default:
			t.Error("Unexpected dispute case")
		}
	}

	actualNotifications, count, err := datastore.Notifications().GetAll("", -1, []string{})
	if err != nil {
		t.Fatal(err)
	}

	if count != 10 {
		t.Errorf("Expected 10 notifications to be produced, but found %d", count)
	}

	var (
		checkNeverNotifiedFifteenDay               bool
		checkNeverNotifiedFourtyDay                bool
		checkNeverNotifiedFourtyFourDay            bool
		checkNeverNotifiedFourtyFiveDay            bool
		checkNotifiedToFifteenDaysFourtyDay        bool
		checkNotifiedToFifteenDaysFourtyFourDay    bool
		checkNotifiedToFifteenDaysFourtyFiveDay    bool
		checkNotifiedToFourtyDaysFourtyFourDay     bool
		checkNotifiedToFourtyDaysFourtyFiveDay     bool
		checkNotifiedToFourtyFourDaysFourtyFiveDay bool

		firstIntervalExpectedExpiresIn  = uint((repo.ModeratorDisputeExpiry_lastInterval - repo.ModeratorDisputeExpiry_firstInterval).Seconds())
		secondIntervalExpectedExpiresIn = uint((repo.ModeratorDisputeExpiry_lastInterval - repo.ModeratorDisputeExpiry_secondInterval).Seconds())
		thirdIntervalExpectedExpiresIn  = uint((repo.ModeratorDisputeExpiry_lastInterval - repo.ModeratorDisputeExpiry_thirdInterval).Seconds())
		lastIntervalExpectedExpiresIn   = uint(0)
	)

	for _, n := range actualNotifications {
		var (
			contract  *pb.RicardianContract
			thumbnail = n.NotifierData.(repo.ModeratorDisputeExpiry).Thumbnail
			refID     = n.NotifierData.(repo.ModeratorDisputeExpiry).CaseID
			expiresIn = n.NotifierData.(repo.ModeratorDisputeExpiry).ExpiresIn
		)
		if refID == neverNotified.CaseID {
			if neverNotified.IsBuyerInitiated {
				contract = neverNotified.BuyerContract
			} else {
				contract = neverNotified.VendorContract
			}
			assertThumbnailValuesAreSet(t, thumbnail, contract)
			if expiresIn == firstIntervalExpectedExpiresIn {
				checkNeverNotifiedFifteenDay = true
				continue
			}
			if expiresIn == secondIntervalExpectedExpiresIn {
				checkNeverNotifiedFourtyDay = true
				continue
			}
			if expiresIn == thirdIntervalExpectedExpiresIn {
				checkNeverNotifiedFourtyFourDay = true
				continue
			}
			if expiresIn == lastIntervalExpectedExpiresIn {
				checkNeverNotifiedFourtyFiveDay = true
				continue
			}
		}
		if refID == notifiedUpToFifteenDay.CaseID {
			if notifiedUpToFifteenDay.IsBuyerInitiated {
				contract = notifiedUpToFifteenDay.BuyerContract
			} else {
				contract = notifiedUpToFifteenDay.VendorContract
			}
			assertThumbnailValuesAreSet(t, thumbnail, contract)
			if expiresIn == secondIntervalExpectedExpiresIn {
				checkNotifiedToFifteenDaysFourtyDay = true
				continue
			}
			if expiresIn == thirdIntervalExpectedExpiresIn {
				checkNotifiedToFifteenDaysFourtyFourDay = true
				continue
			}
			if expiresIn == lastIntervalExpectedExpiresIn {
				checkNotifiedToFifteenDaysFourtyFiveDay = true
				continue
			}
		}
		if refID == notifiedUpToFourtyDays.CaseID {
			if notifiedUpToFourtyDays.IsBuyerInitiated {
				contract = notifiedUpToFourtyDays.BuyerContract
			} else {
				contract = notifiedUpToFourtyDays.VendorContract
			}
			assertThumbnailValuesAreSet(t, thumbnail, contract)
			if expiresIn == thirdIntervalExpectedExpiresIn {
				checkNotifiedToFourtyDaysFourtyFourDay = true
				continue
			}
			if expiresIn == lastIntervalExpectedExpiresIn {
				checkNotifiedToFourtyDaysFourtyFiveDay = true
				continue
			}
		}
		if refID == notifiedUpToFourtyFourDays.CaseID && expiresIn == lastIntervalExpectedExpiresIn {
			if notifiedUpToFourtyFourDays.IsBuyerInitiated {
				contract = notifiedUpToFourtyFourDays.BuyerContract
			} else {
				contract = notifiedUpToFourtyFourDays.VendorContract
			}
			assertThumbnailValuesAreSet(t, thumbnail, contract)
			checkNotifiedToFourtyFourDaysFourtyFiveDay = true
		}
	}

	if checkNeverNotifiedFifteenDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkNeverNotifiedFifteenDay")
	}
	if checkNeverNotifiedFourtyDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkNeverNotifiedFourtyDay")
	}
	if checkNeverNotifiedFourtyFourDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkNeverNotifiedFourtyFourDay")
	}
	if checkNeverNotifiedFourtyFiveDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkNeverNotifiedFourtyFiveDay")
	}
	if checkNotifiedToFifteenDaysFourtyDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkNotifiedToFifteenDaysFourtyDay")
	}
	if checkNotifiedToFifteenDaysFourtyFourDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkNotifiedToFifteenDaysFourtyFourDay")
	}
	if checkNotifiedToFifteenDaysFourtyFiveDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkNotifiedToFifteenDaysFourtyFiveDay")
	}
	if checkNotifiedToFourtyDaysFourtyFourDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkNotifiedToFourtyDaysFourtyFourDay")
	}
	if checkNotifiedToFourtyDaysFourtyFiveDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkNotifiedToFourtyDaysFourtyFiveDay")
	}
	if checkNotifiedToFourtyFourDaysFourtyFiveDay != true {
		t.Errorf("Expected dispute expiry notification missing: checkNotifiedToFourtyFourDaysFourtyFiveDay")
	}
}

// PURCHASES
func TestPerformTaskCreatesBuyerDisputeTimeoutNotifications(t *testing.T) {
	// Start each purchase 50 days ago and have the lastDisputeTimeoutNotifiedAt at a day after
	// each notification is suppose to be sent. With no notifications already queued,
	// it should produce all the old notifications up to the most recent one expected
	var (
		broadcastChannel = make(chan repo.Notifier, 0)
		timeStart        = time.Now().Add(time.Duration(-50*24) * time.Hour)
		twelveHours      = time.Duration(12) * time.Hour
		firstInterval    = repo.BuyerDisputeTimeout_firstInterval
		secondInterval   = repo.BuyerDisputeTimeout_secondInterval
		thirdInterval    = repo.BuyerDisputeTimeout_thirdInterval
		lastInterval     = repo.BuyerDisputeTimeout_lastInterval

		// Produces no notifications as contract is undisputeable
		neverNotifiedButUndisputeable = &repo.PurchaseRecord{
			Contract:                     factory.NewUndisputeableContract(),
			OrderID:                      "neverNotifiedButUndisputed",
			OrderState:                   pb.OrderState_PENDING,
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: time.Unix(0, 0),
		}
		// Produces notification for 15, 40, 44 and 45 days
		neverNotified = &repo.PurchaseRecord{
			Contract:                     factory.NewDisputeableContract(),
			OrderID:                      "neverNotified",
			OrderState:                   pb.OrderState_PENDING,
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: time.Unix(0, 0),
		}
		// Produces notification for 40, 44 and 45 days
		notifiedUpToFifteenDay = &repo.PurchaseRecord{
			Contract:                     factory.NewDisputeableContract(),
			OrderID:                      "notifiedUpToFifteenDay",
			OrderState:                   pb.OrderState_PENDING,
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: timeStart.Add(firstInterval + twelveHours),
		}
		// Produces notification for 44 and 45 days
		notifiedUpToFourtyDay = &repo.PurchaseRecord{
			Contract:                     factory.NewDisputeableContract(),
			OrderID:                      "notifiedUpToFourtyDay",
			OrderState:                   pb.OrderState_PENDING,
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: timeStart.Add(secondInterval + twelveHours),
		}
		// Produces notification for 45 days
		notifiedUpToFourtyFourDays = &repo.PurchaseRecord{
			Contract:                     factory.NewDisputeableContract(),
			OrderID:                      "notifiedUpToFourtyFourDays",
			OrderState:                   pb.OrderState_PENDING,
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: timeStart.Add(thirdInterval + twelveHours),
		}
		// Produces no notifications as all have already been created
		notifiedUpToFourtyFiveDays = &repo.PurchaseRecord{
			Contract:                     factory.NewDisputeableContract(),
			OrderID:                      "notifiedUpToFourtyFiveDays",
			OrderState:                   pb.OrderState_PENDING,
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: timeStart.Add(lastInterval + twelveHours),
		}
		existingRecords = []*repo.PurchaseRecord{
			neverNotifiedButUndisputeable,
			neverNotified,
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
	neverNotified.Contract.VendorListings[0].Item.Images = []*pb.Listing_Item_Image{{Tiny: "never-tinyimagehashOne", Small: "never-smallimagehashOne"}}
	notifiedUpToFifteenDay.Contract.VendorListings[0].Item.Images = []*pb.Listing_Item_Image{{Tiny: "fifteen-tinyimagehashOne", Small: "fifteen-smallimagehashOne"}}
	notifiedUpToFourtyDay.Contract.VendorListings[0].Item.Images = []*pb.Listing_Item_Image{{Tiny: "fourty-tinyimagehashOne", Small: "fourty-smallimagehashOne"}}
	notifiedUpToFourtyFourDays.Contract.VendorListings[0].Item.Images = []*pb.Listing_Item_Image{{Tiny: "fourtyfour-tinyimagehashOne", Small: "fourtyfour-smallimagehashOne"}}
	notifiedUpToFourtyFiveDays.Contract.VendorListings[0].Item.Images = []*pb.Listing_Item_Image{{Tiny: "fourtyfive-tinyimagehashOne", Small: "fourtyfive-smallimagehashOne"}}

	if err := appSchema.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer appSchema.DestroySchemaDirectories()
	if err := appSchema.InitializeDatabase(); err != nil {
		t.Fatal(err)
	}
	database, err := appSchema.OpenDatabase()
	if err != nil {
		t.Fatal(err)
	}

	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	for _, r := range existingRecords {
		contractData, err := m.MarshalToString(r.Contract)
		if err != nil {
			t.Fatal(err)
		}
		_, err = database.Exec("insert into purchases (orderID, contract, state, timestamp, lastDisputeTimeoutNotifiedAt) values (?, ?, ?, ?, ?)", r.OrderID, contractData, int(r.OrderState), int(r.Timestamp.Unix()), int(r.LastDisputeTimeoutNotifiedAt.Unix()))
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
				broadcastCount++ // += 1
			case <-closeAsyncChannelVerifier:
				return
			}
		}
	}()

	datastore := db.NewSQLiteDatastore(database, new(sync.Mutex), wi.Bitcoin)
	worker := &recordAgingNotifier{
		datastore: datastore,
		broadcast: broadcastChannel,
		logger:    logging.MustGetLogger("testRecordAgingNotifier"),
	}

	worker.PerformTask()

	// Verify Notifications received in channel
	closeAsyncChannelVerifier <- true
	if broadcastCount != 10 {
		t.Error("Expected 10 notifications to be broadcast, found", broadcastCount)
	}

	// Verify NotificationRecords in datastore
	rows, err := database.Query("select orderID, lastDisputeTimeoutNotifiedAt from purchases")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var (
			orderID                      string
			lastDisputeTimeoutNotifiedAt int64
		)
		if err := rows.Scan(&orderID, &lastDisputeTimeoutNotifiedAt); err != nil {
			t.Fatal(err)
		}
		switch orderID {
		case neverNotified.OrderID, notifiedUpToFifteenDay.OrderID, notifiedUpToFourtyDay.OrderID, notifiedUpToFourtyFourDays.OrderID:
			durationFromActual := time.Now().Sub(time.Unix(lastDisputeTimeoutNotifiedAt, 0))
			if durationFromActual > (time.Duration(5) * time.Second) {
				t.Errorf("Expected %s to have lastDisputeTimeoutNotifiedAt set when executed, was %s", orderID, time.Unix(lastDisputeTimeoutNotifiedAt, 0).String())
			}
		case notifiedUpToFourtyFiveDays.OrderID:
			if lastDisputeTimeoutNotifiedAt != notifiedUpToFourtyFiveDays.LastDisputeTimeoutNotifiedAt.Unix() {
				t.Error("Expected notifiedUpToFourtyFiveDays to not update LastDisputeExpiredNotifiedAt")
			}
		case neverNotifiedButUndisputeable.OrderID:
			if lastDisputeTimeoutNotifiedAt != neverNotifiedButUndisputeable.LastDisputeTimeoutNotifiedAt.Unix() {
				t.Error("Expected notifiedUpToFourtyFiveDays to not update LastDisputeExpiredNotifiedAt")
			}
		default:
			t.Error("Unexpected purchase")
		}
	}

	var count int64
	err = database.QueryRow("select count(*) from notifications").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 10 {
		t.Errorf("Expected 10 notifications to be produced, but found %d", count)
	}

	rows, err = database.Query("select notifID, serializedNotification, timestamp from notifications")
	if err != nil {
		t.Fatal(err)
	}

	var (
		checkNeverNotifiedPurchaseFirstNotificationSeen  bool
		checkNeverNotifiedPurchaseSecondNotificationSeen bool
		checkNeverNotifiedPurchaseThirdNotificationSeen  bool
		checkNeverNotifiedPurchaseLastNotificationSeen   bool
		checkFifteenDayPurchaseSecondNotificationSeen    bool
		checkFifteenDayPurchaseThirdNotificationSeen     bool
		checkFifteenDayPurchaseLastNotificationSeen      bool
		checkFourtyDayPurchaseThirdNotificationSeen      bool
		checkFourtyDayPurchaseLastNotificationSeen       bool
		checkFourtyFourDayPurchaseLastNotificationSeen   bool

		firstIntervalExpectedExpiresIn  = uint((repo.BuyerDisputeTimeout_totalDuration - repo.BuyerDisputeTimeout_firstInterval).Seconds())
		secondIntervalExpectedExpiresIn = uint((repo.BuyerDisputeTimeout_totalDuration - repo.BuyerDisputeTimeout_secondInterval).Seconds())
		thirdIntervalExpectedExpiresIn  = uint((repo.BuyerDisputeTimeout_totalDuration - repo.BuyerDisputeTimeout_thirdInterval).Seconds())
		lastIntervalExpectedExpiresIn   = uint((repo.BuyerDisputeTimeout_totalDuration - repo.BuyerDisputeTimeout_lastInterval).Seconds())
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
			thumbnail = n.NotifierData.(repo.BuyerDisputeTimeout).Thumbnail
		)
		if refID == neverNotified.OrderID {
			assertThumbnailValuesAreSet(t, thumbnail, neverNotified.Contract)
			if expiresIn == firstIntervalExpectedExpiresIn {
				checkNeverNotifiedPurchaseFirstNotificationSeen = true
				continue
			}
			if expiresIn == secondIntervalExpectedExpiresIn {
				checkNeverNotifiedPurchaseSecondNotificationSeen = true
				continue
			}
			if expiresIn == thirdIntervalExpectedExpiresIn {
				checkNeverNotifiedPurchaseThirdNotificationSeen = true
				continue
			}
			if expiresIn == lastIntervalExpectedExpiresIn {
				checkNeverNotifiedPurchaseLastNotificationSeen = true
				continue
			}
		}
		if refID == notifiedUpToFifteenDay.OrderID {
			assertThumbnailValuesAreSet(t, thumbnail, notifiedUpToFifteenDay.Contract)
			if expiresIn == secondIntervalExpectedExpiresIn {
				checkFifteenDayPurchaseSecondNotificationSeen = true
				continue
			}
			if expiresIn == thirdIntervalExpectedExpiresIn {
				checkFifteenDayPurchaseThirdNotificationSeen = true
				continue
			}
			if expiresIn == lastIntervalExpectedExpiresIn {
				checkFifteenDayPurchaseLastNotificationSeen = true
				continue
			}
		}
		if refID == notifiedUpToFourtyDay.OrderID {
			assertThumbnailValuesAreSet(t, thumbnail, notifiedUpToFourtyDay.Contract)
			if expiresIn == thirdIntervalExpectedExpiresIn {
				checkFourtyDayPurchaseThirdNotificationSeen = true
				continue
			}
			if expiresIn == lastIntervalExpectedExpiresIn {
				checkFourtyDayPurchaseLastNotificationSeen = true
				continue
			}
		}
		if refID == notifiedUpToFourtyFourDays.OrderID && expiresIn == lastIntervalExpectedExpiresIn {
			assertThumbnailValuesAreSet(t, thumbnail, notifiedUpToFourtyFourDays.Contract)
			checkFourtyFourDayPurchaseLastNotificationSeen = true
		}
	}

	if checkNeverNotifiedPurchaseFirstNotificationSeen != true {
		t.Errorf("Expected notification missing: checkNeverNotifiedPurchaseFirstNotificationSeen")
	}
	if checkNeverNotifiedPurchaseSecondNotificationSeen != true {
		t.Errorf("Expected notification missing: checkNeverNotifiedPurchaseSecondNotificationSeen")
	}
	if checkNeverNotifiedPurchaseThirdNotificationSeen != true {
		t.Errorf("Expected notification missing: checkNeverNotifiedPurchaseThirdNotificationSeen")
	}
	if checkNeverNotifiedPurchaseLastNotificationSeen != true {
		t.Errorf("Expected notification missing: checkNeverNotifiedPurchaseLastNotificationSeen")
	}
	if checkFifteenDayPurchaseSecondNotificationSeen != true {
		t.Errorf("Expected notification missing: checkFifteenDayPurchaseSecondNotificationSeen")
	}
	if checkFifteenDayPurchaseThirdNotificationSeen != true {
		t.Errorf("Expected notification missing: checkFifteenDayPurchaseThirdNotificationSeen")
	}
	if checkFifteenDayPurchaseLastNotificationSeen != true {
		t.Errorf("Expected notification missing: checkFifteenDayPurchaseLastNotificationSeen")
	}
	if checkFourtyDayPurchaseThirdNotificationSeen != true {
		t.Errorf("Expected notification missing: checkFourtyDayPurchaseThirdNotificationSeen")
	}
	if checkFourtyDayPurchaseLastNotificationSeen != true {
		t.Errorf("Expected notification missing: checkFourtyDayPurchaseLastNotificationSeen")
	}
	if checkFourtyFourDayPurchaseLastNotificationSeen != true {
		t.Errorf("Expected notification missing: checkFourtyFourDayPurchaseLastNotificationSeen")
	}
}

func TestPerformTaskCreatesPurchaseExpiryNotifications(t *testing.T) {
	// Start each purchase 50 days ago and have the LastDisputeExpiryNotifiedAt at a day after
	// each notification is suppose to be sent. With no notifications already queued,
	// it should produce all the old notifications up to the most recent one expected
	var (
		broadcastChannel = make(chan repo.Notifier, 0)
		timeStart        = time.Now().Add(time.Duration(-50*24) * time.Hour)
		twelveHours      = time.Duration(12) * time.Hour
		firstInterval    = repo.BuyerDisputeExpiry_firstInterval
		secondInterval   = repo.BuyerDisputeExpiry_secondInterval
		lastInterval     = repo.BuyerDisputeExpiry_lastInterval

		// Produces no notifications as state is PENDING and not disputed
		neverNotifiedButUndisputeable = &repo.PurchaseRecord{
			Contract:                    factory.NewUndisputeableContract(),
			OrderID:                     "neverNotifiedButUndisputed",
			OrderState:                  pb.OrderState_PENDING,
			Timestamp:                   timeStart,
			DisputedAt:                  time.Unix(0, 0),
			LastDisputeExpiryNotifiedAt: time.Unix(0, 0),
		}
		// Produces notification for 15, 40 and 44 days
		neverNotified = &repo.PurchaseRecord{
			Contract:                    factory.NewDisputeableContract(),
			OrderID:                     "neverNotified",
			OrderState:                  pb.OrderState_DISPUTED,
			Timestamp:                   timeStart,
			DisputedAt:                  timeStart,
			LastDisputeExpiryNotifiedAt: time.Unix(0, 0),
		}
		// Produces notification for 40 and 44 days
		notifiedUpToFifteenDay = &repo.PurchaseRecord{
			Contract:                    factory.NewDisputeableContract(),
			OrderID:                     "notifiedUpToFifteenDay",
			OrderState:                  pb.OrderState_DISPUTED,
			Timestamp:                   timeStart,
			DisputedAt:                  timeStart,
			LastDisputeExpiryNotifiedAt: timeStart.Add(firstInterval + twelveHours),
		}
		// Produces notification for 44 days
		notifiedUpToFourtyDay = &repo.PurchaseRecord{
			Contract:                    factory.NewDisputeableContract(),
			OrderID:                     "notifiedUpToFourtyDay",
			OrderState:                  pb.OrderState_DISPUTED,
			Timestamp:                   timeStart,
			DisputedAt:                  timeStart,
			LastDisputeExpiryNotifiedAt: timeStart.Add(secondInterval + twelveHours),
		}
		// Produces no notifications as all have already been created
		notifiedUpToFourtyFiveDays = &repo.PurchaseRecord{
			Contract:                    factory.NewDisputeableContract(),
			OrderID:                     "notifiedUpToFourtyFiveDays",
			OrderState:                  pb.OrderState_DISPUTED,
			Timestamp:                   timeStart,
			DisputedAt:                  timeStart,
			LastDisputeExpiryNotifiedAt: timeStart.Add(lastInterval + twelveHours),
		}
		existingRecords = []*repo.PurchaseRecord{
			neverNotifiedButUndisputeable,
			neverNotified,
			notifiedUpToFifteenDay,
			notifiedUpToFourtyDay,
			notifiedUpToFourtyFiveDays,
		}

		appSchema = schema.MustNewCustomSchemaManager(schema.SchemaContext{
			DataPath:        schema.GenerateTempPath(),
			TestModeEnabled: true,
		})
	)
	neverNotified.Contract.VendorListings[0].Item.Images = []*pb.Listing_Item_Image{{Tiny: "never-tinyimagehashOne", Small: "never-smallimagehashOne"}}
	notifiedUpToFifteenDay.Contract.VendorListings[0].Item.Images = []*pb.Listing_Item_Image{{Tiny: "fifteen-tinyimagehashOne", Small: "fifteen-smallimagehashOne"}}
	notifiedUpToFourtyDay.Contract.VendorListings[0].Item.Images = []*pb.Listing_Item_Image{{Tiny: "fourty-tinyimagehashOne", Small: "fourty-smallimagehashOne"}}
	notifiedUpToFourtyFiveDays.Contract.VendorListings[0].Item.Images = []*pb.Listing_Item_Image{{Tiny: "fourtyfive-tinyimagehashOne", Small: "fourtyfive-smallimagehashOne"}}

	if err := appSchema.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer appSchema.DestroySchemaDirectories()
	if err := appSchema.InitializeDatabase(); err != nil {
		t.Fatal(err)
	}
	database, err := appSchema.OpenDatabase()
	if err != nil {
		t.Fatal(err)
	}

	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	for _, r := range existingRecords {
		contractData, err := m.MarshalToString(r.Contract)
		if err != nil {
			t.Fatal(err)
		}
		_, err = database.Exec("insert into purchases (orderID, contract, state, timestamp, lastDisputeExpiryNotifiedAt, disputedAt) values (?, ?, ?, ?, ?, ?)", r.OrderID, contractData, int(r.OrderState), int(r.Timestamp.Unix()), int(r.LastDisputeExpiryNotifiedAt.Unix()), int(r.DisputedAt.Unix()))
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
				if notifier.GetType() == repo.NotifierTypeBuyerDisputeExpiry {
					broadcastCount++ // += 1
					t.Logf("Notification Recieved: %+v\n", notifier)
				} else {
					t.Errorf("Unexpected notification received: %s", notifier.GetType())
				}
			case <-closeAsyncChannelVerifier:
				return
			}
		}
	}()

	datastore := db.NewSQLiteDatastore(database, new(sync.Mutex), wi.Bitcoin)
	worker := &recordAgingNotifier{
		datastore: datastore,
		broadcast: broadcastChannel,
		logger:    logging.MustGetLogger("testRecordAgingNotifier"),
	}

	worker.PerformTask()

	// Verify Notifications received in channel
	closeAsyncChannelVerifier <- true
	if broadcastCount != 6 {
		t.Error("Expected 6 notifications to be broadcast, found", broadcastCount)
	}

	// Verify NotificationRecords in datastore
	rows, err := database.Query("select orderID, lastDisputeExpiryNotifiedAt from purchases")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var (
			orderID                     string
			lastDisputeExpiryNotifiedAt int64
		)
		if err := rows.Scan(&orderID, &lastDisputeExpiryNotifiedAt); err != nil {
			t.Fatal(err)
		}
		switch orderID {
		case neverNotified.OrderID, notifiedUpToFifteenDay.OrderID, notifiedUpToFourtyDay.OrderID:
			durationFromActual := time.Now().Sub(time.Unix(lastDisputeExpiryNotifiedAt, 0))
			if durationFromActual > (time.Duration(5) * time.Second) {
				t.Errorf("Expected %s to have lastDisputeExpiryNotifiedAt set when executed, was %s", orderID, time.Unix(lastDisputeExpiryNotifiedAt, 0).String())
			}
		case notifiedUpToFourtyFiveDays.OrderID:
			if lastDisputeExpiryNotifiedAt != notifiedUpToFourtyFiveDays.LastDisputeExpiryNotifiedAt.Unix() {
				t.Error("Expected notifiedUpToFourtyFiveDays to not update LastDisputeExpiredNotifiedAt")
			}
		case neverNotifiedButUndisputeable.OrderID:
			if lastDisputeExpiryNotifiedAt != neverNotifiedButUndisputeable.LastDisputeExpiryNotifiedAt.Unix() {
				t.Error("Expected notifiedUpToFourtyFiveDays to not update LastDisputeExpiredNotifiedAt")
			}
		default:
			t.Error("Unexpected purchase")
		}
	}

	var count int64
	err = database.QueryRow("select count(*) from notifications").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 6 {
		t.Errorf("Expected 6 notifications to be produced, but found %d", count)
	}

	rows, err = database.Query("select notifID, serializedNotification, timestamp from notifications")
	if err != nil {
		t.Fatal(err)
	}

	var (
		checkNeverNotifiedPurchaseFirstNotificationSeen  bool
		checkNeverNotifiedPurchaseSecondNotificationSeen bool
		checkNeverNotifiedPurchaseLastNotificationSeen   bool
		checkFifteenDayPurchaseSecondNotificationSeen    bool
		checkFifteenDayPurchaseLastNotificationSeen      bool
		checkFourtyDayPurchaseLastNotificationSeen       bool

		firstIntervalExpectedExpiresIn  = uint((repo.BuyerDisputeExpiry_totalDuration - repo.BuyerDisputeExpiry_firstInterval).Seconds())
		secondIntervalExpectedExpiresIn = uint((repo.BuyerDisputeExpiry_totalDuration - repo.BuyerDisputeExpiry_secondInterval).Seconds())
		lastIntervalExpectedExpiresIn   = uint((repo.BuyerDisputeExpiry_totalDuration - repo.BuyerDisputeExpiry_lastInterval).Seconds())
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
			refID     = n.NotifierData.(repo.BuyerDisputeExpiry).OrderID
			expiresIn = n.NotifierData.(repo.BuyerDisputeExpiry).ExpiresIn
			thumbnail = n.NotifierData.(repo.BuyerDisputeExpiry).Thumbnail
		)
		if refID == neverNotified.OrderID {
			assertThumbnailValuesAreSet(t, thumbnail, neverNotified.Contract)
			if expiresIn == firstIntervalExpectedExpiresIn {
				checkNeverNotifiedPurchaseFirstNotificationSeen = true
				continue
			}
			if expiresIn == secondIntervalExpectedExpiresIn {
				checkNeverNotifiedPurchaseSecondNotificationSeen = true
				continue
			}
			if expiresIn == lastIntervalExpectedExpiresIn {
				checkNeverNotifiedPurchaseLastNotificationSeen = true
				continue
			}
		}
		if refID == notifiedUpToFifteenDay.OrderID {
			assertThumbnailValuesAreSet(t, thumbnail, notifiedUpToFifteenDay.Contract)
			if expiresIn == secondIntervalExpectedExpiresIn {
				checkFifteenDayPurchaseSecondNotificationSeen = true
				continue
			}
			if expiresIn == lastIntervalExpectedExpiresIn {
				checkFifteenDayPurchaseLastNotificationSeen = true
				continue
			}
		}
		if refID == notifiedUpToFourtyDay.OrderID {
			assertThumbnailValuesAreSet(t, thumbnail, notifiedUpToFourtyDay.Contract)
			if expiresIn == lastIntervalExpectedExpiresIn {
				checkFourtyDayPurchaseLastNotificationSeen = true
				continue
			}
		}
	}

	if checkNeverNotifiedPurchaseFirstNotificationSeen != true {
		t.Errorf("Expected notification missing: checkNeverNotifiedPurchaseFirstNotificationSeen")
	}
	if checkNeverNotifiedPurchaseSecondNotificationSeen != true {
		t.Errorf("Expected notification missing: checkNeverNotifiedPurchaseSecondNotificationSeen")
	}
	if checkNeverNotifiedPurchaseLastNotificationSeen != true {
		t.Errorf("Expected notification missing: checkNeverNotifiedPurchaseLastNotificationSeen")
	}
	if checkFifteenDayPurchaseSecondNotificationSeen != true {
		t.Errorf("Expected notification missing: checkFifteenDayPurchaseSecondNotificationSeen")
	}
	if checkFifteenDayPurchaseLastNotificationSeen != true {
		t.Errorf("Expected notification missing: checkFifteenDayPurchaseLastNotificationSeen")
	}
	if checkFourtyDayPurchaseLastNotificationSeen != true {
		t.Errorf("Expected notification missing: checkFourtyDayPurchaseLastNotificationSeen")
	}
}

// SALES
func TestPerformTaskCreatesVendorDisputeTimeoutNotifications(t *testing.T) {
	// Start each sale 50 days ago and have the lastDisputeTimeoutNotifiedAt at a day after
	// each notification is suppose to be sent. With no notifications already queued,
	// it should produce all the old notifications up to the most recent one expected
	var (
		broadcastChannel = make(chan repo.Notifier, 0)
		timeStart        = time.Now().Add(time.Duration(-50*24) * time.Hour)
		twelveHours      = time.Duration(12) * time.Hour
		lastInterval     = repo.VendorDisputeTimeout_lastInterval

		// Produces notification for 45 days
		neverNotified = &repo.SaleRecord{
			Contract:                     factory.NewDisputeableContract(),
			OrderID:                      "neverNotified",
			OrderState:                   pb.OrderState_FULFILLED,
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: time.Unix(0, 0),
		}
		// Produces no notifications as all have already been created
		notifiedUpToFourtyFiveDays = &repo.SaleRecord{
			Contract:                     factory.NewDisputeableContract(),
			OrderID:                      "notifiedUpToFourtyFiveDays",
			OrderState:                   pb.OrderState_FULFILLED,
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: timeStart.Add(lastInterval + twelveHours),
		}
		// Produces no notifications as contract is undisputeable
		neverNotifiedButUndisputeable = &repo.SaleRecord{
			Contract:                     factory.NewUndisputeableContract(),
			OrderID:                      "neverNotifiedButUndisputeable",
			OrderState:                   pb.OrderState_FULFILLED,
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: time.Unix(0, 0),
		}
		existingRecords = []*repo.SaleRecord{
			neverNotified,
			notifiedUpToFourtyFiveDays,
			neverNotifiedButUndisputeable,
		}

		appSchema = schema.MustNewCustomSchemaManager(schema.SchemaContext{
			DataPath:        schema.GenerateTempPath(),
			TestModeEnabled: true,
		})
	)
	neverNotified.Contract.VendorListings[0].Item.Images = []*pb.Listing_Item_Image{{Tiny: "never-tinyimagehashOne", Small: "never-smallimagehashOne"}}
	notifiedUpToFourtyFiveDays.Contract.VendorListings[0].Item.Images = []*pb.Listing_Item_Image{{Tiny: "fourtyfive-tinyimagehashOne", Small: "fourtyfive-smallimagehashOne"}}

	if err := appSchema.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer appSchema.DestroySchemaDirectories()
	if err := appSchema.InitializeDatabase(); err != nil {
		t.Fatal(err)
	}
	database, err := appSchema.OpenDatabase()
	if err != nil {
		t.Fatal(err)
	}

	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	for _, r := range existingRecords {
		contractData, err := m.MarshalToString(r.Contract)
		if err != nil {
			t.Fatal(err)
		}
		_, err = database.Exec("insert into sales (orderID, contract, state, timestamp, lastDisputeTimeoutNotifiedAt) values (?, ?, ?, ?, ?)", r.OrderID, contractData, int(r.OrderState), int(r.Timestamp.Unix()), int(r.LastDisputeTimeoutNotifiedAt.Unix()))
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
				t.Logf("notification received: %s", notifier)
				broadcastCount++ // += 1
			case <-closeAsyncChannelVerifier:
				return
			}
		}
	}()

	datastore := db.NewSQLiteDatastore(database, new(sync.Mutex), wi.Bitcoin)
	worker := &recordAgingNotifier{
		datastore: datastore,
		broadcast: broadcastChannel,
		logger:    logging.MustGetLogger("testRecordAgingNotifier"),
	}

	worker.PerformTask()

	// Verify Notifications received in channel
	closeAsyncChannelVerifier <- true
	if broadcastCount != 1 {
		t.Error("Expected 1 notifications to be broadcast, found", broadcastCount)
	}

	// Verify NotificationRecords in datastore
	rows, err := database.Query("select orderID, lastDisputeTimeoutNotifiedAt from sales")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var (
			orderID                      string
			lastDisputeTimeoutNotifiedAt int64
		)
		if err := rows.Scan(&orderID, &lastDisputeTimeoutNotifiedAt); err != nil {
			t.Fatal(err)
		}
		switch orderID {
		case neverNotified.OrderID:
			durationFromActual := time.Now().Sub(time.Unix(lastDisputeTimeoutNotifiedAt, 0))
			if durationFromActual > (time.Duration(5) * time.Second) {
				t.Errorf("Expected %s to have lastDisputeTimeoutNotifiedAt set when executed, was %s", orderID, time.Unix(lastDisputeTimeoutNotifiedAt, 0).String())
			}
		case notifiedUpToFourtyFiveDays.OrderID:
			if lastDisputeTimeoutNotifiedAt != notifiedUpToFourtyFiveDays.LastDisputeTimeoutNotifiedAt.Unix() {
				t.Error("Expected notifiedUpToFourtyFiveDays to not update LastDisputeTimeoutNotifiedAt")
			}
		case neverNotifiedButUndisputeable.OrderID:
			if lastDisputeTimeoutNotifiedAt != neverNotifiedButUndisputeable.LastDisputeTimeoutNotifiedAt.Unix() {
				t.Error("Expected notifiedUpToFourtyFiveDays to not update LastDisputeTimeoutNotifiedAt")
			}
		default:
			t.Error("Unexpected sale")
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
		checkNeverNotifiedSaleLastNotificationSeen bool

		firstIntervalExpectedExpiresIn = uint(0)
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
			refID     = n.NotifierData.(repo.VendorDisputeTimeout).OrderID
			expiresIn = n.NotifierData.(repo.VendorDisputeTimeout).ExpiresIn
			thumbnail = n.NotifierData.(repo.VendorDisputeTimeout).Thumbnail
		)
		if refID == neverNotified.OrderID && expiresIn == firstIntervalExpectedExpiresIn {
			assertThumbnailValuesAreSet(t, thumbnail, neverNotified.Contract)
			checkNeverNotifiedSaleLastNotificationSeen = true
			continue
		}
	}

	if checkNeverNotifiedSaleLastNotificationSeen != true {
		t.Errorf("Expected notification missing: checkNeverNotifiedSale_LastNotificationSeen")
	}
}

func assertThumbnailValuesAreSet(t *testing.T, actualThumbnails repo.Thumbnail, contract *pb.RicardianContract) {
	if len(contract.VendorListings) == 0 {
		t.Error("Expected contract to have VendorListings but was empty. Unable to assert Thumbnail values.")
		return
	}
	if len(contract.VendorListings[0].Item.Images) == 0 {
		t.Error("Expected contract to have Item Images but was empty. Unable to assert Thumbnail values.")
		return
	}
	var (
		expectedTinyThumbnail  = contract.VendorListings[0].Item.Images[0].Tiny
		expectedSmallThumbnail = contract.VendorListings[0].Item.Images[0].Small
	)
	if expectedTinyThumbnail != actualThumbnails.Tiny {
		t.Error("Expected notification to include the tiny thumbnail")
		t.Error("Actual:", actualThumbnails.Tiny, "Expected:", expectedTinyThumbnail)
		t.Logf("Contract: %+v\n", contract)
	}
	if expectedSmallThumbnail != actualThumbnails.Small {
		t.Error("Expected notification to include the small thumbnail")
		t.Error("Actual:", actualThumbnails.Small, "Expected:", expectedSmallThumbnail)
		t.Logf("Contract: %+v\n", contract)
	}
}
