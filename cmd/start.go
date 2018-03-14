package cmd

import (
	"context"
	"errors"
	"fmt"
	ipfslogging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	manet "gx/ipfs/QmX3U3YXCQ6UYBxq2LVWF8dARS1hPUTEYLrSx654Qyxyw6/go-multiaddr-net"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"

	"crypto/rand"
	bstk "github.com/OpenBazaar/go-blockstackclient"
	"github.com/OpenBazaar/openbazaar-go/api"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"github.com/OpenBazaar/openbazaar-go/bitcoin/bitcoind"
	"github.com/OpenBazaar/openbazaar-go/bitcoin/exchange"
	lis "github.com/OpenBazaar/openbazaar-go/bitcoin/listeners"
	"github.com/OpenBazaar/openbazaar-go/bitcoin/resync"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	obns "github.com/OpenBazaar/openbazaar-go/namesys"
	obnet "github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/net/service"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	sto "github.com/OpenBazaar/openbazaar-go/storage"
	"github.com/OpenBazaar/openbazaar-go/storage/dropbox"
	"github.com/OpenBazaar/openbazaar-go/storage/selfhosted"
	"github.com/OpenBazaar/spvwallet"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/base58"
	"github.com/cpacia/BitcoinCash-Wallet"
	cashrates "github.com/cpacia/BitcoinCash-Wallet/exchangerates"
	"github.com/fatih/color"
	"github.com/ipfs/go-ipfs/commands"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/corehttp"
	bitswap "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	"github.com/ipfs/go-ipfs/namesys"
	namepb "github.com/ipfs/go-ipfs/namesys/pb"
	ipath "github.com/ipfs/go-ipfs/path"
	ipfsrepo "github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/config"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/OpenBazaar/openbazaar-go/bitcoin/zcashd"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	lockfile "github.com/ipfs/go-ipfs/repo/fsrepo/lock"
	"github.com/ipfs/go-ipfs/thirdparty/ds-help"
	"github.com/natefinch/lumberjack"
	"github.com/op/go-logging"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/proxy"
	routing "gx/ipfs/QmPR2JzfKd9poHx9XBhzoFeBBC31ZM3W5iUPKJZWyaoZZm/go-libp2p-routing"
	pstore "gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	metrics "gx/ipfs/QmQbh3Rb7KM37As3vkHYnEFnzkVXNCP8EYGtHz6g2fXk14/go-libp2p-metrics"
	"gx/ipfs/QmQq9YzmdFdWNTDdArueGyD7L5yyiRQigrRHJnTGkxcEjT/go-libp2p-interface-pnet"
	p2pbhost "gx/ipfs/QmRQ76P5dgvxTujhfPsCRAG83rC15jgb1G9bKLuomuC6dQ/go-libp2p/p2p/host/basic"
	dht "gx/ipfs/QmUCS9EnqNq1kCnJds2eLDypBiS21aSiCf1MVzSUVB9TGA/go-libp2p-kad-dht"
	dhtutil "gx/ipfs/QmUCS9EnqNq1kCnJds2eLDypBiS21aSiCf1MVzSUVB9TGA/go-libp2p-kad-dht/util"
	addrutil "gx/ipfs/QmVJGsPeK3vwtEyyTxpCs47yjBYMmYsAhEouPDF3Gb2eK3/go-addr-util"
	oniontp "gx/ipfs/QmVYZ6jGE4uogWAZK2w8PrKWDEKMvYaQWTSXWCbYJLEuKs/go-onion-transport"
	swarm "gx/ipfs/QmWpJ4y2vxJ6GZpPfQbpVpQxAYS3UeR6AKNbAHxw7wN3qw/go-libp2p-swarm"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	smux "gx/ipfs/QmY9JXR3FupnYAYJWK9aMr9bCpqWKcToQ1tz8DVGTrHpHw/go-stream-muxer"
	p2phost "gx/ipfs/QmaSxYRuMq4pkpBBG2CYaRrPx2z7NmMVEs34b9g61biQA6/go-libp2p-host"
	recpb "gx/ipfs/QmbxkgUceEcuSZ4ZdBA3x74VUDSSYjHYmmeEqkjxbtZ6Jg/go-libp2p-record/pb"
	"io"
	"syscall"
	"time"
)

var stdoutLogFormat = logging.MustStringFormatter(
	`%{color:reset}%{color}%{time:15:04:05.000} [%{level}] [%{module}/%{shortfunc}] %{message}`,
)

var fileLogFormat = logging.MustStringFormatter(
	`%{time:15:04:05.000} [%{level}] [%{module}/%{shortfunc}] %{message}`,
)

var (
	ErrNoGateways = errors.New("No gateway addresses configured")
)

