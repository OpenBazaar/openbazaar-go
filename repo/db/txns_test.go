package db

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/wire"
)

var txdb repo.TransactionStore

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	txdb = NewTransactionStore(conn, new(sync.Mutex), wallet.Bitcoin)
}

func TestTxnsPut(t *testing.T) {
	tx := wire.NewMsgTx(wire.TxVersion)
	txHex := "01000000018b5d47ec7ae47ae2e158345069fd38a1460f436c486fd3376de24b5df8da62a201000000da00483045022100d9dfd2bd3762fbb06d4b7d37ba3544aefd2ea9913a728b90b446abf530eed03d0220066f3fe0e7a652d2383cfa3b06188301e7ff02320d3a9b32675d1b0d62e9de740147304402206271eb865f0a5f92fdb4306711c17f53c959a09d401cd375bf60904191f70e080220231368d0bacde1775505f6978486b2732179f3d3f971aa67690475763c3d96de01475221024760c9ba5fa6241da6ee8601f0266f0e0592f53735703f0feaae23eda6673ae821038cfa8e97caaafbe21455803043618440c28c501ec32d6ece6865003165a0d4d152aeffffffff0224eb1400000000001976a91411a23852c4554182abb97f811509d60015071a5188acc4c59d260000000017a9140be09225644b4cfdbb472028d8ccaf6df736025c8700000000"
	raw, _ := hex.DecodeString(txHex)
	r := bytes.NewReader(raw)
	tx.Deserialize(r)

	err := txdb.Put(raw, tx.TxHash().String(), 5, 1, time.Now(), false)
	if err != nil {
		t.Error(err)
	}
	stmt, err := txdb.PrepareQuery("select tx, value, height, watchOnly from txns where txid=?")
	if err != nil {
		t.Error(err)
	}
	defer stmt.Close()
	var ret []byte
	var val int
	var height int
	var watchOnly int
	err = stmt.QueryRow(tx.TxHash().String()).Scan(&ret, &val, &height, &watchOnly)
	if err != nil {
		t.Error(err)
	}
	if hex.EncodeToString(ret) != txHex {
		t.Error("Txns db put failed")
	}
	if val != 5 {
		t.Error("Txns db failed to put value")
	}
	if height != 1 {
		t.Error("Txns db failed to put height")
	}
	if watchOnly != 0 {
		t.Error("Txns db failed to put watchOnly")
	}
}

func TestTxnsGet(t *testing.T) {
	tx := wire.NewMsgTx(wire.TxVersion)
	txHex := "0100000001a8c3a68b7bec7ed52ea4a5787e5005e02adbcedb2ac1a38bb3ae499def8994db01000000d900473044022025bd8408492d4c55bc1aba94c0857ff8ce9e0030b4a2e464986411b917d83f4a022070336fb42b2b0e141f428e98e543ba0e5c0c00d7dd3142a3c01f6e4b3c0518600147304402202744e1c27d05d62502d4d2091082bf97ba92f25247e75bcc2856e1f7de472a7002206c64ff5ddf6a039375f296f620b384d9529ff658a449179c094dc588b43497b301475221024760c9ba5fa6241da6ee8601f0266f0e0592f53735703f0feaae23eda6673ae821038cfa8e97caaafbe21455803043618440c28c501ec32d6ece6865003165a0d4d152aeffffffff0249cc4a00000000001976a914429d80ec4980e5e30a9d888f92e087b9bb55f66588ac709246260000000017a9140be09225644b4cfdbb472028d8ccaf6df736025c8700000000"
	raw, _ := hex.DecodeString(txHex)
	r := bytes.NewReader(raw)
	tx.Deserialize(r)

	now := time.Now()
	err := txdb.Put(raw, tx.TxHash().String(), 0, 1, now, false)
	if err != nil {
		t.Error(err)
	}
	txn, err := txdb.Get(tx.TxHash())
	if err != nil {
		t.Error(err)
	}
	tx2 := wire.NewMsgTx(wire.TxVersion)
	tx2.Deserialize(bytes.NewReader(txn.Bytes))
	if tx.TxHash().String() != tx2.TxHash().String() {
		t.Error("Txn db get failed")
	}
	if txn.Height != 1 {
		t.Error("Txn db failed to get height")
	}
	if now.Equal(txn.Timestamp) {
		t.Error("Txn db failed to return correct time")
	}
	if txn.WatchOnly != false {
		t.Error("Txns db failed to put watchOnly")
	}
}

