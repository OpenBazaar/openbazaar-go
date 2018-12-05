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

	addrutil "gx/ipfs/QmNSWW3Sb4eju4o2djPQ1L1c2Zj9XN9sMYJL8r1cbxdc6b/go-addr-util"
	p2pbhost "gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p/p2p/host/basic"
	p2phost "gx/ipfs/QmNmJZL7FQySMtE2BQuLMuZg2EB2CLEunJJUSVSc9YnnbV/go-libp2p-host"
	manet "gx/ipfs/QmRK2LxanhK2gZq6k6R7vk5ZoYZk8ULSSTB7FzDsMUX6CB/go-multiaddr-net"
	dht "gx/ipfs/QmRaVcGchmC1stHHK7YhcgEuTk5k1JiGS568pfYWMgT91H/go-libp2p-kad-dht"
	dhtutil "gx/ipfs/QmRaVcGchmC1stHHK7YhcgEuTk5k1JiGS568pfYWMgT91H/go-libp2p-kad-dht/util"
	ipfslogging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	swarm "gx/ipfs/QmSwZMWwFZSUpe5muU2xgTUwppH24KfMwdPXiwbEp2c6G5/go-libp2p-swarm"
	routing "gx/ipfs/QmTiWLZ6Fo5j4KcTVutZJ5KWRRJrbxzmxA4td8NfEdrPh7/go-libp2p-routing"
	dshelp "gx/ipfs/QmTmqJGRQfuH8eKWD1FjThwPRipt1QhqJQNZ8MpzmfAAxo/go-ipfs-ds-help"
	recpb "gx/ipfs/QmUpttFinNDmNPgFwKN8sZK6BUtBmA68Y4KdSBDXa8t9sJ/go-libp2p-record/pb"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	pstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	smux "gx/ipfs/QmY9JXR3FupnYAYJWK9aMr9bCpqWKcToQ1tz8DVGTrHpHw/go-stream-muxer"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	"gx/ipfs/QmZPrWxuM8GHr4cGKbyF5CCT11sFUP9hgqpeUHALvx2nUr/go-libp2p-interface-pnet"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	metrics "gx/ipfs/QmdeBtQGXjSt7cb97nx9JyLHHv5va2LyEAue7Q5tDFzpLy/go-libp2p-metrics"
	oniontp "gx/ipfs/Qmdh86HZtNap3ktHvjyiVhBnp4uRpQWMCRAASieh8fDH8J/go-onion-transport"

	bstk "github.com/OpenBazaar/go-blockstackclient"
	"github.com/OpenBazaar/multiwallet"
	wi "github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/base58"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/fatih/color"
	"github.com/ipfs/go-ipfs/commands"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/ipfs/go-ipfs/namesys"
	namepb "github.com/ipfs/go-ipfs/namesys/pb"
	ipath "github.com/ipfs/go-ipfs/path"
	ipfsrepo "github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/natefinch/lumberjack"
	"github.com/op/go-logging"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/proxy"

	"github.com/OpenBazaar/openbazaar-go/api"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	obns "github.com/OpenBazaar/openbazaar-go/namesys"
	obnet "github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/net/service"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
	sto "github.com/OpenBazaar/openbazaar-go/storage"
	"github.com/OpenBazaar/openbazaar-go/storage/dropbox"
	"github.com/OpenBazaar/openbazaar-go/storage/selfhosted"
	"github.com/OpenBazaar/openbazaar-go/wallet"
	lis "github.com/OpenBazaar/openbazaar-go/wallet/listeners"
	"github.com/OpenBazaar/openbazaar-go/wallet/resync"
)

