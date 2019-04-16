package mobile

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"gx/ipfs/QmRCrPXk2oUwpK1Cj2FXrUotRpddUxz56setkny2gz13Cx/go-libp2p-routing-helpers"
	"gx/ipfs/QmSY3nkMNLzh9GdbFKK5tT7YMfLpf52iUZ8ZRkr29MJaa5/go-libp2p-kad-dht"
	"gx/ipfs/QmSY3nkMNLzh9GdbFKK5tT7YMfLpf52iUZ8ZRkr29MJaa5/go-libp2p-kad-dht/opts"
	ma "gx/ipfs/QmTZBfrPJmjWsCvHEtX5FE6KimVJhsJg5sBbqEFYf4UZtL/go-multiaddr"
	ipfsconfig "gx/ipfs/QmUAuYuiafnJRZxDDX7MuruMNsicYNuyub5vUeAcupUBNs/go-ipfs-config"
	ds "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	p2phost "gx/ipfs/QmYrWiWM4qtrnCeT3R14jY3ZZyirDNJgwK57q4qFYePgbd/go-libp2p-host"
	"gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing"
	"gx/ipfs/QmbeHtaBy9nZsW4cHRcvgVY4CnDhXudE2Dr6qDxS7yg9rX/go-libp2p-record"
	ipfslogging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log/writer"
	"gx/ipfs/Qmc85NSvmSG4Frn9Vb2cBc1rMyULH6D3TNVEfCzSKoUpip/go-multiaddr-net"

	_ "net/http/pprof"

	"github.com/OpenBazaar/openbazaar-go/api"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	obnet "github.com/OpenBazaar/openbazaar-go/net"
	rep "github.com/OpenBazaar/openbazaar-go/net/repointer"
	ret "github.com/OpenBazaar/openbazaar-go/net/retriever"
	"github.com/OpenBazaar/openbazaar-go/net/service"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	apiSchema "github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/openbazaar-go/storage/selfhosted"
	"github.com/OpenBazaar/openbazaar-go/wallet"
	lis "github.com/OpenBazaar/openbazaar-go/wallet/listeners"
	"github.com/OpenBazaar/openbazaar-go/wallet/resync"
	wi "github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/ipfs/go-ipfs/commands"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/natefinch/lumberjack"
	"github.com/op/go-logging"
	"github.com/tyler-smith/go-bip39"
)

var log = logging.MustGetLogger("mobile")

// Node configuration structure
type Node struct {
	OpenBazaarNode *core.OpenBazaarNode
	config         NodeConfig
	cancel         context.CancelFunc
	ipfsConfig     *ipfscore.BuildCfg
	apiConfig      *apiSchema.APIConfig
	gateway        *api.Gateway
	started        bool
	startMtx       sync.Mutex
}

var (
	fileLogFormat = logging.MustStringFormatter(
		`[Haven] %{time:2006-01-02 15:04:05.000} [%{level}] [%{module}/%{shortfunc}] %{message}`,
	)
	publishUnlocked    = false
	mainLoggingBackend logging.Backend
)

// NewNode create the configuration file for a new node
func NewNode(repoPath string, authenticationToken string, testnet bool, userAgent string, walletTrustedPeer string, password string, mnemonic string, profile bool) *Node {
	// Node config
	nodeconfig := &NodeConfig{
		RepoPath:            repoPath,
		AuthenticationToken: authenticationToken,
		Testnet:             testnet,
		UserAgent:           userAgent,
		WalletTrustedPeer:   walletTrustedPeer,
		Profile:             profile,
	}

	// Use Mobile struct to carry config data
	node, err := NewNodeWithConfig(nodeconfig, password, mnemonic)
	if err != nil {
		fmt.Println(err)
	}
	return node
}

