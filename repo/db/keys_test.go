package db_test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/btcec"
)

func buildNewKeyStore() (repo.KeyStore, func(), error) {
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
	return db.NewKeyStore(database, new(sync.Mutex), wallet.Bitcoin), appSchema.DestroySchemaDirectories, nil
}

func TestGetAll(t *testing.T) {
	var kdb, teardown, err = buildNewKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	for i := 0; i < 100; i++ {
		b := make([]byte, 32)
		_, err = rand.Read(b)
		if err != nil {
			t.Log(err)
		}
		err := kdb.Put(b, wallet.KeyPath{Purpose: wallet.EXTERNAL, Index: i})
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
	var kdb, teardown, err = buildNewKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	b := make([]byte, 32)
	err = kdb.Put(b, wallet.KeyPath{Purpose: wallet.EXTERNAL, Index: 0})
	if err != nil {
		t.Error(err)
	}
	stmt, _ := kdb.PrepareQuery("select scriptAddress, purpose, keyIndex, used from keys where scriptAddress=?")
	defer stmt.Close()

	var scriptAddress string
	var purpose int
	var index int
	var used int
	err = stmt.QueryRow(hex.EncodeToString(b)).Scan(&scriptAddress, &purpose, &index, &used)
	if err != nil {
		t.Error(err)
	}
	if scriptAddress != hex.EncodeToString(b) {
		t.Errorf(`Expected %s got %s`, hex.EncodeToString(b), scriptAddress)
	}
	if purpose != 0 {
		t.Errorf(`Expected 0 got %d`, purpose)
	}
	if index != 0 {
		t.Errorf(`Expected 0 got %d`, index)
	}
	if used != 0 {
		t.Errorf(`Expected 0 got %v`, used)
	}
}

func TestKeysDB_GetImported(t *testing.T) {
	var kdb, teardown, err = buildNewKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	key, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		t.Error(err)
	}
	err = kdb.ImportKey([]byte("fsdfa"), key)
	if err != nil {
		t.Error(err)
	}

	keys, err := kdb.GetImported()
	if err != nil {
		t.Error(err)
	}
	if len(keys) != 1 {
		t.Error("Failed to return imported key")
	}
	if !bytes.Equal(key.Serialize(), keys[0].Serialize()) {
		t.Error("Returned incorrect key")
	}
}

func TestImportKey(t *testing.T) {
	var kdb, teardown, err = buildNewKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	key, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		t.Error(err)
	}
	var b []byte
	for i := 0; i < 32; i++ {
		b = append(b, 0xff)
	}
	err = kdb.ImportKey(b, key)
	if err != nil {
		t.Error(err)
	}
	stmt, _ := kdb.PrepareQuery("select scriptAddress, purpose, used, key from keys where scriptAddress=?")
	defer stmt.Close()

	var scriptAddress string
	var purpose int
	var used int
	var keyHex string
	err = stmt.QueryRow(hex.EncodeToString(b)).Scan(&scriptAddress, &purpose, &used, &keyHex)
	if err != nil {
		t.Error(err)
	}
	if scriptAddress != hex.EncodeToString(b) {
		t.Errorf(`Expected %s got %s`, hex.EncodeToString(b), scriptAddress)
	}
	if purpose != -1 {
		t.Errorf(`Expected -1 got %d`, purpose)
	}
	if used != 0 {
		t.Errorf(`Expected 0 got %v`, used)
	}
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(key.Serialize(), keyBytes) {
		t.Errorf(`Expected %s got %s`, hex.EncodeToString(b), hex.EncodeToString(keyBytes))
	}
}

func TestPutDuplicateKey(t *testing.T) {
	var kdb, teardown, err = buildNewKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	b := make([]byte, 32)
	if err := kdb.Put(b, wallet.KeyPath{Purpose: wallet.EXTERNAL, Index: 0}); err != nil {
		t.Fatal(err)
	}
	err = kdb.Put(b, wallet.KeyPath{Purpose: wallet.EXTERNAL, Index: 0})
	if err == nil {
		t.Error("Expected duplicate key error")
	}
}

func TestMarkKeyAsUsed(t *testing.T) {
	var kdb, teardown, err = buildNewKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	b := make([]byte, 33)
	err = kdb.Put(b, wallet.KeyPath{Purpose: wallet.EXTERNAL, Index: 0})
	if err != nil {
		t.Error(err)
	}
	err = kdb.MarkKeyAsUsed(b)
	if err != nil {
		t.Error(err)
	}
	stmt, _ := kdb.PrepareQuery("select scriptAddress, purpose, keyIndex, used from keys where scriptAddress=?")
	defer stmt.Close()

	var scriptAddress string
	var purpose int
	var index int
	var used int
	err = stmt.QueryRow(hex.EncodeToString(b)).Scan(&scriptAddress, &purpose, &index, &used)
	if err != nil {
		t.Error(err)
	}
	if used != 1 {
		t.Errorf(`Expected 1 got %v`, used)
	}
}

