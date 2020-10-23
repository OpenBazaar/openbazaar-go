package wallet

import (
	"errors"
	"fmt"
	"github.com/OpenBazaar/multiwallet/filecoin"
	"net"
	"net/url"
	"os"
	"path"
	"strings"
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
	"github.com/OpenBazaar/spvwallet"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/op/go-logging"
	"golang.org/x/net/proxy"
)

const InvalidCoinType wallet.CoinType = wallet.CoinType(^uint32(0))

// ErrTrustedPeerRequired is returned when the config is missing the TrustedPeer field
var ErrTrustedPeerRequired = errors.New("trusted peer required in spv wallet config during regtest use")

// WalletConfig describes the options needed to create a MultiWallet
type WalletConfig struct {
	// ConfigFile contains the options of each native wallet
	ConfigFile *schema.WalletsConfig
	// RepoPath is the base path which contains the nodes data directory
	RepoPath string
	// Logger is an interface to support internal wallet logging
	Logger logging.Backend
	// DB is an interface to support internal transaction persistance
	DB *db.DB
	// Mnemonic is the string entropy used to generate the wallet's BIP39-compliant seed
	Mnemonic string
	// WalletCreationDate represents the time when new transactions were added by this wallet
	WalletCreationDate time.Time
	// Params describe the desired blockchain params to enforce on joining the network
	Params *chaincfg.Params
	// Proxy is an interface which allows traffic for the wallet to proxied
	Proxy proxy.Dialer
	// DisableExchangeRates will disable usage of the internal exchange rate API
	DisableExchangeRates bool
}

// NewMultiWallet returns a functional set of wallets using the provided WalletConfig.
// The value of schema.WalletsConfig.<COIN>.Type must be "API" or will be ignored. BTC
// and BCH can also use the "SPV" Type.
func NewMultiWallet(cfg *WalletConfig) (multiwallet.MultiWallet, error) {
	var (
		logger          = logging.MustGetLogger("NewMultiwallet")
		enableAPIWallet = make(map[wallet.CoinType]*schema.CoinConfig)
		enableSPVWallet = make(map[wallet.CoinType]*schema.CoinConfig)
	)
	logger.SetBackend(logging.AddModuleLevel(cfg.Logger))

	if cfg.ConfigFile.BTC != nil {
		switch cfg.ConfigFile.BTC.Type {
		case schema.WalletTypeAPI:
			enableAPIWallet[wallet.Bitcoin] = cfg.ConfigFile.BTC
		case schema.WalletTypeSPV:
			enableSPVWallet[wallet.Bitcoin] = cfg.ConfigFile.BTC
		}
	}
	if cfg.ConfigFile.BCH != nil {
		switch cfg.ConfigFile.BCH.Type {
		case schema.WalletTypeAPI:
			enableAPIWallet[wallet.BitcoinCash] = cfg.ConfigFile.BCH
		case schema.WalletTypeSPV:
			enableSPVWallet[wallet.BitcoinCash] = cfg.ConfigFile.BCH
		}
	}
	if cfg.ConfigFile.ZEC != nil && cfg.ConfigFile.ZEC.Type == "API" {
		enableAPIWallet[wallet.Zcash] = cfg.ConfigFile.ZEC
	}
	if cfg.ConfigFile.LTC != nil && cfg.ConfigFile.LTC.Type == "API" {
		enableAPIWallet[wallet.Litecoin] = cfg.ConfigFile.LTC
	}
	enableAPIWallet[wallet.Ethereum] = cfg.ConfigFile.ETH
	enableAPIWallet[wallet.Filecoin] = cfg.ConfigFile.FIL

	var newMultiwallet = make(multiwallet.MultiWallet)
	for coin, coinConfig := range enableAPIWallet {
		if coinConfig != nil {
			actualCoin, newWallet, err := createAPIWallet(coin, coinConfig, cfg)
			if err != nil {
				logger.Errorf("failed creating wallet for %s: %s", actualCoin, err)
				continue
			}
			newMultiwallet[actualCoin] = newWallet
		}
	}

	for coin, coinConfig := range enableSPVWallet {
		actualCoin, newWallet, err := createSPVWallet(coin, coinConfig, cfg)
		if err != nil {
			logger.Errorf("failed creating wallet for %s: %s", actualCoin, err)
			continue
		}
		newMultiwallet[actualCoin] = newWallet
	}

	return newMultiwallet, nil
}

func createAPIWallet(coin wallet.CoinType, coinConfigOverrides *schema.CoinConfig, cfg *WalletConfig) (wallet.CoinType, wallet.Wallet, error) {
	var (
		actualCoin wallet.CoinType
		testnet    = cfg.Params.Name != chaincfg.MainNetParams.Name
		coinConfig = prepareAPICoinConfig(coin, coinConfigOverrides, cfg)
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
			return InvalidCoinType, nil, err
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
			return InvalidCoinType, nil, err
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
			return InvalidCoinType, nil, err
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
			return InvalidCoinType, nil, err
		}
		return actualCoin, w, nil
	case wallet.Ethereum:
		if testnet {
			actualCoin = wallet.TestnetEthereum
		} else {
			actualCoin = wallet.Ethereum
		}
		//actualCoin = wallet.Ethereum
		w, err := eth.NewEthereumWallet(*coinConfig, cfg.Params, cfg.Mnemonic, cfg.Proxy)
		if err != nil {
			return InvalidCoinType, nil, err
		}
		return actualCoin, w, nil
	case wallet.Filecoin:
		if testnet {
			actualCoin = wallet.TestnetFilecoin
		} else {
			actualCoin = wallet.Filecoin
		}
		//actualCoin = wallet.Filecoin
		w, err := filecoin.NewFilecoinWallet(*coinConfig, cfg.Mnemonic, cfg.Params, cfg.Proxy, cache.NewMockCacher(), cfg.DisableExchangeRates)
		if err != nil {
			return InvalidCoinType, nil, err
		}
		return actualCoin, w, nil
	}
	return InvalidCoinType, nil, fmt.Errorf("unable to create wallet for unknown coin %s", coin.String())
}

