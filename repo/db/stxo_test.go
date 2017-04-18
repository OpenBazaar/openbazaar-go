package db

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"strconv"
	"testing"
)

var sxdb StxoDB
var stxo spvwallet.Stxo

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	sxdb = StxoDB{
		db: conn,
	}
	sh1, _ := chainhash.NewHashFromStr("e941e1c32b3dd1a68edc3af9f7fe711f35aaca60f758c2dd49561e45ca2c41c0")
	sh2, _ := chainhash.NewHashFromStr("82998e18760a5f6e5573cd789269e7853e3ebaba07a8df0929badd69dc644c5f")
	outpoint := wire.NewOutPoint(sh1, 0)
	utxo := spvwallet.Utxo{
		Op:           *outpoint,
		AtHeight:     300000,
		Value:        100000000,
		ScriptPubkey: []byte("scriptpubkey"),
		WatchOnly:    false,
	}
	stxo = spvwallet.Stxo{
		Utxo:        utxo,
		SpendHeight: 300100,
		SpendTxid:   *sh2,
	}
}

func TestStxoPut(t *testing.T) {
	err := sxdb.Put(stxo)
	if err != nil {
		t.Error(err)
	}
	stmt, _ := sxdb.db.Prepare("select outpoint, value, height, watchOnly, scriptPubKey, spendHeight, spendTxid from stxos where outpoint=?")
	defer stmt.Close()

	var outpoint string
	var value int
	var height int
	var scriptPubkey string
	var spendHeight int
	var spendTxid string
	var watchOnly int
	o := stxo.Utxo.Op.Hash.String() + ":" + strconv.Itoa(int(stxo.Utxo.Op.Index))
	err = stmt.QueryRow(o).Scan(&outpoint, &value, &height, &watchOnly, &scriptPubkey, &spendHeight, &spendTxid)
	if err != nil {
		t.Error(err)
	}
	if outpoint != o {
		t.Error("Stxo db returned wrong outpoint")
	}
	if value != int(stxo.Utxo.Value) {
		t.Error("Stxo db returned wrong value")
	}
	if height != int(stxo.Utxo.AtHeight) {
		t.Error("Stxo db returned wrong height")
	}
	if scriptPubkey != hex.EncodeToString(stxo.Utxo.ScriptPubkey) {
		t.Error("Stxo db returned wrong scriptPubKey")
	}
	if spendHeight != int(stxo.SpendHeight) {
		t.Error("Stxo db returned wrong spend height")
	}
	if spendTxid != stxo.SpendTxid.String() {
		t.Error("Stxo db returned wrong spend txid")
	}
	if watchOnly != 0 {
		t.Error("Stxo db returned wrong watch only bool")
	}
}

func TestStxoGetAll(t *testing.T) {
	err := sxdb.Put(stxo)
	if err != nil {
		t.Error(err)
	}
	stxos, err := sxdb.GetAll()
	if err != nil {
		t.Error(err)
	}
	if stxos[0].Utxo.Op.Hash.String() != stxo.Utxo.Op.Hash.String() {
		t.Error("Stxo db returned wrong outpoint hash")
	}
	if stxos[0].Utxo.Op.Index != stxo.Utxo.Op.Index {
		t.Error("Stxo db returned wrong outpoint index")
	}
	if stxos[0].Utxo.Value != stxo.Utxo.Value {
		t.Error("Stxo db returned wrong value")
	}
	if stxos[0].Utxo.AtHeight != stxo.Utxo.AtHeight {
		t.Error("Stxo db returned wrong height")
	}
	if !bytes.Equal(stxos[0].Utxo.ScriptPubkey, stxo.Utxo.ScriptPubkey) {
		t.Error("Stxo db returned wrong scriptPubKey")
	}
	if stxos[0].SpendHeight != stxo.SpendHeight {
		t.Error("Stxo db returned wrong spend height")
	}
	if stxos[0].SpendTxid.String() != stxo.SpendTxid.String() {
		t.Error("Stxo db returned wrong spend txid")
	}
	if stxos[0].Utxo.WatchOnly != stxo.Utxo.WatchOnly {
		t.Error("Stxo db returned wrong watch only bool")
	}
}

func TestDeleteStxo(t *testing.T) {
	err := sxdb.Put(stxo)
	if err != nil {
		t.Error(err)
	}
	err = sxdb.Delete(stxo)
	if err != nil {
		t.Error(err)
	}
	stxos, err := sxdb.GetAll()
	if err != nil {
		t.Error(err)
	}
	if len(stxos) != 0 {
		t.Error("Stxo db delete failed")
	}
}
