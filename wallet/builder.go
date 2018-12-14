package wallet

import (
	"fmt"
	"time"

	eth "github.com/OpenBazaar/go-ethwallet/wallet"
	"github.com/OpenBazaar/multiwallet"
	"github.com/OpenBazaar/multiwallet/bitcoin"
	"github.com/OpenBazaar/multiwallet/bitcoincash"
	"github.com/OpenBazaar/multiwallet/cache"
	mwConfig "github.com/OpenBazaar/multiwallet/config"
	"github.com/OpenBazaar/multiwallet/litecoin"
	"github.com/OpenBazaar/multiwallet/zcash"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/op/go-logging"
	"golang.org/x/net/proxy"
)

type WalletConfig struct {
	ConfigFile           *schema.WalletsConfig
	RepoPath             string
	Logger               logging.Backend
	DB                   *db.DB
	Mnemonic             string
	WalletCreationDate   time.Time
	Params               *chaincfg.Params
	Proxy                proxy.Dialer
	DisableExchangeRates bool
}

// Build a new multiwallet using values from the config file
// If any of the four standard coins are missing from the config file
// we will load it with default values.
func NewMultiWallet(cfg *WalletConfig) (multiwallet.MultiWallet, error) {
	// Create a default config with all coins
	enableAPIWallet := make(map[wallet.CoinType]*schema.CoinConfig)
	if cfg.ConfigFile.BTC != nil && cfg.ConfigFile.BTC.Type == "API" {
		enableAPIWallet[wallet.Bitcoin] = cfg.ConfigFile.BTC
	}
	if cfg.ConfigFile.BCH != nil && cfg.ConfigFile.BCH.Type == "API" {
		enableAPIWallet[wallet.BitcoinCash] = cfg.ConfigFile.BCH
	}
	if cfg.ConfigFile.ZEC != nil && cfg.ConfigFile.ZEC.Type == "API" {
		enableAPIWallet[wallet.Zcash] = cfg.ConfigFile.ZEC
	}
	if cfg.ConfigFile.LTC != nil && cfg.ConfigFile.LTC.Type == "API" {
		enableAPIWallet[wallet.Litecoin] = cfg.ConfigFile.LTC
	}
	enableAPIWallet[wallet.Ethereum] = nil

	// For each coin we want to override the default database with our own sqlite db
	// We'll only override the default settings if the coin exists in the config file
	var newMultiwallet = make(multiwallet.MultiWallet)
	for coin, coinConfig := range enableAPIWallet {
		if coinConfig != nil {
			actualCoin, newWallet, err := createWallet(coin, coinConfig, cfg)
			if err != nil {
				var logger = logging.MustGetLogger("NewMultiwallet")
				logger.SetBackend(logging.AddModuleLevel(cfg.Logger))
				logger.Errorf("failed creating wallet for %s: %s", actualCoin, err)
				continue
			}
			newMultiwallet[actualCoin] = newWallet
		}
	}

	return newMultiwallet, nil
}

func createWallet(coin wallet.CoinType, coinConfigOverrides *schema.CoinConfig, cfg *WalletConfig) (wallet.CoinType, wallet.Wallet, error) {
	var (
		actualCoin wallet.CoinType
		testnet    = cfg.Params.Name != chaincfg.MainNetParams.Name
		coinConfig = prepareCoinConfig(coin, coinConfigOverrides, cfg)
	)

	switch coin {
	case wallet.Bitcoin:
		if testnet {
			actualCoin = wallet.TestnetBitcoin
		} else {
			actualCoin = wallet.Bitcoin
		}
		w, err := bitcoin.NewBitcoinWallet(*coinConfig, cfg.Mnemonic, cfg.Params, cfg.Proxy, cache.NewMockCacher(), cfg.DisableExchangeRates)
		if err != nil {
			return actualCoin, nil, err
		}
		return actualCoin, w, nil
	case wallet.BitcoinCash:
		if testnet {
			actualCoin = wallet.TestnetBitcoinCash
		} else {
			actualCoin = wallet.BitcoinCash
		}
		w, err := bitcoincash.NewBitcoinCashWallet(*coinConfig, cfg.Mnemonic, cfg.Params, cfg.Proxy, cache.NewMockCacher(), cfg.DisableExchangeRates)
		if err != nil {
			return actualCoin, nil, err
		}
		return actualCoin, w, nil
	case wallet.Litecoin:
		if testnet {
			actualCoin = wallet.TestnetLitecoin
		} else {
			actualCoin = wallet.Litecoin
		}
		w, err := litecoin.NewLitecoinWallet(*coinConfig, cfg.Mnemonic, cfg.Params, cfg.Proxy, cache.NewMockCacher(), cfg.DisableExchangeRates)
		if err != nil {
			return actualCoin, nil, err
		}
		return actualCoin, w, nil
	case wallet.Zcash:
		if testnet {
			actualCoin = wallet.TestnetZcash
		} else {
			actualCoin = wallet.Zcash
		}
		w, err := zcash.NewZCashWallet(*coinConfig, cfg.Mnemonic, cfg.Params, cfg.Proxy, cache.NewMockCacher(), cfg.DisableExchangeRates)
		if err != nil {
			return actualCoin, nil, err
		}
		return actualCoin, w, nil
	case wallet.Ethereum:
		actualCoin = wallet.Ethereum
		w, err := eth.NewEthereumWallet(*coinConfig, cfg.Mnemonic, cfg.Proxy)
		if err != nil {
			return actualCoin, nil, err
		}
		return actualCoin, w, nil
	}
	return wallet.CoinType(4294967295), nil, fmt.Errorf("unable to create wallet for unknown coin %s", coin)
}

