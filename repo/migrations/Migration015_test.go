package migrations_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

const preMigrationConfig = `{"Wallet":{
	"Binary": "",
	"FeeAPI": "https://btc.fees.openbazaar.org",
	"HighFeeDefault": 160,
	"LowFeeDefault": 20,
	"MaxFee": 2000,
	"MediumFeeDefault": 60,
	"TrustedPeer": "",
	"Type": "spvwallet"
}}`

func TestMigration015(t *testing.T) {
	var testRepo, err = schema.NewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err = testRepo.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer testRepo.DestroySchemaDirectories()

	var (
		configPath  = testRepo.DataPathJoin("config")
		repoverPath = testRepo.DataPathJoin("repover")
	)
	if err = ioutil.WriteFile(configPath, []byte(preMigrationConfig), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	if err = ioutil.WriteFile(repoverPath, []byte("15"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	var m migrations.Migration015
	err = m.Up(testRepo.DataPath(), "", true)
	if err != nil {
		t.Fatal(err)
	}

	configBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var configInterface map[string]json.RawMessage
	if err = json.Unmarshal(configBytes, &configInterface); err != nil {
		t.Fatalf("unmarshaling invalid json after migration: %s", err.Error())
	}

	if _, ok := configInterface["Wallet"]; ok {
		t.Error("expected 'Wallet' key to be present")
	}

	if _, ok := configInterface["Wallets"]; !ok {
		t.Error("missing expected 'Wallets' key after migration")
	}

	if _, ok := configInterface["LegacyWallet"]; !ok {
		t.Error("missing expected 'LegacyWallet' key after migration")
	}

	var actualWalletsConfig = new(schema.WalletsConfig)
	if err = json.Unmarshal(configInterface["Wallets"], actualWalletsConfig); err != nil {
		t.Fatalf("unmarshaling invalid multiwallet config: %s", err.Error())
	}

	assertCorrectRepoVer(t, repoverPath, "16")

	err = m.Down(testRepo.DataPath(), "", true)
	if err != nil {
		t.Fatal(err)
	}

	configBytes, err = ioutil.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	configInterface = make(map[string]json.RawMessage)
	if err = json.Unmarshal(configBytes, &configInterface); err != nil {
		t.Fatalf("unmarshaling invalid json after migration: %s", err.Error())
	}

	if _, ok := configInterface["Wallet"]; !ok {
		t.Error("missing expected 'Wallet' key after reversing migration")
	}

	if _, ok := configInterface["Wallets"]; ok {
		t.Error("expected 'Wallets' key to be removed")
	}

	if _, ok := configInterface["LegacyWallet"]; ok {
		t.Error("expected 'LegacyWallet' key to be removed")
	}
	assertCorrectRepoVer(t, repoverPath, "15")
}
