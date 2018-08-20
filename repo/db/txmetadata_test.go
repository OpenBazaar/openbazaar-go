package db_test

import (
	"sync"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func buildNewTransactionStore() (repo.TransactionMetadataStore, func(), error) {
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
	return db.NewTransactionMetadataStore(database, new(sync.Mutex)), appSchema.DestroySchemaDirectories, nil
}

func TestTxMetadataDB_Put(t *testing.T) {
	metDB, teardown, err := buildNewTransactionStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	m := repo.Metadata{"16e4a210d8c798f7d7a32584038c1f55074377bdd19f4caa24edb657fff9538f", "1Xtkf3Rdq6eix4tFXpEuHdXfubt3Mt452", "Some memo", "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG", "QmZY1kx6VrNjgDB4SJDByxvSVuiBfsisRLdUMJRDppTTsS", false}
	err = metDB.Put(m)
	if err != nil {
		t.Error(err)
	}
	stmt, err := metDB.PrepareQuery("select txid, address, memo, orderID, thumbnail, canBumpFee from txmetadata where txid=?")
	if err != nil {
		t.Error(err)
	}
	defer stmt.Close()
	var txid, addr, memo, orderId, thumbnail string
	var canBumpFee int
	err = stmt.QueryRow(m.Txid).Scan(&txid, &addr, &memo, &orderId, &thumbnail, &canBumpFee)
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
	if canBumpFee > 0 {
		t.Error("TxMetadataDB failed to put canBumpFee")
	}
}

func TestTxMetadataDB_Get(t *testing.T) {
	metDB, teardown, err := buildNewTransactionStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	m := repo.Metadata{"16e4a210d8c798f7d7a32584038c1f55074377bdd19f4caa24edb657fff9538f", "1Xtkf3Rdq6eix4tFXpEuHdXfubt3Mt452", "Some memo", "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG", "QmZY1kx6VrNjgDB4SJDByxvSVuiBfsisRLdUMJRDppTTsS", false}
	err = metDB.Put(m)
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
	if ret.CanBumpFee != m.CanBumpFee {
		t.Error("TxMetadataDB failed to get canBumpFee")
	}
}

func TestTxMetadataDB_GetAll(t *testing.T) {
	metDB, teardown, err := buildNewTransactionStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	m := repo.Metadata{"16e4a210d8c798f7d7a32584038c1f55074377bdd19f4caa24edb657fff9538f", "1Xtkf3Rdq6eix4tFXpEuHdXfubt3Mt452", "Some memo", "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG", "QmZY1kx6VrNjgDB4SJDByxvSVuiBfsisRLdUMJRDppTTsS", false}
	err = metDB.Put(m)
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
	if ret.CanBumpFee != m.CanBumpFee {
		t.Error("TxMetadataDB failed to get canBumpFee")
	}
}

func TestTxMetadataDB_Delete(t *testing.T) {
	metDB, teardown, err := buildNewTransactionStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	m := repo.Metadata{"16e4a210d8c798f7d7a32584038c1f55074377bdd19f4caa24edb657fff9538f", "1Xtkf3Rdq6eix4tFXpEuHdXfubt3Mt452", "Some memo", "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG", "QmZY1kx6VrNjgDB4SJDByxvSVuiBfsisRLdUMJRDppTTsS", false}
	err = metDB.Put(m)
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