var (
	cfg            *StartConfig
	nodeRepo       ipfsrepo.Repo
	repoPath       string
	onionTransport *oniontp.OnionTransport
	torDialer      proxy.Dialer
	usingTor       bool
	usingClearnet  bool
	controlPort    int
	dnsResolver    namesys.Resolver
	ipfsNode       *ipfscore.IpfsNode
	noLogFiles     bool
	creationDate   time.Time
	params         chaincfg.Params
	pushNodes      []peer.ID
	authCookie     http.Cookie
	banManager     *obnet.BanManager

	stdoutLogFormat = logging.MustStringFormatter(
		`%{color:reset}%{color}%{time:15:04:05.000} [%{level}] [%{module}/%{shortfunc}] %{message}`,
	)

	fileLogFormat = logging.MustStringFormatter(
		`%{time:15:04:05.000} [%{level}] [%{module}/%{shortfunc}] %{message}`,
	)

	// ErrNoGateways - no gateway error
	ErrNoGateways = errors.New("no gateway addresses configured")

	// DHTOption - set the dht option
	DHTOption ipfscore.RoutingOption = constructDHTRouting

	offlineMessageFailoverTimeout = 30 * time.Second
)

// IPNSValidatorTag - validator tag for ipns
const IPNSValidatorTag = "ipns"

// Start - the cmd line arguments for start
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
	BitcoinCash          bool     `long:"bitcoincash" description:"use a Bitcoin Cash wallet in a dedicated data directory"`
	ZCash                string   `long:"zcash" description:"use a ZCash wallet in a dedicated data directory. To use this you must pass in the location of the zcashd binary."`
	Mode                 string   `short:"m" long:"mode" description:"set the mode [desktop, mobile]" default:"desktop"`
}

