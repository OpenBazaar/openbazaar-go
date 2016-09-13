package main

import (
	"errors"
	"fmt"
	ipfslogging "gx/ipfs/QmNQynaz7qfriSUJkiEZUrm2Wen1u3Kj9goZzWtrPyu7XR/go-log"
	manet "gx/ipfs/QmPpRcbNUXauP3zWZ1NJMLWpe4QnmEHrd2ba2D3yqWznw7/go-multiaddr-net"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	"net"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"

	"bufio"
	bstk "github.com/OpenBazaar/go-blockstackclient"
	"github.com/OpenBazaar/openbazaar-go/api"
	"github.com/OpenBazaar/openbazaar-go/bitcoin/exchange"
	lis "github.com/OpenBazaar/openbazaar-go/bitcoin/listeners"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	obnet "github.com/OpenBazaar/openbazaar-go/net"
	rep "github.com/OpenBazaar/openbazaar-go/net/repointer"
	ret "github.com/OpenBazaar/openbazaar-go/net/retriever"
	"github.com/OpenBazaar/openbazaar-go/net/service"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	sto "github.com/OpenBazaar/openbazaar-go/storage"
	"github.com/OpenBazaar/openbazaar-go/storage/dropbox"
	"github.com/OpenBazaar/openbazaar-go/storage/selfhosted"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/fatih/color"
	"github.com/ipfs/go-ipfs/commands"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/ipfs/go-ipfs/namesys"
	namepb "github.com/ipfs/go-ipfs/namesys/pb"
	ipath "github.com/ipfs/go-ipfs/path"
	"github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	lockfile "github.com/ipfs/go-ipfs/repo/fsrepo/lock"
	dhtpb "github.com/ipfs/go-ipfs/routing/dht/pb"
	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/go-homedir"
	"github.com/natefinch/lumberjack"
	"github.com/op/go-logging"
	"net/url"
	"strings"
)

var log = logging.MustGetLogger("main")

var stdoutLogFormat = logging.MustStringFormatter(
	`%{color:reset}%{color}%{time:15:04:05.000} [%{shortfunc}] [%{level}] %{message}`,
)

var fileLogFormat = logging.MustStringFormatter(
	`%{time:15:04:05.000} [%{shortfunc}] [%{level}] %{message}`,
)

var encryptedDatabaseError = errors.New("could not decrypt the database")

type Init struct {
	Password string `short:"p" long:"password" description:"the encryption password if the database is to be encrypted"`
	DataDir  string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Mnemonic string `short:"m" long:"mnemonic" description:"speficy a mnemonic seed to use to derive the keychain"`
	Testnet  bool   `short:"t" long:"testnet" description:"use the test network"`
	Force    bool   `short:"f" long:"force" description:"force overwrite existing repo (dangerous!)"`
}

type Start struct {
	Password      string   `short:"p" long:"password" description:"the encryption password if the database is encrypted"`
	Testnet       bool     `short:"t" long:"testnet" description:"use the test network"`
	Regtest       bool     `short:"r" long:"regtest" description:"run in regression test mode"`
	LogLevel      string   `short:"l" long:"loglevel" description:"set the logging level [debug, info, notice, warning, error, critical]"`
	AllowIP       []string `short:"a" long:"allowip" description:"only allow API connections from these IPs"`
	STUN          bool     `short:"s" long:"stun" description:"use stun on µTP IPv4"`
	DataDir       string   `short:"d" long:"datadir" description:"specify the data directory to be used"`
	DisableWallet bool     `long:"disablewallet" description:"disable the wallet functionality of the node"`
	Storage       string   `long:"storage" description:"set the outgoing message storage option [self-hosted, dropbox] default=self-hosted"`
}
type Stop struct{}
type Restart struct{}
type EncryptDatabase struct{}
type DecryptDatabase struct{}

var initRepo Init
var startServer Start
var stopServer Stop
var restartServer Restart
var encryptDatabase EncryptDatabase
var decryptDatabase DecryptDatabase

