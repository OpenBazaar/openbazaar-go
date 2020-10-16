package util

import (
	"bytes"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"testing"
)

func TestCalcBalance(t *testing.T) {
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

	var utxos []wallet.Utxo
	var txns []wallet.Txn

	// Test confirmed and unconfirmed
	utxos = append(utxos, wallet.Utxo{
		AtHeight: 500,
		Value:    1000,
		Op:       *wire.NewOutPoint(ch1, 0),
	})
	utxos = append(utxos, wallet.Utxo{
		AtHeight: 0,
		Value:    2000,
		Op:       *wire.NewOutPoint(ch2, 0),
	})

	confirmed, unconfirmed := CalcBalance(utxos, txns)
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
	var buf bytes.Buffer
	err = tx.BtcEncode(&buf, wire.ProtocolVersion, wire.WitnessEncoding)
	if err != nil {
		t.Error(err)
	}
	txns = append(txns, wallet.Txn{
		Bytes: buf.Bytes(),
		Txid:  "37aface44f82f6f319957b501030da2595b35d8bbc953bbe237f378c5f715bdd",
	})
	tx = wire.NewMsgTx(1)
	op = wire.NewOutPoint(ch4, 1)
	in = wire.NewTxIn(op, []byte{}, [][]byte{})
	out = wire.NewTxOut(500, []byte{0x00})
	tx.TxIn = append(tx.TxIn, in)
	tx.TxOut = append(tx.TxOut, out)
	var buf2 bytes.Buffer
	err = tx.BtcEncode(&buf, wire.ProtocolVersion, wire.WitnessEncoding)
	if err != nil {
		t.Error(err)
	}
	txns = append(txns, wallet.Txn{
		Bytes:  buf2.Bytes(),
		Txid:   "2d08e0e877ff9d034ca272666d01626e96a0cf9e17004aafb4ae9d5aa109dd20",
		Height: 1999,
	})
	confirmed, unconfirmed = CalcBalance(utxos, txns)
	if confirmed != 3000 || unconfirmed != 0 {
		t.Error("Returned incorrect balance")
	}

	// Test unconfirmed stxo
	txns = []wallet.Txn{}
	txns = append(txns, wallet.Txn{
		Bytes: buf.Bytes(),
		Txid:  "37aface44f82f6f319957b501030da2595b35d8bbc953bbe237f378c5f715bdd",
	})
	txns = append(txns, wallet.Txn{
		Bytes:  buf2.Bytes(),
		Txid:   "2d08e0e877ff9d034ca272666d01626e96a0cf9e17004aafb4ae9d5aa109dd20",
		Height: 0,
	})
	confirmed, unconfirmed = CalcBalance(utxos, txns)
	if confirmed != 1000 || unconfirmed != 2000 {
		t.Error("Returned incorrect balance")
	}

	// Test without stxo in db
	txns = []wallet.Txn{}
	txns = append(txns, wallet.Txn{
		Bytes: buf.Bytes(),
		Txid:  "37aface44f82f6f319957b501030da2595b35d8bbc953bbe237f378c5f715bdd",
	})
	confirmed, unconfirmed = CalcBalance(utxos, txns)
	if confirmed != 1000 || unconfirmed != 2000 {
		t.Error("Returned incorrect balance")
	}
}
