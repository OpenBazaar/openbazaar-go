package config

import (
	"os"
	"time"

	"github.com/OpenBazaar/multiwallet/cache"
	"github.com/OpenBazaar/multiwallet/datastore"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/op/go-logging"
	"golang.org/x/net/proxy"
)

const (
	EthereumRegistryAddressMainnet = "0x403d907982474cdd51687b09a8968346159378f3"
	EthereumRegistryAddressRinkeby = "0x403d907982474cdd51687b09a8968346159378f3"
	EthereumRegistryAddressRopsten = "0x403d907982474cdd51687b09a8968346159378f3"
)

type Config struct {
	// Network parameters. Set mainnet, testnet, or regtest using this.
	Params *chaincfg.Params

	// Bip39 mnemonic string. If empty a new mnemonic will be created.
	Mnemonic string

	// The date the wallet was created.
	// If before the earliest checkpoint the chain will be synced using the earliest checkpoint.
	CreationDate time.Time

	// A Tor proxy can be set here causing the wallet will use Tor
	Proxy proxy.Dialer

	// A logger. You can write the logs to file or stdout or however else you want.
	Logger logging.Backend

	// Cache is a persistable storage provided by the consumer where the wallet can
	// keep state between runtime executions
	Cache cache.Cacher

	// A list of coin configs. One config should be included for each coin to be used.
	Coins []CoinConfig

	// Disable the exchange rate functionality in each wallet
	DisableExchangeRates bool
}

type CoinConfig struct {
	// The type of coin to configure
	CoinType wallet.CoinType

	// The default fee-per-byte for each level
	LowFee    uint64
	MediumFee uint64
	HighFee   uint64

	// The highest allowable fee-per-byte
	MaxFee uint64

	// External API to query to look up fees. If this field is nil then the default fees will be used.
	// If the API is unreachable then the default fees will likewise be used. If the API returns a fee
	// greater than MaxFee then the MaxFee will be used in place. The API response must be formatted as
	// { "fastestFee": 40, "halfHourFee": 20, "hourFee": 10 }
	FeeAPI string

	// The trusted APIs to use for querying for balances and listening to blockchain events.
	ClientAPIs []string

	// An implementation of the Datastore interface for each desired coin
	DB wallet.Datastore

	// Custom options for wallet to use
	Options map[string]interface{}
}

