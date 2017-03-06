package db

import (
	"database/sql"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"testing"
)

var metDB TxMetadataDB
var m repo.Metadata

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	metDB = TxMetadataDB{
		db: conn,
	}
	m = repo.Metadata{"16e4a210d8c798f7d7a32584038c1f55074377bdd19f4caa24edb657fff9538f", "1Xtkf3Rdq6eix4tFXpEuHdXfubt3Mt452", "Some memo", "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG", "QmZY1kx6VrNjgDB4SJDByxvSVuiBfsisRLdUMJRDppTTsS"}
}

func TestTxMetadataDB_Put(t *testing.T) {
	err := metDB.Put(m)
	if err != nil {
		t.Error(err)
	}
	stmt, err := metDB.db.Prepare("select txid, address, memo, orderID, thumbnail from txmetadata where txid=?")
	defer stmt.Close()
	var txid, addr, memo, orderId, thumbnail string
	err = stmt.QueryRow(m.Txid).Scan(&txid, &addr, &memo, &orderId, &thumbnail)
	if err != nil {
		t.Error(err)
	}
	if txid != m.Txid {
		t.Error("TxMetadataDB failed to put txid")
	}
	if addr != m.Address {
		t.Error("TxMetadataDB failed to put address")
	}
	if memo != m.Memo {
		t.Error("TxMetadataDB failed to put memo")
	}
	if orderId != m.OrderId {
		t.Error("TxMetadataDB failed to put order ID")
	}
	if thumbnail != m.Thumbnail {
		t.Error("TxMetadataDB failed to put image hash")
	}
}

func TestTxMetadataDB_Get(t *testing.T) {
	err := metDB.Put(m)
	if err != nil {
		t.Error(err)
	}
	ret, err := metDB.Get(m.Txid)
	if err != nil {
		t.Error(err)
	}
	if ret.Txid != m.Txid {
		t.Error("TxMetadataDB failed to get txid")
	}
	if ret.Address != m.Address {
		t.Error("TxMetadataDB failed to get address")
	}
	if ret.Memo != m.Memo {
		t.Error("TxMetadataDB failed to get memo")
	}
	if ret.OrderId != m.OrderId {
		t.Error("TxMetadataDB failed to get order ID")
	}
	if ret.Thumbnail != m.Thumbnail {
		t.Error("TxMetadataDB failed to get image hash")
	}
}

func TestTxMetadataDB_GetAll(t *testing.T) {
	err := metDB.Put(m)
	if err != nil {
		t.Error(err)
	}
	mds, err := metDB.GetAll()
	if err != nil {
		t.Error(err)
	}
	if len(mds) < 1 {
		t.Error("TxMetaData db get all failed")
	}
	ret, ok := mds[m.Txid]
	if !ok {
		t.Error("TxMetaData db get all failed to fetch correct row")
	}
	if ret.Txid != m.Txid {
		t.Error("TxMetadataDB failed to get txid")
	}
	if ret.Address != m.Address {
		t.Error("TxMetadataDB failed to get address")
	}
	if ret.Memo != m.Memo {
		t.Error("TxMetadataDB failed to get memo")
	}
	if ret.OrderId != m.OrderId {
		t.Error("TxMetadataDB failed to get order ID")
	}
	if ret.Thumbnail != m.Thumbnail {
		t.Error("TxMetadataDB failed to get image hash")
	}
}

func TestTxMetadataDB_Delete(t *testing.T) {
	err := metDB.Put(m)
	if err != nil {
		t.Error(err)
	}
	err = metDB.Delete(m.Txid)
	if err != nil {
		t.Error(err)
	}
	mds, err := metDB.GetAll()
	if err != nil {
		t.Error(err)
	}
	_, ok := mds[m.Txid]
	if ok {
		t.Error("TxMetadataDB failed to delete row")
	}
}