// Execute - start the server
func (x *Start) Execute(args []string) error {
	var err error
	var sqliteDB *db.SQLiteDatastore

	var swarmAddresses []string

	printSplashScreen(x.Verbose)

	if x.Testnet && x.Regtest {
		return errors.New("invalid combination of testnet and regtest modes")
	}

	if x.Tor && x.DualStack {
		return errors.New("invalid combination of tor and dual stack modes")
	}

	// Check if user launched with Testnet
	isTestnet := false
	if x.Testnet || x.Regtest {
		isTestnet = true
	}
	if x.BitcoinCash && x.ZCash != "" {
		return errors.New("coins Bitcoin Cash and ZCash cannot be used at the same time")
	}

	// Set repo path
	repoPath, err = repo.GetRepoPath(isTestnet)
	if err != nil {
		return err
	}
	if x.DataDir != "" {
		repoPath = x.DataDir
	}

	// Remove lockfile if present
	removeLockfile()

	// Logging
	noLogFiles = x.NoLogFiles
	setupLogging(x.Verbose, x.LogLevel)

	// Increase OS file descriptors to prevent crashes
	err = core.CheckAndSetUlimit()
	if err != nil {
		return err
	}

	// Create user-agent file
	err = createUserAgentFile(x.UserAgent)
	if err != nil {
		log.Error("error creating the user agent")
		//return err
	}

	coinType := getCoinType(x)
	migrations.WalletCoinType = coinType

	sqliteDB, err = ensureDatabaseAccess(repoPath, x.Password, isTestnet, coinType)
	if err != nil {
		log.Error("error initializing data folder")
	}

	// Get creation date. Ignore the error and use a default timestamp.
	creationDate, err = sqliteDB.Config().GetCreationDate()
	if err != nil {
		log.Error("error loading wallet creation date from database - using unix epoch.")
	}

	cfg, err = initConfigFiles()
	if err != nil {
		return err
	}

	// If SSL files are not specified it cannot be enabled
	if checkSSLConfiguration(cfg.apiConfig) != nil {
		return err
	}

	// If Gateway is not specified the server cannot listen for network messages
	if len(cfg.ipfsConfig.Addresses.Gateway) <= 0 {
		return ErrNoGateways
	}

	cfg.ipfsConfig.Identity, err = getIPFSIdentity(*sqliteDB)
	if err != nil {
		return err
	}

	if x.Testnet || x.Regtest {
		setupTestnet(cfg.ConfigData)
		setTestmodeRecordAgingIntervals()
	}

	swarmAddresses, err = configureIPFSSwarmForTor(x.Tor, x.DualStack)
	if err != nil {
		return err
	}
	cfg.ipfsConfig.Addresses.Swarm = swarmAddresses

	swarmAddresses, err = processSwarmAddresses(x.STUN, cfg.ipfsConfig.Addresses.Swarm)
	if err != nil {
		return err
	}
	cfg.ipfsConfig.Addresses.Swarm = swarmAddresses

	dnsResolver = namesys.NewDNSResolver() // Custom DNS resolver

	// Set up the Tor transport if user has enabled it
	if usingTor {
		err = setupTorTransport(x.TorPassword, cfg.torConfig.TorControl, cfg.torConfig.Password)
		if err != nil {
			return err
		}
		// If we're using Tor exclusively, set the proxy dialer and dns resolver
		if !usingClearnet {
			log.Notice("Using Tor exclusively")
			torDialer, err = onionTransport.TorDialer()
			if err != nil {
				log.Error("dialing Tor network:", err)
				return err
			}
			// TODO: maybe create a tor resolver impl later
			dnsResolver = nil // Disable DNS resolution due to privacy requirements
		}
	}

	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ipfsNode, err = ipfscore.NewNode(cctx, getNodeConfig(cfg.ipfsConfig.Ipns.UsePersistentCache))
	if err != nil {
		log.Error("create new IPFS node:", err)
		return err
	}

	ctx := commands.Context{}
	ctx.Online = true
	ctx.ConfigRoot = repoPath
	ctx.LoadConfig = func(_ string) (*config.Config, error) {
		return fsrepo.ConfigAt(repoPath)
	}
	ctx.ConstructNode = func() (*ipfscore.IpfsNode, error) {
		return ipfsNode, nil
	}

	// Set IPNS query size
	setDHTQuerySize(cfg.ipfsConfig.Ipns.QuerySize)

	log.Info("Peer ID: ", ipfsNode.Identity.Pretty())
	printSwarmAddrs(ipfsNode)

	// Get current directory root hash
	ipfsRootHash, err := getIPFSRootHash()

	// Set up Push Nodes
	err = setupPushNodes(cfg.dataSharingConfig.PushTo)
	if err != nil {
		return err
	}

	// Set up OpenBazaar multiwallet
	mnemonic, err := sqliteDB.Config().GetMnemonic()
	if err != nil {
		log.Error("get config mnemonic:", err)
		return err
	}

	multiwallet, err := setupWallet(sqliteDB, mnemonic, x.Testnet, x.Regtest, x.DisableExchangeRates)
	if err != nil {
		log.Error("Multiwallet setup failed:")
		return err
	}
	masterPrivateKey, err := getMasterPrivateKey(mnemonic)
	if err != nil {
		return err
	}

	resyncManager := resync.NewResyncManager(sqliteDB.Sales(), multiwallet)

	// Configure cookie for authenticated gateway
	if x.AuthCookie != "" {
		authCookie.Name = "OpenBazaar_Auth_Cookie"
		authCookie.Value = x.AuthCookie
		cfg.apiConfig.Authenticated = true
	} else {
		doAppend, err := setAuthCookie(cfg.ipfsConfig.Addresses.Gateway)
		if err != nil {
			log.Error(err)
			// do we bubble this err
		}
		if doAppend {
			cfg.apiConfig.AllowedIPs = append(cfg.apiConfig.AllowedIPs, x.AllowIP...)
		}
	}

	// Set up the ban manager
	setupBanManager(sqliteDB)

	// Set up custom name system
	nameSystem, err := getNameSystem(cfg.resolverConfig.Id)
	if err != nil {
		log.Error("Could not configure custom name system")
		return err
	}

	// Build pubsub
	pubSub := getPubSub()

	// OpenBazaar node setup
	core.Node = &core.OpenBazaarNode{
		AcceptStoreRequests:           cfg.dataSharingConfig.AcceptStoreRequests,
		BanManager:                    banManager,
		Datastore:                     sqliteDB,
		IPNSBackupAPI:                 cfg.ipfsConfig.Ipns.BackUpAPI,
		IpfsNode:                      ipfsNode,
		MasterPrivateKey:              masterPrivateKey,
		Multiwallet:                   multiwallet,
		NameSystem:                    nameSystem,
		OfflineMessageFailoverTimeout: offlineMessageFailoverTimeout,
		Pubsub:                        pubSub,
		PushNodes:                     pushNodes,
		RegressionTestEnable:          x.Regtest,
		RepoPath:                      repoPath,
		RootHash:                      ipath.Path(ipfsRootHash.Value).String(),
		TestnetEnable:                 x.Testnet,
		TorDialer:                     torDialer,
		UserAgent:                     core.USERAGENT,
	}
	core.PublishLock.Lock()

	// Offline messaging storage
	core.Node.MessageStorage, err = getOfflineStorage(x.Storage)
	if err != nil {
		return err
	}

	// Set up OpenBazaar API Gateway Server
	gateway, err := newHTTPGateway(core.Node, ctx, authCookie, *cfg.apiConfig, x.NoLogFiles)
	if err != nil {
		log.Error(err)
		return err
	}

	// Set up IPFS Server
	if cfg.ipfsConfig.Addresses.API != "" {
		if _, err := serveHTTPAPI(&ctx); err != nil {
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
			for ct, wal := range multiwallet {
				WL := lis.NewWalletListener(core.Node.Datastore, core.Node.Broadcast, ct)
				wal.AddTransactionListener(WL.OnTransactionReceived)
				wal.AddTransactionListener(TL.OnTransactionReceived)
			}
			log.Info("Starting multiwallet...")
			statusUpdater := wallet.NewStatusUpdater(multiwallet, core.Node.Broadcast, ipfsNode.Context())
			go statusUpdater.Start()
			go multiwallet.Start()
			if resyncManager != nil {
				go resyncManager.Start()
				go func() {
					core.Node.WaitForMessageRetrieverCompletion()
					resyncManager.CheckUnfunded()
				}()
			}
		}
		<-dht.DefaultBootstrapConfig.DoneChan
		core.Node.Service = service.New(core.Node, sqliteDB)

		core.Node.StartMessageRetriever()
		core.Node.StartPointerRepublisher()
		core.Node.StartRecordAgingNotifier()

		core.PublishLock.Unlock()
		err = core.Node.UpdateFollow()
		if err != nil {
			log.Error(err)
		}
		if !core.InitalPublishComplete {
			err = core.Node.SeedNode()
			if err != nil {
				log.Error(err)
			}
		}
		core.Node.SetUpRepublisher(cfg.republishInterval)
	}()

	// Start the OpenBazaar API Gateway Server
	err = gateway.Serve()
	if err != nil {
		log.Error(err)
	}

	return nil
}

