package migrations

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

var testConfig2 = `{
    "Crosspost-gateways": [
	    "https://gateway.ob1.io/",
	    "https://gateway.duosear.ch/"
    ]
}`

func TestMigration002(t *testing.T) {
	f, err := os.Create("./config")
	if err != nil {
		t.Error(err)
	}
	f.Write([]byte(testConfig2))
	f.Close()
	var m Migration002

	// Up
	err = m.Up("./", "", false)
	if err != nil {
		t.Error(err)
	}
	newConfig, err := ioutil.ReadFile("./config")
	if err != nil {
		t.Error(err)
	}
	if !strings.Contains(string(newConfig), `DataSharing`) {
		t.Error("Failed to write new DataSharing object")
	}
	if strings.Contains(string(newConfig), `Crosspost-gateways`) {
		t.Error("Failed to delete Crosspost-gateways")
	}
	repoVer, err := ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "3" {
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
	if !strings.Contains(string(newConfig), `Crosspost-gateways`) {
		t.Error("Failed to write new Crosspost-gateways")
	}
	if strings.Contains(string(newConfig), `DataSharing`) {
		t.Errorf("Failed to delete DataSharing")
	}
	repoVer, err = ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "2" {
		t.Error("Failed to write new repo version")
	}

	os.Remove("./config")
	os.Remove("./repover")
}
