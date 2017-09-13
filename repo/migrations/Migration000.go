package migrations

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
)

var Migration000 migration000

type migration000 struct{}

func (migration000) Up(repoPath string) error {
	configFile, err := ioutil.ReadFile(path.Join(repoPath, "config"))
	if err != nil {
		return err
	}
	var cfgIface interface{}
	json.Unmarshal(configFile, &cfgIface)
	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return errors.New("Invalid config file")
	}

	walletIface, ok := cfg["Wallet"]
	if !ok {
		return errors.New("Missing wallet config")
	}
	wallet, ok := walletIface.(map[string]interface{})
	if !ok {
		return errors.New("Error parsing wallet config")
	}
	feeAPI, ok := wallet["FeeAPI"]
	if !ok {
		return errors.New("Missing FeeAPI config")
	}
	feeAPIstr, ok := feeAPI.(string)
	if !ok {
		return errors.New("Error parsing FeeAPI")
	}
	if feeAPIstr == "https://bitcoinfees.21.co/api/v1/fees/recommended" {
		wallet["FeeAPI"] = "https://btc.fees.openbazaar.org"
		cfg["Wallet"] = wallet
		out, err := json.MarshalIndent(cfg, "", "   ")
		if err != nil {
			return err
		}
		f, err := os.Create(path.Join(repoPath, "config"))
		if err != nil {
			return err
		}
		_, err = f.Write(out)
		if err != nil {
			return err
		}
		f.Close()
	}
	f, err := os.Create(path.Join(repoPath, "repover"))
	if err != nil {
		return err
	}
	_, err = f.Write([]byte("1"))
	if err != nil {
		return err
	}
	f.Close()
	return nil
}

func (migration000) Down(repoPath string) error {
	configFile, err := ioutil.ReadFile(path.Join(repoPath, "config"))
	if err != nil {
		return err
	}
	var cfgIface interface{}
	json.Unmarshal(configFile, &cfgIface)
	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return errors.New("Invalid config file")
	}

	walletIface, ok := cfg["Wallet"]
	if !ok {
		return errors.New("Missing wallet config")
	}
	wallet, ok := walletIface.(map[string]interface{})
	if !ok {
		return errors.New("Error parsing wallet config")
	}
	feeAPI, ok := wallet["FeeAPI"]
	if !ok {
		return errors.New("Missing FeeAPI config")
	}
	feeAPIstr, ok := feeAPI.(string)
	if !ok {
		return errors.New("Error parsing FeeAPI")
	}
	if feeAPIstr == "https://btc.fees.openbazaar.org" {
		wallet["FeeAPI"] = "https://bitcoinfees.21.co/api/v1/fees/recommended"
		cfg["Wallet"] = wallet
		out, _ := json.MarshalIndent(cfg, "", "   ")
		f, err := os.Create(path.Join(repoPath, "config"))
		if err != nil {
			return err
		}
		_, err = f.Write(out)
		if err != nil {
			return err
		}
		f.Close()
	}
	f, err := os.Create(path.Join(repoPath, "repover"))
	if err != nil {
		return err
	}
	_, err = f.Write([]byte("0"))
	if err != nil {
		return err
	}
	f.Close()
	return nil
}
