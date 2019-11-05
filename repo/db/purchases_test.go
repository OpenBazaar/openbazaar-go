package db_test

import (
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/golang/protobuf/ptypes"
)

func buildNewPurchaseStore() (repo.PurchaseStore, func(), error) {
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
	return db.NewPurchaseStore(database, new(sync.Mutex)), appSchema.DestroySchemaDirectories, nil
}

func TestPurchasesDB_Count(t *testing.T) {
	purdb, teardown, err := buildNewPurchaseStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	contract := factory.NewContract()
	err = purdb.Put("orderID", *contract, 0, false)
	if err != nil {
		t.Error(err)
	}
	i := purdb.Count()
	if i != 1 {
		t.Error("Returned incorrect number of purchases")
	}
}

func TestPutPurchase(t *testing.T) {
	purdb, teardown, err := buildNewPurchaseStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	contract := factory.NewContract()
	err = purdb.Put("orderID", *contract, 0, false)
	if err != nil {
		t.Error(err)
	}
	stmt, _ := purdb.PrepareQuery("select orderID, contract, state, read, timestamp, total, thumbnail, vendorID, vendorHandle, title, shippingName, shippingAddress from purchases where orderID=?")
	defer stmt.Close()

	var orderID string
	var c []byte
	var state int
	var read int
	var date int
	var total string
	var thumbnail string
	var vendorID string
	var vendorHandle string
	var title string
	var shippingName string
	var shippingAddress string
	err = stmt.QueryRow("orderID").Scan(&orderID, &c, &state, &read, &date, &total, &thumbnail, &vendorID, &vendorHandle, &title, &shippingName, &shippingAddress)
	if err != nil {
		t.Error(err)
	}
	if orderID != "orderID" {
		t.Errorf(`Expected %s got %s`, "orderID", orderID)
	}
	if state != 0 {
		t.Errorf(`Expected 0 got %d`, state)
	}
	if read != 0 {
		t.Errorf(`Expected 0 got %d`, read)
	}
	if date != int(contract.BuyerOrder.Timestamp.Seconds) {
		t.Errorf("Expected %d got %d", int(contract.BuyerOrder.Timestamp.Seconds), date)
	}
	if total != contract.BuyerOrder.Payment.BigAmount {
		t.Errorf("Expected %s got %s", contract.BuyerOrder.Payment.BigAmount, total)
	}
	if thumbnail != contract.VendorListings[0].Item.Images[0].Tiny {
		t.Errorf("Expected %s got %s", contract.VendorListings[0].Item.Images[0].Tiny, thumbnail)
	}
	if vendorID != contract.VendorListings[0].VendorID.PeerID {
		t.Errorf(`Expected %s got %s`, contract.VendorListings[0].VendorID.PeerID, vendorID)
	}
	if vendorHandle != contract.VendorListings[0].VendorID.Handle {
		t.Errorf(`Expected %s got %s`, contract.VendorListings[0].VendorID.Handle, vendorHandle)
	}
	if title != contract.VendorListings[0].Item.Title {
		t.Errorf(`Expected %s got %s`, contract.VendorListings[0].Item.Title, title)
	}
	if shippingName != contract.BuyerOrder.Shipping.ShipTo {
		t.Errorf(`Expected %s got %s`, contract.BuyerOrder.Shipping.ShipTo, shippingName)
	}
	if shippingAddress != contract.BuyerOrder.Shipping.Address {
		t.Errorf(`Expected %s got %s`, contract.BuyerOrder.Shipping.Address, shippingAddress)
	}
}

func TestDeletePurchase(t *testing.T) {
	purdb, teardown, err := buildNewPurchaseStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	contract := factory.NewContract()
	err = purdb.Put("orderID", *contract, 0, false)
	if err != nil {
		t.Log(err)
	}
	err = purdb.Delete("orderID")
	if err != nil {
		t.Error("Purchase delete failed")
	}

	stmt, _ := purdb.PrepareQuery("select orderID, contract, state, read from purchases where orderID=?")
	defer stmt.Close()

	var orderID string
	var contractStr []byte
	var state int
	var read int
	err = stmt.QueryRow("orderID").Scan(&orderID, &contractStr, &state, &read)
	if err == nil {
		t.Error("Purchase delete failed")
	}
}

