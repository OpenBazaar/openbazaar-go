package db

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"
	"time"

	"sync"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/golang/protobuf/ptypes"
)

var saldb repo.SaleStore

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	saldb = NewSaleStore(conn, new(sync.Mutex))

	var err error
	contract, err = newContract()
	if err != nil {
		return
	}
}

func TestSalesDB_Count(t *testing.T) {
	err := saldb.Put("orderID", *contract, 0, false)
	if err != nil {
		t.Error(err)
	}
	i := saldb.Count()
	if i != 1 {
		t.Error("Returned incorrect number of sales")
	}
}

func TestPutSale(t *testing.T) {
	contract, err := newContract()
	if err != nil {
		t.Error(err)
	}
	err = saldb.Put("orderID", *contract, 0, false)
	if err != nil {
		t.Error(err)
	}
	stmt, _ := saldb.PrepareQuery("select orderID, contract, state, read, timestamp, total, thumbnail, buyerID, buyerHandle, title, shippingName, shippingAddress, paymentCoin, coinType from sales where orderID=?")
	defer stmt.Close()

	var orderID string
	var c []byte
	var state int
	var read int
	var date int
	var total int
	var thumbnail string
	var buyerID string
	var buyerHandle string
	var title string
	var shippingName string
	var shippingAddress string
	var paymentCoin string
	var coinType string
	err = stmt.QueryRow("orderID").Scan(&orderID, &c, &state, &read, &date, &total, &thumbnail, &buyerID, &buyerHandle, &title, &shippingName, &shippingAddress, &paymentCoin, &coinType)
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
	if total != int(contract.BuyerOrder.Payment.Amount) {
		t.Errorf("Expected %d got %d", int(contract.BuyerOrder.Payment.Amount), total)
	}
	if thumbnail != contract.VendorListings[0].Item.Images[0].Tiny {
		t.Errorf("Expected %s got %s", contract.VendorListings[0].Item.Images[0].Tiny, thumbnail)
	}
	if buyerID != contract.BuyerOrder.BuyerID.PeerID {
		t.Errorf(`Expected %s got %s`, contract.BuyerOrder.BuyerID.PeerID, buyerID)
	}
	if buyerHandle != contract.BuyerOrder.BuyerID.Handle {
		t.Errorf(`Expected %s got %s`, contract.BuyerOrder.BuyerID.Handle, buyerHandle)
	}
	if title != contract.VendorListings[0].Item.Title {
		t.Errorf(`Expected %s got %s`, strings.ToLower(contract.VendorListings[0].Item.Title), title)
	}
	if shippingName != contract.BuyerOrder.Shipping.ShipTo {
		t.Errorf(`Expected %s got %s`, strings.ToLower(contract.BuyerOrder.Shipping.ShipTo), shippingName)
	}
	if shippingAddress != contract.BuyerOrder.Shipping.Address {
		t.Errorf(`Expected %s got %s`, strings.ToLower(contract.BuyerOrder.Shipping.Address), shippingAddress)
	}
	if paymentCoin != contract.BuyerOrder.Payment.Coin {
		t.Errorf(`Expected %s got %s`, contract.BuyerOrder.Payment.Coin, paymentCoin)
	}
	if coinType != "" {
		t.Errorf(`Expected empty string got %s`, coinType)
	}
}

func TestDeleteSale(t *testing.T) {
	saldb.Put("orderID", *contract, 0, false)
	err := saldb.Delete("orderID")
	if err != nil {
		t.Error("Sale delete failed")
	}

	stmt, _ := saldb.PrepareQuery("select orderID, contract, state from sales where orderID=?")
	defer stmt.Close()

	var orderID string
	var contract []byte
	var state int
	err = stmt.QueryRow("orderID").Scan(&orderID, &contract, &state)
	if err == nil {
		t.Error("Sale delete failed")
	}
}

func TestMarkSaleAsRead(t *testing.T) {
	saldb.Put("orderID", *contract, 0, false)
	err := saldb.MarkAsRead("orderID")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := saldb.PrepareQuery("select read from sales where orderID=?")
	defer stmt.Close()

	var read int
	err = stmt.QueryRow("orderID").Scan(&read)
	if err != nil {
		t.Error("Sale query failed")
	}
	if read != 1 {
		t.Error("Failed to mark sale as read")
	}
}