func TestTxnsGetAll(t *testing.T) {
	tx := wire.NewMsgTx(wire.TxVersion)
	txHex := "0100000001867aee4c46da655cca2fa7a16c39883d34a05b1af416beda611c42bb4769f97601000000d900473044022035c352425d3dfff6df3a7196017df1e758079748b9ef022c0ced3c7e90218710022079016e302accec5d2aaf5e34676c10749767e3a34a64f80a0d0aa489027b7689014730440220612b2be46cb24bcd314827ea1682df8f82ff1fa0106cf8e9cf48b3b46ea6b0cf022052ed5da8dbc6052afd40db80cba6f87129ec6828ceffc48c9f3b6f2b855a246001475221024760c9ba5fa6241da6ee8601f0266f0e0592f53735703f0feaae23eda6673ae821038cfa8e97caaafbe21455803043618440c28c501ec32d6ece6865003165a0d4d152aeffffffff0200360500000000001976a91487dc690fcfe267307762c3d7bdfd96b357d6dde188ac87b069250000000017a9140be09225644b4cfdbb472028d8ccaf6df736025c8700000000"
	raw, _ := hex.DecodeString(txHex)
	r := bytes.NewReader(raw)
	tx.Deserialize(r)

	err := txdb.Put(raw, tx.TxHash().String(), 1, 5, time.Now(), true)
	if err != nil {
		t.Error(err)
	}
	txns, err := txdb.GetAll(true)
	if err != nil {
		t.Error(err)
	}
	if len(txns) < 1 {
		t.Error("Txns db get all failed")
	}
}

func TestDeleteTxns(t *testing.T) {
	tx := wire.NewMsgTx(wire.TxVersion)
	txHex := "0100000001cbfe4948ebc9113244b802a96e4940fa063c0455a16ca1f39a1e1db03837d9c701000000da004830450221008994e3dba54cb0ea23ca008d0e361b4339ee7b44b5e9101f6837e6a1a89ce044022051be859c68a547feaf60ffacc43f528cf2963c088bde33424d859274505e3f450147304402206cd4ef92cc7f2862c67810479013330fcafe4d468f1370563d4dff6be5bcbedc02207688a09163e615bc82299a29e987e1d718cb99a91d46a1ab13d18c0f6e616a1601475221024760c9ba5fa6241da6ee8601f0266f0e0592f53735703f0feaae23eda6673ae821038cfa8e97caaafbe21455803043618440c28c501ec32d6ece6865003165a0d4d152aeffffffff029ae2c700000000001976a914f72f20a739ec3c3df1a1fd7eff122d13bd5ca39188acb64784240000000017a9140be09225644b4cfdbb472028d8ccaf6df736025c8700000000"
	raw, _ := hex.DecodeString(txHex)
	r := bytes.NewReader(raw)
	tx.Deserialize(r)

	err := txdb.Put(raw, tx.TxHash().String(), 0, 1, time.Now(), false)
	if err != nil {
		t.Error(err)
	}
	txid := tx.TxHash()
	err = txdb.Delete(&txid)
	if err != nil {
		t.Error(err)
	}
	txns, err := txdb.GetAll(true)
	if err != nil {
		t.Error(err)
	}
	for _, txn := range txns {
		if txn.Txid == txid.String() {
			t.Error("Txns db delete failed")
		}
	}
}

func TestTxnsDB_UpdateHeight(t *testing.T) {
	tx := wire.NewMsgTx(wire.TxVersion)
	txHex := "0100000001cbfe4948ebc9113244b802a96e4940fa063c0455a16ca1f39a1e1db03837d9c701000000da004830450221008994e3dba54cb0ea23ca008d0e361b4339ee7b44b5e9101f6837e6a1a89ce044022051be859c68a547feaf60ffacc43f528cf2963c088bde33424d859274505e3f450147304402206cd4ef92cc7f2862c67810479013330fcafe4d468f1370563d4dff6be5bcbedc02207688a09163e615bc82299a29e987e1d718cb99a91d46a1ab13d18c0f6e616a1601475221024760c9ba5fa6241da6ee8601f0266f0e0592f53735703f0feaae23eda6673ae821038cfa8e97caaafbe21455803043618440c28c501ec32d6ece6865003165a0d4d152aeffffffff029ae2c700000000001976a914f72f20a739ec3c3df1a1fd7eff122d13bd5ca39188acb64784240000000017a9140be09225644b4cfdbb472028d8ccaf6df736025c8700000000"
	raw, _ := hex.DecodeString(txHex)
	r := bytes.NewReader(raw)
	err := tx.Deserialize(r)
	if err != nil {
		t.Error(err)
	}

	err = txdb.Put(raw, tx.TxHash().String(), 0, 1, time.Now(), false)
	if err != nil {
		t.Error(err)
	}
	err = txdb.UpdateHeight(tx.TxHash(), -1, time.Now())
	if err != nil {
		t.Error(err)
	}
	txn, err := txdb.Get(tx.TxHash())
	if err != nil {
		t.Error(err)
	}
	if txn.Height != -1 {
		t.Error("Txn db failed to update height")
	}
}