func TestMarkPurchaseAsRead(t *testing.T) {
	purdb, teardown, err := buildNewPurchaseStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	contract := factory.NewContract()
	err = purdb.Put("orderID", *contract, 0, false)
	if err != nil {
		t.Log(err)
	}
	err = purdb.MarkAsRead("orderID")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := purdb.PrepareQuery("select read from purchases where orderID=?")
	defer stmt.Close()

	var read int
	err = stmt.QueryRow("orderID").Scan(&read)
	if err != nil {
		t.Error("Purchase query failed")
	}
	if read != 1 {
		t.Error("Failed to mark purchase as read")
	}
}

func TestMarkPurchaseAsUnread(t *testing.T) {
	purdb, teardown, err := buildNewPurchaseStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	contract := factory.NewContract()
	err = purdb.Put("orderID", *contract, 0, false)
	if err != nil {
		t.Log(err)
	}
	err = purdb.MarkAsRead("orderID")
	if err != nil {
		t.Error(err)
	}

	err = purdb.MarkAsUnread("orderID")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := purdb.PrepareQuery("select read from purchases where orderID=?")
	defer stmt.Close()

	var read int
	err = stmt.QueryRow("orderID").Scan(&read)
	if err != nil {
		t.Error("Purchase query failed")
	}
	if read != 0 {
		t.Error("Failed to mark purchase as read")
	}
}