type Start struct {
	Password             string   `short:"p" long:"password" description:"the encryption password if the database is encrypted"`
	Testnet              bool     `short:"t" long:"testnet" description:"use the test network"`
	Regtest              bool     `short:"r" long:"regtest" description:"run in regression test mode"`
	LogLevel             string   `short:"l" long:"loglevel" description:"set the logging level [debug, info, notice, warning, error, critical]" defaut:"debug"`
	NoLogFiles           bool     `short:"f" long:"nologfiles" description:"save logs on disk"`
	AllowIP              []string `short:"a" long:"allowip" description:"only allow API connections from these IPs"`
	STUN                 bool     `short:"s" long:"stun" description:"use stun on µTP IPv4"`
	DataDir              string   `short:"d" long:"datadir" description:"specify the data directory to be used"`
	AuthCookie           string   `short:"c" long:"authcookie" description:"turn on API authentication and use this specific cookie"`
	UserAgent            string   `short:"u" long:"useragent" description:"add a custom user-agent field"`
	Verbose              bool     `short:"v" long:"verbose" description:"print openbazaar logs to stdout"`
	TorPassword          string   `long:"torpassword" description:"Set the tor control password. This will override the tor password in the config."`
	Tor                  bool     `long:"tor" description:"Automatically configure the daemon to run as a Tor hidden service and use Tor exclusively. Requires Tor to be running."`
	DualStack            bool     `long:"dualstack" description:"Automatically configure the daemon to run as a Tor hidden service IN ADDITION to using the clear internet. Requires Tor to be running. WARNING: this mode is not private"`
	DisableWallet        bool     `long:"disablewallet" description:"disable the wallet functionality of the node"`
	DisableExchangeRates bool     `long:"disableexchangerates" description:"disable the exchange rate service to prevent api queries"`
	Storage              string   `long:"storage" description:"set the outgoing message storage option [self-hosted, dropbox] default=self-hosted"`
	BitcoinCash          bool     `long:"bitcoincash" description:"use a Bitcoin Cash wallet in a dedicated data directory"`
	ZCash                string   `long:"zcash" description:"use a ZCash wallet in a dedicated data directory. To use this you must pass in the location of the zcashd binary."`
}