func checkSSLConfiguration(apiCfg *schema.APIConfig) error {
	if (apiCfg.SSL && apiCfg.SSLCert == "") || (apiCfg.SSL && apiCfg.SSLKey == "") {
		return errors.New("api SSL cert and key files must be set when SSL is enabled")
	}
	return nil
}

func getOfflineStorage(customStorage string) (sto.OfflineMessagingStorage, error) {
	var storage sto.OfflineMessagingStorage
	var err error
	if customStorage == "self-hosted" || customStorage == "" {
		storage = selfhosted.NewSelfHostedStorage(repoPath, core.Node.IpfsNode, pushNodes, core.Node.SendStore)
	} else if customStorage == "dropbox" {
		if usingTor && !usingClearnet {
			log.Error("Dropbox can not be used with Tor")
			return nil, errors.New("Dropbox can not be used with Tor")
		}

		if cfg.dropboxToken == "" {
			err = errors.New("Dropbox token not set in config file")
			log.Error(err)
			return nil, err
		}
		storage, err = dropbox.NewDropBoxStorage(cfg.dropboxToken)
		if err != nil {
			log.Error(err)
			return nil, err
		}
	} else {
		err = errors.New("Invalid storage option")
		log.Error(err)
		return nil, err
	}
	return storage, nil
}

