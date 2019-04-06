package migrations

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

const (
	migration022EthereumRegistryAddressMainnet = "0x5c69ccf91eab4ef80d9929b3c1b4d5bc03eb0981"
	migration022EthereumRegistryAddressRinkeby = "0x403d907982474cdd51687b09a8968346159378f3"
	migration022EthereumRegistryAddressRopsten = "0x403d907982474cdd51687b09a8968346159378f3"
)

type Migration022WalletsConfig struct {
	BTC *migration022CoinConfig `json:"BTC"`
	BCH *migration022CoinConfig `json:"BCH"`
	LTC *migration022CoinConfig `json:"LTC"`
	ZEC *migration022CoinConfig `json:"ZEC"`
	ETH *migration022CoinConfig `json:"ETH"`
}

type migration022CoinConfig struct {
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

func migration022DefaultWalletConfig() *Migration022WalletsConfig {
	var feeAPI = "https://btc.fees.openbazaar.org"
	return &Migration022WalletsConfig{
		BTC: &migration022CoinConfig{
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
		BCH: &migration022CoinConfig{
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
		LTC: &migration022CoinConfig{
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
		ZEC: &migration022CoinConfig{
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
		ETH: &migration022CoinConfig{
			Type:             "API",
			APIPool:          []string{"https://mainnet.infura.io"},
			APITestnetPool:   []string{"https://rinkeby.infura.io"},
			FeeAPI:           "", // intentionally blank
			LowFeeDefault:    7,
			MediumFeeDefault: 15,
			HighFeeDefault:   30,
			MaxFee:           200,
			WalletOptions: map[string]interface{}{
				"RegistryAddress":        migration022EthereumRegistryAddressMainnet,
				"RinkebyRegistryAddress": migration022EthereumRegistryAddressRinkeby,
				"RopstenRegistryAddress": migration022EthereumRegistryAddressRopsten,
			},
		},
	}
}

func migration022PreviousWalletConfig() *Migration022WalletsConfig {
	c := migration022DefaultWalletConfig()

	c.BTC.APIPool = []string{"https://btc.api.openbazaar.org/api"}
	c.BTC.APITestnetPool = []string{"https://tbtc.api.openbazaar.org/api"}
	c.BCH.APIPool = []string{"https://bch.api.openbazaar.org/api"}
	c.BCH.APITestnetPool = []string{"https://tbch.api.openbazaar.org/api"}
	c.LTC.APIPool = []string{"https://ltc.api.openbazaar.org/api"}
	c.LTC.APITestnetPool = []string{"https://tltc.api.openbazaar.org/api"}
	c.ZEC.APIPool = []string{"https://zec.api.openbazaar.org/api"}
	c.ZEC.APITestnetPool = []string{"https://tzec.api.openbazaar.org/api"}
	c.ETH.APIPool = []string{"https://mainnet.infura.io"}
	c.ETH.APITestnetPool = []string{"https://rinkeby.infura.io"}

	return c
}

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

	configMap["Wallets"] = migration022DefaultWalletConfig()

	newConfigBytes, err := json.MarshalIndent(configMap, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal migrated config: %s", err.Error())
	}

	if err := ioutil.WriteFile(path.Join(repoPath, "config"), newConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing migrated config: %s", err.Error())
	}

	if err := writeRepoVer(repoPath, 23); err != nil {
		return fmt.Errorf("bumping repover to 23: %s", err.Error())
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

	configMap["Wallets"] = migration022PreviousWalletConfig()

	newConfigBytes, err := json.MarshalIndent(configMap, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal migrated config: %s", err.Error())
	}

	if err := ioutil.WriteFile(path.Join(repoPath, "config"), newConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing migrated config: %s", err.Error())
	}

	if err := writeRepoVer(repoPath, 22); err != nil {
		return fmt.Errorf("dropping repover to 22: %s", err.Error())
	}
	return nil
}
