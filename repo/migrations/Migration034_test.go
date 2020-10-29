package migrations_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func TestMigration034(t *testing.T) {
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

	wcfg := schema.DefaultWalletsConfig()
	wcfg.FIL = nil

	defaultConfig := map[string]interface{}{
		"Wallets": wcfg,
	}

	cfgBytes, err := json.MarshalIndent(defaultConfig, "", "    ")
	if err != nil {
		t.Fatal(err)
	}

	if err = ioutil.WriteFile(configPath, []byte(cfgBytes), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	var m migrations.Migration034
	err = m.Up(testRepo.DataPath(), "", true)
	if err != nil {
		t.Fatal(err)
	}

	configBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	configInterface := make(map[string]interface{})
	if err = json.Unmarshal(configBytes, &configInterface); err != nil {
		t.Fatalf("unmarshaling invalid json after migration: %s", err.Error())
	}

	walletCfgIface, ok := configInterface["Wallets"]
	if !ok {
		t.Error("expected 'Wallets' key to be present")
	}

	walletCfg, ok := walletCfgIface.(map[string]interface{})
	if !ok {
		t.Error("error unmarshalling config")
	}

	if _, ok := walletCfg["FIL"]; !ok {
		t.Error("expected 'FIL' key to be present")
	}

	assertCorrectRepoVer(t, repoverPath, "35")

	err = m.Down(testRepo.DataPath(), "", true)
	if err != nil {
		t.Fatal(err)
	}

	configBytes, err = ioutil.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	configInterface = make(map[string]interface{})
	if err = json.Unmarshal(configBytes, &configInterface); err != nil {
		t.Fatalf("unmarshaling invalid json after migration: %s", err.Error())
	}

	walletCfgIface, ok = configInterface["Wallets"]
	if !ok {
		t.Error("expected 'Wallets' key to be present")
	}

	walletCfg, ok = walletCfgIface.(map[string]interface{})
	if !ok {
		t.Error("error unmarshalling config")
	}

	if _, ok := walletCfg["FIL"]; ok {
		t.Error("expected 'FIL' key to be deleted")
	}

	assertCorrectRepoVer(t, repoverPath, "34")
}