func getPubSub() ipfs.Pubsub {
	publisher := ipfs.NewPubsubPublisher(context.Background(), ipfsNode.PeerHost, ipfsNode.Routing, ipfsNode.Repo.Datastore(), ipfsNode.Floodsub)
	subscriber := ipfs.NewPubsubSubscriber(context.Background(), ipfsNode.PeerHost, ipfsNode.Routing, ipfsNode.Repo.Datastore(), ipfsNode.Floodsub)
	return ipfs.Pubsub{Publisher: publisher, Subscriber: subscriber}
}

func getNameSystem(id string) (*obns.NameSystem, error) {
	resolvers := []obns.Resolver{
		bstk.NewBlockStackClient(id, torDialer),
	}
	if !(usingTor && !usingClearnet) {
		resolvers = append(resolvers, obns.NewDNSResolver())
	}
	ns, err := obns.NewNameSystem(resolvers)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return ns, nil
}

func setupBanManager(sqliteDB *db.SQLiteDatastore) error {
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
	banManager = obnet.NewBanManager(blockedNodes)
	return nil
}

func setAuthCookie(gateway string) (bool, error) {
	appendFlag := false
	gatewayMaddr, err := ma.NewMultiaddr(gateway)
	if err != nil {
		log.Error(err)
		return appendFlag, err
	}
	addr, err := gatewayMaddr.ValueForProtocol(ma.P_IP4)
	if err != nil {
		log.Error(err)
		return appendFlag, err
	}
	// Override config file preference if this is Mainnet, open internet and API enabled
	if addr != "127.0.0.1" && params.Name == chaincfg.MainNetParams.Name {
		cfg.setAPIConfigAuthenticated()
	}
	// apiConfig.AllowedIPs = append(apiConfig.AllowedIPs, allowIP...)
	appendFlag = true

	// Create authentication cookie
	var authCookie http.Cookie
	authCookie.Name = "OpenBazaar_Auth_Cookie"

	cookiePrefix := authCookie.Name + "="
	cookiePath := path.Join(repoPath, ".cookie")
	cookie, err := ioutil.ReadFile(cookiePath)
	if err != nil {
		authBytes := make([]byte, 32)
		_, err = rand.Read(authBytes)
		if err != nil {
			log.Error(err)
			return appendFlag, err
		}
		authCookie.Value = base58.Encode(authBytes)
		f, err := os.Create(cookiePath)
		if err != nil {
			log.Error(err)
			return appendFlag, err
		}
		cookie := cookiePrefix + authCookie.Value
		_, werr := f.Write([]byte(cookie))
		if werr != nil {
			log.Error(werr)
			return appendFlag, werr
		}
		f.Close()
	} else {
		if string(cookie)[:len(cookiePrefix)] != cookiePrefix {
			return appendFlag, errors.New("Invalid authentication cookie. Delete it to generate a new one")
		}
		split := strings.SplitAfter(string(cookie), cookiePrefix)
		authCookie.Value = split[1]
	}
	return appendFlag, nil
}

func setupPushNodes(nodes []string) error {
	for _, pnd := range nodes {
		p, err := peer.IDB58Decode(pnd)
		if err != nil {
			log.Error("invalid peerID in DataSharing config")
			return err
		}
		pushNodes = append(pushNodes, p)
	}
	return nil
}

func getMasterPrivateKey(mnemonic string) (*hdkeychain.ExtendedKey, error) {
	// Master key setup
	seed := bip39.NewSeed(mnemonic, "")
	mPrivKey, err := hdkeychain.NewMaster(seed, &params)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return mPrivKey, nil
}

