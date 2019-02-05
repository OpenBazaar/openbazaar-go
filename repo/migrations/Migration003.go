package migrations

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
)

type Migration003 struct{}

func (Migration003) Up(repoPath string, dbPassword string, testnet bool) error {
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

	cfg["RepublishInterval"] = "24h"

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
	_, err = f1.Write([]byte("4"))
	if err != nil {
		return err
	}
	f1.Close()
	return nil
}

func (Migration003) Down(repoPath string, dbPassword string, testnet bool) error {
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

	delete(cfg, "RepublishInterval")

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
	_, err = f1.Write([]byte("3"))
	if err != nil {
		return err
	}
	f1.Close()
	return nil
}
