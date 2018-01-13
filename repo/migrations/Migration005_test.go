package migrations

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

var testConfig5 string = `{
    "Ipns": {
	    "QuerySize": 5,
	    "RecordLifetime": "7d",
	    "RepublishPeriod": "24h",
	    "ResolveCacheSize": 128,
	    "UsePersistentCache": true
    }
}`

func TestMigration005(t *testing.T) {
	f, err := os.Create("./config")
	if err != nil {
		t.Error(err)
	}
	f.Write([]byte(testConfig5))
	f.Close()
	var m migration005

	// Up
	err = m.Up("./", "", false)
	if err != nil {
		t.Error(err)
	}
	newConfig, err := ioutil.ReadFile("./config")
	if err != nil {
		t.Error(err)
	}
	if !strings.Contains(string(newConfig), `"QuerySize": 1`) {
		t.Error("Failed to write new QuerySize")
	}
	if !strings.Contains(string(newConfig), `"BackUpAPI": "https://gateway.ob1.io/ob/ipns/"`) {
		t.Error("Failed to write new BackUpAPI")
	}
	repoVer, err := ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "5" {
		t.Error("Failed to write new repo version")
	}

	// Down
	err = m.Down("./", "", false)
	if err != nil {
		t.Error(err)
	}
	newConfig, err = ioutil.ReadFile("./config")
	if err != nil {
		t.Error(err)
	}
	if !strings.Contains(string(newConfig), `"QuerySize": 5`) {
		t.Error("Failed to write new QuerySize")
	}
	if !strings.Contains(string(newConfig), `"BackUpAPI": ""`) {
		t.Error("Failed to write new BackUpAPI")
	}
	repoVer, err = ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "4" {
		t.Error("Failed to write new repo version")
	}

	os.Remove("./config")
	os.Remove("./repover")
}