func (x *Start) Execute(args []string) error {
	printSplashScreen(x.Verbose)

	if x.Testnet && x.Regtest {
		return errors.New("Invalid combination of testnet and regtest modes")
	}

	if x.Tor && x.DualStack {
		return errors.New("Invalid combination of tor and dual stack modes")
	}

	isTestnet := false
	if x.Testnet || x.Regtest {
		isTestnet = true
	}
	if x.BitcoinCash && x.ZCash != "" {
		return errors.New("Bitcoin Cash and ZCash cannot be used at the same time")
	}

	// Set repo path
	repoPath, err := repo.GetRepoPath(isTestnet)
	if err != nil {
		return err
	}
	if x.BitcoinCash {
		repoPath += "-bitcoincash"
	} else if x.ZCash != "" {
		repoPath += "-zcash"
	}
	if x.DataDir != "" {
		repoPath = x.DataDir
	}

	repoLockFile := filepath.Join(repoPath, lockfile.LockFile)
	os.Remove(repoLockFile)

	sqliteDB, err := InitializeRepo(repoPath, x.Password, "", isTestnet, time.Now())
	if err != nil && err != repo.ErrRepoExists {
		return err
	}

	// Logging
	w := &lumberjack.Logger{
		Filename:   path.Join(repoPath, "logs", "ob.log"),
		MaxSize:    10, // Megabytes
		MaxBackups: 3,
		MaxAge:     30, // Days
	}
	var backendStdoutFormatter logging.Backend
	if x.Verbose {
		backendStdout := logging.NewLogBackend(os.Stdout, "", 0)
		backendStdoutFormatter = logging.NewBackendFormatter(backendStdout, stdoutLogFormat)
		logging.SetBackend(backendStdoutFormatter)
	}

	if !x.NoLogFiles {
		backendFile := logging.NewLogBackend(w, "", 0)
		backendFileFormatter := logging.NewBackendFormatter(backendFile, fileLogFormat)
		if x.Verbose {
			logging.SetBackend(backendFileFormatter, backendStdoutFormatter)
		} else {
			logging.SetBackend(backendFileFormatter)
		}
		ipfslogging.LdJSONFormatter()
		w2 := &lumberjack.Logger{
			Filename:   path.Join(repoPath, "logs", "ipfs.log"),
			MaxSize:    10, // Megabytes
			MaxBackups: 3,
			MaxAge:     30, // Days
		}
		ipfslogging.Output(w2)()
	}

	var level logging.Level
	switch strings.ToLower(x.LogLevel) {
	case "debug":
		level = logging.DEBUG
	case "info":
		level = logging.INFO
	case "notice":
		level = logging.NOTICE
	case "warning":
		level = logging.WARNING
	case "error":
		level = logging.ERROR
	case "critical":
		level = logging.CRITICAL
	default:
		level = logging.DEBUG
	}
	logging.SetLevel(level, "")

	err = core.CheckAndSetUlimit()
	if err != nil {
		return err
	}

	// If the database cannot be decrypted, exit
	if sqliteDB.Config().IsEncrypted() {
		sqliteDB.Close()
		fmt.Print("Database is encrypted, enter your password: ")
		bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
		fmt.Println("")
		pw := string(bytePassword)
		sqliteDB, err = InitializeRepo(repoPath, pw, "", isTestnet, time.Now())
		if err != nil && err != repo.ErrRepoExists {
			return err
		}
		if sqliteDB.Config().IsEncrypted() {
			log.Error("Invalid password")
			os.Exit(3)
		}
	}

	// Get creation date. Ignore the error and use a default timestamp.
	creationDate, _ := sqliteDB.Config().GetCreationDate()

	// Create user-agent file
	userAgentBytes := []byte(core.USERAGENT + x.UserAgent)
	ioutil.WriteFile(path.Join(repoPath, "root", "user_agent"), userAgentBytes, os.ModePerm)

	// Load config
	configFile, err := ioutil.ReadFile(path.Join(repoPath, "config"))
	if err != nil {
		return err
	}

	apiConfig, err := repo.GetAPIConfig(configFile)
	if err != nil {
		log.Error(err)
		return err
	}
	torConfig, err := repo.GetTorConfig(configFile)
	if err != nil {
		log.Error(err)
		return err
	}
	walletCfg, err := repo.GetWalletConfig(configFile)
	if err != nil {
		log.Error(err)
		return err
	}
	dataSharing, err := repo.GetDataSharing(configFile)
	if err != nil {
		log.Error(err)
		return err
	}
	dropboxToken, err := repo.GetDropboxApiToken(configFile)
	if err != nil {
		log.Error(err)
		return err
	}
	resolverConfig, err := repo.GetResolverConfig(configFile)
	if err != nil {
		log.Error(err)
		return err
	}
	republishInterval, err := repo.GetRepublishInterval(configFile)
	if err != nil {
		log.Error(err)
		return err
	}

	// IPFS node setup
	r, err := fsrepo.Open(repoPath)
	if err != nil {
		log.Error(err)
		return err
	}
	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := r.Config()
	if err != nil {
		log.Error(err)
		return err
	}

	identityKey, err := sqliteDB.Config().GetIdentityKey()
	if err != nil {
		log.Error(err)
		return err
	}
	identity, err := ipfs.IdentityFromKey(identityKey)
	if err != nil {
		return err
	}
	cfg.Identity = identity

	// Setup testnet
	if x.Testnet || x.Regtest {
		testnetBootstrapAddrs, err := repo.GetTestnetBootstrapAddrs(configFile)
		if err != nil {
			log.Error(err)
			return err
		}
		cfg.Bootstrap = testnetBootstrapAddrs
		dht.ProtocolDHT = "/openbazaar/kad/testnet/1.0.0"
		bitswap.ProtocolBitswap = "/openbazaar/bitswap/testnet/1.1.0"
		service.ProtocolOpenBazaar = "/openbazaar/app/testnet/1.0.0"

		dataSharing.PushTo = []string{}
	}

	onionAddr, err := obnet.MaybeCreateHiddenServiceKey(repoPath)
	if err != nil {
		log.Error(err)
		return err
	}
	onionAddrString := "/onion/" + onionAddr + ":4003"
	if x.Tor {
		cfg.Addresses.Swarm = []string{}
		cfg.Addresses.Swarm = append(cfg.Addresses.Swarm, onionAddrString)
	} else if x.DualStack {
		cfg.Addresses.Swarm = []string{}
		cfg.Addresses.Swarm = append(cfg.Addresses.Swarm, onionAddrString)
		cfg.Addresses.Swarm = append(cfg.Addresses.Swarm, "/ip4/0.0.0.0/tcp/4001")
		cfg.Addresses.Swarm = append(cfg.Addresses.Swarm, "/ip6/::/tcp/4001")
		cfg.Addresses.Swarm = append(cfg.Addresses.Swarm, "/ip6/::/tcp/9005/ws")
		cfg.Addresses.Swarm = append(cfg.Addresses.Swarm, "/ip4/0.0.0.0/tcp/9005/ws")
	}
	// Iterate over our address and process them as needed
	var onionTransport *oniontp.OnionTransport
	var torDialer proxy.Dialer
	var usingTor, usingClearnet bool
	var controlPort int
	for i, addr := range cfg.Addresses.Swarm {
		m, err := ma.NewMultiaddr(addr)
		if err != nil {
			log.Error(err)
			return err
		}
		p := m.Protocols()
		// If we are using UTP and the stun option has been select, run stun and replace the port in the address
		if x.STUN && p[0].Name == "ip4" && p[1].Name == "udp" && p[2].Name == "utp" {
			usingClearnet = true
			port, serr := obnet.Stun()
			if serr != nil {
				log.Error(serr)
				return err
			}
			cfg.Addresses.Swarm = append(cfg.Addresses.Swarm[:i], cfg.Addresses.Swarm[i+1:]...)
			cfg.Addresses.Swarm = append(cfg.Addresses.Swarm, "/ip4/0.0.0.0/udp/"+strconv.Itoa(port)+"/utp")
			break
		} else if p[0].Name == "onion" {
			usingTor = true
			addrutil.SupportedTransportStrings = append(addrutil.SupportedTransportStrings, "/onion")
			t, err := ma.ProtocolsWithString("/onion")
			if err != nil {
				log.Error(err)
				return err
			}
			addrutil.SupportedTransportProtocols = append(addrutil.SupportedTransportProtocols, t)
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			usingClearnet = true
		}
	}
	// Create Tor transport
	if usingTor {
		torControl := torConfig.TorControl
		if torControl == "" {
			controlPort, err = obnet.GetTorControlPort()
			if err != nil {
				log.Error(err)
				return err
			}
			torControl = "127.0.0.1:" + strconv.Itoa(controlPort)
		}
		torPw := torConfig.Password
		if x.TorPassword != "" {
			torPw = x.TorPassword
		}
		auth := &proxy.Auth{Password: torPw}
		onionTransport, err = oniontp.NewOnionTransport("tcp4", torControl, auth, repoPath, (usingTor && usingClearnet))
		if err != nil {
			log.Error(err)
			return err
		}
	}
	// If we're only using Tor set the proxy dialer and dns resolver
	dnsResolver := namesys.NewDNSResolver()
	if usingTor && !usingClearnet {
		log.Notice("Using Tor exclusively")
		torDialer, err = onionTransport.TorDialer()
		if err != nil {
			log.Error(err)
			return err
		}
		// TODO: maybe create a tor resolver impl later
		dnsResolver = nil
	}

	// Custom host option used if Tor is enabled
	defaultHostOption := func(ctx context.Context, id peer.ID, ps pstore.Peerstore, bwr metrics.Reporter, fs []*net.IPNet, tpt smux.Transport, protec ipnet.Protector, opts *ipfscore.ConstructPeerHostOpts) (p2phost.Host, error) {
		// no addresses to begin with. we'll start later.
		swrm, err := swarm.NewSwarmWithProtector(ctx, nil, id, ps, protec, tpt, bwr)
		if err != nil {
			return nil, err
		}

		network := (*swarm.Network)(swrm)
		network.Swarm().AddTransport(onionTransport)

		for _, f := range fs {
			network.Swarm().Filters.AddDialFilter(f)
		}

		var host *p2pbhost.BasicHost
		if usingTor && !usingClearnet {
			host = p2pbhost.New(network)
		} else {
			hostOpts := []interface{}{bwr}
			if !opts.DisableNatPortMap {
				hostOpts = append(hostOpts, p2pbhost.NATPortMap)
			}
			host = p2pbhost.New(network, hostOpts...)
		}
		return host, nil
	}

	ncfg := &ipfscore.BuildCfg{
		Repo:   r,
		Online: true,
		ExtraOpts: map[string]bool{
			"mplex": true,
		},
		DNSResolver: dnsResolver,
		Routing:     DHTOption,
	}

	if onionTransport != nil {
		ncfg.Host = defaultHostOption
	}
	nd, err := ipfscore.NewNode(cctx, ncfg)
	if err != nil {
		log.Error(err)
		return err
	}

	ctx := commands.Context{}
	ctx.Online = true
	ctx.ConfigRoot = repoPath
	ctx.LoadConfig = func(path string) (*config.Config, error) {
		return fsrepo.ConfigAt(repoPath)
	}
	ctx.ConstructNode = func() (*ipfscore.IpfsNode, error) {
		return nd, nil
	}

	// Set IPNS query size
	querySize := cfg.Ipns.QuerySize
	if querySize <= 20 && querySize > 0 {
		dhtutil.QuerySize = int(querySize)
	} else {
		dhtutil.QuerySize = 16
	}
	namesys.UsePersistentCache = cfg.Ipns.UsePersistentCache

	log.Info("Peer ID: ", nd.Identity.Pretty())
	printSwarmAddrs(nd)

	// Get current directory root hash
	_, ipnskey := namesys.IpnsKeysForID(nd.Identity)
	ival, hasherr := nd.Repo.Datastore().Get(dshelp.NewKeyFromBinary([]byte(ipnskey)))
	if hasherr != nil {
		log.Error(hasherr)
		return hasherr
	}
	val := ival.([]byte)
	dhtrec := new(recpb.Record)
	proto.Unmarshal(val, dhtrec)
	e := new(namepb.IpnsEntry)
	proto.Unmarshal(dhtrec.GetValue(), e)

	// Wallet
	mn, err := sqliteDB.Config().GetMnemonic()
	if err != nil {
		log.Error(err)
		return err
	}
	var params chaincfg.Params
	if x.Testnet {
		params = chaincfg.TestNet3Params
	} else if x.Regtest {
		params = chaincfg.RegressionNetParams
	} else {
		params = chaincfg.MainNetParams
	}
	if x.Regtest && (strings.ToLower(walletCfg.Type) == "spvwallet" || strings.ToLower(walletCfg.Type) == "bitcoincash") && walletCfg.TrustedPeer == "" {
		return errors.New("Trusted peer must be set if using regtest with the spvwallet")
	}

	// Wallet setup
	if x.BitcoinCash {
		walletCfg.Type = "bitcoincash"
	} else if x.ZCash != "" {
		walletCfg.Type = "zcashd"
		walletCfg.Binary = x.ZCash
	}
	var exchangeRates bitcoin.ExchangeRates
	if !x.DisableExchangeRates {
		exchangeRates = exchange.NewBitcoinPriceFetcher(torDialer)
	}
	var w3 io.Writer
	if x.NoLogFiles {
		w3 = &DummyWriter{}
	} else {
		w3 = &lumberjack.Logger{
			Filename:   path.Join(repoPath, "logs", "bitcoin.log"),
			MaxSize:    10, // Megabytes
			MaxBackups: 3,
			MaxAge:     30, // Days
		}
	}
	bitcoinFile := logging.NewLogBackend(w3, "", 0)
	bitcoinFileFormatter := logging.NewBackendFormatter(bitcoinFile, fileLogFormat)
	ml := logging.MultiLogger(bitcoinFileFormatter)

	var resyncManager *resync.ResyncManager
	var cryptoWallet wallet.Wallet
	var walletTypeStr string
	switch strings.ToLower(walletCfg.Type) {
	case "spvwallet":
		walletTypeStr = "bitcoin spv"
		var tp net.Addr
		if walletCfg.TrustedPeer != "" {
			tp, err = net.ResolveTCPAddr("tcp", walletCfg.TrustedPeer)
			if err != nil {
				log.Error(err)
				return err
			}
		}
		feeApi, err := url.Parse(walletCfg.FeeAPI)
		if err != nil {
			log.Error(err)
			return err
		}
		spvwalletConfig := &spvwallet.Config{
			Mnemonic:     mn,
			Params:       &params,
			MaxFee:       uint64(walletCfg.MaxFee),
			LowFee:       uint64(walletCfg.LowFeeDefault),
			MediumFee:    uint64(walletCfg.MediumFeeDefault),
			HighFee:      uint64(walletCfg.HighFeeDefault),
			FeeAPI:       *feeApi,
			RepoPath:     repoPath,
			CreationDate: creationDate,
			DB:           sqliteDB,
			UserAgent:    "OpenBazaar",
			TrustedPeer:  tp,
			Proxy:        torDialer,
			Logger:       ml,
		}
		cryptoWallet, err = spvwallet.NewSPVWallet(spvwalletConfig)
		if err != nil {
			log.Error(err)
			return err
		}
		resyncManager = resync.NewResyncManager(sqliteDB.Sales(), cryptoWallet)
	case "bitcoincash":
		walletTypeStr = "bitcoin cash spv"
		var tp net.Addr
		if walletCfg.TrustedPeer != "" {
			tp, err = net.ResolveTCPAddr("tcp", walletCfg.TrustedPeer)
			if err != nil {
				log.Error(err)
				return err
			}
		}
		feeApi, err := url.Parse(walletCfg.FeeAPI)
		if err != nil {
			log.Error(err)
			return err
		}
		exchangeRates = cashrates.NewBitcoinCashPriceFetcher(torDialer)
		spvwalletConfig := &bitcoincash.Config{
			Mnemonic:             mn,
			Params:               &params,
			MaxFee:               uint64(walletCfg.MaxFee),
			LowFee:               uint64(walletCfg.LowFeeDefault),
			MediumFee:            uint64(walletCfg.MediumFeeDefault),
			HighFee:              uint64(walletCfg.HighFeeDefault),
			FeeAPI:               *feeApi,
			RepoPath:             repoPath,
			CreationDate:         creationDate,
			DB:                   sqliteDB,
			UserAgent:            "OpenBazaar",
			TrustedPeer:          tp,
			Proxy:                torDialer,
			Logger:               ml,
			ExchangeRateProvider: exchangeRates,
		}
		cryptoWallet, err = bitcoincash.NewSPVWallet(spvwalletConfig)
		if err != nil {
			log.Error(err)
			return err
		}
		resyncManager = resync.NewResyncManager(sqliteDB.Sales(), cryptoWallet)
	case "bitcoind":
		walletTypeStr = "bitcoind"
		if walletCfg.Binary == "" {
			return errors.New("The path to the bitcoind binary must be specified in the config file when using bitcoind")
		}
		usetor := false
		if usingTor && !usingClearnet {
			usetor = true
		}
		cryptoWallet, err = bitcoind.NewBitcoindWallet(mn, &params, repoPath, walletCfg.TrustedPeer, walletCfg.Binary, usetor, controlPort)
		if err != nil {
			return err
		}
	case "zcashd":
		walletTypeStr = "zcashd"
		if walletCfg.Binary == "" {
			return errors.New("The path to the zcashd binary must be specified in the config file when using zcashd")
		}
		usetor := false
		if usingTor && !usingClearnet {
			usetor = true
		}
		cryptoWallet, err = zcashd.NewZcashdWallet(mn, &params, repoPath, walletCfg.TrustedPeer, walletCfg.Binary, usetor, controlPort)
		if err != nil {
			return err
		}
		if !x.DisableExchangeRates {
			exchangeRates = zcashd.NewZcashPriceFetcher(torDialer)
		}
		resyncManager = resync.NewResyncManager(sqliteDB.Sales(), cryptoWallet)
	default:
		log.Fatal("Unknown wallet type")
	}

	// Push nodes
	var pushNodes []peer.ID
	for _, pnd := range dataSharing.PushTo {
		p, err := peer.IDB58Decode(pnd)
		if err != nil {
			log.Error("Invalid peerID in DataSharing config")
			return err
		}
		pushNodes = append(pushNodes, p)
	}

	// Authenticated gateway
	gatewayMaddr, err := ma.NewMultiaddr(cfg.Addresses.Gateway)
	if err != nil {
		log.Error(err)
		return err
	}
	addr, err := gatewayMaddr.ValueForProtocol(ma.P_IP4)
	if err != nil {
		log.Error(err)
		return err
	}
	// Override config file preference if this is Mainnet, open internet and API enabled
	if addr != "127.0.0.1" && cryptoWallet.Params().Name == chaincfg.MainNetParams.Name && apiConfig.Enabled {
		apiConfig.Authenticated = true
	}
	for _, ip := range x.AllowIP {
		apiConfig.AllowedIPs = append(apiConfig.AllowedIPs, ip)
	}

	// Create authentication cookie
	var authCookie http.Cookie
	authCookie.Name = "OpenBazaar_Auth_Cookie"

	if x.AuthCookie != "" {
		authCookie.Value = x.AuthCookie
		apiConfig.Authenticated = true
	} else {
		cookiePrefix := authCookie.Name + "="
		cookiePath := path.Join(repoPath, ".cookie")
		cookie, err := ioutil.ReadFile(cookiePath)
		if err != nil {
			authBytes := make([]byte, 32)
			rand.Read(authBytes)
			authCookie.Value = base58.Encode(authBytes)
			f, err := os.Create(cookiePath)
			if err != nil {
				log.Error(err)
				return err
			}
			cookie := cookiePrefix + authCookie.Value
			_, werr := f.Write([]byte(cookie))
			if werr != nil {
				log.Error(werr)
				return werr
			}
			f.Close()
		} else {
			if string(cookie)[:len(cookiePrefix)] != cookiePrefix {
				return errors.New("Invalid authentication cookie. Delete it to generate a new one.")
			}
			split := strings.SplitAfter(string(cookie), cookiePrefix)
			authCookie.Value = split[1]
		}
	}

	// Set up the ban manager
	settings, err := sqliteDB.Settings().Get()
	if err != nil && err != db.SettingsNotSetError {
		log.Error(err)
		return err
	}
	var blockedNodes []peer.ID
	if settings.BlockedNodes != nil {
		for _, pid := range *settings.BlockedNodes {
			id, err := peer.IDB58Decode(pid)
			if err != nil {
				continue
			}
			blockedNodes = append(blockedNodes, id)
		}
	}
	bm := obnet.NewBanManager(blockedNodes)

	// Create namesys resolvers
	resolvers := []obns.Resolver{
		bstk.NewBlockStackClient(resolverConfig.Id, torDialer),
	}
	if !(usingTor && !usingClearnet) {
		resolvers = append(resolvers, obns.NewDNSResolver())
	}
	ns, err := obns.NewNameSystem(resolvers)
	if err != nil {
		log.Error(err)
		return err
	}

	// OpenBazaar node setup
	core.Node = &core.OpenBazaarNode{
		Context:              ctx,
		IpfsNode:             nd,
		RootHash:             ipath.Path(e.Value).String(),
		RepoPath:             repoPath,
		Datastore:            sqliteDB,
		Wallet:               cryptoWallet,
		NameSystem:           ns,
		ExchangeRates:        exchangeRates,
		PushNodes:            pushNodes,
		AcceptStoreRequests:  dataSharing.AcceptStoreRequests,
		TorDialer:            torDialer,
		UserAgent:            core.USERAGENT,
		BanManager:           bm,
		IPNSBackupAPI:        cfg.Ipns.BackUpAPI,
		TestnetEnable:        x.Testnet,
		RegressionTestEnable: x.Regtest,
	}
	core.PublishLock.Lock()

	// Offline messaging storage
	var storage sto.OfflineMessagingStorage
	if x.Storage == "self-hosted" || x.Storage == "" {
		storage = selfhosted.NewSelfHostedStorage(repoPath, ctx, pushNodes, core.Node.SendStore)
	} else if x.Storage == "dropbox" {
		if usingTor && !usingClearnet {
			log.Error("Dropbox can not be used with Tor")
			return errors.New("Dropbox can not be used with Tor")
		}

		if dropboxToken == "" {
			err = errors.New("Dropbox token not set in config file")
			log.Error(err)
			return err
		}
		storage, err = dropbox.NewDropBoxStorage(dropboxToken)
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		err = errors.New("Invalid storage option")
		log.Error(err)
		return err
	}
	core.Node.MessageStorage = storage

	if len(cfg.Addresses.Gateway) <= 0 {
		return ErrNoGateways
	}
	if (apiConfig.SSL && apiConfig.SSLCert == "") || (apiConfig.SSL && apiConfig.SSLKey == "") {
		return errors.New("SSL cert and key files must be set when SSL is enabled")
	}

	gateway, err := newHTTPGateway(core.Node, authCookie, *apiConfig, x.NoLogFiles)
	if err != nil {
		log.Error(err)
		return err
	}

	if cfg.Addresses.API != "" {
		if _, err := serveHTTPApi(&core.Node.Context); err != nil {
			log.Error(err)
			return err
		}
	}

	go func() {
		<-dht.DefaultBootstrapConfig.DoneChan
		core.Node.Service = service.New(core.Node, ctx, sqliteDB)

		core.Node.StartMessageRetriever()
		core.Node.StartPointerRepublisher()
		core.Node.StartDisputeNotifier()

		if !x.DisableWallet {
			// If the wallet doesn't allow resyncing from a specific height to scan for unpaid orders, wait for all messages to process before continuing.
			if resyncManager == nil {
				core.Node.WaitForMessageRetrieverCompletion()
			}
			TL := lis.NewTransactionListener(core.Node.Datastore, core.Node.Broadcast, core.Node.Wallet)
			WL := lis.NewWalletListener(core.Node.Datastore, core.Node.Broadcast)
			cryptoWallet.AddTransactionListener(TL.OnTransactionReceived)
			cryptoWallet.AddTransactionListener(WL.OnTransactionReceived)
			log.Infof("Starting %s wallet\n", walletTypeStr)
			su := bitcoin.NewStatusUpdater(cryptoWallet, core.Node.Broadcast, nd.Context())
			go su.Start()
			go cryptoWallet.Start()
			if resyncManager != nil {
				go resyncManager.Start()
				go func() {
					core.Node.WaitForMessageRetrieverCompletion()
					resyncManager.CheckUnfunded()
				}()
			}
		}
		core.PublishLock.Unlock()
		core.Node.UpdateFollow()
		if !core.InitalPublishComplete {
			core.Node.SeedNode()
		}
		core.Node.SetUpRepublisher(republishInterval)
	}()

	// Start gateway
	err = gateway.Serve()
	if err != nil {
		log.Error(err)
	}

	return nil
}

