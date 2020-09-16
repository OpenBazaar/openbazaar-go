package cmd

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	routinghelpers "gx/ipfs/QmRCrPXk2oUwpK1Cj2FXrUotRpddUxz56setkny2gz13Cx/go-libp2p-routing-helpers"
	libp2p "gx/ipfs/QmRxk6AUaGaKCfzS1xSNRojiAPd7h2ih8GuCdjJBF3Y6GK/go-libp2p"
	dht "gx/ipfs/QmSY3nkMNLzh9GdbFKK5tT7YMfLpf52iUZ8ZRkr29MJaa5/go-libp2p-kad-dht"
	ma "gx/ipfs/QmTZBfrPJmjWsCvHEtX5FE6KimVJhsJg5sBbqEFYf4UZtL/go-multiaddr"
	config "gx/ipfs/QmUAuYuiafnJRZxDDX7MuruMNsicYNuyub5vUeAcupUBNs/go-ipfs-config"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	oniontp "gx/ipfs/QmYv2MbwHn7qcvAPFisZ94w85crQVpwUuv8G7TuUeBnfPb/go-onion-transport"
	ipfslogging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log/writer"
	manet "gx/ipfs/Qmc85NSvmSG4Frn9Vb2cBc1rMyULH6D3TNVEfCzSKoUpip/go-multiaddr-net"

	"github.com/OpenBazaar/openbazaar-go/api"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	obnet "github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/net/service"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
	sto "github.com/OpenBazaar/openbazaar-go/storage"
	"github.com/OpenBazaar/openbazaar-go/storage/dropbox"
	"github.com/OpenBazaar/openbazaar-go/storage/selfhosted"
	"github.com/OpenBazaar/openbazaar-go/wallet"
	lis "github.com/OpenBazaar/openbazaar-go/wallet/listeners"
	"github.com/OpenBazaar/openbazaar-go/wallet/resync"
	wi "github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/base58"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/fatih/color"
	"github.com/ipfs/go-ipfs/commands"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/natefinch/lumberjack"
	"github.com/op/go-logging"
	"github.com/tyler-smith/go-bip39"

	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/proxy"
)

var stdoutLogFormat = logging.MustStringFormatter(
	`%{color:reset}%{color}%{time:2006-01-02 15:04:05.000} [%{level}] [%{module}/%{shortfunc}] %{message}`,
)

var fileLogFormat = logging.MustStringFormatter(
	`%{time:2006-01-02 15:04:05.000} [%{level}] [%{module}/%{shortfunc}] %{message}`,
)

var ErrNoGateways = errors.New("no gateway addresses configured")

type Start struct {
	Password             string   `short:"p" long:"password" description:"the encryption password if the database is encrypted"`
	Testnet              bool     `short:"t" long:"testnet" description:"use the test network"`
	Regtest              bool     `short:"r" long:"regtest" description:"run in regression test mode"`
	LogLevel             string   `short:"l" long:"loglevel" description:"set the logging level [debug, info, notice, warning, error, critical]" default:"debug"`
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
	InfuraKey            string   `long:"infurakey" description:"if you want to use the ethereum wallet you will need to enter your ethereum infura key. This can acquire for free from infura."`
	ForceKeyCachePurge bool `long:"forcekeypurge" description:"repair test for issue OpenBazaar/openbazaar-go#1593; use as instructed only"`
}

