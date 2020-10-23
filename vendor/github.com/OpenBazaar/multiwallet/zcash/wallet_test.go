package zcash

import (
	"github.com/OpenBazaar/multiwallet/datastore"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"testing"
	"time"
)

func TestZCashWallet_Balance(t *testing.T) {
	ds := datastore.NewMockMultiwalletDatastore()
	db, err := ds.GetDatastoreForWallet(wallet.Zcash)
	if err != nil {
		t.Fatal(err)
	}

	w := ZCashWallet{
		db: db,
	}

	ch1, err := chainhash.NewHashFromStr("ccfd8d91b38e065a4d0f655fffabbdbf61666d1fdf1b54b7432c5d0ad453b76d")
	if err != nil {
		t.Error(err)
	}
	ch2, err := chainhash.NewHashFromStr("37aface44f82f6f319957b501030da2595b35d8bbc953bbe237f378c5f715bdd")
	if err != nil {
		t.Error(err)
	}
	ch3, err := chainhash.NewHashFromStr("2d08e0e877ff9d034ca272666d01626e96a0cf9e17004aafb4ae9d5aa109dd20")
	if err != nil {
		t.Error(err)
	}
	ch4, err := chainhash.NewHashFromStr("c803c8e21a464f0425fda75fb43f5a40bb6188bab9f3bfe0c597b46899e30045")
	if err != nil {
		t.Error(err)
	}

	err = db.Utxos().Put(wallet.Utxo{
		AtHeight: 500,
		Value:    1000,
		Op:       *wire.NewOutPoint(ch1, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	err = db.Utxos().Put(wallet.Utxo{
		AtHeight: 0,
		Value:    2000,
		Op:       *wire.NewOutPoint(ch2, 0),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test unconfirmed
	confirmed, unconfirmed := w.Balance()
	if confirmed != 1000 || unconfirmed != 2000 {
		t.Error("Returned incorrect balance")
	}

	// Test confirmed stxo
	tx := wire.NewMsgTx(1)
	op := wire.NewOutPoint(ch3, 1)
	in := wire.NewTxIn(op, []byte{}, [][]byte{})
	out := wire.NewTxOut(500, []byte{0x00})
	tx.TxIn = append(tx.TxIn, in)
	tx.TxOut = append(tx.TxOut, out)
	buf, err := serializeVersion4Transaction(tx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Txns().Put(buf, "37aface44f82f6f319957b501030da2595b35d8bbc953bbe237f378c5f715bdd", 0, 0, time.Now(), false); err != nil {
		t.Fatal(err)
	}

	tx = wire.NewMsgTx(1)
	op = wire.NewOutPoint(ch4, 1)
	in = wire.NewTxIn(op, []byte{}, [][]byte{})
	out = wire.NewTxOut(500, []byte{0x00})
	tx.TxIn = append(tx.TxIn, in)
	tx.TxOut = append(tx.TxOut, out)
	buf2, err := serializeVersion4Transaction(tx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Txns().Put(buf2, "2d08e0e877ff9d034ca272666d01626e96a0cf9e17004aafb4ae9d5aa109dd20", 0, 1999, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	confirmed, unconfirmed = w.Balance()
	if confirmed != 3000 || unconfirmed != 0 {
		t.Error("Returned incorrect balance")
	}
}
