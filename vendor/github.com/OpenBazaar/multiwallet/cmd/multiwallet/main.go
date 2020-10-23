package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/OpenBazaar/multiwallet"
	"github.com/OpenBazaar/multiwallet/api"
	"github.com/OpenBazaar/multiwallet/cli"
	"github.com/OpenBazaar/multiwallet/config"
	wi "github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/jessevdk/go-flags"
)

const WALLET_VERSION = "0.1.0"

var parser = flags.NewParser(nil, flags.Default)

type Start struct {
	Testnet bool `short:"t" long:"testnet" description:"use the test network"`
}
type Version struct{}

var start Start
var version Version
var mw multiwallet.MultiWallet

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			fmt.Println("Multiwallet shutting down...")
			os.Exit(1)
		}
	}()
	parser.AddCommand("start",
		"start the wallet",
		"The start command starts the wallet daemon",
		&start)
	parser.AddCommand("version",
		"print the version number",
		"Print the version number and exit",
		&version)
	cli.SetupCli(parser)
	if _, err := parser.Parse(); err != nil {
		os.Exit(1)
	}
}

func (x *Version) Execute(args []string) error {
	fmt.Println(WALLET_VERSION)
	return nil
}

func (x *Start) Execute(args []string) error {
	m := make(map[wi.CoinType]bool)
	m[wi.Bitcoin] = true
	m[wi.BitcoinCash] = true
	m[wi.Zcash] = true
	m[wi.Litecoin] = true
	m[wi.Ethereum] = true
	params := &chaincfg.MainNetParams
	if x.Testnet {
		params = &chaincfg.TestNet3Params
	}
	cfg := config.NewDefaultConfig(m, params)
	cfg.Mnemonic = "bottle author ability expose illegal saddle antique setup pledge wife innocent treat"
	var err error
	mw, err = multiwallet.NewMultiWallet(cfg)
	if err != nil {
		return err
	}
	go api.ServeAPI(mw)
	var wg sync.WaitGroup
	wg.Add(1)
	mw.Start()
	wg.Wait()
	return nil
}
