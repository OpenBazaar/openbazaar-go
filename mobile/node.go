package mobile

import (
	"context"
	"os"
	"path/filepath"

	"github.com/OpenBazaar/openbazaar-go/api"
	obns "github.com/OpenBazaar/openbazaar-go/namesys"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/ipfs/go-ipfs/core/corehttp"
	manet "gx/ipfs/QmX3U3YXCQ6UYBxq2LVWF8dARS1hPUTEYLrSx654Qyxyw6/go-multiaddr-net"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"

	lis "github.com/OpenBazaar/openbazaar-go/bitcoin/listeners"
	rep "github.com/OpenBazaar/openbazaar-go/net/repointer"
	ret "github.com/OpenBazaar/openbazaar-go/net/retriever"
	"github.com/OpenBazaar/openbazaar-go/net/service"

	"errors"
	"fmt"
	bstk "github.com/OpenBazaar/go-blockstackclient"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"github.com/OpenBazaar/openbazaar-go/bitcoin/exchange"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	obnet "github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/storage/selfhosted"
	"github.com/OpenBazaar/spvwallet"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ipfs/go-ipfs/commands"
	ipfscore "github.com/ipfs/go-ipfs/core"
	bitswap "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	"github.com/ipfs/go-ipfs/namesys"
	namepb "github.com/ipfs/go-ipfs/namesys/pb"
	ipath "github.com/ipfs/go-ipfs/path"
	ipfsrepo "github.com/ipfs/go-ipfs/repo"
	ipfsconfig "github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	lockfile "github.com/ipfs/go-ipfs/repo/fsrepo/lock"
	"github.com/ipfs/go-ipfs/thirdparty/ds-help"
	"github.com/op/go-logging"
	routing "gx/ipfs/QmPR2JzfKd9poHx9XBhzoFeBBC31ZM3W5iUPKJZWyaoZZm/go-libp2p-routing"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	p2phost "gx/ipfs/QmaSxYRuMq4pkpBBG2CYaRrPx2z7NmMVEs34b9g61biQA6/go-libp2p-host"
	recpb "gx/ipfs/QmbxkgUceEcuSZ4ZdBA3x74VUDSSYjHYmmeEqkjxbtZ6Jg/go-libp2p-record/pb"
	dht "gx/ipfs/Qmcjua7379qzY63PJ5a8w3mDteHZppiX2zo6vFeaqjVcQi/go-libp2p-kad-dht"
	dhtutil "gx/ipfs/Qmcjua7379qzY63PJ5a8w3mDteHZppiX2zo6vFeaqjVcQi/go-libp2p-kad-dht/util"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"
)

type Node struct {
	node       *core.OpenBazaarNode
	config     NodeConfig
	cancel     context.CancelFunc
	ipfsConfig *ipfscore.BuildCfg
	apiConfig  *repo.APIConfig
}

