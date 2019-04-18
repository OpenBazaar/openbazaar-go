package migrations

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

// Migration022 migrates the config file to set the IPNSExtra: APIRouter option
// in the config file. Also deletes IPNSExtra: FallbackAPI if it exists.
type Migration022 struct{}

func (Migration022) Up(repoPath, dbPassword string, testnet bool) error {
	var (
		configMap        = map[string]interface{}{}
		configBytes, err = ioutil.ReadFile(path.Join(repoPath, "config"))
	)
	if err != nil {
		return fmt.Errorf("reading config: %s", err.Error())
	}

	if err = json.Unmarshal(configBytes, &configMap); err != nil {
		return fmt.Errorf("unmarshal config: %s", err.Error())
	}

	iIPNSExtra, ok := configMap["IpnsExtra"]
	if !ok {
		return fmt.Errorf("unmarshal config: missing IpnsExtra key")
	}

	ipnsExtra, ok := iIPNSExtra.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unmarshal config: error type asserting IpnsExtra")
	}

	ipnsExtra["APIRouter"] = "https://routing.api.openbazaar.org"
	delete(ipnsExtra, "FallbackAPI")
	configMap["IpnsExtra"] = ipnsExtra

	newConfigBytes, err := json.MarshalIndent(configMap, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal migrated config: %s", err.Error())
	}

	if err := ioutil.WriteFile(path.Join(repoPath, "config"), newConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing migrated config: %s", err.Error())
	}

	if err := writeRepoVer(repoPath, 23); err != nil {
		return fmt.Errorf("bumping repover to 18: %s", err.Error())
	}
	return nil
}

func (Migration022) Down(repoPath, dbPassword string, testnet bool) error {
	var (
		configMap        = map[string]interface{}{}
		configBytes, err = ioutil.ReadFile(path.Join(repoPath, "config"))
	)
	if err != nil {
		return fmt.Errorf("reading config: %s", err.Error())
	}

	if err = json.Unmarshal(configBytes, &configMap); err != nil {
		return fmt.Errorf("unmarshal config: %s", err.Error())
	}

	iIPNSExtra, ok := configMap["IpnsExtra"]
	if !ok {
		return fmt.Errorf("unmarshal config: missing IpnsExtra key")
	}

	ipnsExtra, ok := iIPNSExtra.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unmarshal config: error type asserting IpnsExtra")
	}

	delete(ipnsExtra, "APIRouter")
	ipnsExtra["FallbackAPI"] = "https://gateway.ob1.io"
	configMap["IpnsExtra"] = ipnsExtra

	newConfigBytes, err := json.MarshalIndent(configMap, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal migrated config: %s", err.Error())
	}

	if err := ioutil.WriteFile(path.Join(repoPath, "config"), newConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing migrated config: %s", err.Error())
	}

	if err := writeRepoVer(repoPath, 22); err != nil {
		return fmt.Errorf("dropping repover to 16: %s", err.Error())
	}
	return nil
}