// NewNodeWithConfig create a new node using the configuration file from NewNode()
func NewNodeWithConfig(config *NodeConfig, password string, mnemonic string) (*Node, error) {
	// Lockfile
	repoLockFile := filepath.Join(config.RepoPath, fsrepo.LockFile)
	os.Remove(repoLockFile)

	ipfs.UpdateIPFSGlobalProtocolVars(config.Testnet)

	// Logging
	ipfsLog := &lumberjack.Logger{
		Filename:   path.Join(config.RepoPath, "logs", "ipfs.log"),
		MaxSize:    5, // Megabytes
		MaxBackups: 3,
		MaxAge:     30, // Days
	}
	ipfslogging.LdJSONFormatter()
	ipfslogging.Output(ipfsLog)()

	obLog := &lumberjack.Logger{
		Filename:   path.Join(config.RepoPath, "logs", "ob.log"),
		MaxSize:    5, // Megabytes
		MaxBackups: 3,
		MaxAge:     30, // Days
	}
	obFileBackend := logging.NewLogBackend(obLog, "", 0)
	obFileBackendFormatted := logging.NewBackendFormatter(obFileBackend, fileLogFormat)
	stdoutBackend := logging.NewLogBackend(os.Stdout, "", 0)
	stdoutBackendFormatted := logging.NewBackendFormatter(stdoutBackend, fileLogFormat)
	mainLoggingBackend = logging.SetBackend(obFileBackendFormatted, stdoutBackendFormatted)
	logging.SetLevel(logging.INFO, "")

	sqliteDB, err := initializeRepo(config.RepoPath, "", "", true, time.Now(), wi.Bitcoin)
	if err != nil && err != repo.ErrRepoExists {
		return nil, err
	}

	// Get creation date. Ignore the error and use a default timestamp.
	creationDate, _ := sqliteDB.Config().GetCreationDate()

	// Load configs
	configFile, err := ioutil.ReadFile(path.Join(config.RepoPath, "config"))
	if err != nil {
		return nil, err
	}

	apiConfig, err := apiSchema.GetAPIConfig(configFile)
	if err != nil {
		return nil, err
	}

	dataSharing, err := apiSchema.GetDataSharing(configFile)
	if err != nil {
		return nil, err
	}

	walletsConfig, err := apiSchema.GetWalletsConfig(configFile)
	if err != nil {
		return nil, err
	}

	ipnsExtraConfig, err := apiSchema.GetIPNSExtraConfig(configFile)
	if err != nil {
		return nil, err
	}

	// Create user-agent file
	userAgentBytes := []byte(core.USERAGENT + config.UserAgent)
	err = ioutil.WriteFile(path.Join(config.RepoPath, "root", "user_agent"), userAgentBytes, os.ModePerm)
	if err != nil {
		log.Error(err)
	}

	// IPFS node setup
	r, err := fsrepo.Open(config.RepoPath)
	if err != nil {
		return nil, err
	}

	cfg, err := r.Config()
	if err != nil {
		return nil, err
	}

	identityKey, err := sqliteDB.Config().GetIdentityKey()
	if err != nil {
		return nil, err
	}
	identity, err := ipfs.IdentityFromKey(identityKey)
	if err != nil {
		return nil, err
	}
	cfg.Identity = identity
	cfg.Swarm.DisableNatPortMap = false
	cfg.Swarm.EnableAutoRelay = true
	cfg.Swarm.EnableAutoNATService = false

	// Temporary override of bootstrap nodes
	cfg.Bootstrap = []string{
		"/ip6/2604:a880:2:d0::219d:9001/tcp/4001/ipfs/QmWUdwXW3bTXS19MtMjmfpnRYgssmbJCwnq8Lf9vjZwDii",
		"/ip6/2604:a880:400:d1::c1d:2001/tcp/4001/ipfs/QmcXwJePGLsP1x7gTXLE51BmE7peUKe2eQuR5LGbmasekt",
		"/ip6/2604:a880:400:d1::99a:8001/tcp/4001/ipfs/Qmb8i7uy6rk47hNorNLMVRMer4Nv9YWRhzZrWVqnvk5mSk",
		"/ip4/138.68.10.227/tcp/4001/ipfs/QmWUdwXW3bTXS19MtMjmfpnRYgssmbJCwnq8Lf9vjZwDii",
		"/ip4/157.230.59.219/tcp/4001/ipfs/QmcXwJePGLsP1x7gTXLE51BmE7peUKe2eQuR5LGbmasekt",
		"/ip4/206.189.224.90/tcp/4001/ipfs/Qmb8i7uy6rk47hNorNLMVRMer4Nv9YWRhzZrWVqnvk5mSk",
	}

	// Setup testnet
	if config.Testnet {
		// set testnet bootstrap addrs
		testnetBootstrapAddrs, err := apiSchema.GetTestnetBootstrapAddrs(configFile)
		if err != nil {
			log.Error(err)
			return nil, err
		}
		cfg.Bootstrap = testnetBootstrapAddrs

		// don't use pushnodes on testnet
		dataSharing.PushTo = []string{}
	}

	// Mnemonic
	mn, err := sqliteDB.Config().GetMnemonic()
	if err != nil {
		return nil, err
	}
	var params chaincfg.Params
	if config.Testnet {
		params = chaincfg.TestNet3Params
	} else {
		params = chaincfg.MainNetParams
	}

	// Master key setup
	seed := bip39.NewSeed(mn, "")
	mPrivKey, err := hdkeychain.NewMaster(seed, &params)
	if err != nil {
		return nil, err
	}

	// Multiwallet setup
	multiwalletConfig := &wallet.WalletConfig{
		ConfigFile:           walletsConfig,
		DB:                   sqliteDB.DB(),
		Params:               &params,
		RepoPath:             config.RepoPath,
		Logger:               mainLoggingBackend,
		WalletCreationDate:   creationDate,
		Mnemonic:             mn,
		DisableExchangeRates: config.DisableExchangerates,
	}
	mw, err := wallet.NewMultiWallet(multiwalletConfig)
	if err != nil {
		return nil, err
	}

	// Set up the ban manager
	settings, err := sqliteDB.Settings().Get()
	if err != nil && err != db.SettingsNotSetError {
		return nil, err
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

	// Push nodes
	var pushNodes []peer.ID
	for _, pnd := range dataSharing.PushTo {
		p, err := peer.IDB58Decode(pnd)
		if err != nil {
			return nil, err
		}
		pushNodes = append(pushNodes, p)
	}

	// OpenBazaar node setup
	node := &core.OpenBazaarNode{
		BanManager:                    bm,
		Datastore:                     sqliteDB,
		MasterPrivateKey:              mPrivKey,
		Multiwallet:                   mw,
		OfflineMessageFailoverTimeout: 3 * time.Second,
		PushNodes:                     pushNodes,
		RepoPath:                      config.RepoPath,
		UserAgent:                     core.USERAGENT,
		IPNSQuorumSize:                uint(ipnsExtraConfig.DHTQuorumSize),
	}

	if len(cfg.Addresses.Gateway) <= 0 {
		return nil, errors.New("no gateway addresses configured")
	}

	// override with mobile routing config
	ignoredURI := ""
	ncfg := ipfs.PrepareIPFSConfig(r, ignoredURI, config.Testnet, config.Testnet)
	ncfg.Routing = constructMobileRouting

	node.PublishLock.Lock()

	// assert reserve wallet is available on startup for later usage
	_, err = node.ReserveCurrencyConverter()
	if err != nil {
		return nil, fmt.Errorf("verifying reserve currency converter: %s", err.Error())
	}

	return &Node{OpenBazaarNode: node, config: *config, ipfsConfig: ncfg, apiConfig: apiConfig, startMtx: sync.Mutex{}}, nil
}

func constructMobileRouting(ctx context.Context, host p2phost.Host, dstore ds.Batching, validator record.Validator) (routing.IpfsRouting, error) {
	return dht.New(
		ctx, host,
		dhtopts.Client(true),
		dhtopts.Datastore(dstore),
		dhtopts.Validator(validator),
	)
}

// startIPFSNode start the node
func (n *Node) startIPFSNode(repoPath string, config *ipfscore.BuildCfg) (*ipfscore.IpfsNode, commands.Context, error) {
	cctx, cancel := context.WithCancel(context.Background())
	n.cancel = cancel

	ctx := commands.Context{}

	ipfscore.DefaultBootstrapConfig = ipfscore.BootstrapConfig{
		MinPeerThreshold:  8,
		Period:            time.Second * 10,
		ConnectionTimeout: time.Second * 10 / 3,
	}

	nd, err := ipfscore.NewNode(cctx, config)
	if err != nil {
		return nil, ctx, err
	}

	ctx.Online = true
	ctx.ConfigRoot = repoPath
	ctx.LoadConfig = func(_ string) (*ipfsconfig.Config, error) {
		return fsrepo.ConfigAt(repoPath)
	}
	ctx.ConstructNode = func() (*ipfscore.IpfsNode, error) {
		return nd, nil
	}
	return nd, ctx, nil
}

// Start start openbazaard (OpenBazaar daemon)
func (n *Node) Start() error {
	n.startMtx.Lock()
	defer n.startMtx.Unlock()

	return n.start()
}

func (n *Node) mountProfileHandlerAndListen() {
	listenAddr := net.JoinHostPort("", "6060")
	profileRedirect := http.RedirectHandler("/debug/pprof",
		http.StatusSeeOther)
	http.Handle("/", profileRedirect)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Errorf("serving debug profiler: %s", err.Error())
	}
}

