package migrations

import (
	"path"

	"encoding/json"
	"errors"
	_ "github.com/mutecomm/go-sqlcipher"
	"io/ioutil"
	"os"
)

var Migration005 migration005

type migration005 struct{}

func (migration005) Up(repoPath string, dbPassword string, testnet bool) error {
	type TorConfig struct {
		Password          string
		TorControl        string
		Socks5            string
		AutoHiddenService bool
	}
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

	tcIface, ok := cfg["Tor-config"]
	if !ok {
		return errors.New("Invalid config file")
	}
	tc, ok := tcIface.(map[string]interface{})

	pw, ok := tc["Password"]
	if !ok {
		return errors.New("Invalid config file")
	}
	pwStr, ok := pw.(string)
	if !ok {
		return errors.New("Invalid config file")
	}
	controlUrl, ok := tc["TorControl"]
	if !ok {
		return errors.New("Invalid config file")
	}
	controlUrlStr, ok := controlUrl.(string)
	if !ok {
		return errors.New("Invalid config file")
	}

	newConfig := TorConfig{
		Password:          pwStr,
		TorControl:        controlUrlStr,
		AutoHiddenService: true,
	}
	cfg["Tor-config"] = newConfig

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

func (migration005) Down(repoPath string, dbPassword string, testnet bool) error {
	type OldConfig struct {
		Password   string
		TorControl string
	}

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

	tcIface, ok := cfg["Tor-config"]
	if !ok {
		return errors.New("Invalid config file")
	}
	tc, ok := tcIface.(map[string]interface{})

	pw, ok := tc["Password"]
	if !ok {
		return errors.New("Invalid config file")
	}
	pwStr, ok := pw.(string)
	if !ok {
		return errors.New("Invalid config file")
	}
	controlUrl, ok := tc["TorControl"]
	if !ok {
		return errors.New("Invalid config file")
	}
	controlUrlStr, ok := controlUrl.(string)
	if !ok {
		return errors.New("Invalid config file")
	}

	newConfig := OldConfig{
		Password:   pwStr,
		TorControl: controlUrlStr,
	}
	cfg["Tor-config"] = newConfig

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