func setupWallet(db *db.SQLiteDatastore, mnemonic string, isTestnet bool, isRegtest bool, disableExchangeRates bool) (multiwallet.MultiWallet, error) {
	if isTestnet {
		params = chaincfg.TestNet3Params
	} else if isRegtest {
		params = chaincfg.RegressionNetParams
	} else {
		params = chaincfg.MainNetParams
	}

	// Multiwallet setup
	var walletLogWriter io.Writer
	if noLogFiles {
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
		ConfigFile:           cfg.walletsConfig,
		DB:                   db.DB(),
		Params:               &params,
		RepoPath:             repoPath,
		Logger:               walletLogger,
		Proxy:                torDialer,
		WalletCreationDate:   creationDate,
		Mnemonic:             mnemonic,
		DisableExchangeRates: disableExchangeRates,
	}
	mw, err := wallet.NewMultiWallet(multiwalletConfig)
	if err != nil {
		return nil, err
	}

	return mw, nil
}

func setDHTQuerySize(qSize int) {
	if qSize <= 20 && qSize > 0 {
		dhtutil.QuerySize = qSize
	} else {
		dhtutil.QuerySize = 16
	}
}

func getIPFSRootHash() (*namepb.IpnsEntry, error) {
	var err error
	_, ipnskey := namesys.IpnsKeysForID(ipfsNode.Identity)
	ival, hasherr := ipfsNode.Repo.Datastore().Get(dshelp.NewKeyFromBinary([]byte(ipnskey)))
	if hasherr != nil {
		log.Error("get ipns key:", hasherr)
		return &namepb.IpnsEntry{}, hasherr
	}
	val := ival.([]byte)
	dhtrec := new(recpb.Record)
	err = proto.Unmarshal(val, dhtrec)
	if err != nil {
		log.Error("unmarshal record", err)
		return &namepb.IpnsEntry{}, err
	}
	e := new(namepb.IpnsEntry)
	err = proto.Unmarshal(dhtrec.GetValue(), e)
	if err != nil {
		log.Error("unmarshal record value", err)
		return &namepb.IpnsEntry{}, err
	}
	return e, nil
}

func getNodeConfig(useIPNSCache bool) *ipfscore.BuildCfg {
	nodeConfig := &ipfscore.BuildCfg{
		Repo:   nodeRepo,
		Online: true,
		ExtraOpts: map[string]bool{
			"mplex": true,
		},
		DNSResolver: dnsResolver,
		Routing:     DHTOption,
	}

	if useIPNSCache {
		nodeConfig.ExtraOpts["ipnsps"] = true
	}

	if onionTransport != nil {
		nodeConfig.Host = defaultHostOption
	}
	return nodeConfig
}