func (n *Node) start() error {
	if n.config.Profile {
		go n.mountProfileHandlerAndListen()
	}
	nd, ctx, err := n.startIPFSNode(n.config.RepoPath, n.ipfsConfig)
	if err != nil {
		return err
	}

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

	n.OpenBazaarNode.IpfsNode = nd
	n.OpenBazaarNode.DHT = dhtRouting

	// Get current directory root hash
	rec, err := ipfs.GetCachedIPNSRecord(nd.Repo.Datastore(), nd.Identity)
	if err != nil {
		log.Warning(err)
		if err := ipfs.DeleteCachedIPNSRecord(nd.Repo.Datastore(), nd.Identity); err != nil {
			log.Errorf("deleting invalid IPNS record: %s", err.Error())
		}
	} else {
		n.OpenBazaarNode.RootHash = string(rec.Value)
	}

	configFile, err := ioutil.ReadFile(path.Join(n.OpenBazaarNode.RepoPath, "config"))
	if err != nil {
		return err
	}
	republishInterval, err := apiSchema.GetRepublishInterval(configFile)
	if err != nil {
		return err
	}

	// Offline messaging storage
	n.OpenBazaarNode.MessageStorage = selfhosted.NewSelfHostedStorage(n.OpenBazaarNode.RepoPath, n.OpenBazaarNode.IpfsNode, n.OpenBazaarNode.PushNodes, n.OpenBazaarNode.SendStore)

	// Build pubsub
	publisher := ipfs.NewPubsubPublisher(context.Background(), nd.PeerHost, nd.Routing, nd.Repo.Datastore(), nd.PubSub)
	subscriber := ipfs.NewPubsubSubscriber(context.Background(), nd.PeerHost, nd.Routing, nd.Repo.Datastore(), nd.PubSub)
	ps := ipfs.Pubsub{Publisher: publisher, Subscriber: subscriber}
	n.OpenBazaarNode.Pubsub = ps

	// Start gateway
	// Create authentication cookie
	var authCookie http.Cookie
	authCookie.Name = "OpenBazaar_Auth_Cookie"

	if n.config.AuthenticationToken != "" {
		authCookie.Value = n.config.AuthenticationToken
		n.apiConfig.Authenticated = true
	}
	gateway, err := newHTTPGateway(n, ctx, authCookie, *n.apiConfig)
	if err != nil {
		return err
	}
	n.gateway = gateway
	go func() {
		if err := gateway.Serve(); err != nil {
			log.Error(err)
		}
	}()

	go func() {
		resyncManager := resync.NewResyncManager(n.OpenBazaarNode.Datastore.Sales(), n.OpenBazaarNode.Datastore.Purchases(), n.OpenBazaarNode.Multiwallet)
		if !n.config.DisableWallet {
			if resyncManager == nil {
				n.OpenBazaarNode.WaitForMessageRetrieverCompletion()
			}
			TL := lis.NewTransactionListener(n.OpenBazaarNode.Multiwallet, n.OpenBazaarNode.Datastore, n.OpenBazaarNode.Broadcast)
			for ct, wal := range n.OpenBazaarNode.Multiwallet {
				WL := lis.NewWalletListener(n.OpenBazaarNode.Datastore, n.OpenBazaarNode.Broadcast, ct)
				wal.AddTransactionListener(WL.OnTransactionReceived)
				wal.AddTransactionListener(TL.OnTransactionReceived)
			}
			su := wallet.NewStatusUpdater(n.OpenBazaarNode.Multiwallet, n.OpenBazaarNode.Broadcast, n.OpenBazaarNode.IpfsNode.Context())
			go su.Start()
			go n.OpenBazaarNode.Multiwallet.Start()
			if resyncManager != nil {
				go resyncManager.Start()
				go func() {
					n.OpenBazaarNode.WaitForMessageRetrieverCompletion()
					resyncManager.CheckUnfunded()
				}()
			}
		}
		n.OpenBazaarNode.Service = service.New(n.OpenBazaarNode, n.OpenBazaarNode.Datastore)
		n.OpenBazaarNode.Service.WaitForReady()
		MR := ret.NewMessageRetriever(ret.MRConfig{
			Db:        n.OpenBazaarNode.Datastore,
			IPFSNode:  n.OpenBazaarNode.IpfsNode,
			DHT:       n.OpenBazaarNode.DHT,
			BanManger: n.OpenBazaarNode.BanManager,
			Service:   n.OpenBazaarNode.Service,
			PrefixLen: 14,
			PushNodes: n.OpenBazaarNode.PushNodes,
			Dialer:    nil,
			SendAck:   n.OpenBazaarNode.SendOfflineAck,
			SendError: n.OpenBazaarNode.SendError,
		})
		go MR.Run()
		n.OpenBazaarNode.MessageRetriever = MR
		PR := rep.NewPointerRepublisher(n.OpenBazaarNode.DHT, n.OpenBazaarNode.Datastore, n.OpenBazaarNode.PushNodes, n.OpenBazaarNode.IsModerator)
		go PR.Run()
		n.OpenBazaarNode.PointerRepublisher = PR
		// MR.Wait()

		n.OpenBazaarNode.PublishLock.Unlock()
		publishUnlocked = true
		err = n.OpenBazaarNode.UpdateFollow()
		if err != nil {
			log.Error(err)
		}
		if !n.OpenBazaarNode.InitalPublishComplete {
			err = n.OpenBazaarNode.SeedNode()
			if err != nil {
				log.Error(err)
			}
		}
		n.OpenBazaarNode.SetUpRepublisher(republishInterval)
	}()
	n.started = true
	return nil
}