func NewNode(config NodeConfig) (*Node, error) {

	repoLockFile := filepath.Join(config.RepoPath, lockfile.LockFile)
	os.Remove(repoLockFile)

	// Logging
	backendStdout := logging.NewLogBackend(os.Stdout, "", 0)
	logger = logging.NewBackendFormatter(backendStdout, stdoutLogFormat)
	logging.SetBackend(logger)

	sqliteDB, err := initializeRepo(config.RepoPath, "", "", true, time.Now())
	if err != nil && err != repo.ErrRepoExists {
		return nil, err
	}

	// Get creation date. Ignore the error and use a default timestamp.
	creationDate, _ := sqliteDB.Config().GetCreationDate()

	// Load config
	configFile, err := ioutil.ReadFile(path.Join(config.RepoPath, "config"))
	if err != nil {
		return nil, err
	}

	apiConfig, err := repo.GetAPIConfig(configFile)
	if err != nil {
		return nil, err
	}

	walletCfg, err := repo.GetWalletConfig(configFile)
	if err != nil {
		return nil, err
	}
	gatewayUrlStrings, err := repo.GetCrosspostGateway(configFile)
	if err != nil {
		return nil, err
	}
	resolverConfig, err := repo.GetResolverConfig(configFile)
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
		testnetBootstrapAddrs, err := repo.GetTestnetBootstrapAddrs(configFile)
		if err != nil {
			return nil, err
		}
		cfg.Bootstrap = testnetBootstrapAddrs
		dht.ProtocolDHT = "/openbazaar/kad/testnet/1.0.0"
		bitswap.ProtocolBitswap = "/openbazaar/bitswap/testnet/1.1.0"
		service.ProtocolOpenBazaar = "/openbazaar/app/testnet/1.0.0"

		gatewayUrlStrings = []string{}
	}

	ncfg := &ipfscore.BuildCfg{
		Repo:    r,
		Online:  true,
		Routing: DHTClientOption,
	}

	// Set IPNS query size
	querySize := cfg.Ipns.QuerySize
	if querySize <= 20 && querySize > 0 {
		dhtutil.QuerySize = int(querySize)
	} else {
		dhtutil.QuerySize = 16
	}
	namesys.UsePersistentCache = cfg.Ipns.UsePersistentCache

	// Wallet
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

	var wallet wallet.Wallet
	var tp net.Addr
	if config.WalletTrustedPeer != "" {
		tp, err = net.ResolveTCPAddr("tcp", walletCfg.TrustedPeer)
		if err != nil {
			return nil, err
		}
	}
	feeApi, err := url.Parse(walletCfg.FeeAPI)
	if err != nil {
		return nil, err
	}
	spvwalletConfig := &spvwallet.Config{
		Mnemonic:     mn,
		Params:       &params,
		MaxFee:       uint64(walletCfg.MaxFee),
		LowFee:       uint64(walletCfg.LowFeeDefault),
		MediumFee:    uint64(walletCfg.MediumFeeDefault),
		HighFee:      uint64(walletCfg.HighFeeDefault),
		FeeAPI:       *feeApi,
		RepoPath:     config.RepoPath,
		CreationDate: creationDate,
		DB:           sqliteDB,
		UserAgent:    "OpenBazaar",
		TrustedPeer:  tp,
		Logger:       logger,
	}
	wallet, err = spvwallet.NewSPVWallet(spvwalletConfig)
	if err != nil {
		return nil, err
	}

	// Crosspost gateway
	var gatewayUrls []*url.URL
	for _, gw := range gatewayUrlStrings {
		if gw != "" {
			u, err := url.Parse(gw)
			if err != nil {
				return nil, err
			}
			gatewayUrls = append(gatewayUrls, u)
		}
	}

	exchangeRates := exchange.NewBitcoinPriceFetcher(nil)

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

	// OpenBazaar node setup
	core.Node = &core.OpenBazaarNode{
		RepoPath:          config.RepoPath,
		Datastore:         sqliteDB,
		Wallet:            wallet,
		NameSystem:        ns,
		ExchangeRates:     exchangeRates,
		CrosspostGateways: gatewayUrls,
		UserAgent:         core.USERAGENT,
		BanManager:        bm,
	}

	if len(cfg.Addresses.Gateway) <= 0 {
		return nil, errors.New("No gateway addresses configured")
	}

	return &Node{node: core.Node, config: config, ipfsConfig: ncfg, apiConfig: apiConfig}, nil
}

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

