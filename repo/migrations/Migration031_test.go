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

const preAM01Config = `{
	"OtherConfigProperty11": [1, 2, 3],
	"OtherConfigProperty21": "abc123",
	"Wallets": {
	  "BTC":{
	    "APIPool": [
				"https://btc.blockbook.api.openbazaar.org/api"
			],
			"APITestnetPool": [
				"https://tbtc.blockbook.api.openbazaar.org/api"
			]
	  },
	  "BCH":{
	    "APIPool": [
				"https://bch.blockbook.api.openbazaar.org/api"
			],
			"APITestnetPool": [
				"https://bch.blockbook.api.openbazaar.org/api"
			]
	  },
	  "LTC":{
	    "APIPool": [
				"https://ltc.blockbook.api.openbazaar.org/api"
			],
			"APITestnetPool": [
				"https://tltc.blockbook.api.openbazaar.org/api"
			]
	  },
	  "ZEC":{
	    "APIPool": [
				"https://zec.blockbook.api.openbazaar.org/api"
			],
			"APITestnetPool": [
				"https://tzec.blockbook.api.openbazaar.org/api"
			]
	  },
		"ETH": {
			"APIPool": [
				"https://mainnet.infura.io"
			],
			"APITestnetPool": [
				"https://rinkeby.infura.io"
			],
			"WalletOptions": {
				"RegistryAddress": "0x5c69ccf91eab4ef80d9929b3c1b4d5bc03eb0981",
				"RinkebyRegistryAddress": "0x5cEF053c7b383f430FC4F4e1ea2F7D31d8e2D16C",
				"RopstenRegistryAddress": "0x403d907982474cdd51687b09a8968346159378f3"
			}
		}
	}
}`

const postAM01Config = `{
	"OtherConfigProperty11": [1, 2, 3],
	"OtherConfigProperty21": "abc123",
	"Wallets": {
	  "BCH":{
	    "APIPool": [
				"https://bch.api.openbazaar.org/api"
			],
			"APITestnetPool": [
				"https://tbch.api.openbazaar.org/api"
			]
	  },
	  "BTC":{
	    "APIPool": [
				"https://btc.api.openbazaar.org/api"
			],
			"APITestnetPool": [
				"https://tbtc.api.openbazaar.org/api"
			]
	  },
	  "ETH": {
			"APIPool": [
				"https://mainnet.infura.io"
			],
			"APITestnetPool": [
				"https://rinkeby.infura.io"
			],
			"WalletOptions": {
				"RegistryAddress": "0x5c69ccf91eab4ef80d9929b3c1b4d5bc03eb0981",
				"RinkebyRegistryAddress": "0x5cEF053c7b383f430FC4F4e1ea2F7D31d8e2D16C",
				"RopstenRegistryAddress": "0x403d907982474cdd51687b09a8968346159378f3"
			}
		},
	  "LTC":{
	    "APIPool": [
				"https://ltc.api.openbazaar.org/api"
			],
			"APITestnetPool": [
				"https://tltc.api.openbazaar.org/api"
			]
	  },
	  "ZEC":{
	    "APIPool": [
				"https://zec.api.openbazaar.org/api"
			],
			"APITestnetPool": [
				"https://tzec.api.openbazaar.org/api"
			]
	  }
	}
}`

func AM01AssertAPI(t *testing.T, actual interface{}, expected string) {
	actualSlice := actual.([]interface{})
	if len(actualSlice) != 1 || actualSlice[0] != expected {
		t.Fatalf("incorrect api endpoint.\n\twanted: %s\n\tgot: %s\n", expected, actual)
	}
}

func TestAM01(t *testing.T) {
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
	if err = ioutil.WriteFile(configPath, []byte(preAM01Config), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	if err = ioutil.WriteFile(repoverPath, []byte("29"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	var m migrations.Migration031
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

	AM01AssertAPI(t, eth["APIPool"], "https://mainnet.infura.io")
	AM01AssertAPI(t, eth["APITestnetPool"], "https://rinkeby.infura.io")

	btc := w["BTC"].(map[string]interface{})

	AM01AssertAPI(t, btc["APIPool"], "https://btc.api.openbazaar.org/api")
	AM01AssertAPI(t, btc["APITestnetPool"], "https://tbtc.api.openbazaar.org/api")

	bch := w["BCH"].(map[string]interface{})

	AM01AssertAPI(t, bch["APIPool"], "https://bch.api.openbazaar.org/api")
	AM01AssertAPI(t, bch["APITestnetPool"], "https://tbch.api.openbazaar.org/api")

	ltc := w["LTC"].(map[string]interface{})

	AM01AssertAPI(t, ltc["APIPool"], "https://ltc.api.openbazaar.org/api")
	AM01AssertAPI(t, ltc["APITestnetPool"], "https://tltc.api.openbazaar.org/api")

	zec := w["ZEC"].(map[string]interface{})

	AM01AssertAPI(t, zec["APIPool"], "https://zec.api.openbazaar.org/api")
	AM01AssertAPI(t, zec["APITestnetPool"], "https://tzec.api.openbazaar.org/api")

	var re = regexp.MustCompile(`\s`)
	if re.ReplaceAllString(string(configBytes), "") != re.ReplaceAllString(string(postAM01Config), "") {
		t.Logf("actual: %s", re.ReplaceAllString(string(configBytes), ""))
		t.Logf("expected: %s", re.ReplaceAllString(string(postAM01Config), ""))
		t.Fatal("incorrect post-migration config")
	}

	assertCorrectRepoVer(t, repoverPath, "31")

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
	eth = w["ETH"].(map[string]interface{})

	AM01AssertAPI(t, eth["APIPool"], "https://mainnet.infura.io")
	AM01AssertAPI(t, eth["APITestnetPool"], "https://rinkeby.infura.io")

	btc = w["BTC"].(map[string]interface{})

	AM01AssertAPI(t, btc["APIPool"], "https://btc.blockbook.api.openbazaar.org/api")
	AM01AssertAPI(t, btc["APITestnetPool"], "https://tbtc.blockbook.api.openbazaar.org/api")

	bch = w["BCH"].(map[string]interface{})

	AM01AssertAPI(t, bch["APIPool"], "https://bch.blockbook.api.openbazaar.org/api")
	AM01AssertAPI(t, bch["APITestnetPool"], "https://tbch.blockbook.api.openbazaar.org/api")

	ltc = w["LTC"].(map[string]interface{})

	AM01AssertAPI(t, ltc["APIPool"], "https://ltc.blockbook.api.openbazaar.org/api")
	AM01AssertAPI(t, ltc["APITestnetPool"], "https://tltc.blockbook.api.openbazaar.org/api")

	zec = w["ZEC"].(map[string]interface{})

	AM01AssertAPI(t, zec["APIPool"], "https://zec.blockbook.api.openbazaar.org/api")
	AM01AssertAPI(t, zec["APITestnetPool"], "https://tzec.blockbook.api.openbazaar.org/api")

	assertCorrectRepoVer(t, repoverPath, "30")
}
