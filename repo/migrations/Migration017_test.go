package migrations_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

const preMigration017Config = `{
	"DataSharing": {
		"AcceptStoreRequests": false,
		"PushTo": [
			"QmY8puEnVx66uEet64gAf4VZRo7oUyMCwG6KdB9KM92EGQ",
			"QmPPg2qeF3n2KvTRXRZLaTwHCw8JxzF4uZK93RfMoDvf2o"
		]
	},
	"OtherConfigProperty1": [1, 2, 3],
	"OtherConfigProperty2": "abc123"
}`

const postMigration017Config = `{
	"DataSharing": {
		"AcceptStoreRequests": false,
		"PushTo": [
			"QmY8puEnVx66uEet64gAf4VZRo7oUyMCwG6KdB9KM92EGQ",
			"QmPPg2qeF3n2KvTRXRZLaTwHCw8JxzF4uZK93RfMoDvf2o"
		]
	},
	"OtherConfigProperty1": [1, 2, 3],
	"OtherConfigProperty2": "abc123"
}`

func migration017AssertAPI(t *testing.T, actual interface{}, expected string) {
	actualSlice := actual.([]interface{})
	if len(actualSlice) != 1 || actualSlice[0] != expected {
		t.Fatalf("incorrect api endpoint.\n\twanted: %s\n\tgot: %s\n", expected, actual)
	}
}

func TestMigration017(t *testing.T) {
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
	if err = ioutil.WriteFile(configPath, []byte(preMigration017Config), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	if err = ioutil.WriteFile(repoverPath, []byte("15"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	var m migrations.Migration017
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

	ds := config["DataSharing"].(map[string]interface{})

	if accept, ok := ds["AcceptStoreRequests"].(bool); !ok || accept {
		t.Fatal("Incorrect config value for AcceptStoreRequests")
	}

	pushTo := ds["PushTo"].([]interface{})
	for i, peer := range pushTo {
		if peer != migrations.Migration017PushToAfter[i] {
			t.Fatal("Unexpected peer", peer, "wanted:", migrations.Migration017PushToAfter[i])
		}
	}
	assertCorrectRepoVer(t, repoverPath, "18")

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

	ds = config["DataSharing"].(map[string]interface{})

	if accept, ok := ds["AcceptStoreRequests"].(bool); !ok || accept {
		t.Fatal("Incorrect config value for AcceptStoreRequests")
	}

	pushTo = ds["PushTo"].([]interface{})
	for i, peer := range pushTo {
		if peer != migrations.Migration017PushToBefore[i] {
			t.Fatal("Unexpected peer", peer, "wanted:", migrations.Migration017PushToBefore[i])
		}
	}

	assertCorrectRepoVer(t, repoverPath, "17")
}
