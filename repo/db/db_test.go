package db

import (
	"github.com/OpenBazaar/wallet-interface"
	"os"
	"path"
	"testing"
	"time"
)

var testDB *SQLiteDatastore

func TestMain(m *testing.M) {
	setup()
	retCode := m.Run()
	teardown()
	os.Exit(retCode)
}

func setup() {
	os.MkdirAll(path.Join("./", "datastore"), os.ModePerm)
	testDB, _ = Create("", "LetMeIn", false, wallet.Bitcoin)
	testDB.config.Init("Mnemonic Passphrase", []byte("Private Key"), "LetMeIn", time.Now())
}

func teardown() {
	os.RemoveAll(path.Join("./", "datastore"))
}

func TestCreate(t *testing.T) {
	if _, err := os.Stat(path.Join("./", "datastore", "mainnet.db")); os.IsNotExist(err) {
		t.Error("Failed to create database file")
	}
}

func TestInit(t *testing.T) {
	mn, err := testDB.config.GetMnemonic()
	if err != nil {
		t.Error(err)
	}
	if mn != "Mnemonic Passphrase" {
		t.Error("Config returned wrong mnemonic")
	}
	pk, err := testDB.config.GetIdentityKey()
	if err != nil {
		t.Error(err)
	}
	testKey := []byte("Private Key")
	for i := range pk {
		if pk[i] != testKey[i] {
			t.Error("Config returned wrong identity key")
		}
	}
}

func TestInterface(t *testing.T) {
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
	encrypted := testDB.Config().IsEncrypted()
	if encrypted {
		t.Error("IsEncrypted returned incorrectly")
	}
}
