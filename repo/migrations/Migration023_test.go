package migrations_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"regexp"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

const preMigration023Config = `{
	"OtherConfigProperty11": [1, 2, 3],
	"OtherConfigProperty21": "abc123",
	"Wallets":{
		"ETH": {
			"API": [
					"https://mainnet.infura.io"
			],
			"APITestnet": [
					"https://rinkeby.infura.io"
			]
		}
	}
}`

const postMigration023Config = `{
	"OtherConfigProperty11": [1, 2, 3],
	"OtherConfigProperty21": "abc123",
	"Wallets": {
		"ETH": {
			"Type": "API",
			"API": [
				"https://mainnet.infura.io"
			],
			"APITestnet": [
				"https://rinkeby.infura.io"
			],
			"MaxFee": 200,
			"FeeAPI": "",
			"HighFeeDefault": 30,
			"MediumFeeDefault": 15,
			"LowFeeDefault": 7,
			"TrustedPeer": "",
			"WalletOptions": {
				"RegistryAddress": "0x5c69ccf91eab4ef80d9929b3c1b4d5bc03eb0981",
				"RinkebyRegistryAddress": "0x5cEF053c7b383f430FC4F4e1ea2F7D31d8e2D16C",
				"RopstenRegistryAddress": "0x403d907982474cdd51687b09a8968346159378f3"
			}
		}
	}
}`

func migration023AssertAPI(t *testing.T, actual interface{}, expected string) {
	actualSlice := actual.([]interface{})
	if len(actualSlice) != 1 || actualSlice[0] != expected {
		t.Fatalf("incorrect api endpoint.\n\twanted: %s\n\tgot: %s\n", expected, actual)
	}
}

func TestMigration023(t *testing.T) {
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
	if err = ioutil.WriteFile(configPath, []byte(preMigration023Config), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	if err = ioutil.WriteFile(repoverPath, []byte("21"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	var m migrations.Migration023
	err = m.Up(testRepo.DataPath(), "", true)
	if err != nil {
		t.Fatal(err)
	}

	configBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	config := map[string]interface{}{}
	if err = json.Unmarshal(configBytes, &config); err != nil {
		t.Fatal(err)
	}

	w := config["Wallets"].(map[string]interface{})
	eth := w["ETH"].(map[string]interface{})

	migration023AssertAPI(t, eth["API"], "https://mainnet.infura.io")
	migration023AssertAPI(t, eth["APITestnet"], "https://rinkeby.infura.io")

	var re = regexp.MustCompile(`\s`)
	if re.ReplaceAllString(string(configBytes), "") != re.ReplaceAllString(string(postMigration023Config), "") {
		t.Logf("actual: %s", re.ReplaceAllString(string(configBytes), ""))
		t.Fatal("incorrect post-migration config")
	}

	assertCorrectRepoVer(t, repoverPath, "24")

	err = m.Down(testRepo.DataPath(), "", true)
	if err != nil {
		t.Fatal(err)
	}

	configBytes, err = ioutil.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	config = map[string]interface{}{}
	if err = json.Unmarshal(configBytes, &config); err != nil {
		t.Fatal(err)
	}

	assertCorrectRepoVer(t, repoverPath, "23")
}