var parser = flags.NewParser(nil, flags.Default)

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			log.Noticef("Received %s\n", sig)
			log.Info("OpenBazaar Server shutting down...")
			if core.Node != nil {
				core.Node.Datastore.Close()
				repoLockFile := filepath.Join(core.Node.RepoPath, lockfile.LockFile)
				os.Remove(repoLockFile)
				core.Node.Wallet.Close()
				core.Node.IpfsNode.Close()
			}
			os.Exit(1)
		}
	}()

	parser.AddCommand("init",
		"initialize a new repo and exit",
		"Initializes a new repo without starting the server",
		&initRepo)
	parser.AddCommand("start",
		"start the OpenBazaar-Server",
		"The start command starts the OpenBazaar-Server",
		&startServer)
	parser.AddCommand("stop",
		"shutdown the server and disconnect",
		"The stop command disconnects from peers and shuts down OpenBazaar-Server",
		&stopServer)
	parser.AddCommand("restart",
		"restart the server",
		"The restart command shuts down the server and restarts",
		&restartServer)
	parser.AddCommand("encryptdatabase",
		"encrypt your database",
		"This command encrypts the database containing your bitcoin private keys, identity key, and contracts",
		&encryptDatabase)
	parser.AddCommand("decryptdatabase",
		"decrypt your database",
		"This command decrypts the database containing your bitcoin private keys, identity key, and contracts.\n [Warning] doing so may put your bitcoins at risk.",
		&decryptDatabase)

	if _, err := parser.Parse(); err != nil {
		os.Exit(1)
	}
}

func (x *EncryptDatabase) Execute(args []string) error {
	return db.Encrypt()
}

func (x *DecryptDatabase) Execute(args []string) error {
	return db.Decrypt()
}

func (x *Init) Execute(args []string) error {
	// set repo path
	repoPath, err := getRepoPath(x.Testnet)
	if err != nil {
		return err
	}
	if x.DataDir != "" {
		repoPath = x.DataDir
	}
	if x.Password != "" {
		x.Password = strings.Replace(x.Password, "'", "''", -1)
	}

	_, err = initializeRepo(repoPath, x.Password, x.Mnemonic, x.Testnet)
	if err == repo.ErrRepoExists && x.Force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Force overwriting the db will destroy your existing keys and history. Are you really, really sure you want to continue? (y/n): ")
		resp, _ := reader.ReadString('\n')
		if strings.ToLower(resp) == "y\n" || strings.ToLower(resp) == "yes\n" {
			os.RemoveAll(repoPath)
			_, err = initializeRepo(repoPath, x.Password, x.Mnemonic, x.Testnet)
			if err != nil {
				return err
			}
			fmt.Printf("OpenBazaar repo initialized at %s\n", repoPath)
			return nil
		} else {
			return nil
		}
	} else if err != nil {
		return err
	}
	fmt.Printf("OpenBazaar repo initialized at %s\n", repoPath)
	return nil
}

