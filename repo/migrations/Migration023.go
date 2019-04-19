package migrations

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

const (
	migration023EthereumRegistryAddressMainnet = "0x5c69ccf91eab4ef80d9929b3c1b4d5bc03eb0981"
	migration023EthereumRegistryAddressRinkeby = "0x5cEF053c7b383f430FC4F4e1ea2F7D31d8e2D16C"
	migration023EthereumRegistryAddressRopsten = "0x403d907982474cdd51687b09a8968346159378f3"
)

// Migration023WalletsConfig - used to hold the coin cfg
type Migration023WalletsConfig struct {
	BTC *migration023CoinConfig `json:"BTC"`
	BCH *migration023CoinConfig `json:"BCH"`
	LTC *migration023CoinConfig `json:"LTC"`
	ZEC *migration023CoinConfig `json:"ZEC"`
	ETH *migration023CoinConfig `json:"ETH"`
}

type migration023CoinConfig struct {
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

// Migration023 - required migration struct
type Migration023 struct{}

// Up - upgrade the state
func (Migration023) Up(repoPath, dbPassword string, testnet bool) error {
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

	c, ok := configMap["Wallets"]
	if !ok {
		return errors.New("invalid config: missing key Wallets")
	}

	walletCfg, ok := c.(map[string]interface{})
	if !ok {
		return errors.New("invalid config: invalid key Wallets")
	}

	btc, ok := walletCfg["BTC"]
	if !ok {
		return errors.New("invalid config: missing BTC Wallet")
	}

	btcWalletCfg, ok := btc.(map[string]interface{})
	if !ok {
		return errors.New("invalid config: invalid BTC Wallet")
	}

	btcWalletCfg["APIPool"] = []string{"https://btc.api.openbazaar.org/api"}
	btcWalletCfg["APITestnetPool"] = []string{"https://tbtc.api.openbazaar.org/api"}

	bch, ok := walletCfg["BCH"]
	if !ok {
		return errors.New("invalid config: missing BCH Wallet")
	}

	bchWalletCfg, ok := bch.(map[string]interface{})
	if !ok {
		return errors.New("invalid config: invalid BCH Wallet")
	}

	bchWalletCfg["APIPool"] = []string{"https://bch.api.openbazaar.org/api"}
	bchWalletCfg["APITestnetPool"] = []string{"https://tbch.api.openbazaar.org/api"}

	ltc, ok := walletCfg["LTC"]
	if !ok {
		return errors.New("invalid config: missing LTC Wallet")
	}

	ltcWalletCfg, ok := ltc.(map[string]interface{})
	if !ok {
		return errors.New("invalid config: invalid LTC Wallet")
	}

	ltcWalletCfg["APIPool"] = []string{"https://ltc.api.openbazaar.org/api"}
	ltcWalletCfg["APITestnetPool"] = []string{"https://tltc.api.openbazaar.org/api"}

	zec, ok := walletCfg["ZEC"]
	if !ok {
		return errors.New("invalid config: missing ZEC Wallet")
	}

	zecWalletCfg, ok := zec.(map[string]interface{})
	if !ok {
		return errors.New("invalid config: invalid ZEC Wallet")
	}

	zecWalletCfg["APIPool"] = []string{"https://zec.api.openbazaar.org/api"}
	zecWalletCfg["APITestnetPool"] = []string{"https://tzec.api.openbazaar.org/api"}

	eth, ok := walletCfg["ETH"]
	if !ok {
		return errors.New("invalid config: missing ETH Wallet")
	}

	ethWalletCfg, ok := eth.(map[string]interface{})
	if !ok {
		return errors.New("invalid config: invalid ETH Wallet")
	}

	ethWalletCfg["APIPool"] = []string{"https://mainnet.infura.io"}
	ethWalletCfg["APITestnetPool"] = []string{"https://rinkeby.infura.io"}
	ethWalletCfg["WalletOptions"] = map[string]interface{}{
		"RegistryAddress":        migration023EthereumRegistryAddressMainnet,
		"RinkebyRegistryAddress": migration023EthereumRegistryAddressRinkeby,
		"RopstenRegistryAddress": migration023EthereumRegistryAddressRopsten,
	}

	newConfigBytes, err := json.MarshalIndent(configMap, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal migrated config: %s", err.Error())
	}

	if err := ioutil.WriteFile(path.Join(repoPath, "config"), newConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing migrated config: %s", err.Error())
	}

	if err := writeRepoVer(repoPath, 24); err != nil {
		return fmt.Errorf("bumping repover to 24: %s", err.Error())
	}
	return nil
}

// Down - downgrade/restore the state
func (Migration023) Down(repoPath, dbPassword string, testnet bool) error {
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

	c, ok := configMap["Wallets"]
	if !ok {
		return errors.New("invalid config: missing key Wallets")
	}

	walletCfg, ok := c.(map[string]interface{})
	if !ok {
		return errors.New("invalid config: invalid key Wallets")
	}

	btc, ok := walletCfg["BTC"]
	if !ok {
		return errors.New("invalid config: missing BTC Wallet")
	}

	btcWalletCfg, ok := btc.(map[string]interface{})
	if !ok {
		return errors.New("invalid config: invalid BTC Wallet")
	}

	btcWalletCfg["APIPool"] = []string{"https://btc.blockbook.api.openbazaar.org/api"}
	btcWalletCfg["APITestnetPool"] = []string{"https://tbtc.blockbook.api.openbazaar.org/api"}

	bch, ok := walletCfg["BCH"]
	if !ok {
		return errors.New("invalid config: missing BCH Wallet")
	}

	bchWalletCfg, ok := bch.(map[string]interface{})
	if !ok {
		return errors.New("invalid config: invalid BCH Wallet")
	}

	bchWalletCfg["APIPool"] = []string{"https://bch.blockbook.api.openbazaar.org/api"}
	bchWalletCfg["APITestnetPool"] = []string{"https://tbch.blockbook.api.openbazaar.org/api"}

	ltc, ok := walletCfg["LTC"]
	if !ok {
		return errors.New("invalid config: missing LTC Wallet")
	}

	ltcWalletCfg, ok := ltc.(map[string]interface{})
	if !ok {
		return errors.New("invalid config: invalid LTC Wallet")
	}

	ltcWalletCfg["APIPool"] = []string{"https://ltc.blockbook.api.openbazaar.org/api"}
	ltcWalletCfg["APITestnetPool"] = []string{"https://tltc.blockbook.api.openbazaar.org/api"}

	zec, ok := walletCfg["ZEC"]
	if !ok {
		return errors.New("invalid config: missing ZEC Wallet")
	}

	zecWalletCfg, ok := zec.(map[string]interface{})
	if !ok {
		return errors.New("invalid config: invalid ZEC Wallet")
	}

	zecWalletCfg["APIPool"] = []string{"https://zec.blockbook.api.openbazaar.org/api"}
	zecWalletCfg["APITestnetPool"] = []string{"https://tzec.blockbook.api.openbazaar.org/api"}

	eth, ok := walletCfg["ETH"]
	if !ok {
		return errors.New("invalid config: missing ETH Wallet")
	}

	ethWalletCfg, ok := eth.(map[string]interface{})
	if !ok {
		return errors.New("invalid config: invalid ETH Wallet")
	}

	ethWalletCfg["APIPool"] = []string{"https://mainnet.infura.io"}
	ethWalletCfg["APITestnetPool"] = []string{"https://rinkeby.infura.io"}
	ethWalletCfg["WalletOptions"] = map[string]interface{}{
		"RegistryAddress":        migration023EthereumRegistryAddressMainnet,
		"RinkebyRegistryAddress": migration023EthereumRegistryAddressRinkeby,
		"RopstenRegistryAddress": migration023EthereumRegistryAddressRopsten,
	}

	newConfigBytes, err := json.MarshalIndent(configMap, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal migrated config: %s", err.Error())
	}

	if err := ioutil.WriteFile(path.Join(repoPath, "config"), newConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing migrated config: %s", err.Error())
	}

	if err := writeRepoVer(repoPath, 23); err != nil {
		return fmt.Errorf("dropping repover to 23: %s", err.Error())
	}
	return nil
}