// Prints the addresses of the host
func printSwarmAddrs(node *ipfscore.IpfsNode) {
	var addrs []string
	for _, addr := range node.PeerHost.Addrs() {
		addrs = append(addrs, addr.String())
	}
	sort.Sort(sort.StringSlice(addrs))

	for _, addr := range addrs {
		log.Infof("Swarm listening on %s\n", addr)
	}
}

type DummyWriter struct{}

func (d *DummyWriter) Write(p []byte) (n int, err error) {
	return 0, nil
}

type DummyListener struct {
	addr net.Addr
}

func (d *DummyListener) Addr() net.Addr {
	return d.addr
}

func (d *DummyListener) Accept() (net.Conn, error) {
	conn, _ := net.FileConn(nil)
	return conn, nil
}

func (d *DummyListener) Close() error {
	return nil
}

// Collects options, creates listener, prints status message and starts serving requests
func newHTTPGateway(node *core.OpenBazaarNode, authCookie http.Cookie, config repo.APIConfig, noLogFiles bool) (*api.Gateway, error) {
	// Get API configuration
	cfg, err := node.Context.GetConfig()
	if err != nil {
		return nil, err
	}

	// Create a network listener
	gatewayMaddr, err := ma.NewMultiaddr(cfg.Addresses.Gateway)
	if err != nil {
		return nil, fmt.Errorf("newHTTPGateway: invalid gateway address: %q (err: %s)", cfg.Addresses.Gateway, err)
	}
	var gwLis manet.Listener
	if config.SSL {
		netAddr, err := manet.ToNetAddr(gatewayMaddr)
		if err != nil {
			return nil, err
		}
		gwLis, err = manet.WrapNetListener(&DummyListener{netAddr})
		if err != nil {
			return nil, err
		}
	} else {
		gwLis, err = manet.Listen(gatewayMaddr)
		if err != nil {
			return nil, fmt.Errorf("newHTTPGateway: manet.Listen(%s) failed: %s", gatewayMaddr, err)
		}
	}

	// We might have listened to /tcp/0 - let's see what we are listing on
	gatewayMaddr = gwLis.Multiaddr()
	log.Infof("Gateway/API server listening on %s\n", gatewayMaddr)

	// Setup an options slice
	var opts = []corehttp.ServeOption{
		corehttp.MetricsCollectionOption("gateway"),
		corehttp.CommandsROOption(node.Context),
		corehttp.VersionOption(),
		corehttp.IPNSHostnameOption(),
		corehttp.GatewayOption(cfg.Gateway.Writable, "/ipfs", "/ipns"),
	}

	if len(cfg.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
	}

	if err != nil {
		return nil, fmt.Errorf("newHTTPGateway: ConstructNode() failed: %s", err)
	}

	// Create and return an API gateway
	var w4 io.Writer
	if noLogFiles {
		w4 = &DummyWriter{}
	} else {
		w4 = &lumberjack.Logger{
			Filename:   path.Join(node.RepoPath, "logs", "api.log"),
			MaxSize:    10, // Megabytes
			MaxBackups: 3,
			MaxAge:     30, // Days
		}
	}
	apiFile := logging.NewLogBackend(w4, "", 0)
	apiFileFormatter := logging.NewBackendFormatter(apiFile, fileLogFormat)
	ml := logging.MultiLogger(apiFileFormatter)

	return api.NewGateway(node, authCookie, gwLis.NetListener(), config, ml, opts...)
}