func (x *Start) Execute(args []string) error {
	printSplashScreen()
	var err error

	if x.Testnet && x.Regtest {
		return errors.New("Invalid combination of testnet and regtest modes")
	}

	isTestnet := false
	if x.Testnet || x.Regtest {
		isTestnet = true
	}

	// set repo path
	repoPath, err := getRepoPath(isTestnet)
	if err != nil {
		return err
	}
	if x.DataDir != "" {
		repoPath = x.DataDir
	}

	repoLockFile := filepath.Join(repoPath, lockfile.LockFile)
	os.Remove(repoLockFile)

	sqliteDB, err := initializeRepo(repoPath, x.Password, "", isTestnet)
	if err != nil && err != repo.ErrRepoExists {
		return err
	}

	// logging
	w := &lumberjack.Logger{
		Filename:   path.Join(repoPath, "logs", "ob.log"),
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     30, //days
	}
	backendStdout := logging.NewLogBackend(os.Stdout, "", 0)
	backendFile := logging.NewLogBackend(w, "", 0)
	backendStdoutFormatter := logging.NewBackendFormatter(backendStdout, stdoutLogFormat)
	backendFileFormatter := logging.NewBackendFormatter(backendFile, fileLogFormat)
	logging.SetBackend(backendFileFormatter, backendStdoutFormatter)

	ipfslogging.LdJSONFormatter()
	w2 := &lumberjack.Logger{
		Filename:   path.Join(repoPath, "logs", "ipfs.log"),
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     30, //days
	}
	ipfslogging.Output(w2)()

	// if the db can't be decrypted, exit
	if sqliteDB.Config().IsEncrypted() {
		return encryptedDatabaseError
	}

	// ipfs node setup
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

	// Run stun and set uTP port
	if x.STUN {
		for i, addr := range cfg.Addresses.Swarm {
			m, _ := ma.NewMultiaddr(addr)
			p := m.Protocols()
			if p[0].Name == "ip4" && p[1].Name == "udp" && p[2].Name == "utp" {
				port, serr := obnet.Stun()
				if serr != nil {
					log.Error(serr)
					return err
				}
				cfg.Addresses.Swarm = append(cfg.Addresses.Swarm[:i], cfg.Addresses.Swarm[i+1:]...)
				cfg.Addresses.Swarm = append(cfg.Addresses.Swarm, "/ip4/0.0.0.0/udp/"+strconv.Itoa(port)+"/utp")
				break
			}
		}
	}

	ncfg := &ipfscore.BuildCfg{
		Repo:   r,
		Online: true,
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

	log.Info("Peer ID: ", nd.Identity.Pretty())
	printSwarmAddrs(nd)

	// Get current directory root hash
	_, ipnskey := namesys.IpnsKeysForID(nd.Identity)
	ival, hasherr := nd.Repo.Datastore().Get(ipnskey.DsKey())
	if hasherr != nil {
		log.Error("Error getting current directory root hash")
		log.Error(hasherr)
		return hasherr
	}
	val := ival.([]byte)
	dhtrec := new(dhtpb.Record)
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
	maxFee, err := repo.GetMaxFee(path.Join(repoPath, "config"))
	if err != nil {
		log.Error(err)
		return err
	}
	feeApi, err := repo.GetFeeAPI(path.Join(repoPath, "config"))
	if err != nil {
		log.Error(err)
		return err
	}
	low, medium, high, err := repo.GetDefaultFees(path.Join(repoPath, "config"))
	if err != nil {
		log.Error(err)
		return err
	}
	trustedPeer, err := repo.GetTrustedBitcoinPeer(path.Join(repoPath, "config"))
	if err != nil {
		log.Error(err)
		return err
	}

	w3 := &lumberjack.Logger{
		Filename:   path.Join(repoPath, "logs", "bitcoin.log"),
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     30, //days
	}
	bitcoinFile := logging.NewLogBackend(w3, "", 0)
	bitcoinFileFormatter := logging.NewBackendFormatter(bitcoinFile, fileLogFormat)
	ml := logging.MultiLogger(bitcoinFileFormatter)
	wallet := spvwallet.NewSPVWallet(mn, &params, maxFee, high, medium, low, feeApi, repoPath, sqliteDB, "OpenBazaar", trustedPeer, ml)

	// Crosspost gateway
	gatewayUrlStrings, err := repo.GetCrosspostGateway(path.Join(repoPath, "config"))
	if err != nil {
		log.Error(err)
		return err
	}
	var gatewayUrls []*url.URL
	for _, gw := range gatewayUrlStrings {
		if gw != "" {
			u, err := url.Parse(gw)
			if err != nil {
				log.Error(err)
				return err
			}
			gatewayUrls = append(gatewayUrls, u)
		}
	}

	// Offline messaging storage
	var storage sto.OfflineMessagingStorage
	if x.Storage == "self-hosted" || x.Storage == "" {
		storage = selfhosted.NewSelfHostedStorage(repoPath, ctx, gatewayUrls)
	} else if x.Storage == "dropbox" {
		token, err := repo.GetDropboxApiToken(path.Join(repoPath, "config"))
		if err != nil {
			log.Error(err)
			return err
		} else if token == "" {
			err = errors.New("Dropbox token not set in config file")
			log.Error(err)
			return err
		}
		storage, err = dropbox.NewDropBoxStorage(token)
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		err = errors.New("Invalid storage option")
		log.Error(err)
		return err
	}

	// Resolver
	resolverUrl, err := repo.GetResolverUrl(path.Join(repoPath, "config"))
	if err != nil {
		log.Error(err)
		return err
	}

	// OpenBazaar node setup
	core.Node = &core.OpenBazaarNode{
		Context:           ctx,
		IpfsNode:          nd,
		RootHash:          ipath.Path(e.Value).String(),
		RepoPath:          repoPath,
		Datastore:         sqliteDB,
		Wallet:            wallet,
		MessageStorage:    storage,
		Resolver:          bstk.NewBlockStackClient(resolverUrl),
		ExchangeRates:     exchange.NewBitcoinPriceFetcher(),
		CrosspostGateways: gatewayUrls,
	}

	var gwErrc <-chan error
	var cb <-chan bool
	if len(cfg.Addresses.Gateway) > 0 {
		sslEnabled, certFile, keyFile, err := repo.GetAPISSL(path.Join(repoPath, "config"))
		if err != nil {
			log.Error(err)
			return err
		}
		if (sslEnabled && certFile == "") || (sslEnabled && keyFile == "") {
			return errors.New("SSL cert and key files must be set when SSL is enabled")
		}
		err, cb, gwErrc = serveHTTPGateway(core.Node, sslEnabled, certFile, keyFile)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	// Wait for gateway to start before starting the network service.
	// This way the websocket channel we pass into the service gets created first.
	// FIXME: There has to be a better way
	for b := range cb {
		if b == true {
			OBService := service.SetupOpenBazaarService(core.Node, ctx, sqliteDB)
			core.Node.Service = OBService
			MR := ret.NewMessageRetriever(sqliteDB, ctx, nd, OBService, 16, core.Node.SendOfflineAck)
			go MR.Run()
			core.Node.MessageRetriever = MR
			PR := rep.NewPointerRepublisher(nd, sqliteDB)
			go PR.Run()
			core.Node.PointerRepublisher = PR
			if !x.DisableWallet {
				TL := lis.NewTransactionListener(core.Node.Datastore, core.Node.Broadcast, core.Node.Wallet.Params())
				core.Node.Wallet.AddTransactionListener(TL.OnTransactionReceived)
				log.Info("Starting bitcoin wallet...")
				go wallet.Start()
			}
			core.Node.SeedNode()
		}
		break
	}

	for err := range gwErrc {
		fmt.Println(err)
	}

	return nil
}

func initializeRepo(dataDir, password, mnemonic string, testnet bool) (*db.SQLiteDatastore, error) {
	// Database
	sqliteDB, err := db.Create(dataDir, password, testnet)
	if err != nil {
		return sqliteDB, err
	}

	// initialize the ipfs repo if it doesn't already exist
	err = repo.DoInit(dataDir, 4096, testnet, password, mnemonic, sqliteDB.Config().Init)
	if err != nil {
		return sqliteDB, err
	}
	return sqliteDB, nil
}

// printSwarmAddrs prints the addresses of the host
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

// serveHTTPGateway collects options, creates listener, prints status message and starts serving requests
func serveHTTPGateway(node *core.OpenBazaarNode, sslEnabled bool, certFile, keyFile string) (error, <-chan bool, <-chan error) {

	cfg, err := node.Context.GetConfig()
	if err != nil {
		return err, nil, nil
	}

	gatewayMaddr, err := ma.NewMultiaddr(cfg.Addresses.Gateway)
	if err != nil {
		return fmt.Errorf("serveHTTPGateway: invalid gateway address: %q (err: %s)", cfg.Addresses.Gateway, err), nil, nil
	}
	var gwLis manet.Listener
	if sslEnabled {
		netAddr, err := manet.ToNetAddr(gatewayMaddr)
		if err != nil {
			return err, nil, nil
		}
		gwLis, err = manet.WrapNetListener(&DummyListener{netAddr})
		if err != nil {
			return err, nil, nil
		}
	} else {
		gwLis, err = manet.Listen(gatewayMaddr)
		if err != nil {
			return fmt.Errorf("serveHTTPGateway: manet.Listen(%s) failed: %s", gatewayMaddr, err), nil, nil
		}
	}
	// we might have listened to /tcp/0 - lets see what we are listing on
	gatewayMaddr = gwLis.Multiaddr()

	log.Infof("Gateway/API server listening on %s\n", gatewayMaddr)

	var opts = []corehttp.ServeOption{
		corehttp.MetricsCollectionOption("gateway"),
		corehttp.CommandsROOption(node.Context),
		corehttp.VersionOption(),
		corehttp.IPNSHostnameOption(),
		corehttp.GatewayOption(node.Resolver, "/ipfs", "/ipns"),
	}

	if len(cfg.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
	}

	if err != nil {
		return fmt.Errorf("serveHTTPGateway: ConstructNode() failed: %s", err), nil, nil
	}
	errc := make(chan error)
	cb := make(chan bool)
	go func() {
		errc <- api.Serve(cb, node, node.Context, gwLis.NetListener(), sslEnabled, certFile, keyFile, opts...)
		close(errc)
	}()
	return nil, cb, errc
}

// getRepoPath returns the directory to store repo data in. It depends on the
// operating system and whether or not we're on testnet.
func getRepoPath(isTestnet bool) (string, error) {
	// Set default base path and directory name
	path := "~"
	directoryName := "OpenBazaar2.0"

	// Override OS-specific names
	switch runtime.GOOS {
	case "linux":
		directoryName = ".openbazaar2.0"
	case "darwin":
		path = "~/Library/Application Support"
	}

	// Append testnet flag if on testnet
	if isTestnet {
		directoryName = directoryName + "-testnet"
	}

	// Join the path and directory name, then expand the home path
	fullPath, err := homedir.Expand(filepath.Join(path, directoryName))
	if err != nil {
		return "", nil
	}

	// Return the shortest lexical representation of the path
	return filepath.Clean(fullPath), nil
}

func printSplashScreen() {
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
	fmt.Println("OpenBazaar Server v2.0 starting...")
}
