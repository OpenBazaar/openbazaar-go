package migrations

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
)

type Migration005 struct{}

func (Migration005) Up(repoPath string, dbPassword string, testnet bool) error {
	configFile, err := ioutil.ReadFile(path.Join(repoPath, "config"))
	if err != nil {
		return err
	}
	var cfgIface interface{}
	err = json.Unmarshal(configFile, &cfgIface)
	if err != nil {
		return err
	}
	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return errors.New("invalid config file")
	}

	ipns, ok := cfg["Ipns"]
	if !ok {
		return errors.New("ipns config not found")
	}
	ipnsCfg, ok := ipns.(map[string]interface{})
	if !ok {
		return errors.New("ipns config not found")
	}
	ipnsCfg["QuerySize"] = 1
	ipnsCfg["BackUpAPI"] = "https://gateway.ob1.io/ob/ipns/"

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

	f1, err := os.Create(path.Join(repoPath, "repover"))
	if err != nil {
		return err
	}
	_, err = f1.Write([]byte("6"))
	if err != nil {
		return err
	}
	f1.Close()
	return nil
}

func (Migration005) Down(repoPath string, dbPassword string, testnet bool) error {
	configFile, err := ioutil.ReadFile(path.Join(repoPath, "config"))
	if err != nil {
		return err
	}
	var cfgIface interface{}
	err = json.Unmarshal(configFile, &cfgIface)
	if err != nil {
		return err
	}
	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return errors.New("invalid config file")
	}

	ipns, ok := cfg["Ipns"]
	if !ok {
		return errors.New("ipns config not found")
	}
	ipnsCfg, ok := ipns.(map[string]interface{})
	if !ok {
		return errors.New("ipns config not found")
	}
	ipnsCfg["QuerySize"] = 5
	ipnsCfg["BackUpAPI"] = ""

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

	f1, err := os.Create(path.Join(repoPath, "repover"))
	if err != nil {
		return err
	}
	_, err = f1.Write([]byte("5"))
	if err != nil {
		return err
	}
	f1.Close()
	return nil
}
