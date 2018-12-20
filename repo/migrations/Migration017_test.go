package migrations_test

import (
	"io/ioutil"
	"os"
	"regexp"
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
			"QmbwN82MVyBukT7WTdaQDppaACo62oUfma8dUa5R9nBFHm",
			"QmY8puEnVx66uEet64gAf4VZRo7oUyMCwG6KdB9KM92EGQ",
			"QmPPg2qeF3n2KvTRXRZLaTwHCw8JxzF4uZK93RfMoDvf2o"
		]
	},
	"OtherConfigProperty1": [1, 2, 3],
	"OtherConfigProperty2": "abc123"
}`

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

	var re = regexp.MustCompile(`\s`)
	if re.ReplaceAllString(string(configBytes), "") != re.ReplaceAllString(string(postMigration017Config), "") {
		t.Logf("actual: %s", re.ReplaceAllString(string(configBytes), ""))
		t.Fatal("incorrect post-migration config")
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

	if re.ReplaceAllString(string(configBytes), "") != re.ReplaceAllString(string(preMigration017Config), "") {
		t.Logf("actual: %s", re.ReplaceAllString(string(configBytes), ""))
		t.Fatal("incorrect post-migration config")
	}

	assertCorrectRepoVer(t, repoverPath, "17")
}