var DHTOption ipfscore.RoutingOption = constructDHTRouting

func constructDHTRouting(ctx context.Context, host p2phost.Host, dstore ipfsrepo.Datastore) (routing.IpfsRouting, error) {
	dhtRouting := dht.NewDHT(ctx, host, dstore)
	dhtRouting.Validator[ipfscore.IpnsValidatorTag] = namesys.IpnsRecordValidator
	dhtRouting.Selector[ipfscore.IpnsValidatorTag] = namesys.IpnsSelectorFunc
	return dhtRouting, nil
}

// serveHTTPApi collects options, creates listener, prints status message and starts serving requests
func serveHTTPApi(cctx *commands.Context) (<-chan error, error) {
	cfg, err := cctx.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("serveHTTPApi: GetConfig() failed: %s", err)
	}

	apiAddr := cfg.Addresses.API
	apiMaddr, err := ma.NewMultiaddr(apiAddr)
	if err != nil {
		return nil, fmt.Errorf("serveHTTPApi: invalid API address: %q (err: %s)", apiAddr, err)
	}

	apiLis, err := manet.Listen(apiMaddr)
	if err != nil {
		return nil, fmt.Errorf("serveHTTPApi: manet.Listen(%s) failed: %s", apiMaddr, err)
	}
	// we might have listened to /tcp/0 - lets see what we are listing on
	apiMaddr = apiLis.Multiaddr()
	fmt.Printf("API server listening on %s\n", apiMaddr)

	// by default, we don't let you load arbitrary ipfs objects through the api,
	// because this would open up the api to scripting vulnerabilities.
	// only the webui objects are allowed.
	// if you know what you're doing, go ahead and pass --unrestricted-api.
	unrestricted := false
	gatewayOpt := corehttp.GatewayOption(false, corehttp.WebUIPaths...)
	if unrestricted {
		gatewayOpt = corehttp.GatewayOption(true, "/ipfs", "/ipns")
	}

	var opts = []corehttp.ServeOption{
		corehttp.MetricsCollectionOption("api"),
		corehttp.CommandsOption(*cctx),
		corehttp.WebUIOption,
		gatewayOpt,
		corehttp.VersionOption(),
		corehttp.MetricsScrapingOption("/debug/metrics/prometheus"),
		corehttp.LogOption(),
	}

	if len(cfg.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
	}

	node, err := cctx.ConstructNode()
	if err != nil {
		return nil, fmt.Errorf("serveHTTPApi: ConstructNode() failed: %s", err)
	}

	if err := node.Repo.SetAPIAddr(apiMaddr); err != nil {
		return nil, fmt.Errorf("serveHTTPApi: SetAPIAddr() failed: %s", err)
	}

	errc := make(chan error)
	go func() {
		errc <- corehttp.Serve(node, apiLis.NetListener(), opts...)
		close(errc)
	}()
	return errc, nil
}

