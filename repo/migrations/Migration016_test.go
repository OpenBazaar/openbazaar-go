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

const preMigration016Config = `{
	"OtherConfigProperty1": [1, 2, 3],
	"OtherConfigProperty2": "abc123",
	"Wallets":{
		"BTC": {
			"API": [
					"https://btc.api.openbazaar.org/api"
			],
			"APITestnet": [
					"https://tbtc.api.openbazaar.org/api"
			]
		},
		"BCH": {
			"API": [
					"https://bch.api.openbazaar.org/api"
			],
			"APITestnet": [
					"https://tbch.api.openbazaar.org/api"
			]
		},
		"LTC": {
			"API": [
					"https://ltc.api.openbazaar.org/api"
			],
			"APITestnet": [
					"https://tltc.api.openbazaar.org/api"
			]
		},
		"ZEC": {
			"API": [
					"https://zec.api.openbazaar.org/api"
			],
			"APITestnet": [
					"https://tzec.api.openbazaar.org/api"
			]
		}
	}
}`

const postMigration016Config = `{
	"OtherConfigProperty1": [1, 2, 3],
	"OtherConfigProperty2": "abc123",
	"Wallets": {
		"BTC": {
			"Type": "API",
			"API": [
					"https://btc.blockbook.api.openbazaar.org/api"
			],
			"APITestnet": [
					"https://tbtc.blockbook.api.openbazaar.org/api"
			],
			"MaxFee": 200,
			"FeeAPI": "https://btc.fees.openbazaar.org",
			"HighFeeDefault": 50,
			"MediumFeeDefault": 10,
			"LowFeeDefault": 1,
			"TrustedPeer": "",
			"WalletOptions": null
		},
		"BCH": {
			"Type": "API",
			"API": [
				"https://bch.blockbook.api.openbazaar.org/api"
			],
			"APITestnet": [
				"https://tbch.blockbook.api.openbazaar.org/api"
			],
			"MaxFee": 200,
			"FeeAPI": "",
			"HighFeeDefault": 10,
			"MediumFeeDefault": 5,
			"LowFeeDefault": 1,
			"TrustedPeer": "",
			"WalletOptions": null
		},
		"LTC": {
			"Type": "API",
			"API": [
				"https://ltc.blockbook.api.openbazaar.org/api"
			],
			"APITestnet": [
				"https://tltc.blockbook.api.openbazaar.org/api"
			],
			"MaxFee": 200,
			"FeeAPI": "",
			"HighFeeDefault": 20,
			"MediumFeeDefault": 10,
			"LowFeeDefault": 5,
			"TrustedPeer": "",
			"WalletOptions": null
		},
		"ZEC": {
			"Type": "API",
			"API": [
				"https://zec.blockbook.api.openbazaar.org/api"
			],
			"APITestnet": [
				"https://tzec.blockbook.api.openbazaar.org/api"
			],
			"MaxFee": 200,
			"FeeAPI": "",
			"HighFeeDefault": 20,
			"MediumFeeDefault": 10,
			"LowFeeDefault": 5,
			"TrustedPeer": "",
			"WalletOptions": null
		},
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
				"RegistryAddress": "0x403d907982474cdd51687b09a8968346159378f3",
				"RinkebyRegistryAddress": "0x403d907982474cdd51687b09a8968346159378f3",
				"RopstenRegistryAddress": "0x403d907982474cdd51687b09a8968346159378f3"
			}
		}
	}
}`

func migration016AssertAPI(t *testing.T, actual interface{}, expected string) {
	actualSlice := actual.([]interface{})
	if len(actualSlice) != 1 || actualSlice[0] != expected {
		t.Fatalf("incorrect api endpoint.\n\twanted: %s\n\tgot: %s\n", expected, actual)
	}
}

func TestMigration016(t *testing.T) {
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
	if err = ioutil.WriteFile(configPath, []byte(preMigration016Config), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	if err = ioutil.WriteFile(repoverPath, []byte("15"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	var m migrations.Migration016
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
	btc := w["BTC"].(map[string]interface{})
	bch := w["BCH"].(map[string]interface{})
	ltc := w["LTC"].(map[string]interface{})
	zec := w["ZEC"].(map[string]interface{})

	migration016AssertAPI(t, btc["API"], "https://btc.blockbook.api.openbazaar.org/api")
	migration016AssertAPI(t, btc["APITestnet"], "https://tbtc.blockbook.api.openbazaar.org/api")
	migration016AssertAPI(t, bch["API"], "https://bch.blockbook.api.openbazaar.org/api")
	migration016AssertAPI(t, bch["APITestnet"], "https://tbch.blockbook.api.openbazaar.org/api")
	migration016AssertAPI(t, ltc["API"], "https://ltc.blockbook.api.openbazaar.org/api")
	migration016AssertAPI(t, ltc["APITestnet"], "https://tltc.blockbook.api.openbazaar.org/api")
	migration016AssertAPI(t, zec["API"], "https://zec.blockbook.api.openbazaar.org/api")
	migration016AssertAPI(t, zec["APITestnet"], "https://tzec.blockbook.api.openbazaar.org/api")

	var re = regexp.MustCompile(`\s`)
	if re.ReplaceAllString(string(configBytes), "") != re.ReplaceAllString(string(postMigration016Config), "") {
		t.Logf("actual: %s", re.ReplaceAllString(string(configBytes), ""))
		t.Fatal("incorrect post-migration config")
	}

	assertCorrectRepoVer(t, repoverPath, "17")

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

	w = config["Wallets"].(map[string]interface{})
	btc = w["BTC"].(map[string]interface{})
	bch = w["BCH"].(map[string]interface{})
	ltc = w["LTC"].(map[string]interface{})
	zec = w["ZEC"].(map[string]interface{})

	migration016AssertAPI(t, btc["API"], "https://btc.api.openbazaar.org/api")
	migration016AssertAPI(t, btc["APITestnet"], "https://tbtc.api.openbazaar.org/api")
	migration016AssertAPI(t, bch["API"], "https://bch.api.openbazaar.org/api")
	migration016AssertAPI(t, bch["APITestnet"], "https://tbch.api.openbazaar.org/api")
	migration016AssertAPI(t, ltc["API"], "https://ltc.api.openbazaar.org/api")
	migration016AssertAPI(t, ltc["APITestnet"], "https://tltc.api.openbazaar.org/api")
	migration016AssertAPI(t, zec["API"], "https://zec.api.openbazaar.org/api")
	migration016AssertAPI(t, zec["APITestnet"], "https://tzec.api.openbazaar.org/api")

	assertCorrectRepoVer(t, repoverPath, "16")
}