func TestGetLastKeyIndex(t *testing.T) {
	var kdb, teardown, err = buildNewKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	var last []byte
	for i := 0; i < 100; i++ {
		b := make([]byte, 32)
		_, err = rand.Read(b)
		if err != nil {
			t.Log(err)
		}
		err := kdb.Put(b, wallet.KeyPath{Purpose: wallet.EXTERNAL, Index: i})
		if err != nil {
			t.Error(err)
		}
		last = b
	}
	idx, used, err := kdb.GetLastKeyIndex(wallet.EXTERNAL)
	if err != nil || idx != 99 || used {
		t.Error("Failed to fetch correct last index")
	}
	err = kdb.MarkKeyAsUsed(last)
	if err != nil {
		t.Log(err)
	}
	_, used, err = kdb.GetLastKeyIndex(wallet.EXTERNAL)
	if err != nil || !used {
		t.Error("Failed to fetch correct last index")
	}
}

func TestGetPathForKey(t *testing.T) {
	var kdb, teardown, err = buildNewKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	b := make([]byte, 32)
	_, err = rand.Read(b)
	if err != nil {
		t.Log(err)
	}
	err = kdb.Put(b, wallet.KeyPath{Purpose: wallet.EXTERNAL, Index: 15})
	if err != nil {
		t.Error(err)
	}
	path, err := kdb.GetPathForKey(b)
	if err != nil {
		t.Error(err)
	}
	if path.Index != 15 || path.Purpose != wallet.EXTERNAL {
		t.Error("Returned incorrect key path")
	}
}

func TestGetKey(t *testing.T) {
	var kdb, teardown, err = buildNewKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	key, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		t.Error(err)
	}
	var b []byte
	for i := 0; i < 32; i++ {
		b = append(b, 0xee)
	}
	err = kdb.ImportKey(b, key)
	if err != nil {
		t.Error(err)
	}
	k, err := kdb.GetKey(b)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(key.Serialize(), k.Serialize()) {
		t.Error("Failed to return imported key")
	}
}

func TestKeyNotFound(t *testing.T) {
	var kdb, teardown, err = buildNewKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	b := make([]byte, 32)
	_, err = rand.Read(b)
	if err != nil {
		t.Log(err)
	}
	_, err = kdb.GetPathForKey(b)
	if err == nil {
		t.Error("Return key when it shouldn't have")
	}
}

func TestGetUnsed(t *testing.T) {
	var kdb, teardown, err = buildNewKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	for i := 0; i < 100; i++ {
		b := make([]byte, 32)
		_, err = rand.Read(b)
		if err != nil {
			t.Log(err)
		}
		err := kdb.Put(b, wallet.KeyPath{Purpose: wallet.INTERNAL, Index: i})
		if err != nil {
			t.Error(err)
		}
	}
	idx, err := kdb.GetUnused(wallet.INTERNAL)
	if err != nil {
		t.Error("Failed to fetch correct unused")
	}
	if len(idx) != 100 {
		t.Error("Failed to fetch correct unused")
	}
}

func TestGetLookaheadWindows(t *testing.T) {
	var kdb, teardown, err = buildNewKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	for i := 0; i < 100; i++ {
		b := make([]byte, 32)
		_, err = rand.Read(b)
		if err != nil {
			t.Log(err)
		}
		err := kdb.Put(b, wallet.KeyPath{Purpose: wallet.EXTERNAL, Index: i})
		if err != nil {
			t.Error(err)
		}
		if i < 50 {
			err = kdb.MarkKeyAsUsed(b)
			if err != nil {
				t.Log(err)
			}
		}
		b = make([]byte, 32)
		_, err = rand.Read(b)
		if err != nil {
			t.Log(err)
		}
		err = kdb.Put(b, wallet.KeyPath{Purpose: wallet.INTERNAL, Index: i})
		if err != nil {
			t.Error(err)
		}
		if i < 50 {
			err = kdb.MarkKeyAsUsed(b)
			if err != nil {
				t.Log(err)
			}
		}
	}
	windows := kdb.GetLookaheadWindows()
	if windows[wallet.EXTERNAL] != 50 || windows[wallet.INTERNAL] != 50 {
		t.Error("Fetched incorrect lookahead windows")
	}

}