func InitializeRepo(dataDir, password, mnemonic string, testnet bool, creationDate time.Time) (*db.SQLiteDatastore, error) {
	// Database
	sqliteDB, err := db.Create(dataDir, password, testnet)
	if err != nil {
		return sqliteDB, err
	}

	// Initialize the IPFS repo if it does not already exist
	err = repo.DoInit(dataDir, 4096, testnet, password, mnemonic, creationDate, sqliteDB.Config().Init)
	if err != nil {
		return sqliteDB, err
	}
	return sqliteDB, nil
}

func printSplashScreen(verbose bool) {
	blue := color.New(color.FgBlue)
	white := color.New(color.FgWhite)
	white.Printf("________             ")
	blue.Println("         __________")
	white.Printf(`\_____  \ ______   ____   ____`)
	blue.Println(`\______   \_____  _____________  _____ _______`)
	white.Printf(` /   |   \\____ \_/ __ \ /    \`)
	blue.Println(`|    |  _/\__  \ \___   /\__  \ \__  \\_  __ \ `)
	white.Printf(`/    |    \  |_> >  ___/|   |  \    `)
	blue.Println(`|   \ / __ \_/    /  / __ \_/ __ \|  | \/`)
	white.Printf(`\_______  /   __/ \___  >___|  /`)
	blue.Println(`______  /(____  /_____ \(____  (____  /__|`)
	white.Printf(`        \/|__|        \/     \/  `)
	blue.Println(`     \/      \/      \/     \/     \/`)
	blue.DisableColor()
	white.DisableColor()
	fmt.Println("")
	fmt.Println("OpenBazaar Server v" + core.VERSION)
	if !verbose {
		fmt.Println("[Press Ctrl+C to exit]")
	}
}
