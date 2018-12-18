package migrations

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

const (
	migration016EthereumRegistryAddressMainnet = "0x403d907982474cdd51687b09a8968346159378f3"
	migration016EthereumRegistryAddressRinkeby = "0x403d907982474cdd51687b09a8968346159378f3"
	migration016EthereumRegistryAddressRopsten = "0x403d907982474cdd51687b09a8968346159378f3"
)

type Migration016WalletsConfig struct {
	BTC *migration016CoinConfig `json:"BTC"`
	BCH *migration016CoinConfig `json:"BCH"`
	LTC *migration016CoinConfig `json:"LTC"`
	ZEC *migration016CoinConfig `json:"ZEC"`
	ETH *migration016CoinConfig `json:"ETH"`
}

type migration016CoinConfig struct {
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

func migration016DefaultWalletConfig() *Migration016WalletsConfig {
	var feeAPI = "https://btc.fees.openbazaar.org"
	return &Migration016WalletsConfig{
		BTC: &migration016CoinConfig{
			Type:             "API",
			APIPool:          []string{"https://btc.blockbook.api.openbazaar.org/api"},
			APITestnetPool:   []string{"https://tbtc.blockbook.api.openbazaar.org/api"},
			FeeAPI:           feeAPI,
			LowFeeDefault:    1,
			MediumFeeDefault: 10,
			HighFeeDefault:   50,
			MaxFee:           200,
			WalletOptions:    nil,
		},
		BCH: &migration016CoinConfig{
			Type:             "API",
			APIPool:          []string{"https://bch.blockbook.api.openbazaar.org/api"},
			APITestnetPool:   []string{"https://tbch.blockbook.api.openbazaar.org/api"},
			FeeAPI:           "", // intentionally blank
			LowFeeDefault:    1,
			MediumFeeDefault: 5,
			HighFeeDefault:   10,
			MaxFee:           200,
			WalletOptions:    nil,
		},
		LTC: &migration016CoinConfig{
			Type:             "API",
			APIPool:          []string{"https://ltc.blockbook.api.openbazaar.org/api"},
			APITestnetPool:   []string{"https://tltc.blockbook.api.openbazaar.org/api"},
			FeeAPI:           "", // intentionally blank
			LowFeeDefault:    5,
			MediumFeeDefault: 10,
			HighFeeDefault:   20,
			MaxFee:           200,
			WalletOptions:    nil,
		},
		ZEC: &migration016CoinConfig{
			Type:             "API",
			APIPool:          []string{"https://zec.blockbook.api.openbazaar.org/api"},
			APITestnetPool:   []string{"https://tzec.blockbook.api.openbazaar.org/api"},
			FeeAPI:           "", // intentionally blank
			LowFeeDefault:    5,
			MediumFeeDefault: 10,
			HighFeeDefault:   20,
			MaxFee:           200,
			WalletOptions:    nil,
		},
		ETH: &migration016CoinConfig{
			Type:             "API",
			APIPool:          []string{"https://rinkeby.infura.io"},
			APITestnetPool:   []string{"https://rinkeby.infura.io"},
			FeeAPI:           "", // intentionally blank
			LowFeeDefault:    7,
			MediumFeeDefault: 15,
			HighFeeDefault:   30,
			MaxFee:           200,
			WalletOptions: map[string]interface{}{
				"RegistryAddress":        migration016EthereumRegistryAddressMainnet,
				"RinkebyRegistryAddress": migration016EthereumRegistryAddressRinkeby,
				"RopstenRegistryAddress": migration016EthereumRegistryAddressRopsten,
			},
		},
	}
}

func migration016PreviousWalletConfig() *Migration016WalletsConfig {
	c := migration016DefaultWalletConfig()

	c.BTC.APIPool = []string{"https://btc.api.openbazaar.org/api"}
	c.BTC.APITestnetPool = []string{"https://tbtc.api.openbazaar.org/api"}
	c.BCH.APIPool = []string{"https://bch.api.openbazaar.org/api"}
	c.BCH.APITestnetPool = []string{"https://tbch.api.openbazaar.org/api"}
	c.LTC.APIPool = []string{"https://ltc.api.openbazaar.org/api"}
	c.LTC.APITestnetPool = []string{"https://tltc.api.openbazaar.org/api"}
	c.ZEC.APIPool = []string{"https://zec.api.openbazaar.org/api"}
	c.ZEC.APITestnetPool = []string{"https://tzec.api.openbazaar.org/api"}

	return c
}

type Migration016 struct{}

func (Migration016) Up(repoPath, dbPassword string, testnet bool) error {
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

	configMap["Wallets"] = migration016DefaultWalletConfig()

	newConfigBytes, err := json.MarshalIndent(configMap, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal migrated config: %s", err.Error())
	}

	if err := ioutil.WriteFile(path.Join(repoPath, "config"), newConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing migrated config: %s", err.Error())
	}

	if err := writeRepoVer(repoPath, 17); err != nil {
		return fmt.Errorf("bumping repover to 17: %s", err.Error())
	}
	return nil
}

func (Migration016) Down(repoPath, dbPassword string, testnet bool) error {
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

	configMap["Wallets"] = migration016PreviousWalletConfig()

	newConfigBytes, err := json.MarshalIndent(configMap, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal migrated config: %s", err.Error())
	}

	if err := ioutil.WriteFile(path.Join(repoPath, "config"), newConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing migrated config: %s", err.Error())
	}

	if err := writeRepoVer(repoPath, 16); err != nil {
		return fmt.Errorf("dropping repover to 16: %s", err.Error())
	}
	return nil
}