func TestUpdatePurchaseFunding(t *testing.T) {
	purdb, teardown, err := buildNewPurchaseStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	contract := factory.NewContract()
	err = purdb.Put("orderID", *contract, 1, false)
	if err != nil {
		t.Error(err)
	}
	record := &wallet.TransactionRecord{
		Txid: "abc123",
	}
	records := []*wallet.TransactionRecord{record}
	err = purdb.UpdateFunding("orderID", true, records)
	if err != nil {
		t.Error(err)
	}
	addr, err := btcutil.DecodeAddress(contract.BuyerOrder.Payment.Address, &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	_, _, funded, rcds, err := purdb.GetByPaymentAddress(addr)
	if err != nil {
		t.Error(err)
		return
	}
	if !funded {
		t.Error("Update funding failed to update the funded bool")
		return
	}
	if len(rcds) == 0 {
		t.Error("Failed to return transaction records")
		return
	}
	if rcds[0].Txid != "abc123" {
		t.Error("Failed to return correct txid on record")
	}
}

func TestPurchasePutAfterFundingUpdate(t *testing.T) {
	purdb, teardown, err := buildNewPurchaseStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	contract := factory.NewContract()
	err = purdb.Put("orderID", *contract, 1, false)
	if err != nil {
		t.Error(err)
	}
	record := &wallet.TransactionRecord{
		Txid: "abc123",
	}
	records := []*wallet.TransactionRecord{record}
	err = purdb.UpdateFunding("orderID", true, records)
	if err != nil {
		t.Error(err)
	}
	err = purdb.Put("orderID", *contract, 3, false)
	if err != nil {
		t.Error(err)
	}
	addr, err := btcutil.DecodeAddress(contract.BuyerOrder.Payment.Address, &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	_, _, funded, rcds, err := purdb.GetByPaymentAddress(addr)
	if err != nil {
		t.Error(err)
		return
	}
	if !funded {
		t.Error("Update funding failed to update the funded bool")
		return
	}
	if len(rcds) == 0 {
		t.Error("Failed to return transaction records")
		return
	}
	if rcds[0].Txid != "abc123" {
		t.Error("Failed to return correct txid on record")
	}
}

func TestPurchasesGetByPaymentAddress(t *testing.T) {
	purdb, teardown, err := buildNewPurchaseStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	contract := factory.NewContract()
	err = purdb.Put("orderID", *contract, 0, false)
	if err != nil {
		t.Log(err)
	}
	addr, err := btcutil.DecodeAddress(contract.BuyerOrder.Payment.Address, &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	_, _, _, _, err = purdb.GetByPaymentAddress(addr)
	if err != nil {
		t.Error(err)
	}
	addr, err = btcutil.DecodeAddress("19bsDJeYjH6JX1pvsCcA8Qt5LQmPYt7Mry", &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	_, _, _, _, err = purdb.GetByPaymentAddress(addr)
	if err == nil {
		t.Error("Get by unknown address failed to return error")
	}

}

func TestPurchasesGetByOrderId(t *testing.T) {
	var (
		expectedCoin         = "BTC"
		purdb, teardown, err = buildNewPurchaseStore()
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	_, _, _, _, _, _, err = purdb.GetByOrderId("fasdfas")
	if err == nil {
		t.Error("Get by unknown orderId failed to return error")
	}

	contract := factory.NewContract()
	//contract.BuyerOrder.Payment.Coin = expectedCoin
	if err := purdb.Put("orderID", *contract, 0, false); err != nil {
		t.Fatal(err)
	}

	_, _, _, _, _, actualCoin, err := purdb.GetByOrderId("orderID")
	if err != nil {
		t.Error(err)
	}
	if actualCoin == nil || actualCoin.String() != expectedCoin {
		t.Errorf("expected paymentCoin to be returned in the result")
	}
}

func TestPurchasesDB_GetAll(t *testing.T) {
	purdb, teardown, err := buildNewPurchaseStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	c0 := factory.NewContract()
	ts, _ := ptypes.TimestampProto(time.Now())
	c0.BuyerOrder.Timestamp = ts
	err = purdb.Put("orderID", *c0, 0, false)
	if err != nil {
		t.Log(err)
	}
	c1 := factory.NewContract()
	ts, _ = ptypes.TimestampProto(time.Now().Add(time.Minute))
	c1.BuyerOrder.Timestamp = ts
	err = purdb.Put("orderID2", *c1, 1, false)
	if err != nil {
		t.Log(err)
	}
	c2 := factory.NewContract()
	ts, _ = ptypes.TimestampProto(time.Now().Add(time.Hour))
	c2.BuyerOrder.Timestamp = ts
	err = purdb.Put("orderID3", *c2, 1, false)
	if err != nil {
		t.Log(err)
	}
	// Test no offset no limit
	purchases, ct, err := purdb.GetAll([]pb.OrderState{}, "", false, false, -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(purchases) != 3 {
		t.Error("Returned incorrect number of purchases")
	}
	if ct != 3 {
		t.Error("Returned incorrect number of query purchases")
	}

	// Test no offset limit 1
	purchases, ct, err = purdb.GetAll([]pb.OrderState{}, "", false, false, 1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(purchases) != 1 {
		t.Error("Returned incorrect number of purchases")
	}
	if ct != 3 {
		t.Error("Returned incorrect number of query purchases")
	}

	// Test offset no limit
	purchases, ct, err = purdb.GetAll([]pb.OrderState{}, "", false, false, -1, []string{"orderID"})
	if err != nil {
		t.Error(err)
	}
	if len(purchases) != 2 {
		t.Error("Returned incorrect number of purchases")
	}
	if ct != 3 {
		t.Error("Returned incorrect number of query purchases")
	}

	// Test no offset no limit with state filter
	purchases, ct, err = purdb.GetAll([]pb.OrderState{pb.OrderState_AWAITING_PAYMENT}, "", false, false, -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(purchases) != 2 {
		t.Error("Returned incorrect number of purchases")
	}
	if ct != 2 {
		t.Error("Returned incorrect number of query purchases")
	}

	// Test offset no limit with state filter
	purchases, ct, err = purdb.GetAll([]pb.OrderState{pb.OrderState_AWAITING_PAYMENT}, "", false, false, -1, []string{"orderID3"})
	if err != nil {
		t.Error(err)
	}
	if len(purchases) != 1 {
		t.Error("Returned incorrect number of purchases")
	}
	if ct != 2 {
		t.Error("Returned incorrect number of query purchases")
	}

	// Test no offset no limit with multiple state filters
	purchases, ct, err = purdb.GetAll([]pb.OrderState{pb.OrderState_AWAITING_PAYMENT, pb.OrderState_PENDING}, "", false, false, -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(purchases) != 3 {
		t.Error("Returned incorrect number of purchases")
	}
	if ct != 3 {
		t.Error("Returned incorrect number of query purchases")
	}

	// Test no offset no limit with search term
	purchases, ct, err = purdb.GetAll([]pb.OrderState{}, "orderid2", false, false, -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(purchases) != 1 {
		t.Error("Returned incorrect number of purchases")
	}
	if ct != 1 {
		t.Error("Returned incorrect number of query purchases")
	}
}

func TestPurchasesDB_GetUnfunded(t *testing.T) {
	var purdb, teardown, err = buildNewPurchaseStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	contract := factory.NewContract()
	if err := purdb.Put("orderID", *contract, 1, false); err != nil {
		t.Fatal(err)
	}
	if err := purdb.Put("orderID1", *contract, 1, false); err != nil {
		t.Fatal(err)
	}
	if err := purdb.Put("x0", *contract, 0, false); err != nil {
		t.Fatal(err)
	}
	for i := 2; i < 15; i++ {
		if err := purdb.Put("x"+strconv.Itoa(i), *contract, pb.OrderState(i), false); err != nil {
			t.Fatal(err)
		}
	}
	unfunded, err := purdb.GetUnfunded()
	if err != nil {
		t.Error(err)
	}
	if len(unfunded) != 2 {
		t.Error("Return incorrect number of unfunded orders")
	}
	var a, b bool
	for _, uf := range unfunded {
		if uf.OrderId == "orderID" {
			a = true
		} else if uf.OrderId == "orderID1" {
			b = true
		}
		if uf.PaymentAddress != contract.BuyerOrder.Payment.Address {
			t.Errorf("Incorrect payment address. Expected %s, got %s", contract.BuyerOrder.Payment.Address, uf.PaymentAddress)
		}
	}
	if !a || !b {
		t.Error("Failed to return correct unfunded orders")
	}
}

func TestGetPurchasesForDisputeTimeoutReturnsRelevantRecords(t *testing.T) {
	appSchema := schema.MustNewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
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

	// Artificially start purchases 50 days ago
	var (
		now                           = time.Unix(time.Now().Unix(), 0)
		timeStart                     = now.Add(time.Duration(-50*24) * time.Hour)
		expectedImagesOne             = []*pb.Listing_Item_Image{{Tiny: "tinyimagehashOne", Small: "smallimagehashOne"}}
		expectedContractOne           = factory.NewDisputeableContract()
		expectedImagesTwo             = []*pb.Listing_Item_Image{{Tiny: "tinyimagehashTwo", Small: "smallimagehashTwo"}}
		expectedContractTwo           = factory.NewDisputeableContract()
		neverNotifiedButUndisputeable = &repo.PurchaseRecord{
			Contract:                     factory.NewUndisputeableContract(),
			OrderID:                      "neverNotifiedButUndisputed",
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: time.Unix(0, 0),
		}
		neverNotified = &repo.PurchaseRecord{
			Contract:                     expectedContractOne,
			OrderID:                      "neverNotified",
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: time.Unix(0, 0),
		}
		initialNotified = &repo.PurchaseRecord{
			Contract:                     expectedContractTwo,
			OrderID:                      "initialNotificationSent",
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: timeStart,
		}
		finallyNotified = &repo.PurchaseRecord{
			Contract:                     factory.NewContract(),
			OrderID:                      "finalNotificationSent",
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: now,
		}
		existingRecords = []*repo.PurchaseRecord{
			neverNotifiedButUndisputeable,
			neverNotified,
			initialNotified,
			finallyNotified,
		}
	)
	expectedContractOne.VendorListings[0].Item.Images = expectedImagesOne
	expectedContractTwo.VendorListings[0].Item.Images = expectedImagesTwo

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
		if _, err := database.Exec("insert into purchases (orderID, contract, state, timestamp, lastDisputeTimeoutNotifiedAt) values (?, ?, ?, ?, ?);", r.OrderID, contractData, int(r.OrderState), int(r.Timestamp.Unix()), int(r.LastDisputeTimeoutNotifiedAt.Unix())); err != nil {
			t.Fatal(err)
		}
	}

	purchaseDatabase := db.NewPurchaseStore(database, new(sync.Mutex))
	purchases, err := purchaseDatabase.GetPurchasesForDisputeTimeoutNotification()
	if err != nil {
		t.Fatal(err)
	}

	var sawNeverNotifiedPurchase, sawInitialNotifiedPurchase, sawFinallyNotifiedPurchase, sawNeverNotifiedButUndisputeable bool
	for _, p := range purchases {
		switch p.OrderID {
		case neverNotified.OrderID:
			sawNeverNotifiedPurchase = true
		case initialNotified.OrderID:
			sawInitialNotifiedPurchase = true
		case finallyNotified.OrderID:
			sawFinallyNotifiedPurchase = true
		case neverNotifiedButUndisputeable.OrderID:
			sawNeverNotifiedButUndisputeable = true
		default:
			t.Errorf("Found unexpected purchase: %+v", p)
		}
	}

	if !sawNeverNotifiedPurchase {
		t.Error("Expected to see purchase which was never notified")
	}
	if !sawInitialNotifiedPurchase {
		t.Error("Expected to see purchase which was initially notified")
	}
	if sawFinallyNotifiedPurchase {
		t.Error("Expected NOT to see purchase which received it's final notification")
	}
	if sawNeverNotifiedButUndisputeable {
		t.Error("Expected NOT to see undisputeable purchase")
	}
}

func TestUpdatePurchaseLastDisputeTimeoutNotifiedAt(t *testing.T) {
	appSchema := schema.MustNewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
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

	// Artificially start purchases 50 days ago
	var (
		timeStart   = time.Now().Add(time.Duration(-50*24) * time.Hour)
		purchaseOne = &repo.PurchaseRecord{
			OrderID:                      "purchase1",
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: time.Unix(123, 0),
		}
		purchaseTwo = &repo.PurchaseRecord{
			OrderID:                      "purchase2",
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: time.Unix(456, 0),
		}
		existingPurchases = []*repo.PurchaseRecord{purchaseOne, purchaseTwo}
	)
	s, err := database.Prepare("insert into purchases (orderID, contract, timestamp, lastDisputeTimeoutNotifiedAt) values (?, ?, ?, ?);")
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range existingPurchases {
		_, err = s.Exec(p.OrderID, p.Contract, p.Timestamp, p.LastDisputeTimeoutNotifiedAt.Unix())
		if err != nil {
			t.Fatal(err)
		}
	}

	// Simulate LastDisputeTimeoutNotifiedAt has been changed
	purchaseOne.LastDisputeTimeoutNotifiedAt = time.Unix(987, 0)
	purchaseTwo.LastDisputeTimeoutNotifiedAt = time.Unix(765, 0)
	purchaseDatabase := db.NewPurchaseStore(database, new(sync.Mutex))
	err = purchaseDatabase.UpdatePurchasesLastDisputeTimeoutNotifiedAt(existingPurchases)
	if err != nil {
		t.Fatal(err)
	}

	s, err = database.Prepare("select orderID, lastDisputeTimeoutNotifiedAt from purchases")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := s.Query()
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var (
			orderID                      string
			lastDisputeTimeoutNotifiedAt int64
		)
		if err = rows.Scan(&orderID, &lastDisputeTimeoutNotifiedAt); err != nil {
			t.Fatal(err)
		}

		switch orderID {
		case purchaseOne.OrderID:
			if !time.Unix(lastDisputeTimeoutNotifiedAt, 0).Equal(purchaseOne.LastDisputeTimeoutNotifiedAt) {
				t.Error("Expected purchaseOne.LastDisputeTimeoutNotifiedAt to be updated")
			}
		case purchaseTwo.OrderID:
			if !time.Unix(lastDisputeTimeoutNotifiedAt, 0).Equal(purchaseTwo.LastDisputeTimeoutNotifiedAt) {
				t.Error("Expected purchaseTwo.LastDisputeTimeoutNotifiedAt to be updated")
			}
		default:
			t.Error("Unexpected purchase encounted")
			t.Error(orderID, lastDisputeTimeoutNotifiedAt)
		}

	}
}

func newDisputedPurchaseRecord() *repo.PurchaseRecord {
	p := factory.NewPurchaseRecord()
	p.Contract = factory.NewDisputedContract()
	p.OrderState = pb.OrderState_DISPUTED
	return p
}

func TestGetPurchasesForDisputeExpiryNotificationReturnsRelevantRecords(t *testing.T) {
	appSchema := schema.MustNewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
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

	// Artificially start purchases disputed 50 days ago
	var (
		now                        = time.Unix(time.Now().Unix(), 0)
		timeStart                  = now.Add(time.Duration(-50*24) * time.Hour)
		neverNotifiedButUndisputed = factory.NewPurchaseRecord()
		neverNotified              = newDisputedPurchaseRecord()
		initialNotified            = newDisputedPurchaseRecord()
		finallyNotified            = newDisputedPurchaseRecord()
		existingRecords            = []*repo.PurchaseRecord{
			neverNotifiedButUndisputed,
			neverNotified,
			initialNotified,
			finallyNotified,
		}
	)
	neverNotifiedButUndisputed.OrderID = "neverNotifiedButUndisputed"
	neverNotified.OrderID = "neverNotified"
	initialNotified.OrderID = "initiallyNotified"
	finallyNotified.OrderID = "finallyNotified"
	neverNotified.DisputedAt = timeStart
	initialNotified.DisputedAt = timeStart
	finallyNotified.DisputedAt = timeStart
	neverNotified.LastDisputeExpiryNotifiedAt = time.Unix(0, 0)
	initialNotified.LastDisputeExpiryNotifiedAt = timeStart
	finallyNotified.LastDisputeExpiryNotifiedAt = now
	neverNotified.Timestamp = timeStart
	initialNotified.Timestamp = timeStart
	finallyNotified.Timestamp = timeStart

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
		if _, err := database.Exec("insert into purchases (orderID, contract, state, timestamp, lastDisputeExpiryNotifiedAt, disputedAt) values (?, ?, ?, ?, ?, ?);", r.OrderID, contractData, int(r.OrderState), int(r.Timestamp.Unix()), int(r.LastDisputeExpiryNotifiedAt.Unix()), int(r.DisputedAt.Unix())); err != nil {
			t.Fatal(err)
		}
	}

	purchaseDatabase := db.NewPurchaseStore(database, new(sync.Mutex))
	purchases, err := purchaseDatabase.GetPurchasesForDisputeExpiryNotification()
	if err != nil {
		t.Fatal(err)
	}

	var sawNeverNotifiedPurchase, sawInitialNotifiedPurchase, sawFinallyNotifiedPurchase, sawNeverNotifiedButUndisputed bool
	for _, p := range purchases {
		switch p.OrderID {
		case neverNotified.OrderID:
			sawNeverNotifiedPurchase = true
			if !reflect.DeepEqual(p, neverNotified) {
				t.Error("Expected neverNotified to match, but did not")
				t.Errorf("Expected: %+v", neverNotified)
				t.Errorf("Actual: %+v", p)
			}
		case initialNotified.OrderID:
			sawInitialNotifiedPurchase = true
			if !reflect.DeepEqual(p, initialNotified) {
				t.Error("Expected initialNotified to match, but did not")
				t.Errorf("Expected: %+v", initialNotified)
				t.Errorf("Actual: %+v", p)
			}
		case finallyNotified.OrderID:
			sawFinallyNotifiedPurchase = true
		case neverNotifiedButUndisputed.OrderID:
			sawNeverNotifiedButUndisputed = true
		default:
			t.Errorf("Found unexpected purchase: %+v", p)
		}
	}

	if !sawNeverNotifiedPurchase {
		t.Error("Expected to see purchase which was never notified")
	}
	if !sawInitialNotifiedPurchase {
		t.Error("Expected to see purchase which was initially notified")
	}
	if sawFinallyNotifiedPurchase {
		t.Error("Expected NOT to see purchase which received it's final notification")
	}
	if sawNeverNotifiedButUndisputed {
		t.Error("Expected NOT to see undisputed purchase")
	}
}

func TestUpdatePurchaseLastDisputeExpiryNotifiedAt(t *testing.T) {
	appSchema := schema.MustNewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
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

	// Artificially start purchases 50 days ago
	var (
		timeStart         = time.Now().Add(time.Duration(-50*24) * time.Hour)
		purchaseOne       = newDisputedPurchaseRecord()
		purchaseTwo       = newDisputedPurchaseRecord()
		existingPurchases = []*repo.PurchaseRecord{purchaseOne, purchaseTwo}
	)
	purchaseOne.OrderID = "purchase1"
	purchaseTwo.OrderID = "purchase2"
	purchaseOne.DisputedAt = timeStart
	purchaseTwo.DisputedAt = timeStart
	purchaseOne.LastDisputeExpiryNotifiedAt = time.Unix(123, 0)
	purchaseTwo.LastDisputeExpiryNotifiedAt = time.Unix(456, 0)

	s, err := database.Prepare("insert into purchases (orderID, lastDisputeExpiryNotifiedAt, disputedAt) values (?, ?, ?);")
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range existingPurchases {
		_, err = s.Exec(p.OrderID, p.LastDisputeExpiryNotifiedAt.Unix(), p.DisputedAt.Unix())
		if err != nil {
			t.Fatal(err)
		}
	}

	// Simulate LastDisputeExpiryNotifiedAt has been changed
	purchaseOne.LastDisputeExpiryNotifiedAt = time.Unix(987, 0)
	purchaseTwo.LastDisputeExpiryNotifiedAt = time.Unix(765, 0)
	purchaseDatabase := db.NewPurchaseStore(database, new(sync.Mutex))
	err = purchaseDatabase.UpdatePurchasesLastDisputeExpiryNotifiedAt(existingPurchases)
	if err != nil {
		t.Fatal(err)
	}

	s, err = database.Prepare("select orderID, lastDisputeExpiryNotifiedAt from purchases")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := s.Query()
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var (
			orderID                     string
			lastDisputeExpiryNotifiedAt int64
		)
		if err = rows.Scan(&orderID, &lastDisputeExpiryNotifiedAt); err != nil {
			t.Fatal(err)
		}

		switch orderID {
		case purchaseOne.OrderID:
			if !time.Unix(lastDisputeExpiryNotifiedAt, 0).Equal(purchaseOne.LastDisputeExpiryNotifiedAt) {
				t.Error("Expected purchaseOne.LastDisputeExpiryNotifiedAt to be updated")
			}
		case purchaseTwo.OrderID:
			if !time.Unix(lastDisputeExpiryNotifiedAt, 0).Equal(purchaseTwo.LastDisputeExpiryNotifiedAt) {
				t.Error("Expected purchaseTwo.LastDisputeExpiryNotifiedAt to be updated")
			}
		default:
			t.Error("Unexpected purchase encounted")
			t.Error(orderID, lastDisputeExpiryNotifiedAt)
		}

	}
}
func TestPurchasesDB_Put_PaymentCoin(t *testing.T) {
	var (
		contract = factory.NewContract()
		tests    = []struct {
			acceptedCurrencies []string
			paymentCoin        string
			expected           string
		}{
			{[]string{"TBTC"}, "TBTC", "TBTC"},
			{[]string{"TBTC", "TBCH"}, "TBTC", "TBTC"},
			{[]string{"TBCH", "TBTC"}, "TBTC", "TBTC"},
			{[]string{"TBTC", "TBCH"}, "TBCH", "TBCH"},
			{[]string{"TBTC", "TBCH"}, "", "TBTC"},
			{[]string{"TBCH", "TBTC"}, "", "TBCH"},
			{[]string{}, "", ""},
		}
	)

	for _, test := range tests {
		var purdb, teardown, err = buildNewPurchaseStore()
		if err != nil {
			t.Fatal(err)
		}

		contract.VendorListings[0].Metadata.AcceptedCurrencies = test.acceptedCurrencies
		//contract.BuyerOrder.Payment.Coin = test.paymentCoin
		contract.BuyerOrder.Payment.AmountCurrency = &pb.CurrencyDefinition{Code: test.paymentCoin, Divisibility: 8}

		err = purdb.Put("orderID", *contract, 0, false)
		if err != nil {
			t.Error(err)
		}

		purchases, count, err := purdb.GetAll(nil, "", false, false, 1, nil)
		if err != nil {
			t.Error(err)
		}
		if count != 1 {
			t.Errorf(`Expected %d record got %d`, 1, count)
		}
		if purchases[0].PaymentCoin != test.expected {
			t.Errorf(`Expected %s got %s`, test.expected, purchases[0].PaymentCoin)
		}
		teardown()
	}
}

func TestPurchasesDB_Put_CoinType(t *testing.T) {
	var (
		contract   = factory.NewContract()
		testsCoins = []struct {
			coinType      string
			cryptoListing bool
		}{
			{
				"",
				true,
			},
			{
				"TBTC",
				true,
			},
			{
				"TETH",
				true,
			},
			{
				"TBCH",
				false,
			},
		}
	)

	for _, test := range testsCoins {
		var purdb, teardown, err = buildNewPurchaseStore()
		if err != nil {
			t.Fatal(err)
		}

		contract.VendorListings[0].Item.PriceCurrency = &pb.CurrencyDefinition{
			Code:         test.coinType,
			Divisibility: 8,
		}
		if test.cryptoListing {
			contract.VendorListings[0].Metadata.CryptoCurrencyCode = test.coinType
			contract.VendorListings[0].Metadata.ContractType = pb.Listing_Metadata_CRYPTOCURRENCY
		} else {
			contract.VendorListings[0].Metadata.ContractType = pb.Listing_Metadata_PHYSICAL_GOOD
		}

		err = purdb.Put("orderID", *contract, 0, false)
		if err != nil {
			t.Error(err)
		}

		purchases, count, err := purdb.GetAll(nil, "", false, false, 1, nil)
		if err != nil {
			t.Error(err)
		}
		if count != 1 {
			t.Errorf(`Expected %d record got %d`, 1, count)
		}
		if test.cryptoListing && purchases[0].CoinType != test.coinType {
			t.Errorf(`Expected %s got %s`, test.coinType, purchases[0].CoinType)
		} else if !test.cryptoListing && purchases[0].CoinType != "" {
			t.Errorf(`Expected "" got %s`, purchases[0].CoinType)
		}
		teardown()
	}
}

func TestPurchaseDB_Put_DisputedAt(t *testing.T) {
	var (
		now                  = time.Now()
		nowData, tErr        = ptypes.TimestampProto(now)
		contract             = factory.NewDisputedContract()
		purdb, teardown, err = buildNewPurchaseStore()
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()
	if tErr != nil {
		t.Fatal(tErr)
	}
	contract.Dispute.Timestamp = nowData

	err = purdb.Put("orderID", *contract, pb.OrderState_DISPUTED, false)
	if err != nil {
		t.Fatal(err)
	}

	p, err := purdb.GetPurchasesForDisputeExpiryNotification()
	if err != nil {
		t.Fatal(err)
	}

	if len(p) != 1 {
		t.Fatalf("expected one purchase to be returned but found %d", len(p))
	}

	if p[0].DisputedAt.Unix() != now.Unix() {
		t.Errorf("expected disputedAt to be %s, but was %s", now.String(), p[0].DisputedAt.String())
	}
}
