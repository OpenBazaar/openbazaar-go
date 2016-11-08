package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"github.com/OpenBazaar/spvwallet"
	"sync"
	"testing"
)

var kdb KeysDB

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	kdb = KeysDB{
		db:   conn,
		lock: new(sync.Mutex),
	}
}

func TestGetAll(t *testing.T) {
	for i := 0; i < 100; i++ {
		b := make([]byte, 32)
		rand.Read(b)
		err := kdb.Put(b, spvwallet.KeyPath{spvwallet.EXTERNAL, i})
		if err != nil {
			t.Error(err)
		}
	}
	all, err := kdb.GetAll()
	if err != nil || len(all) != 100 {
		t.Error("Failed to fetch all keys")
	}
}

func TestPutKey(t *testing.T) {
	b := make([]byte, 32)
	err := kdb.Put(b, spvwallet.KeyPath{spvwallet.EXTERNAL, 0})
	if err != nil {
		t.Error(err)
	}
	stmt, _ := kdb.db.Prepare("select scriptPubKey, purpose, keyIndex, used from keys where scriptPubKey=?")
	defer stmt.Close()

	var scriptPubKey string
	var purpose int
	var index int
	var used int
	err = stmt.QueryRow(hex.EncodeToString(b)).Scan(&scriptPubKey, &purpose, &index, &used)
	if err != nil {
		t.Error(err)
	}
	if scriptPubKey != hex.EncodeToString(b) {
		t.Errorf(`Expected %s got %s`, hex.EncodeToString(b), scriptPubKey)
	}
	if purpose != 0 {
		t.Errorf(`Expected 0 got %d`, purpose)
	}
	if index != 0 {
		t.Errorf(`Expected 0 got %d`, index)
	}
	if used != 0 {
		t.Errorf(`Expected 0 got %s`, used)
	}
}

func TestPutDuplicateKey(t *testing.T) {
	b := make([]byte, 32)
	kdb.Put(b, spvwallet.KeyPath{spvwallet.EXTERNAL, 0})
	err := kdb.Put(b, spvwallet.KeyPath{spvwallet.EXTERNAL, 0})
	if err == nil {
		t.Error("Expected duplicate key error")
	}
}

func TestMarkKeyAsUsed(t *testing.T) {
	b := make([]byte, 33)
	err := kdb.Put(b, spvwallet.KeyPath{spvwallet.EXTERNAL, 0})
	if err != nil {
		t.Error(err)
	}
	err = kdb.MarkKeyAsUsed(b)
	if err != nil {
		t.Error(err)
	}
	stmt, _ := kdb.db.Prepare("select scriptPubKey, purpose, keyIndex, used from keys where scriptPubKey=?")
	defer stmt.Close()

	var scriptPubKey string
	var purpose int
	var index int
	var used int
	err = stmt.QueryRow(hex.EncodeToString(b)).Scan(&scriptPubKey, &purpose, &index, &used)
	if err != nil {
		t.Error(err)
	}
	if used != 1 {
		t.Errorf(`Expected 1 got %s`, used)
	}
}

func TestGetLastKeyIndex(t *testing.T) {
	var last []byte
	for i := 0; i < 100; i++ {
		b := make([]byte, 32)
		rand.Read(b)
		err := kdb.Put(b, spvwallet.KeyPath{spvwallet.EXTERNAL, i})
		if err != nil {
			t.Error(err)
		}
		last = b
	}
	idx, used, err := kdb.GetLastKeyIndex(spvwallet.EXTERNAL)
	if err != nil || idx != 99 || used != false {
		t.Error("Failed to fetch correct last index")
	}
	kdb.MarkKeyAsUsed(last)
	_, used, err = kdb.GetLastKeyIndex(spvwallet.EXTERNAL)
	if err != nil || used != true {
		t.Error("Failed to fetch correct last index")
	}
}

func TestGetPathForScript(t *testing.T) {
	b := make([]byte, 32)
	rand.Read(b)
	err := kdb.Put(b, spvwallet.KeyPath{spvwallet.EXTERNAL, 15})
	if err != nil {
		t.Error(err)
	}
	path, err := kdb.GetPathForScript(b)
	if err != nil {
		t.Error(err)
	}
	if path.Index != 15 || path.Purpose != spvwallet.EXTERNAL {
		t.Error("Returned incorrect key path")
	}
}

func TestKeyNotFound(t *testing.T) {
	b := make([]byte, 32)
	rand.Read(b)
	_, err := kdb.GetPathForScript(b)
	if err == nil {
		t.Error("Return key when it should not have")
	}
}

func TestGetUnsed(t *testing.T) {
	for i := 0; i < 100; i++ {
		b := make([]byte, 32)
		rand.Read(b)
		err := kdb.Put(b, spvwallet.KeyPath{spvwallet.EXTERNAL, i})
		if err != nil {
			t.Error(err)
		}
	}
	idx, err := kdb.GetUnused(spvwallet.EXTERNAL)
	if err != nil || idx != 0 {
		t.Error("Failed to fetch correct unused")
	}
}

func TestGetLookaheadWindows(t *testing.T) {
	for i := 0; i < 100; i++ {
		b := make([]byte, 32)
		rand.Read(b)
		err := kdb.Put(b, spvwallet.KeyPath{spvwallet.EXTERNAL, i})
		if err != nil {
			t.Error(err)
		}
		if i < 50 {
			kdb.MarkKeyAsUsed(b)
		}
		b = make([]byte, 32)
		rand.Read(b)
		err = kdb.Put(b, spvwallet.KeyPath{spvwallet.INTERNAL, i})
		if err != nil {
			t.Error(err)
		}
		if i < 50 {
			kdb.MarkKeyAsUsed(b)
		}
	}
	windows := kdb.GetLookaheadWindows()
	if windows[spvwallet.EXTERNAL] != 50 || windows[spvwallet.INTERNAL] != 50 {
		t.Error("Fetched incorrect lookahead windows")
	}

}
