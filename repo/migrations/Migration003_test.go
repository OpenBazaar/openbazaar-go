package migrations

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

var testConfig3 = `{
    "RepublishInterval": "24h"
}`

func TestMigration003(t *testing.T) {
	f, err := os.Create("./config")
	if err != nil {
		t.Error(err)
	}
	f.Write([]byte(testConfig3))
	f.Close()
	var m Migration003

	// Up
	err = m.Up("./", "", false)
	if err != nil {
		t.Error(err)
	}
	newConfig, err := ioutil.ReadFile("./config")
	if err != nil {
		t.Error(err)
	}
	if !strings.Contains(string(newConfig), `"RepublishInterval": "24h"`) {
		t.Error("Failed to write new RepublishInterval object")
	}
	repoVer, err := ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "4" {
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
	if strings.Contains(string(newConfig), `RepublishInterval`) {
		t.Error("Failed to delete RepublishInterval")
	}
	repoVer, err = ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "3" {
		t.Error("Failed to write new repo version")
	}

	os.Remove("./config")
	os.Remove("./repover")
}
