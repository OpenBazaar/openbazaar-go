package migrations

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
)

type Migration002 struct{}

func (Migration002) Up(repoPath string, dbPassword string, testnet bool) error {
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

	pushNodes := []string{
		"QmY8puEnVx66uEet64gAf4VZRo7oUyMCwG6KdB9KM92EGQ",
		"QmPPg2qeF3n2KvTRXRZLaTwHCw8JxzF4uZK93RfMoDvf2o",
		"QmPPegaeM4rXfQDF3uu784d93pLEzV8A4zXU7akEgYnTFd",
	}
	cfg["DataSharing"] = map[string]interface{}{
		"AcceptStoreRequests": false,
		"PushTo":              pushNodes,
	}

	delete(cfg, "Crosspost-gateways")

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

func (Migration002) Down(repoPath string, dbPassword string, testnet bool) error {
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

	cfg["Crosspost-gateways"] = []string{"https://gateway.ob1.io/", "https://gateway.duosear.ch/"}

	delete(cfg, "DataSharing")

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
	_, err = f1.Write([]byte("2"))
	if err != nil {
		return err
	}
	f1.Close()
	return nil
}
