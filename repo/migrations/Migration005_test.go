package migrations

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

var testConfig5 string = `{
    "Tor-config": {
    	"Password": "letmein",
    	"TorControl": "127.0.0.1:9151"
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
	if !strings.Contains(string(newConfig), `"Socks5": ""`) || !strings.Contains(string(newConfig), `"AutoHiddenService": true`) {
		t.Error("Failed to write new Tor-config object")
	}
	repoVer, err := ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "6" {
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
	if strings.Contains(string(newConfig), `Socks5`) || strings.Contains(string(newConfig), `AutoHiddenService`) {
		t.Error("Failed to write new Tor-config object")
	}
	repoVer, err = ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "5" {
		t.Error("Failed to write new repo version")
	}

	os.Remove("./config")
	os.Remove("./repover")
}
