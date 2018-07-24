package db

import (
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/schema"
)

func buildNewDatastore() (*SQLiteDatastore, func(), error) {
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
	datastore := NewSQLiteDatastore(database, new(sync.Mutex))
	return datastore, appSchema.DestroySchemaDirectories, nil
}

func TestCreate(t *testing.T) {
	if err := os.MkdirAll(path.Join("./", "datastore"), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path.Join("./", "datastore"))
	_, err := Create("", "LetMeIn", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path.Join("./", "datastore", "mainnet.db")); os.IsNotExist(err) {
		t.Error("Failed to create database file")
	}
}

func TestInit(t *testing.T) {
	var (
		mnemonic  = "Mnemonic Passphrase"
		key       = "Private Key"
		createdAt = time.Now()
	)

	if err := os.MkdirAll(path.Join("./", "datastore"), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path.Join("./", "datastore"))
	testDB, err := Create("", "LetMeIn", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := testDB.Config().Init(mnemonic, []byte(key), "", createdAt); err != nil {
		t.Fatal(err)
	}

	mn, err := testDB.Config().GetMnemonic()
	if err != nil {
		t.Error(err)
	}
	if mn != mnemonic {
		t.Error("Config returned wrong mnemonic")
	}
	pk, err := testDB.Config().GetIdentityKey()
	if err != nil {
		t.Error(err)
	}
	testKey := []byte(key)
	for i := range pk {
		if pk[i] != testKey[i] {
			t.Error("Config returned wrong identity key")
		}
	}
}

func TestInterface(t *testing.T) {
	testDB, teardown, err := buildNewDatastore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	if testDB.Config() != testDB.config {
		t.Error("Config() return wrong value")
	}
	if testDB.Followers() != testDB.followers {
		t.Error("Followers() return wrong value")
	}
	if testDB.Following() != testDB.following {
		t.Error("Following() return wrong value")
	}
	if testDB.OfflineMessages() != testDB.offlineMessages {
		t.Error("OfflineMessages() return wrong value")
	}
	if testDB.Pointers() != testDB.pointers {
		t.Error("Pointers() return wrong value")
	}
	if testDB.Keys() != testDB.keys {
		t.Error("Keys() return wrong value")
	}
	if testDB.Txns() != testDB.txns {
		t.Error("Txns() return wrong value")
	}
	if testDB.Stxos() != testDB.stxos {
		t.Error("Stxos() return wrong value")
	}
	if testDB.Utxos() != testDB.utxos {
		t.Error("Utxos() return wrong value")
	}
	if testDB.Settings() != testDB.settings {
		t.Error("Settings() return wrong value")
	}
	if testDB.Inventory() != testDB.inventory {
		t.Error("Inventory() return wrong value")
	}
}

func TestEncryptedDb(t *testing.T) {
	testDB, teardown, err := buildNewDatastore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	encrypted := testDB.Config().IsEncrypted()
	if encrypted {
		t.Error("IsEncrypted returned incorrectly")
	}
}
