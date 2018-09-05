package mobile

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	bstk "github.com/OpenBazaar/go-blockstackclient"
	"github.com/OpenBazaar/openbazaar-go/api"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	obns "github.com/OpenBazaar/openbazaar-go/namesys"
	obnet "github.com/OpenBazaar/openbazaar-go/net"
	rep "github.com/OpenBazaar/openbazaar-go/net/repointer"
	ret "github.com/OpenBazaar/openbazaar-go/net/retriever"
	"github.com/OpenBazaar/openbazaar-go/net/service"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	apiSchema "github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/openbazaar-go/storage/selfhosted"
	"github.com/OpenBazaar/openbazaar-go/wallet"
	lis "github.com/OpenBazaar/openbazaar-go/wallet/listeners"
	"github.com/OpenBazaar/spvwallet/exchangerates"
	wi "github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ipfs/go-ipfs/commands"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/corehttp"
	bitswap "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	"github.com/ipfs/go-ipfs/namesys"
	namepb "github.com/ipfs/go-ipfs/namesys/pb"
	ipath "github.com/ipfs/go-ipfs/path"
	ipfsconfig "github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/op/go-logging"
	p2phost "gx/ipfs/QmNmJZL7FQySMtE2BQuLMuZg2EB2CLEunJJUSVSc9YnnbV/go-libp2p-host"
	manet "gx/ipfs/QmRK2LxanhK2gZq6k6R7vk5ZoYZk8ULSSTB7FzDsMUX6CB/go-multiaddr-net"
	dht "gx/ipfs/QmRaVcGchmC1stHHK7YhcgEuTk5k1JiGS568pfYWMgT91H/go-libp2p-kad-dht"
	dhtutil "gx/ipfs/QmRaVcGchmC1stHHK7YhcgEuTk5k1JiGS568pfYWMgT91H/go-libp2p-kad-dht/util"
	routing "gx/ipfs/QmTiWLZ6Fo5j4KcTVutZJ5KWRRJrbxzmxA4td8NfEdrPh7/go-libp2p-routing"
	"gx/ipfs/QmTmqJGRQfuH8eKWD1FjThwPRipt1QhqJQNZ8MpzmfAAxo/go-ipfs-ds-help"
	recpb "gx/ipfs/QmUpttFinNDmNPgFwKN8sZK6BUtBmA68Y4KdSBDXa8t9sJ/go-libp2p-record/pb"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
)

// Node configuration structure
type Node struct {
	OpenBazaarNode *core.OpenBazaarNode
	config         NodeConfig
	cancel         context.CancelFunc
	ipfsConfig     *ipfscore.BuildCfg
	apiConfig      *apiSchema.APIConfig
}