func defaultHostOption(ctx context.Context, id peer.ID, ps pstore.Peerstore, bwr metrics.Reporter, fs []*net.IPNet, tpt smux.Transport, protec ipnet.Protector, opts *ipfscore.ConstructPeerHostOpts) (p2phost.Host, error) {
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

func setupTorTransport(password, tControl, tPassword string) error {
	var err error
	torControl := tControl
	if torControl == "" {
		controlPort, err = obnet.GetTorControlPort()
		if err != nil {
			log.Error("get tor control port:", err)
			return err
		}
		torControl = "127.0.0.1:" + strconv.Itoa(controlPort)
	}
	torPw := tPassword
	if password != "" {
		torPw = password
	}
	onionTransport, err = oniontp.NewOnionTransport("tcp4", torControl, torPw, nil, repoPath, (usingTor && usingClearnet))
	if err != nil {
		log.Error("setup tor transport:", err)
		return err
	}
	return nil
}

func processSwarmAddresses(isUsingSTUN bool, swarm []string) ([]string, error) {
	for i, addr := range swarm {
		m, err := ma.NewMultiaddr(addr)

		if err != nil {
			log.Error("creating swarm multihash:", err)
			return swarm, err
		}
		p := m.Protocols()

		// If we are using UTP and the stun option has been select, run stun and replace the port in the address
		if isUsingSTUN && p[0].Name == "ip4" && p[1].Name == "udp" && p[2].Name == "utp" {
			usingClearnet = true
			port, serr := obnet.Stun()
			if serr != nil {
				log.Error("stun setup:", serr)
				return swarm, err
			}
			swarm = append(swarm[:i], swarm[i+1:]...)
			swarm = append(swarm, "/ip4/0.0.0.0/udp/"+strconv.Itoa(port)+"/utp")
			break
		} else if p[0].Name == "onion" {
			usingTor = true
			addrutil.SupportedTransportStrings = append(addrutil.SupportedTransportStrings, "/onion")
			t, err := ma.ProtocolsWithString("/onion")
			if err != nil {
				log.Error("wrapping onion protocol:", err)
				return swarm, err
			}
			addrutil.SupportedTransportProtocols = append(addrutil.SupportedTransportProtocols, t)
		} else {
			usingClearnet = true
		}
	}
	return swarm, nil
}

func getOnionAddress() (string, error) {
	onionAddr, err := obnet.MaybeCreateHiddenServiceKey(repoPath)
	if err != nil {
		log.Error("create onion key:", err)
		return "", err
	}
	return "/onion/" + onionAddr + ":4003", nil
}

// Configure IPFS Swarm for Tor and dual stack nodes
func configureIPFSSwarmForTor(isTor bool, isDualStack bool) ([]string, error) {
	retSwarmAddresses := []string{}
	onionAddrString, err := getOnionAddress()
	if err != nil {
		return retSwarmAddresses, err
	}

	if isTor {
		retSwarmAddresses = append(retSwarmAddresses, onionAddrString)
	} else if isDualStack {
		retSwarmAddresses = append(retSwarmAddresses, onionAddrString)
		retSwarmAddresses = append(retSwarmAddresses, "/ip4/0.0.0.0/tcp/4001")
		retSwarmAddresses = append(retSwarmAddresses, "/ip6/::/tcp/4001")
		retSwarmAddresses = append(retSwarmAddresses, "/ip6/::/tcp/9005/ws")
		retSwarmAddresses = append(retSwarmAddresses, "/ip4/0.0.0.0/tcp/9005/ws")
	}
	return retSwarmAddresses, nil
}

// Retrieves identity key from the database and uses it to get the
// IPFS identity which contains the private key and peer ID
func getIPFSIdentity(db db.SQLiteDatastore) (config.Identity, error) {
	identityKey, err := db.Config().GetIdentityKey()
	if err != nil {
		log.Error("get identity key:", err)
		return config.Identity{}, err
	}
	identity, err := ipfs.IdentityFromKey(identityKey)
	if err != nil {
		log.Error("get identity from key:", err)
	}
	return identity, err
}

func ensureDatabaseAccess(dbPath, password string, testnet bool, ct wi.CoinType) (*db.SQLiteDatastore, error) {
	db, err := InitializeRepo(dbPath, password, "", testnet, time.Now(), ct)
	if err != nil && err != repo.ErrRepoExists {
		return nil, err
	}
	if db.Config().IsEncrypted() {
		db.Close()
		fmt.Print("Database is encrypted, enter your password: ")
		// nolint:unconvert
		bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
		fmt.Println("")
		pw := string(bytePassword)
		db, err = InitializeRepo(repoPath, pw, "", testnet, time.Now(), ct)
		if err != nil && err != repo.ErrRepoExists {
			return db, err
		}
		if db.Config().IsEncrypted() {
			log.Error("Invalid password")
			os.Exit(3)
		}
	}
	return db, nil
}

func createUserAgentFile(useragent string) error {
	userAgentBytes := []byte(core.USERAGENT + useragent)
	err := ioutil.WriteFile(path.Join(repoPath, "root", "user_agent"), userAgentBytes, os.ModePerm)
	if err != nil {
		log.Error("write user_agent:", err)
		return err
	}
	return nil
}

func getCoinType(x *Start) wi.CoinType {
	if x.BitcoinCash {
		return wi.BitcoinCash
	} else if x.ZCash != "" {
		return wi.Zcash
	}
	return wi.Bitcoin
}

func setupLogging(verbose bool, loglevel string) {
	w := &lumberjack.Logger{
		Filename:   path.Join(repoPath, "logs", "ob.log"),
		MaxSize:    10, // Megabytes
		MaxBackups: 3,
		MaxAge:     30, // Days
	}
	var backendStdoutFormatter logging.Backend
	if verbose {
		backendStdout := logging.NewLogBackend(os.Stdout, "", 0)
		backendStdoutFormatter = logging.NewBackendFormatter(backendStdout, stdoutLogFormat)
		logging.SetBackend(backendStdoutFormatter)
	}

	if !noLogFiles {
		backendFile := logging.NewLogBackend(w, "", 0)
		backendFileFormatter := logging.NewBackendFormatter(backendFile, fileLogFormat)
		if verbose {
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
	switch strings.ToLower(loglevel) {
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
}

func removeLockfile() {
	repoLockFile := filepath.Join(repoPath, fsrepo.LockFile)
	os.Remove(repoLockFile)
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

// Collects options, creates listener, prints status message and starts serving requests
func newHTTPGateway(node *core.OpenBazaarNode, ctx commands.Context, authCookie http.Cookie, config schema.APIConfig, noLogFilesFlag bool) (*api.Gateway, error) {
	// Get API configuration
	cfg, err := ctx.GetConfig()
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
	if noLogFilesFlag {
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

func constructDHTRouting(ctx context.Context, host p2phost.Host, dstore ds.Batching) (routing.IpfsRouting, error) {
	dhtRouting := dht.NewDHT(ctx, host, dstore)
	dhtRouting.Validator[IPNSValidatorTag] = namesys.NewIpnsRecordValidator(host.Peerstore())
	dhtRouting.Selector[IPNSValidatorTag] = namesys.IpnsSelectorFunc
	return dhtRouting, nil
}

// serveHTTPAPI collects options, creates listener, prints status message and starts serving requests
func serveHTTPAPI(cctx *commands.Context) (<-chan error, error) {
	cfg, err := cctx.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("serveHTTPAPI: GetConfig() failed: %s", err)
	}

	apiAddr := cfg.Addresses.API
	apiMaddr, err := ma.NewMultiaddr(apiAddr)
	if err != nil {
		return nil, fmt.Errorf("serveHTTPAPI: invalid API address: %q (err: %s)", apiAddr, err)
	}

	apiLis, err := manet.Listen(apiMaddr)
	if err != nil {
		return nil, fmt.Errorf("serveHTTPAPI: manet.Listen(%s) failed: %s", apiMaddr, err)
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
		return nil, fmt.Errorf("serveHTTPAPI: ConstructNode() failed: %s", err)
	}

	if err := node.Repo.SetAPIAddr(apiMaddr); err != nil {
		return nil, fmt.Errorf("serveHTTPAPI: SetAPIAddr() failed: %s", err)
	}

	errc := make(chan error)
	go func() {
		errc <- corehttp.Serve(node, apiLis.NetListener(), opts...)
		close(errc)
	}()
	return errc, nil
}

// InitializeRepo - return the sqlite db for the coin
func InitializeRepo(dataDir, password, mnemonic string, testnet bool, creationDate time.Time, coinType wi.CoinType) (*db.SQLiteDatastore, error) {
	// Database
	sqliteDB, err := db.Create(dataDir, password, testnet, coinType)
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
	if !verbose {
		fmt.Println("[Press Ctrl+C to exit]")
	}
}
