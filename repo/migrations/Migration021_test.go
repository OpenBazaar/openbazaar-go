package migrations_test

import (
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"io/ioutil"
	"os"
	"regexp"
	"testing"
)

const preMigration021Config = `{
	"Swarm": {
		"AddrFilters": null,
     	 	"ConnMgr": {
         		"GracePeriod": "",
         		"HighWater": 0,
         		"LowWater": 0,
         		"Type": ""
      		},
		"DisableBandwidthMetrics": false,
      		"DisableNatPortMap": false,
      		"DisableRelay": false,
      		"EnableRelayHop": false
   	}
}`

const postMigration021Config = `{
	"Swarm": {
		"AddrFilters": null,
     	 	"ConnMgr": {
         		"GracePeriod": "",
         		"HighWater": 0,
         		"LowWater": 0,
         		"Type": ""
      		},
		"DisableBandwidthMetrics": false,
      		"DisableNatPortMap": false,
      		"DisableRelay": false,
		"EnableAutoRelay": true,
      		"EnableRelayHop": false
   	}
}`

func TestMigration021(t *testing.T) {
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
	if err = ioutil.WriteFile(configPath, []byte(preMigration021Config), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	if err = ioutil.WriteFile(repoverPath, []byte("15"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	var m migrations.Migration021
	err = m.Up(testRepo.DataPath(), "", true)
	if err != nil {
		t.Fatal(err)
	}

	configBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	var re = regexp.MustCompile(`\s`)
	if re.ReplaceAllString(string(configBytes), "") != re.ReplaceAllString(string(postMigration021Config), "") {
		t.Logf("actual: %s", re.ReplaceAllString(string(configBytes), ""))
		t.Fatal("incorrect post-migration config")
	}

	assertCorrectRepoVer(t, repoverPath, "22")

	err = m.Down(testRepo.DataPath(), "", true)
	if err != nil {
		t.Fatal(err)
	}

	configBytes, err = ioutil.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if re.ReplaceAllString(string(configBytes), "") != re.ReplaceAllString(string(preMigration021Config), "") {
		t.Logf("actual: %s", re.ReplaceAllString(string(configBytes), ""))
		t.Fatal("incorrect post-migration config")
	}

	assertCorrectRepoVer(t, repoverPath, "21")
}
