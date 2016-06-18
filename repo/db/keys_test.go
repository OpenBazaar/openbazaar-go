package db

import (
	"sync"
	"database/sql"
	"testing"
	"github.com/tyler-smith/go-bip32"
	btc "github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
)

var keysdb KeysDB
var bip32key *bip32.Key

func init(){
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	keysdb = KeysDB{
		db: conn,
		lock: new(sync.Mutex),
	}
	seed, _ := bip32.NewSeed()
	bip32key, _ = bip32.NewMasterKey(seed)
}

func TestPutKey(t *testing.T) {
	addr, _ := btc.NewAddressPubKey(bip32key.PublicKey().Key, &chaincfg.MainNetParams)
	err := keysdb.Put(bip32key, addr.ScriptAddress(), bitcoin.RECEIVING)
	if err != nil {
		t.Error(err)
	}
	stmt, err := keysdb.db.Prepare("select key from keys where key=?")
	defer stmt.Close()
	var retKey string
	err = stmt.QueryRow(bip32key.String()).Scan(&retKey)
	if err != nil {
		t.Error(err)
	}
	if retKey != bip32key.String() {
		t.Errorf(`Expected %s got %s`, bip32key.String(), retKey)
	}
}
func TestPutDuplicateKey(t *testing.T) {
	addr, _ := btc.NewAddressPubKey(bip32key.PublicKey().Key, &chaincfg.MainNetParams)
	keysdb.Put(bip32key, addr.ScriptAddress(), bitcoin.RECEIVING)
	err := keysdb.Put(bip32key, addr.ScriptAddress(), bitcoin.RECEIVING)
	if err == nil {
		t.Error("Expected unquire constriant error to be thrown")
	}
}