func (n *Node) Start() error {
	nd, ctx, err := n.startIPFSNode(n.config.RepoPath, n.ipfsConfig)
	if err != nil {
		return err
	}

	n.node.Context = ctx
	n.node.IpfsNode = nd

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
	n.node.RootHash = ipath.Path(e.Value).String()

	// Offline messaging storage
	n.node.MessageStorage = selfhosted.NewSelfHostedStorage(n.node.RepoPath, ctx, n.node.CrosspostGateways, nil)

	// Start gateway
	// Create authentication cookie
	var authCookie http.Cookie
	authCookie.Name = "OpenBazaar_Auth_Cookie"

	if n.config.AuthenticationToken != "" {
		authCookie.Value = n.config.AuthenticationToken
		n.apiConfig.Authenticated = true
	}
	gateway, err := newHTTPGateway(core.Node, authCookie, *n.apiConfig)
	if err != nil {
		return err
	}
	go gateway.Serve()

	go func() {
		<-ipfscore.DefaultBootstrapConfig.DoneChan
		n.node.Service = service.New(n.node, n.node.Context, n.node.Datastore)
		MR := ret.NewMessageRetriever(n.node.Datastore, n.node.Context, n.node.IpfsNode, n.node.BanManager, n.node.Service, 14, nil, n.node.CrosspostGateways, n.node.SendOfflineAck)
		go MR.Run()
		n.node.MessageRetriever = MR
		PR := rep.NewPointerRepublisher(n.node.IpfsNode, n.node.Datastore, n.node.IsModerator)
		go PR.Run()
		n.node.PointerRepublisher = PR
		MR.Wait()
		TL := lis.NewTransactionListener(n.node.Datastore, n.node.Broadcast, n.node.Wallet)
		WL := lis.NewWalletListener(n.node.Datastore, n.node.Broadcast)
		n.node.Wallet.AddTransactionListener(TL.OnTransactionReceived)
		n.node.Wallet.AddTransactionListener(WL.OnTransactionReceived)
		su := bitcoin.NewStatusUpdater(n.node.Wallet, n.node.Broadcast, n.node.IpfsNode.Context())
		go su.Start()
		go n.node.Wallet.Start()

		n.node.UpdateFollow()
		n.node.SeedNode()
	}()

	return nil
}

func (n *Node) Stop() error {
	core.OfflineMessageWaitGroup.Wait()
	core.Node.Datastore.Close()
	repoLockFile := filepath.Join(core.Node.RepoPath, lockfile.LockFile)
	os.Remove(repoLockFile)
	core.Node.Wallet.Close()
	core.Node.IpfsNode.Close()
	return nil
}

func initializeRepo(dataDir, password, mnemonic string, testnet bool, creationDate time.Time) (*db.SQLiteDatastore, error) {
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

// Collects options, creates listener, prints status message and starts serving requests
func newHTTPGateway(node *core.OpenBazaarNode, authCookie http.Cookie, config repo.APIConfig) (*api.Gateway, error) {
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

	gwLis, err := manet.Listen(gatewayMaddr)
	if err != nil {
		return nil, fmt.Errorf("newHTTPGateway: manet.Listen(%s) failed: %s", gatewayMaddr, err)
	}

	// We might have listened to /tcp/0 - let's see what we are listing on
	gatewayMaddr = gwLis.Multiaddr()

	// Setup an options slice
	var opts = []corehttp.ServeOption{
		corehttp.MetricsCollectionOption("gateway"),
		corehttp.CommandsROOption(node.Context),
		corehttp.VersionOption(),
		corehttp.IPNSHostnameOption(),
		corehttp.GatewayOption(config.Authenticated, config.AllowedIPs, authCookie, config.Username, config.Password, cfg.Gateway.Writable, "/ipfs", "/ipns"),
	}

	if len(cfg.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
	}

	if err != nil {
		return nil, fmt.Errorf("newHTTPGateway: ConstructNode() failed: %s", err)
	}

	return api.NewGateway(node, authCookie, gwLis.NetListener(), config, logger, opts...)
}

var DHTClientOption ipfscore.RoutingOption = constructClientDHTRouting

func constructClientDHTRouting(ctx context.Context, host p2phost.Host, dstore ipfsrepo.Datastore) (routing.IpfsRouting, error) {
	dhtRouting := dht.NewDHTClient(ctx, host, dstore)
	dhtRouting.Validator[ipfscore.IpnsValidatorTag] = namesys.IpnsRecordValidator
	dhtRouting.Selector[ipfscore.IpnsValidatorTag] = namesys.IpnsSelectorFunc
	return dhtRouting, nil
}
