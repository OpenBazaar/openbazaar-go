package db

import (
	"database/sql"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/golang/protobuf/ptypes/timestamp"
	"strings"
	"sync"
	"testing"
	"time"
)

var saldb SalesDB

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	saldb = SalesDB{
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
	image.Hash = "test image hash"
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

func TestPutSale(t *testing.T) {
	err := saldb.Put("orderID", *contract, 0, false)
	if err != nil {
		t.Error(err)
	}
	stmt, _ := saldb.db.Prepare("select orderID, contract, state, read, date, total, thumbnail, buyerID, buyerBlockchainID, title, shippingName, shippingAddress from sales where orderID=?")
	defer stmt.Close()

	var orderID string
	var c []byte
	var state int
	var read int
	var date int
	var total int
	var thumbnail string
	var buyerID string
	var buyerBlockchainID string
	var title string
	var shippingName string
	var shippingAddress string
	err = stmt.QueryRow("orderID").Scan(&orderID, &c, &state, &read, &date, &total, &thumbnail, &buyerID, &buyerBlockchainID, &title, &shippingName, &shippingAddress)
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
	if thumbnail != contract.VendorListings[0].Item.Images[0].Hash {
		t.Errorf("Expected %s got %s", contract.VendorListings[0].Item.Images[0].Hash, thumbnail)
	}
	if buyerID != contract.BuyerOrder.BuyerID.Guid {
		t.Errorf(`Expected %s got %s`, contract.BuyerOrder.BuyerID.Guid, buyerID)
	}
	if buyerBlockchainID != contract.BuyerOrder.BuyerID.BlockchainID {
		t.Errorf(`Expected %s got %s`, contract.BuyerOrder.BuyerID.BlockchainID, buyerBlockchainID)
	}
	if title != strings.ToLower(contract.VendorListings[0].Item.Title) {
		t.Errorf(`Expected %s got %s`, strings.ToLower(contract.VendorListings[0].Item.Title), title)
	}
	if shippingName != strings.ToLower(contract.BuyerOrder.Shipping.ShipTo) {
		t.Errorf(`Expected %s got %s`, strings.ToLower(contract.BuyerOrder.Shipping.ShipTo), shippingName)
	}
	if shippingAddress != strings.ToLower(contract.BuyerOrder.Shipping.Address) {
		t.Errorf(`Expected %s got %s`, strings.ToLower(contract.BuyerOrder.Shipping.Address), shippingAddress)
	}
}

func TestDeleteSale(t *testing.T) {
	saldb.Put("orderID", *contract, 0, false)
	err := saldb.Delete("orderID")
	if err != nil {
		t.Error("Purchase delete failed")
	}

	stmt, _ := saldb.db.Prepare("select orderID, contract, state from sales where orderID=?")
	defer stmt.Close()

	var orderID string
	var contract []byte
	var state int
	err = stmt.QueryRow("orderID").Scan(&orderID, &contract, &state)
	if err == nil {
		t.Error("Pointer delete failed")
	}
}

func TestMarkSaleAsRead(t *testing.T) {
	saldb.Put("orderID", *contract, 0, false)
	err := saldb.MarkAsRead("orderID")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := saldb.db.Prepare("select read from sales where orderID=?")
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

func TestSalesGetByPaymentAddress(t *testing.T) {
	saldb.Put("orderID", *contract, 0, false)
	addr, err := btcutil.DecodeAddress(contract.BuyerOrder.Payment.Address, &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	_, err = saldb.GetByPaymentAddress(addr)
	if err != nil {
		t.Error(err)
	}
	addr, err = btcutil.DecodeAddress("19bsDJeYjH6JX1pvsCcA8Qt5LQmPYt7Mry", &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	_, err = saldb.GetByPaymentAddress(addr)
	if err == nil {
		t.Error("Get by unknown address failed to return error")
	}

}