// Stop stop openbazaard
func (n *Node) Stop() error {
	n.startMtx.Lock()
	defer n.startMtx.Unlock()

	return n.stop()
}

func (n *Node) stop() error {
	core.OfflineMessageWaitGroup.Wait()
	n.OpenBazaarNode.Datastore.Close()
	repoLockFile := filepath.Join(n.OpenBazaarNode.RepoPath, fsrepo.LockFile)
	if err := os.Remove(repoLockFile); err != nil {
		log.Error(err)
	}
	n.OpenBazaarNode.Multiwallet.Close()
	if err := n.OpenBazaarNode.IpfsNode.Close(); err != nil {
		log.Error(err)
	}
	if err := n.gateway.Close(); err != nil {
		log.Error(err)
	}
	n.started = false
	return nil
}

func (n *Node) Restart() error {
	n.startMtx.Lock()
	defer n.startMtx.Unlock()

	var wg sync.WaitGroup

	if n.started {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := n.stop(); err != nil {
				log.Error(err)
			}
		}()
		wg.Wait()
	}

	// This node has been stopped by the stop command so we need to create
	// a new one before starting it again.
	newNode, err := NewNodeWithConfig(&n.config, "", "")
	if err != nil {
		return err
	}
	n.OpenBazaarNode = newNode.OpenBazaarNode
	n.config = newNode.config
	n.ipfsConfig = newNode.ipfsConfig
	n.apiConfig = newNode.apiConfig

	return n.start()
}

// PublishUnlocked return true if publish is unlocked
func (n *Node) PublishUnlocked() bool {
	return publishUnlocked
}

// initializeRepo create the database
func initializeRepo(dataDir, password, mnemonic string, testnet bool, creationDate time.Time, coinType wi.CoinType) (*db.SQLiteDatastore, error) {
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

// Collects options, creates listener, prints status message and starts serving requests
func newHTTPGateway(node *Node, ctx commands.Context, authCookie http.Cookie, config apiSchema.APIConfig) (*api.Gateway, error) {
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
	gwLis, err = manet.Listen(gatewayMaddr)
	if err != nil {
		return nil, fmt.Errorf("newHTTPGateway: manet.Listen(%s) failed: %s", gatewayMaddr, err)
	}

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

	// Create and return an API gateway
	return api.NewGateway(node.OpenBazaarNode, authCookie, manet.NetListener(gwLis), config, mainLoggingBackend, opts...)
}