func createSPVWallet(coin wallet.CoinType, coinConfigOverrides *schema.CoinConfig, cfg *WalletConfig) (wallet.CoinType, wallet.Wallet, error) {
	var (
		actualCoin         wallet.CoinType
		notMainnet         = cfg.Params.Name != chaincfg.MainNetParams.Name
		usingRegnet        = cfg.Params.Name == chaincfg.RegressionNetParams.Name
		missingTrustedPeer = coinConfigOverrides.TrustedPeer == ""
		defaultConfigSet   = schema.DefaultWalletsConfig()
	)

	if usingRegnet && missingTrustedPeer {
		return InvalidCoinType, nil, ErrTrustedPeerRequired
	}

	trustedPeer, err := net.ResolveTCPAddr("tcp", coinConfigOverrides.TrustedPeer)
	if err != nil {
		return InvalidCoinType, nil, fmt.Errorf("resolving tcp address %s: %s", coinConfigOverrides.TrustedPeer, err.Error())
	}
	feeAPI, err := url.Parse(coinConfigOverrides.FeeAPI)
	if err != nil {
		return InvalidCoinType, nil, fmt.Errorf("parsing fee api: %s", err.Error())
	}
	walletRepoPath, err := ensureWalletRepoPath(cfg.RepoPath, coin, "ob1")
	if err != nil {
		return InvalidCoinType, nil, fmt.Errorf("creating wallet repository: %s", err.Error())
	}

	switch coin {
	case wallet.Bitcoin:
		defaultConfig := defaultConfigSet.BTC
		preparedConfig := &spvwallet.Config{
			Mnemonic:             cfg.Mnemonic,
			Params:               cfg.Params,
			MaxFee:               coinConfigOverrides.MaxFee,
			SuperLowFee:          coinConfigOverrides.SuperLowFeeDefault,
			LowFee:               coinConfigOverrides.LowFeeDefault,
			MediumFee:            coinConfigOverrides.MediumFeeDefault,
			HighFee:              coinConfigOverrides.HighFeeDefault,
			FeeAPI:               *feeAPI,
			RepoPath:             walletRepoPath,
			CreationDate:         cfg.WalletCreationDate,
			DB:                   CreateWalletDB(cfg.DB, coin),
			UserAgent:            "OpenBazaar",
			TrustedPeer:          trustedPeer,
			Proxy:                cfg.Proxy,
			Logger:               cfg.Logger,
			DisableExchangeRates: cfg.DisableExchangeRates,
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

		newSPVWallet, err := spvwallet.NewSPVWallet(preparedConfig)
		if err != nil {
			return InvalidCoinType, nil, err
		}

		if notMainnet {
			actualCoin = wallet.TestnetBitcoin
		} else {
			actualCoin = wallet.Bitcoin
		}
		return actualCoin, newSPVWallet, nil
	}
	return InvalidCoinType, nil, fmt.Errorf("unable to create wallet for unknown coin %s", coin.String())
}

func ensureWalletRepoPath(repoPath string, coin wallet.CoinType, devGUID string) (string, error) {
	var (
		walletPathName = walletPath(coin, devGUID)
		walletRepoPath = path.Join(repoPath, "wallets", walletPathName)
	)
	if err := os.MkdirAll(walletRepoPath, os.ModePerm); err != nil {
		return "", err
	}
	return walletRepoPath, nil
}

func walletPath(coin wallet.CoinType, devGUID string) string {
	var (
		raw         = fmt.Sprintf("%s.%s", coin.String(), devGUID)
		lowerCoin   = strings.ToLower(raw)
		trimmedCoin = strings.Replace(lowerCoin, " ", "", -1)
	)
	return trimmedCoin
}

func prepareAPICoinConfig(coin wallet.CoinType, override *schema.CoinConfig, walletConfig *WalletConfig) *mwConfig.CoinConfig {
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
	case wallet.Filecoin:
		defaultConfig = defaultConfigSet.FIL
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
		ClientAPIs:  overrideWalletEndpoints,
		CoinType:    coin,
		DB:          CreateWalletDB(walletConfig.DB, coin),
		FeeAPI:      override.FeeAPI,
		HighFee:     override.HighFeeDefault,
		SuperLowFee: override.SuperLowFeeDefault,
		LowFee:      override.LowFeeDefault,
		MaxFee:      override.MaxFee,
		MediumFee:   override.MediumFeeDefault,
		Options:     override.WalletOptions,
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
	if len(preparedConfig.ClientAPIs) == 0 {
		preparedConfig.ClientAPIs = defaultWalletEndpoints
	}
	if preparedConfig.CoinType == wallet.Bitcoin && preparedConfig.FeeAPI == "" {
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
