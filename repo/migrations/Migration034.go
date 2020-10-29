package migrations

import (
	"encoding/json"
	"fmt"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"io/ioutil"
	"os"
	"path"
)

var filecoinConfig = &schema.CoinConfig{
	Type:             schema.WalletTypeAPI,
	APIPool:          schema.CoinPoolFIL,
	APITestnetPool:   schema.CoinPoolTFIL,
	FeeAPI:           "", // intentionally blank
	LowFeeDefault:    7,
	MediumFeeDefault: 15,
	HighFeeDefault:   30,
	MaxFee:           200,
	WalletOptions:    nil,
}

type Migration034 struct{}

func (Migration034) Up(repoPath, dbPassword string, testnet bool) error {
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

	walletCfgIface, ok := configMap["Wallets"]
	if !ok {
		return fmt.Errorf("unmarshal config: %s", err.Error())
	}
	walletCfg, ok := walletCfgIface.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unmarshal config: %s", err.Error())
	}
	walletCfg["FIL"] = filecoinConfig
	configMap["Wallets"] = walletCfg

	newConfigBytes, err := json.MarshalIndent(configMap, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal migrated config: %s", err.Error())
	}

	if err := ioutil.WriteFile(path.Join(repoPath, "config"), newConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing migrated config: %s", err.Error())
	}

	if err := writeRepoVer(repoPath, 35); err != nil {
		return fmt.Errorf("bumping repover to 16: %s", err.Error())
	}
	return nil
}

func (Migration034) Down(repoPath, dbPassword string, testnet bool) error {
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

	walletCfgIface, ok := configMap["Wallets"]
	if !ok {
		return fmt.Errorf("unmarshal config: %s", err.Error())
	}
	walletCfg, ok := walletCfgIface.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unmarshal config: %s", err.Error())
	}

	delete(walletCfg, "FIL")
	configMap["Wallets"] = walletCfg

	newConfigBytes, err := json.MarshalIndent(configMap, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal migrated config: %s", err.Error())
	}

	if err := ioutil.WriteFile(path.Join(repoPath, "config"), newConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing migrated config: %s", err.Error())
	}

	if err := writeRepoVer(repoPath, 34); err != nil {
		return fmt.Errorf("reducing repover to 15: %s", err.Error())
	}
	return nil
}
