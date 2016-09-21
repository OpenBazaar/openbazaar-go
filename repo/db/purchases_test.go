package db

import (
	"database/sql"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/golang/protobuf/ptypes/timestamp"
	"strings"
	"sync"
	"testing"
	"time"
)

var purdb PurchasesDB
var contract *pb.RicardianContract

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	purdb = PurchasesDB{
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

func TestPutPurchase(t *testing.T) {
	err := purdb.Put("orderID", *contract, 0, false)
	if err != nil {
		t.Error(err)
	}
	stmt, _ := purdb.db.Prepare("select orderID, contract, state, read, date, total, thumbnail, vendorID, vendorBlockchainID, title, shippingName, shippingAddress from purchases where orderID=?")
	defer stmt.Close()

	var orderID string
	var c []byte
	var state int
	var read int
	var date int
	var total int
	var thumbnail string
	var vendorID string
	var vendorBlockchainID string
	var title string
	var shippingName string
	var shippingAddress string
	err = stmt.QueryRow("orderID").Scan(&orderID, &c, &state, &read, &date, &total, &thumbnail, &vendorID, &vendorBlockchainID, &title, &shippingName, &shippingAddress)
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
	if vendorID != contract.VendorListings[0].VendorID.Guid {
		t.Errorf(`Expected %s got %s`, contract.VendorListings[0].VendorID.Guid, vendorID)
	}
	if vendorBlockchainID != contract.VendorListings[0].VendorID.BlockchainID {
		t.Errorf(`Expected %s got %s`, contract.VendorListings[0].VendorID.BlockchainID, vendorBlockchainID)
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

func TestDeletePurchase(t *testing.T) {
	purdb.Put("orderID", *contract, 0, false)
	err := purdb.Delete("orderID")
	if err != nil {
		t.Error("Purchase delete failed")
	}

	stmt, _ := purdb.db.Prepare("select orderID, contract, state, read from purchases where orderID=?")
	defer stmt.Close()

	var orderID string
	var contract []byte
	var state int
	var read int
	err = stmt.QueryRow("orderID").Scan(&orderID, &contract, &state, &read)
	if err == nil {
		t.Error("Purchase delete failed")
	}
}

func TestMarkPurchaseAsRead(t *testing.T) {
	purdb.Put("orderID", *contract, 0, false)
	err := purdb.MarkAsRead("orderID")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := purdb.db.Prepare("select read from purchases where orderID=?")
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

func TestUpdatePurchaseFunding(t *testing.T) {
	err := purdb.Put("orderID", *contract, 1, false)
	if err != nil {
		t.Error(err)
	}
	record := spvwallet.TransactionRecord{
		Txid: "abc123",
	}
	records := []spvwallet.TransactionRecord{record}
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
	err := purdb.Put("orderID", *contract, 1, false)
	if err != nil {
		t.Error(err)
	}
	record := spvwallet.TransactionRecord{
		Txid: "abc123",
	}
	records := []spvwallet.TransactionRecord{record}
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
	purdb.Put("orderID", *contract, 0, false)
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
	purdb.Put("orderID", *contract, 0, false)
	_, _, _, _, _, err := purdb.GetByOrderId("orderID")
	if err != nil {
		t.Error(err)
	}
	_, _, _, _, _, err = purdb.GetByOrderId("fasdfas")
	if err == nil {
		t.Error("Get by unknown orderId failed to return error")
	}
}