// NewNode create the configuration file for a new node
func NewNode(repoPath string, authenticationToken string, testnet bool, userAgent string, walletTrustedPeer string, password string, mnemonic string) *Node {
	// Node config
	nodeconfig := &NodeConfig{
		RepoPath:            repoPath,
		AuthenticationToken: "",
		Testnet:             testnet,
		UserAgent:           userAgent,
		WalletTrustedPeer:   walletTrustedPeer,
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

	// Logging
	backendStdout := logging.NewLogBackend(os.Stdout, "", 0)
	logger = logging.NewBackendFormatter(backendStdout, stdoutLogFormat)
	logging.SetBackend(logger)

	// Coin type
	ct := wi.Bitcoin
	migrations.WalletCoinType = ct

	// Database
	sqliteDB, err := initializeRepo(config.RepoPath, password, mnemonic, config.Testnet, time.Now(), ct)
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

	resolverConfig, err := apiSchema.GetResolverConfig(configFile)
	if err != nil {
		return nil, err
	}
	walletsConfig, err := apiSchema.GetWalletsConfig(configFile)
	if err != nil {
		return nil, err
	}

	// Create user-agent file
	userAgentBytes := []byte(core.USERAGENT + config.UserAgent)
	ioutil.WriteFile(path.Join(config.RepoPath, "root", "user_agent"), userAgentBytes, os.ModePerm)

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
	cfg.Swarm.DisableNatPortMap = true

	// Setup testnet
	if config.Testnet {
		testnetBootstrapAddrs, err := apiSchema.GetTestnetBootstrapAddrs(configFile)
		if err != nil {
			return nil, err
		}
		cfg.Bootstrap = testnetBootstrapAddrs
		dht.ProtocolDHT = "/openbazaar/kad/testnet/1.0.0"
		bitswap.ProtocolBitswap = "/openbazaar/bitswap/testnet/1.1.0"
		service.ProtocolOpenBazaar = "/openbazaar/app/testnet/1.0.0"

		dataSharing.PushTo = []string{}
	}

	ncfg := &ipfscore.BuildCfg{
		Repo:    r,
		Online:  true,
		Routing: DHTClientOption,
		ExtraOpts: map[string]bool{
			"mplex":  true,
			"ipnsps": true,
		},
	}

	// Set IPNS query size
	querySize := cfg.Ipns.QuerySize
	if querySize <= 20 && querySize > 0 {
		dhtutil.QuerySize = int(querySize)
	} else {
		dhtutil.QuerySize = 16
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

	// Multiwallet setup
	multiwalletConfig := &wallet.WalletConfig{
		ConfigFile:         walletsConfig,
		DB:                 sqliteDB.DB(),
		Params:             &params,
		RepoPath:           config.RepoPath,
		Logger:             logger,
		WalletCreationDate: creationDate,
		Mnemonic:           mn,
	}
	mw, err := wallet.NewMultiWallet(multiwalletConfig)
	if err != nil {
		return nil, err
	}

	// Wallet
	core.PublishLock.Lock()
	if !config.DisableWallet {
	}

	// Exchange rates
	var exchangeRates wi.ExchangeRates
	if !config.DisableExchangerates {
		exchangeRates = exchangerates.NewBitcoinPriceFetcher(nil)
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

	// Create namesys resolvers
	resolvers := []obns.Resolver{
		bstk.NewBlockStackClient(resolverConfig.Id, nil),
		obns.NewDNSResolver(),
	}
	ns, err := obns.NewNameSystem(resolvers)
	if err != nil {
		return nil, err
	}

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
	core.Node = &core.OpenBazaarNode{
		RepoPath:      config.RepoPath,
		Datastore:     sqliteDB,
		Multiwallet:   mw,
		NameSystem:    ns,
		ExchangeRates: exchangeRates,
		UserAgent:     core.USERAGENT,
		PushNodes:     pushNodes,
		BanManager:    bm,
	}

	if len(cfg.Addresses.Gateway) <= 0 {
		return nil, errors.New("No gateway addresses configured")
	}

	return &Node{OpenBazaarNode: core.Node, config: *config, ipfsConfig: ncfg, apiConfig: apiConfig}, nil
}

// startIPFSNode start the node
func (n *Node) startIPFSNode(repoPath string, config *ipfscore.BuildCfg) (*ipfscore.IpfsNode, commands.Context, error) {
	cctx, cancel := context.WithCancel(context.Background())
	n.cancel = cancel

	ctx := commands.Context{}
	nd, err := ipfscore.NewNode(cctx, config)
	if err != nil {
		return nil, ctx, err
	}

	ctx.Online = true
	ctx.ConfigRoot = repoPath
	ctx.LoadConfig = func(path string) (*ipfsconfig.Config, error) {
		return fsrepo.ConfigAt(repoPath)
	}
	ctx.ConstructNode = func() (*ipfscore.IpfsNode, error) {
		return nd, nil
	}
	return nd, ctx, nil
}

// Start start openbazaard (OpenBazaar daemon)
func (n *Node) Start() error {
	nd, ctx, err := n.startIPFSNode(n.config.RepoPath, n.ipfsConfig)
	if err != nil {
		return err
	}

	n.OpenBazaarNode.IpfsNode = nd

	// Get current directory root hash
	_, ipnskey := namesys.IpnsKeysForID(nd.Identity)
	ival, hasherr := nd.Repo.Datastore().Get(dshelp.NewKeyFromBinary([]byte(ipnskey)))
	if hasherr != nil {
		return hasherr
	}
	val := ival.([]byte)
	dhtrec := new(recpb.Record)
	proto.Unmarshal(val, dhtrec)
	e := new(namepb.IpnsEntry)
	proto.Unmarshal(dhtrec.GetValue(), e)
	n.OpenBazaarNode.RootHash = ipath.Path(e.Value).String()

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
	publisher := ipfs.NewPubsubPublisher(context.Background(), nd.PeerHost, nd.Routing, nd.Repo.Datastore(), nd.Floodsub)
	subscriber := ipfs.NewPubsubSubscriber(context.Background(), nd.PeerHost, nd.Routing, nd.Repo.Datastore(), nd.Floodsub)
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
	gateway, err := newHTTPGateway(core.Node, ctx, authCookie, *n.apiConfig)
	if err != nil {
		return err
	}
	go gateway.Serve()

	go func() {
		<-dht.DefaultBootstrapConfig.DoneChan
		n.OpenBazaarNode.Service = service.New(n.OpenBazaarNode, n.OpenBazaarNode.Datastore)
		MR := ret.NewMessageRetriever(ret.MRConfig{
			Db:        n.OpenBazaarNode.Datastore,
			IPFSNode:  n.OpenBazaarNode.IpfsNode,
			BanManger: n.OpenBazaarNode.BanManager,
			Service:   core.Node.Service,
			PrefixLen: 14,
			PushNodes: core.Node.PushNodes,
			Dialer:    nil,
			SendAck:   n.OpenBazaarNode.SendOfflineAck,
			SendError: n.OpenBazaarNode.SendError,
		})
		go MR.Run()
		n.OpenBazaarNode.MessageRetriever = MR
		PR := rep.NewPointerRepublisher(n.OpenBazaarNode.IpfsNode, n.OpenBazaarNode.Datastore, n.OpenBazaarNode.PushNodes, n.OpenBazaarNode.IsModerator)
		go PR.Run()
		n.OpenBazaarNode.PointerRepublisher = PR
		MR.Wait()
		if n.OpenBazaarNode.Multiwallet != nil {
			TL := lis.NewTransactionListener(core.Node.Datastore, core.Node.Broadcast)
			for ct, wal := range n.OpenBazaarNode.Multiwallet {
				WL := lis.NewWalletListener(core.Node.Datastore, core.Node.Broadcast, ct)
				wal.AddTransactionListener(WL.OnTransactionReceived)
				wal.AddTransactionListener(TL.OnTransactionReceived)
			}
			su := wallet.NewStatusUpdater(n.OpenBazaarNode.Multiwallet, n.OpenBazaarNode.Broadcast, n.OpenBazaarNode.IpfsNode.Context())
			go su.Start()
			go n.OpenBazaarNode.Multiwallet.Start()
		}

		core.PublishLock.Unlock()
		core.Node.UpdateFollow()
		if !core.InitalPublishComplete {
			core.Node.SeedNode()
		}
		core.Node.SetUpRepublisher(republishInterval)
	}()

	return nil
}

// Stop stop openbazaard
func (n *Node) Stop() error {
	core.OfflineMessageWaitGroup.Wait()
	core.Node.Datastore.Close()
	repoLockFile := filepath.Join(core.Node.RepoPath, fsrepo.LockFile)
	os.Remove(repoLockFile)
	core.Node.Wallet.Close()
	core.Node.Multiwallet.Close()
	core.Node.IpfsNode.Close()
	return nil
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

// newHttpGateway Collects options, creates listener, prints status message and starts serving requests
func newHTTPGateway(node *core.OpenBazaarNode, ctx commands.Context, authCookie http.Cookie, config apiSchema.APIConfig) (*api.Gateway, error) {
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

	gwLis, err := manet.Listen(gatewayMaddr)
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

	if err != nil {
		return nil, fmt.Errorf("newHTTPGateway: ConstructNode() failed: %s", err)
	}

	return api.NewGateway(node, authCookie, gwLis.NetListener(), config, logger, opts...)
}

// DHTClientOption required for constructClientDHTRouting()
var DHTClientOption ipfscore.RoutingOption = constructClientDHTRouting

// IpnsValidatorTag required for constructClientDHTRouting()
const IpnsValidatorTag = "ipns"

// constructClientDHTRouting create DHT routing
func constructClientDHTRouting(ctx context.Context, host p2phost.Host, dstore ds.Batching) (routing.IpfsRouting, error) {
	dhtRouting := dht.NewDHTClient(ctx, host, dstore)
	dhtRouting.Validator[IpnsValidatorTag] = namesys.NewIpnsRecordValidator(host.Peerstore())
	dhtRouting.Selector[IpnsValidatorTag] = namesys.IpnsSelectorFunc
	return dhtRouting, nil
}
