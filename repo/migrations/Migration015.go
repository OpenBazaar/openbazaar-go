package migrations

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

const (
	migration015EthereumRegistryAddressMainnet = "0x403d907982474cdd51687b09a8968346159378f3"
	migration015EthereumRegistryAddressRinkeby = "0x403d907982474cdd51687b09a8968346159378f3"
	migration015EthereumRegistryAddressRopsten = "0x403d907982474cdd51687b09a8968346159378f3"
)

type migration015WalletsConfig struct {
	BTC *migration015CoinConfig `json:"BTC"`
	BCH *migration015CoinConfig `json:"BCH"`
	LTC *migration015CoinConfig `json:"LTC"`
	ZEC *migration015CoinConfig `json:"ZEC"`
	ETH *migration015CoinConfig `json:"ETH"`
}

type migration015CoinConfig struct {
	Type             string                 `json:"Type"`
	APIPool          []string               `json:"API"`
	APITestnetPool   []string               `json:"APITestnet"`
	MaxFee           uint64                 `json:"MaxFee"`
	FeeAPI           string                 `json:"FeeAPI"`
	HighFeeDefault   uint64                 `json:"HighFeeDefault"`
	MediumFeeDefault uint64                 `json:"MediumFeeDefault"`
	LowFeeDefault    uint64                 `json:"LowFeeDefault"`
	TrustedPeer      string                 `json:"TrustedPeer"`
	WalletOptions    map[string]interface{} `json:"WalletOptions"`
}

func migration015DefaultWalletConfig() *migration015WalletsConfig {
	var feeAPI = "https://btc.fees.openbazaar.org"
	return &migration015WalletsConfig{
		BTC: &migration015CoinConfig{
			Type:             "API",
			APIPool:          []string{"https://btc.api.openbazaar.org/api"},
			APITestnetPool:   []string{"https://tbtc.api.openbazaar.org/api"},
			FeeAPI:           feeAPI,
			LowFeeDefault:    1,
			MediumFeeDefault: 10,
			HighFeeDefault:   50,
			MaxFee:           200,
			WalletOptions:    nil,
		},
		BCH: &migration015CoinConfig{
			Type:             "API",
			APIPool:          []string{"https://bch.api.openbazaar.org/api"},
			APITestnetPool:   []string{"https://tbch.api.openbazaar.org/api"},
			FeeAPI:           "", // intentionally blank
			LowFeeDefault:    1,
			MediumFeeDefault: 5,
			HighFeeDefault:   10,
			MaxFee:           200,
			WalletOptions:    nil,
		},
		LTC: &migration015CoinConfig{
			Type:             "API",
			APIPool:          []string{"https://ltc.api.openbazaar.org/api"},
			APITestnetPool:   []string{"https://tltc.api.openbazaar.org/api"},
			FeeAPI:           "", // intentionally blank
			LowFeeDefault:    5,
			MediumFeeDefault: 10,
			HighFeeDefault:   20,
			MaxFee:           200,
			WalletOptions:    nil,
		},
		ZEC: &migration015CoinConfig{
			Type:             "API",
			APIPool:          []string{"https://zec.api.openbazaar.org/api"},
			APITestnetPool:   []string{"https://tzec.api.openbazaar.org/api"},
			FeeAPI:           "", // intentionally blank
			LowFeeDefault:    5,
			MediumFeeDefault: 10,
			HighFeeDefault:   20,
			MaxFee:           200,
			WalletOptions:    nil,
		},
		ETH: &migration015CoinConfig{
			Type:             "API",
			APIPool:          []string{"https://rinkeby.infura.io"},
			APITestnetPool:   []string{"https://rinkeby.infura.io"},
			FeeAPI:           "", // intentionally blank
			LowFeeDefault:    7,
			MediumFeeDefault: 15,
			HighFeeDefault:   30,
			MaxFee:           200,
			WalletOptions: map[string]interface{}{
				"RegistryAddress":        migration015EthereumRegistryAddressMainnet,
				"RinkebyRegistryAddress": migration015EthereumRegistryAddressRinkeby,
				"RopstenRegistryAddress": migration015EthereumRegistryAddressRopsten,
			},
		},
	}
}

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
	configMap["Wallets"] = migration015DefaultWalletConfig()

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
