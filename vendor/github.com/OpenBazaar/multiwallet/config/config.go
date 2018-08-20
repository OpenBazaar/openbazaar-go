package config

import (
	"github.com/OpenBazaar/multiwallet/datastore"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/op/go-logging"
	"golang.org/x/net/proxy"
	"net/url"
	"os"
	"time"
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

	// A list of coin configs. One config should be included for each coin to be used.
	Coins []CoinConfig
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
	FeeAPI url.URL

	// The trusted API to use for querying for balances and listening to blockchain events.
	ClientAPI url.URL

	// An implementation of the Datastore interface for each desired coin
	DB wallet.Datastore
}

func NewDefaultConfig(coinTypes map[wallet.CoinType]bool, params *chaincfg.Params) *Config {
	cfg := &Config{
		Params: params,
		Logger: logging.NewLogBackend(os.Stdout, "", 0),
	}
	var testnet bool
	if params.Name == chaincfg.TestNet3Params.Name {
		testnet = true
	}
	mockDB := datastore.NewMockMultiwalletDatastore()
	if coinTypes[wallet.Bitcoin] {
		var apiEndpoint string
		if !testnet {
			apiEndpoint = "https://btc.bloqapi.net/insight-api"
		} else {
			apiEndpoint = "https://test-insight.bitpay.com/api"
		}
		feeApi, _ := url.Parse("https://btc.fees.openbazaar.org")
		clientApi, _ := url.Parse(apiEndpoint)
		db, _ := mockDB.GetDatastoreForWallet(wallet.Bitcoin)
		btcCfg := CoinConfig{
			CoinType:  wallet.Bitcoin,
			FeeAPI:    *feeApi,
			LowFee:    140,
			MediumFee: 160,
			HighFee:   180,
			MaxFee:    2000,
			ClientAPI: *clientApi,
			DB:        db,
		}
		cfg.Coins = append(cfg.Coins, btcCfg)
	}
	if coinTypes[wallet.BitcoinCash] {
		var apiEndpoint string
		if !testnet {
			apiEndpoint = "https://bitcoincash.blockexplorer.com/api"
		} else {
			apiEndpoint = "https://test-bch-insight.bitpay.com/api"
		}
		clientApi, _ := url.Parse(apiEndpoint)
		db, _ := mockDB.GetDatastoreForWallet(wallet.BitcoinCash)
		bchCfg := CoinConfig{
			CoinType:  wallet.BitcoinCash,
			FeeAPI:    url.URL{},
			LowFee:    140,
			MediumFee: 160,
			HighFee:   180,
			MaxFee:    2000,
			ClientAPI: *clientApi,
			DB:        db,
		}
		cfg.Coins = append(cfg.Coins, bchCfg)
	}
	if coinTypes[wallet.Zcash] {
		var apiEndpoint string
		if !testnet {
			apiEndpoint = "https://zcashnetwork.info/api"
		} else {
			apiEndpoint = "https://explorer.testnet.z.cash/api"
		}
		clientApi, _ := url.Parse(apiEndpoint)
		db, _ := mockDB.GetDatastoreForWallet(wallet.Zcash)
		zecCfg := CoinConfig{
			CoinType:  wallet.Zcash,
			FeeAPI:    url.URL{},
			LowFee:    140,
			MediumFee: 160,
			HighFee:   180,
			MaxFee:    2000,
			ClientAPI: *clientApi,
			DB:        db,
		}
		cfg.Coins = append(cfg.Coins, zecCfg)
	}
	if coinTypes[wallet.Litecoin] {
		var apiEndpoint string
		if !testnet {
			apiEndpoint = "https://ltc.coin.space/api"
		} else {
			apiEndpoint = "https://testnet.litecore.io/api"
		}
		clientApi, _ := url.Parse(apiEndpoint)
		db, _ := mockDB.GetDatastoreForWallet(wallet.Litecoin)
		ltcCfg := CoinConfig{
			CoinType:  wallet.Litecoin,
			FeeAPI:    url.URL{},
			LowFee:    140,
			MediumFee: 160,
			HighFee:   180,
			MaxFee:    2000,
			ClientAPI: *clientApi,
			DB:        db,
		}
		cfg.Coins = append(cfg.Coins, ltcCfg)
	}
	return cfg
}