func (x *Start) Execute(args []string) error {
	printSplashScreen(x.Verbose)
	ipfs.UpdateIPFSGlobalProtocolVars(x.Testnet || x.Regtest)

	if x.Testnet && x.Regtest {
		return errors.New("invalid combination of testnet and regtest modes")
	}

	if x.Tor && x.DualStack {
		return errors.New("invalid combination of tor and dual stack modes")
	}

	isTestnet := false
	if x.Testnet || x.Regtest {
		isTestnet = true
	}

	// Set repo path
	repoPath, err := repo.GetRepoPath(isTestnet, x.DataDir)
	if err != nil {
		return err
	}
	if x.DataDir != "" {
		repoPath = x.DataDir
	}

	repoLockFile := filepath.Join(repoPath, fsrepo.LockFile)
	os.Remove(repoLockFile)

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

	sqliteDB, err := InitializeRepo(repoPath, x.Password, "", isTestnet, time.Now(), wi.Bitcoin)
	if err != nil && err != repo.ErrRepoExists {
		log.Error("repo init:", err)
		return err
	}

	// Create user-agent file
	userAgentBytes := []byte(core.USERAGENT + x.UserAgent)
	err = ioutil.WriteFile(path.Join(repoPath, "root", "user_agent"), userAgentBytes, os.FileMode(0644))
	if err != nil {
		log.Error("write user_agent:", err)
		return err
	}

	// Get creation date. Ignore the error and use a default timestamp.
	creationDate, err := sqliteDB.Config().GetCreationDate()
	if err != nil {
		log.Error("error loading wallet creation date from database - using unix epoch.")
	}

	// Load config
	configFile, err := ioutil.ReadFile(path.Join(repoPath, "config"))
	if err != nil {
		log.Error("read config:", err)
		return err
	}

	apiConfig, err := schema.GetAPIConfig(configFile)
	if err != nil {
		log.Error("scan api config:", err)
		return err
	}
	torConfig, err := schema.GetTorConfig(configFile)
	if err != nil {
		log.Error("scan tor config:", err)
		return err
	}
	dataSharing, err := schema.GetDataSharing(configFile)
	if err != nil {
		log.Error("scan data sharing config:", err)
		return err
	}
	dropboxToken, err := schema.GetDropboxApiToken(configFile)
	if err != nil {
		log.Error("scan dropbox api token:", err)
		return err
	}
	republishInterval, err := schema.GetRepublishInterval(configFile)
	if err != nil {
		log.Error("scan republish interval config:", err)
		return err
	}
	walletsConfig, err := schema.GetWalletsConfig(configFile)
	if err != nil {
		log.Error("scan wallets config:", err)
		return err
	}
	ipnsExtraConfig, err := schema.GetIPNSExtraConfig(configFile)
	if err != nil {
		log.Error("scan ipns extra config:", err)
		return err
	}

	// IPFS node setup
	r, err := fsrepo.Open(repoPath)
	if err != nil {
		log.Error("open repo:", err)
		return err
	}
	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := r.Config()
	if err != nil {
		log.Error("get repo config:", err)
		return err
	}

	identityKey, err := sqliteDB.Config().GetIdentityKey()
	if err != nil {
		log.Error("get identity key:", err)
		return err
	}
	identity, err := ipfs.IdentityFromKey(identityKey)
	if err != nil {
		log.Error("get identity from key:", err)
		return err
	}
	cfg.Identity = identity

	// Setup testnet
	if x.Testnet || x.Regtest {
		// set testnet bootstrap addrs
		testnetBootstrapAddrs, err := schema.GetTestnetBootstrapAddrs(configFile)
		if err != nil {
			log.Error(err)
			return err
		}
		cfg.Bootstrap = testnetBootstrapAddrs

		// don't use pushnodes on testnet
		dataSharing.PushTo = []string{}
	}

	onionAddr, err := obnet.MaybeCreateHiddenServiceKey(repoPath)
	if err != nil {
		log.Error("create onion key:", err)
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
	var (
		torDialer               proxy.Dialer
		usingTor, usingClearnet bool
		controlPort             int
		torControl              = torConfig.TorControl
		torPw                   = torConfig.Password
	)
	for i, addr := range cfg.Addresses.Swarm {
		m, err := ma.NewMultiaddr(addr)
		if err != nil {
			log.Error("creating swarm multihash:", err)
			return err
		}
		p := m.Protocols()
		// If we are using UTP and the stun option has been select, run stun and replace the port in the address
		if x.STUN && p[0].Name == "ip4" && p[1].Name == "udp" && p[2].Name == "utp" {
			usingClearnet = true
			port, serr := obnet.Stun()
			if serr != nil {
				log.Error("stun setup:", serr)
				return err
			}
			cfg.Addresses.Swarm = append(cfg.Addresses.Swarm[:i], cfg.Addresses.Swarm[i+1:]...)
			cfg.Addresses.Swarm = append(cfg.Addresses.Swarm, "/ip4/0.0.0.0/udp/"+strconv.Itoa(port)+"/utp")
			break
		} else if p[0].Name == "onion" {
			usingTor = true
		} else {
			usingClearnet = true
		}
	}
	// Create Tor transport
	if usingTor {
		if torControl == "" {
			controlPort, err = obnet.GetTorControlPort()
			if err != nil {
				log.Error("get tor control port:", err)
				return err
			}
			torControl = "127.0.0.1:" + strconv.Itoa(controlPort)
		}
		if x.TorPassword != "" {
			torPw = x.TorPassword
		}
		transportOptions := libp2p.ChainOptions(libp2p.Transport(oniontp.NewOnionTransportC("tcp4", torControl, torPw, nil, repoPath, (usingTor && usingClearnet))))
		if usingClearnet {
			transportOptions = libp2p.ChainOptions(
				transportOptions,
				libp2p.DefaultTransports,
			)
		}
		libp2p.DefaultTransports = transportOptions
	}

	if usingTor && !usingClearnet {
		log.Notice("Using Tor exclusively")
		cfg.Swarm.DisableNatPortMap = true
	}

	ncfg := ipfs.PrepareIPFSConfig(r, ipnsExtraConfig.APIRouter, x.Testnet, x.Regtest)
	nd, err := ipfscore.NewNode(cctx, ncfg)
	if err != nil {
		log.Error("create new ipfs node:", err)
		return err
	}

	ctx := commands.Context{}
	ctx.Online = true
	ctx.ConfigRoot = repoPath
	ctx.LoadConfig = func(_ string) (*config.Config, error) {
		return fsrepo.ConfigAt(repoPath)
	}
	ctx.ConstructNode = func() (*ipfscore.IpfsNode, error) {
		return nd, nil
	}
	torDialer = oniontp.TorDialer

	log.Info("Peer ID: ", nd.Identity.Pretty())
	printSwarmAddrs(nd)

	// Extract the DHT from the tiered routing so it will be more accessible later
	tiered, ok := nd.Routing.(routinghelpers.Tiered)
	if !ok {
		return errors.New("IPFS routing is not a type routinghelpers.Tiered")
	}
	var dhtRouting *dht.IpfsDHT
	for _, router := range tiered.Routers {
		if r, ok := router.(*dht.IpfsDHT); ok {
			dhtRouting = r
		}
	}
	if dhtRouting == nil {
		return errors.New("IPFS DHT routing is not configured")
	}

	// Get current directory root hash
	cachedIPNSRecord, hasherr := ipfs.GetCachedIPNSRecord(nd.Repo.Datastore(), nd.Identity)
	if hasherr != nil {
		log.Warning(err)
		if err := ipfs.DeleteCachedIPNSRecord(nd.Repo.Datastore(), nd.Identity); err != nil {
			log.Errorf("deleting invalid IPNS record: %s", err.Error())
		}
	}

	if x.ForceKeyCachePurge {
		log.Infof("forcing key purge from namesys cache...")
		if err := ipfs.DeleteCachedIPNSRecord(nd.Repo.Datastore(), nd.Identity); err != nil {
			log.Errorf("force-purging IPNS record: %s", err.Error())
		}
	}

	// Wallet
	mn, err := sqliteDB.Config().GetMnemonic()
	if err != nil {
		log.Error("get config mnemonic:", err)
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

	// Multiwallet setup
	var walletLogWriter io.Writer
	if x.NoLogFiles {
		walletLogWriter = &DummyWriter{}
	} else {
		walletLogWriter = &lumberjack.Logger{
			Filename:   path.Join(repoPath, "logs", "wallet.log"),
			MaxSize:    10, // Megabytes
			MaxBackups: 3,
			MaxAge:     30, // Days
		}
	}
	walletLogFile := logging.NewLogBackend(walletLogWriter, "", 0)
	walletFileFormatter := logging.NewBackendFormatter(walletLogFile, fileLogFormat)
	walletLogger := logging.MultiLogger(walletFileFormatter)
	multiwalletConfig := &wallet.WalletConfig{
		ConfigFile:           walletsConfig,
		DB:                   sqliteDB.DB(),
		Params:               &params,
		RepoPath:             repoPath,
		Logger:               walletLogger,
		Proxy:                torDialer,
		WalletCreationDate:   creationDate,
		Mnemonic:             mn,
		DisableExchangeRates: x.DisableExchangeRates,
		InfuraKey:            x.InfuraKey,
	}
	mw, err := wallet.NewMultiWallet(multiwalletConfig)
	if err != nil {
		return err
	}
	resyncManager := resync.NewResyncManager(sqliteDB.Sales(), sqliteDB.Purchases(), mw)

	// Master key setup
	seed := bip39.NewSeed(mn, "")
	mPrivKey, err := hdkeychain.NewMaster(seed, &params)
	if err != nil {
		log.Error(err)
		return err
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
	gatewayMaddr, err := ma.NewMultiaddr(cfg.Addresses.Gateway[0])
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
	if addr != "127.0.0.1" && params.Name == chaincfg.MainNetParams.Name && apiConfig.Enabled {
		apiConfig.Authenticated = true
	}
	apiConfig.AllowedIPs = append(apiConfig.AllowedIPs, x.AllowIP...)

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
			_, err = rand.Read(authBytes)
			if err != nil {
				log.Error(err)
				return err
			}
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
				return errors.New("invalid authentication cookie. Delete it to generate a new one")
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

	if x.Testnet {
		setTestmodeRecordAgingIntervals()
	}

	// Build pubsub
	publisher := ipfs.NewPubsubPublisher(context.Background(), nd.PeerHost, nd.Routing, nd.Repo.Datastore(), nd.PubSub)
	subscriber := ipfs.NewPubsubSubscriber(context.Background(), nd.PeerHost, nd.Routing, nd.Repo.Datastore(), nd.PubSub)
	ps := ipfs.Pubsub{Publisher: publisher, Subscriber: subscriber}

	var rootHash string
	if cachedIPNSRecord != nil {
		rootHash = string(cachedIPNSRecord.Value)
	}

	// OpenBazaar node setup
	core.Node = &core.OpenBazaarNode{
		AcceptStoreRequests:           dataSharing.AcceptStoreRequests,
		BanManager:                    bm,
		Datastore:                     sqliteDB,
		IpfsNode:                      nd,
		DHT:                           dhtRouting,
		MasterPrivateKey:              mPrivKey,
		Multiwallet:                   mw,
		OfflineMessageFailoverTimeout: 30 * time.Second,
		Pubsub:                        ps,
		PushNodes:                     pushNodes,
		RegressionTestEnable:          x.Regtest,
		RepoPath:                      repoPath,
		RootHash:                      rootHash,
		TestnetEnable:                 x.Testnet,
		TorDialer:                     torDialer,
		UserAgent:                     core.USERAGENT,
		IPNSQuorumSize:                uint(ipnsExtraConfig.DHTQuorumSize),
	}
	core.Node.PublishLock.Lock()

	// assert reserve wallet is available on startup for later usage
	_, err = core.Node.ReserveCurrencyConverter()
	if err != nil {
		return fmt.Errorf("verifying reserve currency converter: %s", err.Error())
	}

	// Offline messaging storage
	var storage sto.OfflineMessagingStorage
	if x.Storage == "self-hosted" || x.Storage == "" {
		storage = selfhosted.NewSelfHostedStorage(repoPath, core.Node.IpfsNode, pushNodes, core.Node.SendStore)
	} else if x.Storage == "dropbox" {
		if usingTor && !usingClearnet {
			log.Error("dropbox can not be used with tor")
			return errors.New("dropbox can not be used with tor")
		}

		if dropboxToken == "" {
			err = errors.New("dropbox token not set in config file")
			log.Error(err)
			return err
		}
		storage, err = dropbox.NewDropBoxStorage(dropboxToken)
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		err = errors.New("invalid storage option")
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

	gateway, err := newHTTPGateway(core.Node, ctx, authCookie, *apiConfig, x.NoLogFiles)
	if err != nil {
		log.Error(err)
		return err
	}

	if len(cfg.Addresses.API) > 0 && cfg.Addresses.API[0] != "" {
		if _, err := serveHTTPApi(&ctx); err != nil {
			log.Error(err)
			return err
		}
	}

	go func() {
		if !x.DisableWallet {
			// If the wallet doesn't allow resyncing from a specific height to scan for unpaid orders, wait for all messages to process before continuing.
			if resyncManager == nil {
				core.Node.WaitForMessageRetrieverCompletion()
			}
			TL := lis.NewTransactionListener(core.Node.Multiwallet, core.Node.Datastore, core.Node.Broadcast)
			for ct, wal := range mw {
				WL := lis.NewWalletListener(core.Node.Datastore, core.Node.Broadcast, ct)
				wal.AddTransactionListener(WL.OnTransactionReceived)
				wal.AddTransactionListener(TL.OnTransactionReceived)
			}
			log.Info("Starting multiwallet...")
			su := wallet.NewStatusUpdater(mw, core.Node.Broadcast, nd.Context())
			go su.Start()
			go mw.Start()
			if resyncManager != nil {
				go resyncManager.Start()
				go func() {
					core.Node.WaitForMessageRetrieverCompletion()
					resyncManager.CheckUnfunded()
				}()
			}
		}
		core.Node.Service = service.New(core.Node, sqliteDB)
		core.Node.Service.WaitForReady()
		log.Info("OpenBazaar Service Ready")

		core.Node.StartMessageRetriever()
		core.Node.StartPointerRepublisher()
		core.Node.StartRecordAgingNotifier()
		core.Node.StartInboundMsgScanner()

		if err := core.Node.RemoveDisabledCurrenciesFromListings(); err != nil {
			log.Error(err)
		}

		core.Node.PublishLock.Unlock()
		err = core.Node.UpdateFollow()
		if err != nil {
			log.Error(err)
		}
		if !core.Node.InitalPublishComplete {
			err = core.Node.SeedNode()
			if err != nil {
				log.Error(err)
			}
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

func setTestmodeRecordAgingIntervals() {
	repo.VendorDisputeTimeout_lastInterval = time.Duration(60) * time.Minute

	repo.ModeratorDisputeExpiry_firstInterval = time.Duration(20) * time.Minute
	repo.ModeratorDisputeExpiry_secondInterval = time.Duration(40) * time.Minute
	repo.ModeratorDisputeExpiry_thirdInterval = time.Duration(59) * time.Minute
	repo.ModeratorDisputeExpiry_lastInterval = time.Duration(60) * time.Minute

	repo.BuyerDisputeTimeout_firstInterval = time.Duration(20) * time.Minute
	repo.BuyerDisputeTimeout_secondInterval = time.Duration(40) * time.Minute
	repo.BuyerDisputeTimeout_thirdInterval = time.Duration(59) * time.Minute
	repo.BuyerDisputeTimeout_lastInterval = time.Duration(60) * time.Minute
	repo.BuyerDisputeTimeout_totalDuration = time.Duration(60) * time.Minute

	repo.BuyerDisputeExpiry_firstInterval = time.Duration(20) * time.Minute
	repo.BuyerDisputeExpiry_secondInterval = time.Duration(40) * time.Minute
	repo.BuyerDisputeExpiry_lastInterval = time.Duration(59) * time.Minute
	repo.BuyerDisputeExpiry_totalDuration = time.Duration(60) * time.Minute
}

// Prints the addresses of the host
func printSwarmAddrs(node *ipfscore.IpfsNode) {
	var addrs []string
	for _, addr := range node.PeerHost.Addrs() {
		addrs = append(addrs, addr.String())
	}
	sort.Strings(addrs)

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
func newHTTPGateway(node *core.OpenBazaarNode, ctx commands.Context, authCookie http.Cookie, config schema.APIConfig, noLogFiles bool) (*api.Gateway, error) {
	// Get API configuration
	cfg, err := ctx.GetConfig()
	if err != nil {
		return nil, err
	}

	// Create a network listener
	gatewayMaddr, err := ma.NewMultiaddr(cfg.Addresses.Gateway[0])
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
		corehttp.CommandsROOption(ctx),
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

	return api.NewGateway(node, authCookie, manet.NetListener(gwLis), config, ml, opts...)
}

// serveHTTPApi collects options, creates listener, prints status message and starts serving requests
func serveHTTPApi(cctx *commands.Context) (<-chan error, error) {
	cfg, err := cctx.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("serveHTTPApi: GetConfig() failed: %s", err)
	}

	apiAddr := cfg.Addresses.API[0]
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
		errc <- corehttp.Serve(node, manet.NetListener(apiLis), opts...)
		close(errc)
	}()
	return errc, nil
}

func InitializeRepo(dataDir, password, mnemonic string, testnet bool, creationDate time.Time, coinType wi.CoinType) (*db.SQLiteDatastore, error) {
	// Database
	sqliteDB, err := db.Create(dataDir, password, testnet, coinType)
	if err != nil {
		return sqliteDB, err
	}

	// Initialize the IPFS repo if it does not already exist
	err = repo.DoInit(dataDir, 4096, testnet, password, mnemonic, creationDate, sqliteDB.Config().Init)
	if err != nil {
		// handle encrypted databases
		if sqliteDB.Config().IsEncrypted() {
			sqliteDB.Close()
			fmt.Printf("Database at %s appears encrypted.\nEnter your password (will not appear while typing): ", dataDir)

			// nolint:unconvert
			bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
			fmt.Println("")
			pw := string(bytePassword)

			sqliteDB, err = db.Create(dataDir, pw, testnet, coinType)
			if err != nil {
				return sqliteDB, err
			}

			// try again with the password
			err = repo.DoInit(dataDir, 4096, testnet, pw, mnemonic, creationDate, sqliteDB.Config().Init)
			if err != nil && err != repo.ErrRepoExists {
				log.Errorf("unable to open encrypted database: %s", err.Error())
				return sqliteDB, fmt.Errorf("repo init encrypted: %s", err.Error())
			}
		}
		return sqliteDB, err
	}
	return sqliteDB, nil
}

func printSplashScreen(verbose bool) {
	blue := color.New(color.FgBlue)
	white := color.New(color.FgWhite)

	for i, l := range []string{
		"________             ",
		"         __________",
		`\_____  \ ______   ____   ____`,
		`\______   \_____  _____________  _____ _______`,
		` /   |   \\____ \_/ __ \ /    \`,
		`|    |  _/\__  \ \___   /\__  \ \__  \\_  __ \ `,
		`/    |    \  |_> >  ___/|   |  \    `,
		`|   \ / __ \_/    /  / __ \_/ __ \|  | \/`,
		`\_______  /   __/ \___  >___|  /`,
		`______  /(____  /_____ \(____  (____  /__|`,
		`        \/|__|        \/     \/  `,
		`     \/      \/      \/     \/     \/`,
	} {
		if i%2 == 0 {
			if _, err := white.Printf(l); err != nil {
				log.Debug(err)
				return
			}
			continue
		}
		if _, err := blue.Println(l); err != nil {
			log.Debug(err)
			return
		}
	}

	blue.DisableColor()
	white.DisableColor()
	fmt.Println("")
	fmt.Println("OpenBazaar Server v" + core.VERSION)
	log.Infof("\nOpenBazaar Server v%s", core.VERSION)
	if !verbose {
		fmt.Println("[Press Ctrl+C to exit]")
	}
}