func TestMarkSaleAsUnread(t *testing.T) {
	saldb.Put("orderID", *contract, 0, false)
	err := saldb.MarkAsRead("orderID")
	if err != nil {
		t.Error(err)
	}
	err = saldb.MarkAsUnread("orderID")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := saldb.PrepareQuery("select read from sales where orderID=?")
	defer stmt.Close()

	var read int
	err = stmt.QueryRow("orderID").Scan(&read)
	if err != nil {
		t.Error("Sale query failed")
	}
	if read != 0 {
		t.Error("Failed to mark sale as read")
	}
}

func TestUpdateSaleFunding(t *testing.T) {
	err := saldb.Put("orderID", *contract, 1, false)
	if err != nil {
		t.Error(err)
	}
	record := &wallet.TransactionRecord{
		Txid: "abc123",
	}
	records := []*wallet.TransactionRecord{record}
	err = saldb.UpdateFunding("orderID", true, records)
	if err != nil {
		t.Error(err)
	}
	addr, err := btcutil.DecodeAddress(contract.BuyerOrder.Payment.Address, &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	_, _, funded, rcds, err := saldb.GetByPaymentAddress(addr)
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

func TestSalePutAfterFundingUpdate(t *testing.T) {
	err := saldb.Put("orderID", *contract, 1, false)
	if err != nil {
		t.Error(err)
	}
	record := &wallet.TransactionRecord{
		Txid: "abc123",
	}
	records := []*wallet.TransactionRecord{record}
	err = saldb.UpdateFunding("orderID", true, records)
	if err != nil {
		t.Error(err)
	}
	err = saldb.Put("orderID", *contract, 3, false)
	if err != nil {
		t.Error(err)
	}
	addr, err := btcutil.DecodeAddress(contract.BuyerOrder.Payment.Address, &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	_, _, funded, rcds, err := saldb.GetByPaymentAddress(addr)
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

func TestSalesGetByPaymentAddress(t *testing.T) {
	saldb.Put("orderID", *contract, 0, false)
	addr, err := btcutil.DecodeAddress(contract.BuyerOrder.Payment.Address, &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	_, _, _, _, err = saldb.GetByPaymentAddress(addr)
	if err != nil {
		t.Error(err)
	}
	addr, err = btcutil.DecodeAddress("19bsDJeYjH6JX1pvsCcA8Qt5LQmPYt7Mry", &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	_, _, _, _, err = saldb.GetByPaymentAddress(addr)
	if err == nil {
		t.Error("Get by unknown address failed to return error")
	}
}

func TestSalesGetByOrderId(t *testing.T) {
	saldb.Put("orderID", *contract, 0, false)
	_, _, _, _, _, err := saldb.GetByOrderId("orderID")
	if err != nil {
		t.Error(err)
	}
	_, _, _, _, _, err = saldb.GetByOrderId("adsfads")
	if err == nil {
		t.Error("Get by unknown orderID failed to return error")
	}
}

func TestSalesDB_GetAll(t *testing.T) {
	c0 := *contract
	ts, _ := ptypes.TimestampProto(time.Now())
	c0.BuyerOrder.Timestamp = ts
	saldb.Put("orderID", c0, 0, false)
	c1 := *contract
	ts, _ = ptypes.TimestampProto(time.Now().Add(time.Minute))
	c1.BuyerOrder.Timestamp = ts
	saldb.Put("orderID2", c1, 1, false)
	c2 := *contract
	ts, _ = ptypes.TimestampProto(time.Now().Add(time.Hour))
	c2.BuyerOrder.Timestamp = ts
	saldb.Put("orderID3", c2, 1, false)
	// Test no offset no limit
	sales, ct, err := saldb.GetAll([]pb.OrderState{}, "", false, false, -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(sales) != 3 {
		t.Error("Returned incorrect number of sales")
	}
	if ct != 3 {
		t.Error("Returned incorrect number of query sales")
	}

	// Test no offset limit 1
	sales, ct, err = saldb.GetAll([]pb.OrderState{}, "", false, false, 1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(sales) != 1 {
		t.Error("Returned incorrect number of sales")
	}
	if ct != 3 {
		t.Error("Returned incorrect number of query sales")
	}

	// Test offset no limit
	sales, ct, err = saldb.GetAll([]pb.OrderState{}, "", false, false, -1, []string{"orderID"})
	if err != nil {
		t.Error(err)
	}
	if len(sales) != 2 {
		t.Error("Returned incorrect number of sales")
	}
	if ct != 3 {
		t.Error("Returned incorrect number of query sales")
	}

	// Test no offset no limit with state filter
	sales, ct, err = saldb.GetAll([]pb.OrderState{pb.OrderState_AWAITING_PAYMENT}, "", false, false, -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(sales) != 2 {
		t.Error("Returned incorrect number of sales")
	}
	if ct != 2 {
		t.Error("Returned incorrect number of query sales")
	}

	// Test offset no limit with state filter
	sales, ct, err = saldb.GetAll([]pb.OrderState{pb.OrderState_AWAITING_PAYMENT}, "", false, false, -1, []string{"orderID"})
	if err != nil {
		t.Error(err)
	}
	if len(sales) != 2 {
		t.Error("Returned incorrect number of sales")
	}
	if ct != 2 {
		t.Error("Returned incorrect number of query sales")
	}

	// Test no offset no limit with multiple state filters
	sales, ct, err = saldb.GetAll([]pb.OrderState{pb.OrderState_PENDING, pb.OrderState_AWAITING_PAYMENT}, "", false, false, -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(sales) != 3 {
		t.Error("Returned incorrect number of sales")
	}
	if ct != 3 {
		t.Error("Returned incorrect number of query sales")
	}

	// Test no offset no limit with search term
	sales, ct, err = saldb.GetAll([]pb.OrderState{}, "orderid2", false, false, -1, []string{})
	if err != nil {
		t.Error(err)
	}
	if len(sales) != 1 {
		t.Error("Returned incorrect number of sales")
	}
	if ct != 1 {
		t.Error("Returned incorrect number of query sales")
	}
}

func TestSalesDB_SetNeedsResync(t *testing.T) {
	saldb.Put("orderID", *contract, 0, false)
	err := saldb.SetNeedsResync("orderID", true)
	if err != nil {
		t.Error(err)
	}
	stmt, _ := saldb.PrepareQuery("select needsSync from sales where orderID=?")
	defer stmt.Close()
	var needsSyncInt int
	err = stmt.QueryRow("orderID").Scan(&needsSyncInt)
	if err != nil {
		t.Error(err)
	}
	if needsSyncInt != 1 {
		t.Errorf(`Expected %d got %d`, 1, needsSyncInt)
	}
	err = saldb.SetNeedsResync("orderID", false)
	if err != nil {
		t.Error(err)
	}
	err = stmt.QueryRow("orderID").Scan(&needsSyncInt)
	if err != nil {
		t.Error(err)
	}
	if needsSyncInt != 0 {
		t.Errorf(`Expected %d got %d`, 0, needsSyncInt)
	}
}

func TestSalesDB_GetNeedsResync(t *testing.T) {
	saldb.Put("orderID", *contract, 1, false)
	saldb.Put("orderID1", *contract, 1, false)
	err := saldb.SetNeedsResync("orderID", true)
	if err != nil {
		t.Error(err)
	}
	err = saldb.SetNeedsResync("orderID1", true)
	if err != nil {
		t.Error(err)
	}
	unfunded, err := saldb.GetNeedsResync()
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
	}
	if !a || !b {
		t.Error("Failed to return correct unfunded orders")
	}
}

func TestGetSalesForDisputeTimeoutReturnsRelevantRecords(t *testing.T) {
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

	// Artificially start sales 50 days ago
	var (
		now                           = time.Unix(time.Now().Unix(), 0)
		timeStart                     = now.Add(time.Duration(-50*24) * time.Hour)
		expectedImagesOne             = []*pb.Listing_Item_Image{{Tiny: "tinyimagehashOne", Small: "smallimagehashOne"}}
		expectedContractOne           = factory.NewDisputeableContract()
		neverNotifiedButUndisputeable = &repo.SaleRecord{
			Contract:                     factory.NewUndisputeableContract(),
			OrderID:                      "neverNotifiedButUndisputed",
			OrderState:                   pb.OrderState(pb.OrderState_FULFILLED),
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: time.Unix(0, 0),
		}
		neverNotified = &repo.SaleRecord{
			Contract:                     expectedContractOne,
			OrderID:                      "neverNotified",
			OrderState:                   pb.OrderState(pb.OrderState_FULFILLED),
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: time.Unix(0, 0),
		}
		finallyNotified = &repo.SaleRecord{
			Contract:                     factory.NewContract(),
			OrderState:                   pb.OrderState(pb.OrderState_FULFILLED),
			OrderID:                      "finalNotificationSent",
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: time.Now(),
		}
		existingRecords = []*repo.SaleRecord{
			neverNotifiedButUndisputeable,
			neverNotified,
			finallyNotified,
		}
	)
	expectedContractOne.VendorListings[0].Item.Images = expectedImagesOne

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
		if _, err := database.Exec("insert into sales (orderID, contract, state, timestamp, lastDisputeTimeoutNotifiedAt) values (?, ?, ?, ?, ?);", r.OrderID, contractData, int(r.OrderState), int(r.Timestamp.Unix()), int(r.LastDisputeTimeoutNotifiedAt.Unix())); err != nil {
			t.Fatal(err)
		}
	}

	saleDatabase := NewSaleStore(database, new(sync.Mutex))
	sales, err := saleDatabase.GetSalesForDisputeTimeoutNotification()
	if err != nil {
		t.Fatal(err)
	}

	var sawNeverNotifiedButUndisputeable, sawNeverNotifiedSale, sawFinallyNotifiedSale bool
	for _, s := range sales {
		switch s.OrderID {
		case neverNotified.OrderID:
			sawNeverNotifiedSale = true
			if reflect.DeepEqual(s, neverNotified) != true {
				t.Error("Expected neverNotified to match, but did not")
				t.Error("Expected:", neverNotified)
				t.Error("Actual:", s)
			}
		case finallyNotified.OrderID:
			sawFinallyNotifiedSale = true
		case neverNotifiedButUndisputeable.OrderID:
			sawNeverNotifiedButUndisputeable = true
		default:
			t.Errorf("Found unexpected sale: %+v", s)
		}
	}

	if sawNeverNotifiedSale == false {
		t.Error("Expected to see sale which was never notified")
	}
	if sawFinallyNotifiedSale == true {
		t.Error("Expected NOT to see sale which recieved it's final notification")
	}
	if sawNeverNotifiedButUndisputeable == true {
		t.Error("Expected NOT to see sale which is undisputeable")
	}
}

func TestUpdateSaleLastDisputeTimeoutNotifiedAt(t *testing.T) {
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

	// Artificially start sales 50 days ago
	var (
		timeStart = time.Now().Add(time.Duration(-50*24) * time.Hour)
		saleOne   = &repo.SaleRecord{
			OrderID:                      "sale1",
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: time.Unix(123, 0),
		}
		saleTwo = &repo.SaleRecord{
			OrderID:                      "sale2",
			Timestamp:                    timeStart,
			LastDisputeTimeoutNotifiedAt: time.Unix(456, 0),
		}
		existingSales = []*repo.SaleRecord{saleOne, saleTwo}
	)
	s, err := database.Prepare("insert into sales (orderID, timestamp, lastDisputeTimeoutNotifiedAt) values (?, ?, ?);")
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range existingSales {
		_, err = s.Exec(p.OrderID, p.Timestamp, p.LastDisputeTimeoutNotifiedAt.Unix())
		if err != nil {
			t.Fatal(err)
		}
	}

	// Simulate LastDisputeTimeoutNotifiedAt has been changed
	saleOne.LastDisputeTimeoutNotifiedAt = time.Unix(987, 0)
	saleTwo.LastDisputeTimeoutNotifiedAt = time.Unix(765, 0)
	saleDatabase := NewSaleStore(database, new(sync.Mutex))
	err = saleDatabase.UpdateSalesLastDisputeTimeoutNotifiedAt(existingSales)
	if err != nil {
		t.Fatal(err)
	}

	s, err = database.Prepare("select orderID, lastDisputeTimeoutNotifiedAt from sales")
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
		case saleOne.OrderID:
			if time.Unix(lastDisputeTimeoutNotifiedAt, 0).Equal(saleOne.LastDisputeTimeoutNotifiedAt) != true {
				t.Error("Expected saleOne.LastDisputeTimeoutNotifiedAt to be updated")
			}
		case saleTwo.OrderID:
			if time.Unix(lastDisputeTimeoutNotifiedAt, 0).Equal(saleTwo.LastDisputeTimeoutNotifiedAt) != true {
				t.Error("Expected saleTwo.LastDisputeTimeoutNotifiedAt to be updated")
			}
		default:
			t.Error("Unexpected sale encounted")
			t.Error(orderID, lastDisputeTimeoutNotifiedAt)
		}

	}
}

func TestSalesDB_Put_PaymentCoin(t *testing.T) {
	err := deleteAllSales()
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
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

	for _, test := range tests {
		err := deleteAllSales()
		if err != nil {
			t.Error(err)
		}

		contract.VendorListings[0].Metadata.AcceptedCurrencies = test.acceptedCurrencies
		contract.BuyerOrder.Payment.Coin = test.paymentCoin

		err = saldb.Put("orderID", *contract, 0, false)
		if err != nil {
			t.Error(err)
		}

		sales, count, err := saldb.GetAll(nil, "", false, false, 1, nil)
		if err != nil {
			t.Error(err)
		}
		if count != 1 {
			t.Errorf(`Expected %d record got %d`, 1, count)
		}
		if sales[0].PaymentCoin != test.expected {
			t.Errorf(`Expected %s got %s`, test.expected, sales[0].PaymentCoin)
		}
	}
}

func TestSalesDB_Put_CoinType(t *testing.T) {
	for _, testCoin := range []string{"", "TBTC", "TETH"} {
		err := deleteAllSales()
		if err != nil {
			t.Error(err)
		}

		contract.VendorListings[0].Metadata.CoinType = testCoin

		err = saldb.Put("orderID", *contract, 0, false)
		if err != nil {
			t.Error(err)
		}

		sales, count, err := saldb.GetAll(nil, "", false, false, 1, nil)
		if err != nil {
			t.Error(err)
		}
		if count != 1 {
			t.Errorf(`Expected %d record got %d`, 1, count)
		}
		if sales[0].CoinType != testCoin {
			t.Errorf(`Expected %s got %s`, testCoin, sales[0].CoinType)
		}
	}
}

func deleteAllSales() error {
	_, err := saldb.(*SalesDB).db.Exec("delete from sales;")
	return err
}

func newContract() (*pb.RicardianContract, error) {
	contract = new(pb.RicardianContract)
	listing := new(pb.Listing)
	item := new(pb.Listing_Item)
	item.Title = "Test listing"
	listing.Item = item
	vendorID := new(pb.ID)
	vendorID.PeerID = "vendor id"
	vendorID.Handle = "@testvendor"
	listing.VendorID = vendorID
	image := new(pb.Listing_Item_Image)
	image.Tiny = "test image hash"
	listing.Metadata = &pb.Listing_Metadata{
		ContractType:       pb.Listing_Metadata_PHYSICAL_GOOD,
		AcceptedCurrencies: []string{"TBTC"},
	}
	listing.Item.Images = []*pb.Listing_Item_Image{image}
	contract.VendorListings = []*pb.Listing{listing}
	order := new(pb.Order)
	buyerID := new(pb.ID)
	buyerID.PeerID = "buyer id"
	buyerID.Handle = "@testbuyer"
	order.BuyerID = buyerID
	shipping := new(pb.Order_Shipping)
	shipping.Address = "1234 test ave."
	shipping.ShipTo = "buyer name"
	order.Shipping = shipping
	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return nil, err
	}
	order.Timestamp = ts
	payment := new(pb.Order_Payment)
	payment.Amount = 10
	payment.Coin = "TBTC"
	payment.Method = pb.Order_Payment_DIRECT
	payment.Address = "3BDbGsH5h5ctDiFtWMmZawcf3E7iWirVms"
	order.Payment = payment
	contract.BuyerOrder = order
	return contract, nil
}