func prepareCoinConfig(coin wallet.CoinType, override *schema.CoinConfig, walletConfig *WalletConfig) *mwConfig.CoinConfig {
	var (
		defaultCoinOptions      map[string]interface{}
		defaultConfig           *schema.CoinConfig
		overrideWalletEndpoints []string
		defaultWalletEndpoints  []string

		defaultConfigSet = schema.DefaultWalletsConfig()
		testnet          = walletConfig.Params.Name != chaincfg.MainNetParams.Name
	)

	switch coin {
	case wallet.Bitcoin:
		defaultConfig = defaultConfigSet.BTC
	case wallet.BitcoinCash:
		defaultConfig = defaultConfigSet.BCH
	case wallet.Litecoin:
		defaultConfig = defaultConfigSet.LTC
	case wallet.Zcash:
		defaultConfig = defaultConfigSet.ZEC
	case wallet.Ethereum:
		defaultConfig = defaultConfigSet.ETH
		defaultCoinOptions = schema.EthereumDefaultOptions()
	}

	if testnet {
		overrideWalletEndpoints = override.APITestnetPool
		defaultWalletEndpoints = defaultConfig.APITestnetPool
	} else {
		overrideWalletEndpoints = override.APIPool
		defaultWalletEndpoints = defaultConfig.APIPool
	}

	var preparedConfig = &mwConfig.CoinConfig{
		ClientAPIs: overrideWalletEndpoints,
		CoinType:   coin,
		DB:         CreateWalletDB(walletConfig.DB, coin),
		FeeAPI:     override.FeeAPI,
		HighFee:    override.HighFeeDefault,
		LowFee:     override.LowFeeDefault,
		MaxFee:     override.MaxFee,
		MediumFee:  override.MediumFeeDefault,
		Options:    override.WalletOptions,
	}

	if preparedConfig.HighFee == 0 {
		preparedConfig.HighFee = defaultConfig.HighFeeDefault
	}
	if preparedConfig.MediumFee == 0 {
		preparedConfig.MediumFee = defaultConfig.MediumFeeDefault
	}
	if preparedConfig.LowFee == 0 {
		preparedConfig.LowFee = defaultConfig.LowFeeDefault
	}
	if preparedConfig.MaxFee == 0 {
		preparedConfig.MaxFee = defaultConfig.MaxFee
	}
	if preparedConfig.ClientAPIs == nil || len(preparedConfig.ClientAPIs) == 0 {
		preparedConfig.ClientAPIs = defaultWalletEndpoints
	}
	if preparedConfig.FeeAPI == "" {
		preparedConfig.FeeAPI = "https://btc.fees.openbazaar.org"
	}
	if len(preparedConfig.Options) == 0 {
		preparedConfig.Options = defaultCoinOptions
	}

	return preparedConfig
}

type WalletDatastore struct {
	keys           repo.KeyStore
	stxos          repo.SpentTransactionOutputStore
	txns           repo.TransactionStore
	utxos          repo.UnspentTransactionOutputStore
	watchedScripts repo.WatchedScriptStore
}

func (d *WalletDatastore) Keys() wallet.Keys {
	return d.keys
}
func (d *WalletDatastore) Stxos() wallet.Stxos {
	return d.stxos
}
func (d *WalletDatastore) Txns() wallet.Txns {
	return d.txns
}
func (d *WalletDatastore) Utxos() wallet.Utxos {
	return d.utxos
}
func (d *WalletDatastore) WatchedScripts() wallet.WatchedScripts {
	return d.watchedScripts
}

func CreateWalletDB(database *db.DB, coinType wallet.CoinType) *WalletDatastore {
	return &WalletDatastore{
		keys:           db.NewKeyStore(database.SqlDB, database.Lock, coinType),
		utxos:          db.NewUnspentTransactionStore(database.SqlDB, database.Lock, coinType),
		stxos:          db.NewSpentTransactionStore(database.SqlDB, database.Lock, coinType),
		txns:           db.NewTransactionStore(database.SqlDB, database.Lock, coinType),
		watchedScripts: db.NewWatchedScriptStore(database.SqlDB, database.Lock, coinType),
	}
}
