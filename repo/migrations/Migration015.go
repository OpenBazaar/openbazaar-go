package migrations

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/OpenBazaar/openbazaar-go/schema"
)

type Migration015 struct{}

func (Migration015) Up(repoPath, dbPassword string, testnet bool) error {
	var (
		configMap        = make(map[string]interface{})
		configBytes, err = ioutil.ReadFile(path.Join(repoPath, "config"))
	)
	if err != nil {
		return fmt.Errorf("reading config: %s", err.Error())
	}

	if err = json.Unmarshal(configBytes, &configMap); err != nil {
		return fmt.Errorf("unmarshal config: %s", err.Error())
	}

	configMap["LegacyWallet"] = configMap["Wallet"]
	delete(configMap, "Wallet")
	configMap["Wallets"] = schema.DefaultWalletsConfig()

	newConfigBytes, err := json.MarshalIndent(configMap, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal migrated config: %s", err.Error())
	}

	if err := ioutil.WriteFile(path.Join(repoPath, "config"), newConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing migrated config: %s", err.Error())
	}

	if err := writeRepoVer(repoPath, 16); err != nil {
		return fmt.Errorf("bumping repover to 16: %s", err.Error())
	}
	return nil
}

func (Migration015) Down(repoPath, dbPassword string, testnet bool) error {
	var (
		configMap        = make(map[string]interface{})
		configBytes, err = ioutil.ReadFile(path.Join(repoPath, "config"))
	)
	if err != nil {
		return fmt.Errorf("reading config: %s", err.Error())
	}

	if err = json.Unmarshal(configBytes, &configMap); err != nil {
		return fmt.Errorf("unmarshal config: %s", err.Error())
	}

	configMap["Wallet"] = configMap["LegacyWallet"]
	delete(configMap, "Wallets")
	delete(configMap, "LegacyWallet")

	newConfigBytes, err := json.MarshalIndent(configMap, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal migrated config: %s", err.Error())
	}

	if err := ioutil.WriteFile(path.Join(repoPath, "config"), newConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing migrated config: %s", err.Error())
	}

	if err := writeRepoVer(repoPath, 15); err != nil {
		return fmt.Errorf("bumping repover to 16: %s", err.Error())
	}
	return nil
}
