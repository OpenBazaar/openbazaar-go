package migrations_test

import (
	"io/ioutil"
	"os"
	"regexp"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

const preMigration018Config = `{
 	"Addresses": {
		"API": ""
	}, 
        "Experimental": {
        	"FilestoreEnabled": false,
        	"Libp2pStreamMounting": false,
        	"ShardingEnabled": false
    	},
	"Gateway":{},
	"Ipns": {
		"BackUpAPI": "https://gateway.ob1.io",
        	"QuerySize": 1,
        	"RecordLifetime": "7d",
        	"RepublishPeriod": "24h",
        	"ResolveCacheSize": 128,
        	"UsePersistentCache": true
	},
	"OtherConfigProperty1": [1, 2, 3],
	"OtherConfigProperty2": "abc123",
        "Resolvers": {
		".eth": "",
        	".id": "https://resolver.onename.com/"
	}
}`

const postMigration018Config = `{
	"Addresses": {
		"API": null
	},
	"Experimental": {
        	"FilestoreEnabled": false,
        	"Libp2pStreamMounting": false,
        	"P2pHttpProxy": false,
        	"QUIC": false,
        	"ShardingEnabled": false,
        	"UrlstoreEnabled": false
    	},
	"Gateway": {
		"APICommands": null
	},
	"Ipns": {
        	"RecordLifetime": "168h",
        	"RepublishPeriod": "24h",
        	"ResolveCacheSize": 128
	},
	"IpnsExtra": {
        	"DHTQuorumSize": 1,
        	"FallbackAPI": "https://gateway.ob1.io"
	},
	"OtherConfigProperty1": [1, 2, 3],
	"OtherConfigProperty2": "abc123",
        "Pubsub": {
        	"DisableSigning": false,
        	"Router": "",
        	"StrictSignatureVerification": false
    	}
}`

func TestMigration018(t *testing.T) {
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
		ipfsverPath = testRepo.DataPathJoin("version")
	)
	if err = ioutil.WriteFile(configPath, []byte(preMigration018Config), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	if err = ioutil.WriteFile(repoverPath, []byte("17"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	var m migrations.Migration018
	err = m.Up(testRepo.DataPath(), "", true)
	if err != nil {
		t.Fatal(err)
	}

	configBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	var re = regexp.MustCompile(`\s`)
	if re.ReplaceAllString(string(configBytes), "") != re.ReplaceAllString(string(postMigration018Config), "") {
		t.Logf("actual: %s, expected %s", re.ReplaceAllString(string(configBytes), ""), re.ReplaceAllString(string(postMigration018Config), ""))
		t.Fatal("incorrect post-migration config")
	}

	assertCorrectRepoVer(t, repoverPath, "19")
	assertCorrectRepoVer(t, ipfsverPath, "7")

	err = m.Down(testRepo.DataPath(), "", true)
	if err != nil {
		t.Fatal(err)
	}

	configBytes, err = ioutil.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if re.ReplaceAllString(string(configBytes), "") != re.ReplaceAllString(string(preMigration018Config), "") {
		t.Logf("actual: %s, expected %s", re.ReplaceAllString(string(configBytes), ""), re.ReplaceAllString(string(preMigration018Config), ""))
		t.Fatal("incorrect post-migration config")
	}

	assertCorrectRepoVer(t, repoverPath, "18")
	assertCorrectRepoVer(t, ipfsverPath, "6")
}
