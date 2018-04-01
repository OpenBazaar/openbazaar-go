package migrations

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

var testConfig string = `{
    "Wallet": {
	    "Binary": "",
	    "FeeAPI": "https://bitcoinfees.21.co/api/v1/fees/recommended",
	    "HighFeeDefault": 160,
	    "LowFeeDefault": 120,
	    "MaxFee": 2000,
	    "MediumFeeDefault": 140,
	    "RPCPassword": "",
	    "RPCUser": "",
	    "TrustedPeer": "",
	    "Type": "spvwallet"
  }
}`

func TestMigration000(t *testing.T) {
	f, err := os.Create("./config")
	if err != nil {
		t.Error(err)
	}
	f.Write([]byte(testConfig))
	f.Close()
	var m Migration000

	// Up
	err = m.Up("./", "", false)
	if err != nil {
		t.Error(err)
	}
	newConfig, err := ioutil.ReadFile("./config")
	if err != nil {
		t.Error(err)
	}
	if !strings.Contains(string(newConfig), `"FeeAPI": "https://btc.fees.openbazaar.org"`) {
		t.Error("Failed to write new feeAPI")
	}
	repoVer, err := ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "1" {
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
	if !strings.Contains(string(newConfig), `"FeeAPI": "https://bitcoinfees.21.co/api/v1/fees/recommended"`) {
		t.Error("Failed to write new feeAPI")
	}
	repoVer, err = ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "0" {
		t.Error("Failed to write new repo version")
	}

	os.Remove("./config")
	os.Remove("./repover")
}
