package spvwallet

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/mitchellh/go-homedir"
	"github.com/op/go-logging"
	"golang.org/x/net/proxy"
	"net"
	"net/url"
	"os"
	"path/filepath"
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

	// The user-agent that shall be visible to peers
	UserAgent string

	// Location of the data directory
	RepoPath string

	// An implementation of the Datastore interface
	DB Datastore

	// If you wish to connect to a single trusted peer set this. Otherwise leave nil.
	TrustedPeer net.Addr

	// A Tor proxy can be set here causing the wallet will use Tor
	Proxy proxy.Dialer

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

	// A logger. You can write the logs to file or stdout or however else you want.
	Logger logging.Backend
}

func NewDefaultConfig() *Config {
	repoPath, _ := homedir.Expand(filepath.Join("~", ".spvwallet"))
	_, ferr := os.Stat(repoPath)
	if os.IsNotExist(ferr) {
		os.Mkdir(repoPath, os.ModePerm)
	}
	feeApi, _ := url.Parse("https://bitcoinfees.21.co/api/v1/fees/recommended")
	return &Config{
		Params:    &chaincfg.MainNetParams,
		UserAgent: "spvwallet",
		RepoPath:  repoPath,
		LowFee:    140,
		MediumFee: 160,
		HighFee:   180,
		MaxFee:    2000,
		FeeAPI:    *feeApi,
		Logger:    logging.NewLogBackend(os.Stdout, "", 0),
	}
}
