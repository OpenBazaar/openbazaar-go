package wallet

import (
	"github.com/OpenBazaar/multiwallet"
	"github.com/OpenBazaar/multiwallet/config"
	"github.com/OpenBazaar/openbazaar-go/repo"
	db "github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/spvwallet"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cpacia/BitcoinCash-Wallet"
	"github.com/op/go-logging"
	"golang.org/x/net/proxy"
	"net"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

type WalletConfig struct {
	ConfigFile         schema.WalletsConfig
	RepoPath           string
	Logger             logging.LeveledBackend
	DB                 *db.DB
	Mnemonic           string
	WalletCreationDate time.Time
	Params             *chaincfg.Params
	Proxy              proxy.Dialer
}

// Build a new multiwallet using values from the config file
// If any of the four standard coins are missing from the config file
// we will load it with default values.
func NewMultiWallet(cfg *WalletConfig) (*multiwallet.MultiWallet, error) {
	var testnet bool
	if cfg.Params.Name != chaincfg.MainNetParams.Name {
		testnet = true
	}

	// Create a default config with all four coins
	coinsToUse := make(map[wallet.CoinType]bool)
	coinsToUse[wallet.Bitcoin] = true
	coinsToUse[wallet.BitcoinCash] = true
	coinsToUse[wallet.Zcash] = true
	coinsToUse[wallet.Litecoin] = true

	// Apply our openbazaar settings
	defaultConfig := config.NewDefaultConfig(coinsToUse, cfg.Params)
	defaultConfig.Mnemonic = cfg.Mnemonic
	defaultConfig.CreationDate = cfg.WalletCreationDate
	defaultConfig.Proxy = cfg.Proxy
	defaultConfig.Params = cfg.Params
	defaultConfig.Logger = cfg.Logger

	// For each coin we want to override the default database with our own sqlite db
	// We'll only override the default settings if the coin exists in the config file
	for _, coin := range defaultConfig.Coins {
		switch coin.CoinType {
		case wallet.Bitcoin:
			walletDB := CreateWalletDB(cfg.DB, coin.CoinType)
			coin.DB = walletDB
			if cfg.ConfigFile.BTC != nil {
				api, err := url.Parse(cfg.ConfigFile.BTC.FeeAPI)
				if err != nil {
					return nil, err
				}
				coin.FeeAPI = *api
				coin.LowFee = uint64(cfg.ConfigFile.BTC.LowFeeDefault)
				coin.MediumFee = uint64(cfg.ConfigFile.BTC.MediumFeeDefault)
				coin.HighFee = uint64(cfg.ConfigFile.BTC.HighFeeDefault)
				coin.MaxFee = uint64(cfg.ConfigFile.BTC.MaxFee)
				if !testnet {
					api, err := url.Parse(cfg.ConfigFile.BTC.API)
					if err != nil {
						return nil, err
					}
					coin.ClientAPI = *api
				} else {
					api, err := url.Parse(cfg.ConfigFile.BTC.APITestnet)
					if err != nil {
						return nil, err
					}
					coin.ClientAPI = *api
				}
			}
		case wallet.BitcoinCash:
			walletDB := CreateWalletDB(cfg.DB, coin.CoinType)
			coin.DB = walletDB
			if cfg.ConfigFile.BCH != nil {
				api, err := url.Parse(cfg.ConfigFile.BCH.FeeAPI)
				if err != nil {
					return nil, err
				}
				coin.FeeAPI = *api
				coin.LowFee = uint64(cfg.ConfigFile.BCH.LowFeeDefault)
				coin.MediumFee = uint64(cfg.ConfigFile.BCH.MediumFeeDefault)
				coin.HighFee = uint64(cfg.ConfigFile.BCH.HighFeeDefault)
				coin.MaxFee = uint64(cfg.ConfigFile.BCH.MaxFee)
				if !testnet {
					api, err := url.Parse(cfg.ConfigFile.BCH.API)
					if err != nil {
						return nil, err
					}
					coin.ClientAPI = *api
				} else {
					api, err := url.Parse(cfg.ConfigFile.BCH.APITestnet)
					if err != nil {
						return nil, err
					}
					coin.ClientAPI = *api
				}
			}
		case wallet.Zcash:
			walletDB := CreateWalletDB(cfg.DB, coin.CoinType)
			coin.DB = walletDB
			if cfg.ConfigFile.ZEC != nil {
				api, err := url.Parse(cfg.ConfigFile.ZEC.FeeAPI)
				if err != nil {
					return nil, err
				}
				coin.FeeAPI = *api
				coin.LowFee = uint64(cfg.ConfigFile.ZEC.LowFeeDefault)
				coin.MediumFee = uint64(cfg.ConfigFile.ZEC.MediumFeeDefault)
				coin.HighFee = uint64(cfg.ConfigFile.ZEC.HighFeeDefault)
				coin.MaxFee = uint64(cfg.ConfigFile.ZEC.MaxFee)
				if !testnet {
					api, err := url.Parse(cfg.ConfigFile.ZEC.API)
					if err != nil {
						return nil, err
					}
					coin.ClientAPI = *api
				} else {
					api, err := url.Parse(cfg.ConfigFile.ZEC.APITestnet)
					if err != nil {
						return nil, err
					}
					coin.ClientAPI = *api
				}
			}
		case wallet.Litecoin:
			walletDB := CreateWalletDB(cfg.DB, coin.CoinType)
			coin.DB = walletDB
			if cfg.ConfigFile.LTC != nil {
				api, err := url.Parse(cfg.ConfigFile.LTC.FeeAPI)
				if err != nil {
					return nil, err
				}
				coin.FeeAPI = *api
				coin.LowFee = uint64(cfg.ConfigFile.LTC.LowFeeDefault)
				coin.MediumFee = uint64(cfg.ConfigFile.LTC.MediumFeeDefault)
				coin.HighFee = uint64(cfg.ConfigFile.LTC.HighFeeDefault)
				coin.MaxFee = uint64(cfg.ConfigFile.LTC.MaxFee)
				if !testnet {
					api, err := url.Parse(cfg.ConfigFile.LTC.API)
					if err != nil {
						return nil, err
					}
					coin.ClientAPI = *api
				} else {
					api, err := url.Parse(cfg.ConfigFile.LTC.APITestnet)
					if err != nil {
						return nil, err
					}
					coin.ClientAPI = *api
				}
			}
		}
	}

	mw, err := multiwallet.NewMultiWallet(defaultConfig)
	if err != nil {
		return nil, err
	}

	// Now that we have our multiwallet let's go back and check to see if the user
	// requested SPV for either Bitcoin or BitcoinCash. If so, we'll override the
	// API implementation in the multiwallet map with an SPV implementation.
	if cfg.ConfigFile.BTC != nil && strings.ToUpper(cfg.ConfigFile.BTC.Type) == "SPV" {
		var tp net.Addr
		if cfg.ConfigFile.BTC.TrustedPeer != "" {
			tp, err = net.ResolveTCPAddr("tcp", cfg.ConfigFile.BTC.TrustedPeer)
			if err != nil {
				return nil, err
			}
		}
		feeAPI, err := url.Parse(cfg.ConfigFile.BTC.FeeAPI)
		if err != nil {
			return nil, err
		}
		bitcoinPath := path.Join(cfg.RepoPath, "bitcoin")
		os.Mkdir(bitcoinPath, os.ModePerm)
		spvwalletConfig := &spvwallet.Config{
			Mnemonic:     cfg.Mnemonic,
			Params:       cfg.Params,
			MaxFee:       uint64(cfg.ConfigFile.BTC.MaxFee),
			LowFee:       uint64(cfg.ConfigFile.BTC.LowFeeDefault),
			MediumFee:    uint64(cfg.ConfigFile.BTC.MediumFeeDefault),
			HighFee:      uint64(cfg.ConfigFile.BTC.HighFeeDefault),
			FeeAPI:       *feeAPI,
			RepoPath:     bitcoinPath,
			CreationDate: cfg.WalletCreationDate,
			DB:           CreateWalletDB(cfg.DB, wallet.Bitcoin),
			UserAgent:    "OpenBazaar",
			TrustedPeer:  tp,
			Proxy:        cfg.Proxy,
			Logger:       cfg.Logger,
		}
		bitcoinSPVWallet, err := spvwallet.NewSPVWallet(spvwalletConfig)
		if err != nil {
			return nil, err
		}
		mw[wallet.Bitcoin] = bitcoinSPVWallet
	}
	if cfg.ConfigFile.BCH != nil && strings.ToUpper(cfg.ConfigFile.BCH.Type) == "SPV" {
		var tp net.Addr
		if cfg.ConfigFile.BCH.TrustedPeer != "" {
			tp, err = net.ResolveTCPAddr("tcp", cfg.ConfigFile.BCH.TrustedPeer)
			if err != nil {
				return nil, err
			}
		}
		feeAPI, err := url.Parse(cfg.ConfigFile.BCH.FeeAPI)
		if err != nil {
			return nil, err
		}
		bitcoinCashPath := path.Join(cfg.RepoPath, "bitcoincash")
		os.Mkdir(bitcoinCashPath, os.ModePerm)
		bitcoinCashConfig := &bitcoincash.Config{
			Mnemonic:     cfg.Mnemonic,
			Params:       cfg.Params,
			MaxFee:       uint64(cfg.ConfigFile.BCH.MaxFee),
			LowFee:       uint64(cfg.ConfigFile.BCH.LowFeeDefault),
			MediumFee:    uint64(cfg.ConfigFile.BCH.MediumFeeDefault),
			HighFee:      uint64(cfg.ConfigFile.BCH.HighFeeDefault),
			FeeAPI:       *feeAPI,
			RepoPath:     bitcoinCashPath,
			CreationDate: cfg.WalletCreationDate,
			DB:           CreateWalletDB(cfg.DB, wallet.BitcoinCash),
			UserAgent:    "OpenBazaar",
			TrustedPeer:  tp,
			Proxy:        cfg.Proxy,
			Logger:       cfg.Logger,
		}
		bitcoinCashSPVWallet, err := bitcoincash.NewSPVWallet(bitcoinCashConfig)
		if err != nil {
			return nil, err
		}
		mw[wallet.BitcoinCash] = bitcoinCashSPVWallet
	}

	return &mw, nil
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
