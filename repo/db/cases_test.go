package db

import (
	"database/sql"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/ptypes/timestamp"
	"strings"
	"sync"
	"testing"
	"time"
)

var casesdb CasesDB

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	casesdb = CasesDB{
		db:   conn,
		lock: new(sync.Mutex),
	}
	contract = new(pb.RicardianContract)
	listing := new(pb.Listing)
	item := new(pb.Listing_Item)
	item.Title = "Test listing"
	listing.Item = item
	vendorID := new(pb.ID)
	vendorID.Guid = "vendor guid"
	vendorID.BlockchainID = "@testvendor"
	listing.VendorID = vendorID
	image := new(pb.Listing_Item_Image)
	image.Tiny = "test image hash"
	listing.Item.Images = []*pb.Listing_Item_Image{image}
	contract.VendorListings = []*pb.Listing{listing}
	order := new(pb.Order)
	buyerID := new(pb.ID)
	buyerID.Guid = "buyer guid"
	buyerID.BlockchainID = "@testbuyer"
	order.BuyerID = buyerID
	shipping := new(pb.Order_Shipping)
	shipping.Address = "1234 test ave."
	shipping.ShipTo = "buyer name"
	order.Shipping = shipping
	ts := new(timestamp.Timestamp)
	ts.Seconds = time.Now().Unix()
	order.Timestamp = ts
	payment := new(pb.Order_Payment)
	payment.Amount = 10
	payment.Method = pb.Order_Payment_DIRECT
	payment.Address = "3BDbGsH5h5ctDiFtWMmZawcf3E7iWirVms"
	order.Payment = payment
	contract.BuyerOrder = order
}

func TestPutCase(t *testing.T) {
	err := casesdb.Put("orderID", contract, contract, 0, false)
	if err != nil {
		t.Error(err)
	}
	stmt, err := casesdb.db.Prepare("select orderID, buyerContract, vendorContract, state, read, date, thumbnail, buyerID, buyerBlockchainID, vendorID, vendorBlockchainID, title from cases where orderID=?")
	defer stmt.Close()

	var orderID string
	var c1 []byte
	var c2 []byte
	var state int
	var read int
	var date int
	var thumbnail string
	var buyerID string
	var buyerBlockchainID string
	var vendorID string
	var vendorBlockchainID string
	var title string
	err = stmt.QueryRow("orderID").Scan(&orderID, &c1, &c2, &state, &read, &date, &thumbnail, &buyerID, &buyerBlockchainID, &vendorID, &vendorBlockchainID, &title)
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
	if thumbnail != contract.VendorListings[0].Item.Images[0].Tiny {
		t.Errorf("Expected %s got %s", contract.VendorListings[0].Item.Images[0].Tiny, thumbnail)
	}
	if buyerID != contract.BuyerOrder.BuyerID.Guid {
		t.Errorf(`Expected %s got %s`, contract.BuyerOrder.BuyerID.Guid, buyerID)
	}
	if buyerBlockchainID != contract.BuyerOrder.BuyerID.BlockchainID {
		t.Errorf(`Expected %s got %s`, contract.BuyerOrder.BuyerID.BlockchainID, buyerBlockchainID)
	}
	if vendorID != contract.VendorListings[0].VendorID.Guid {
		t.Errorf(`Expected %s got %s`, contract.VendorListings[0].VendorID.Guid, vendorID)
	}
	if vendorBlockchainID != contract.VendorListings[0].VendorID.BlockchainID {
		t.Errorf(`Expected %s got %s`, contract.VendorListings[0].VendorID.BlockchainID, vendorBlockchainID)
	}
	if title != strings.ToLower(contract.VendorListings[0].Item.Title) {
		t.Errorf(`Expected %s got %s`, strings.ToLower(contract.VendorListings[0].Item.Title), title)
	}
}

func TestPutNil(t *testing.T) {
	err := casesdb.Put("orderID", contract, nil, 0, false)
	if err != nil {
		t.Error(err)
	}
	_, vendorContract, _, _, err := casesdb.GetByOrderId("orderID")
	if err != nil {
		t.Error(err)
	}
	if vendorContract != nil {
		t.Error("Vendor contract was not nil")
	}
}

func TestDeleteCase(t *testing.T) {
	err := casesdb.Put("orderID", contract, contract, 0, false)
	if err != nil {
		t.Error(err)
	}
	err = casesdb.Delete("orderID")
	if err != nil {
		t.Error("Case delete failed")
	}

	stmt, _ := casesdb.db.Prepare("select orderID from cases where orderID=?")
	defer stmt.Close()

	var orderID string
	err = stmt.QueryRow("orderID").Scan(&orderID)
	if err == nil {
		t.Error("Case delete failed")
	}
}

func TestMarkCaseAsRead(t *testing.T) {
	err := casesdb.Put("orderID", contract, contract, 0, false)
	if err != nil {
		t.Error(err)
	}
	err = casesdb.MarkAsRead("orderID")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := casesdb.db.Prepare("select read from cases where orderID=?")
	defer stmt.Close()

	var read int
	err = stmt.QueryRow("orderID").Scan(&read)
	if err != nil {
		t.Error("Case query failed")
	}
	if read != 1 {
		t.Error("Failed to mark case as read")
	}
}

func TestCasesGetByOrderId(t *testing.T) {
	err := casesdb.Put("orderID", contract, contract, 0, false)
	if err != nil {
		t.Error(err)
	}
	_, _, _, _, err = casesdb.GetByOrderId("orderID")
	if err != nil {
		t.Error(err)
	}
	_, _, _, _, err = casesdb.GetByOrderId("adsfads")
	if err == nil {
		t.Error("Get by unknown orderID failed to return error")
	}
}

func TestCasesGetAll(t *testing.T) {
	err := casesdb.Put("orderID", contract, contract, 0, false)
	if err != nil {
		t.Error(err)
	}
	ids, err := casesdb.GetAll()
	if err != nil {
		t.Error(err)
	}
	if len(ids) != 1 {
		t.Error("Get all returned incorrent number of IDs")
	}
	if ids[0] != "orderID" {
		t.Error("Get all returned incorrent number of IDs")
	}
}
