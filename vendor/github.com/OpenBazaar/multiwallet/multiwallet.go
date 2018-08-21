package multiwallet

import (
	"errors"
	"github.com/OpenBazaar/multiwallet/bitcoin"
	"github.com/OpenBazaar/multiwallet/bitcoincash"
	"github.com/OpenBazaar/multiwallet/client"
	"github.com/OpenBazaar/multiwallet/config"
	"github.com/OpenBazaar/multiwallet/litecoin"
	"github.com/OpenBazaar/multiwallet/service"
	"github.com/OpenBazaar/multiwallet/zcash"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/op/go-logging"
	"github.com/tyler-smith/go-bip39"
	"strings"
	"time"
)

var log = logging.MustGetLogger("multiwallet")

var UnsuppertedCoinError = errors.New("multiwallet does not contain an implementation for the given coin")

type MultiWallet map[wallet.CoinType]wallet.Wallet

func NewMultiWallet(cfg *config.Config) (MultiWallet, error) {
	log.SetBackend(logging.AddModuleLevel(cfg.Logger))
	service.Log = log
	client.Log = log

	if cfg.Mnemonic == "" {
		ent, err := bip39.NewEntropy(128)
		if err != nil {
			return nil, err
		}
		mnemonic, err := bip39.NewMnemonic(ent)
		if err != nil {
			return nil, err
		}
		cfg.Mnemonic = mnemonic
		cfg.CreationDate = time.Now()
	}

	multiwallet := make(MultiWallet)
	var err error
	for _, coin := range cfg.Coins {
		var w wallet.Wallet
		switch coin.CoinType {
		case wallet.Bitcoin:
			w, err = bitcoin.NewBitcoinWallet(coin, cfg.Mnemonic, cfg.Params, cfg.Proxy)
			if err != nil {
				return nil, err
			}
			multiwallet[coin.CoinType] = w
		case wallet.BitcoinCash:
			w, err = bitcoincash.NewBitcoinCashWallet(coin, cfg.Mnemonic, cfg.Params, cfg.Proxy)
			if err != nil {
				return nil, err
			}
			multiwallet[coin.CoinType] = w
		case wallet.Zcash:
			w, err = zcash.NewZCashWallet(coin, cfg.Mnemonic, cfg.Params, cfg.Proxy)
			if err != nil {
				return nil, err
			}
			multiwallet[coin.CoinType] = w
		case wallet.Litecoin:
			w, err = litecoin.NewLitecoinWallet(coin, cfg.Mnemonic, cfg.Params, cfg.Proxy)
			if err != nil {
				return nil, err
			}
			multiwallet[coin.CoinType] = w
		}
	}
	return multiwallet, nil
}

func (w *MultiWallet) Start() {
	for _, wallet := range *w {
		wallet.Start()
	}
}

func (w *MultiWallet) Close() {
	for _, wallet := range *w {
		wallet.Close()
	}
}

func (w *MultiWallet) WalletForCurrencyCode(currencyCode string) (wallet.Wallet, error) {
	for _, wl := range *w {
		if strings.ToUpper(wl.CurrencyCode()) == strings.ToUpper(currencyCode) {
			return wl, nil
		}
	}
	return nil, UnsuppertedCoinError
}