func NewDefaultConfig(coinTypes map[wallet.CoinType]bool, params *chaincfg.Params) *Config {
	cfg := &Config{
		Cache:  cache.NewMockCacher(),
		Params: params,
		Logger: logging.NewLogBackend(os.Stdout, "", 0),
	}
	var testnet bool
	if params.Name == chaincfg.TestNet3Params.Name {
		testnet = true
	}
	mockDB := datastore.NewMockMultiwalletDatastore()
	if coinTypes[wallet.Bitcoin] {
		var apiEndpoints []string
		if !testnet {
			apiEndpoints = []string{
				"https://btc.blockbook.api.openbazaar.org/api",
				// temporarily deprecated Insight endpoints
				//"https://btc.bloqapi.net/insight-api",
				//"https://btc.insight.openbazaar.org/insight-api",
			}
		} else {
			apiEndpoints = []string{
				"https://tbtc.blockbook.api.openbazaar.org/api",
				// temporarily deprecated Insight endpoints
				//"https://test-insight.bitpay.com/api",
			}
		}
		feeApi := "https://btc.fees.openbazaar.org"
		db, _ := mockDB.GetDatastoreForWallet(wallet.Bitcoin)
		btcCfg := CoinConfig{
			CoinType:   wallet.Bitcoin,
			FeeAPI:     feeApi,
			LowFee:     140,
			MediumFee:  160,
			HighFee:    180,
			MaxFee:     2000,
			ClientAPIs: apiEndpoints,
			DB:         db,
		}
		cfg.Coins = append(cfg.Coins, btcCfg)
	}
	if coinTypes[wallet.BitcoinCash] {
		var apiEndpoints []string
		if !testnet {
			apiEndpoints = []string{
				"https://bch.blockbook.api.openbazaar.org/api",
				// temporarily deprecated Insight endpoints
				//"https://bitcoincash.blockexplorer.com/api",
			}
		} else {
			apiEndpoints = []string{
				"https://tbch.blockbook.api.openbazaar.org/api",
				// temporarily deprecated Insight endpoints
				//"https://test-bch-insight.bitpay.com/api",
			}
		}
		db, _ := mockDB.GetDatastoreForWallet(wallet.BitcoinCash)
		bchCfg := CoinConfig{
			CoinType:   wallet.BitcoinCash,
			FeeAPI:     "",
			LowFee:     140,
			MediumFee:  160,
			HighFee:    180,
			MaxFee:     2000,
			ClientAPIs: apiEndpoints,
			DB:         db,
		}
		cfg.Coins = append(cfg.Coins, bchCfg)
	}
	if coinTypes[wallet.Zcash] {
		var apiEndpoints []string
		if !testnet {
			apiEndpoints = []string{
				"https://zec.blockbook.api.openbazaar.org/api",
				// temporarily deprecated Insight endpoints
				//"https://zcashnetwork.info/api",
			}
		} else {
			apiEndpoints = []string{
				"https://tzec.blockbook.api.openbazaar.org/api",
				// temporarily deprecated Insight endpoints
				//"https://explorer.testnet.z.cash/api",
			}
		}
		db, _ := mockDB.GetDatastoreForWallet(wallet.Zcash)
		zecCfg := CoinConfig{
			CoinType:   wallet.Zcash,
			FeeAPI:     "",
			LowFee:     140,
			MediumFee:  160,
			HighFee:    180,
			MaxFee:     2000,
			ClientAPIs: apiEndpoints,
			DB:         db,
		}
		cfg.Coins = append(cfg.Coins, zecCfg)
	}
	if coinTypes[wallet.Litecoin] {
		var apiEndpoints []string
		if !testnet {
			apiEndpoints = []string{
				"https://ltc.blockbook.api.openbazaar.org/api",
				// temporarily deprecated Insight endpoints
				//"https://ltc.coin.space/api",
				//"https://ltc.insight.openbazaar.org/insight-lite-api",
			}
		} else {
			apiEndpoints = []string{
				"https://tltc.blockbook.api.openbazaar.org/api",
				// temporarily deprecated Insight endpoints
				//"https://testnet.litecore.io/api",
			}
		}
		db, _ := mockDB.GetDatastoreForWallet(wallet.Litecoin)
		ltcCfg := CoinConfig{
			CoinType:   wallet.Litecoin,
			FeeAPI:     "",
			LowFee:     140,
			MediumFee:  160,
			HighFee:    180,
			MaxFee:     2000,
			ClientAPIs: apiEndpoints,
			DB:         db,
		}
		cfg.Coins = append(cfg.Coins, ltcCfg)
	}
	if coinTypes[wallet.Ethereum] {
		var apiEndpoints []string
		if !testnet {
			apiEndpoints = []string{
				"https://rinkeby.infura.io",
			}
		} else {
			apiEndpoints = []string{
				"https://rinkeby.infura.io",
			}
		}
		db, _ := mockDB.GetDatastoreForWallet(wallet.Ethereum)
		ethCfg := CoinConfig{
			CoinType:   wallet.Ethereum,
			FeeAPI:     "",
			LowFee:     140,
			MediumFee:  160,
			HighFee:    180,
			MaxFee:     2000,
			ClientAPIs: apiEndpoints,
			DB:         db,
			Options: map[string]interface{}{
				"RegistryAddress":        EthereumRegistryAddressMainnet,
				"RinkebyRegistryAddress": EthereumRegistryAddressRinkeby,
				"RopstenRegistryAddress": EthereumRegistryAddressRopsten,
			},
		}
		cfg.Coins = append(cfg.Coins, ethCfg)
	}
	return cfg
}
