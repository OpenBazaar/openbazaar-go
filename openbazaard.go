package main

import (
	"errors"
	"fmt"
	"github.com/OpenBazaar/openbazaar-go/api"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
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
	manet "gx/ipfs/QmUBa4w6CbHJUMeGJPDiMEDWsM93xToK1fTnFXnrC8Hksw/go-multiaddr-net"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	ipfslogging "gx/ipfs/Qmazh5oNUVsDZTs2g59rq8aYQqwpss8tcUWQzor5sCCEuH/go-log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
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

type Start struct {
	Password      string   `short:"p" long:"password" description:"the encryption password if the database is encrypted"`
	Testnet       bool     `short:"t" long:"testnet" description:"use the test network"`
	LogLevel      string   `short:"l" long:"loglevel" description:"set the logging level [debug, info, notice, warning, error, critical]"`
	AllowIP       []string `short:"a" long:"allowip" description:"only allow API connections from these IPs"`
	STUN          bool     `short:"s" long:"stun" description:"use stun on µTP IPv4"`
	DisableWallet bool     `long:"disablewallet" description:"disable the wallet functionality of the node"`
	Storage       string   `long:"storage" description:"set the outgoing message storage option [self-hosted, dropbox] default=self-hosted"`
}
type Stop struct{}
type Restart struct{}
type EncryptDatabase struct{}
type DecryptDatabase struct{}

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
				core.Node.IpfsNode.Close()
			}
			os.Exit(1)
		}
	}()

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

func (x *Start) Execute(args []string) error {
	printSplashScreen()
	var err error

	// set repo path
	var obFolderName string
	if x.Testnet {
		obFolderName = "~/OpenBazaar2.0-testnet"
	} else {
		obFolderName = "~/OpenBazaar2.0"
	}
	if runtime.GOOS == "linux" {
		obFolderName = "~/." + strings.ToLower(obFolderName[2:])
	}
	expPath, _ := homedir.Expand(filepath.Clean(obFolderName))

	// Database
	sqliteDB, err := db.Create(expPath, x.Password, x.Testnet)
	if err != nil {
		return err
	}

	// logging
	w := &lumberjack.Logger{
		Filename:   path.Join(expPath, "logs", "ob.log"),
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
		Filename:   path.Join(expPath, "logs", "ipfs.log"),
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     30, //days
	}
	ipfslogging.Output(w2)()

	// initialize the ipfs repo if it doesn't already exist
	err = repo.DoInit(os.Stdout, expPath, 4096, x.Testnet, x.Password, sqliteDB.Config().Init)
	if err != nil && err != repo.ErrRepoExists {
		log.Error(err)
		return err
	}

	// if the db can't be decrypted, exit
	if sqliteDB.Config().IsEncrypted() {
		return encryptedDatabaseError
	}

	// ipfs node setup
	r, err := fsrepo.Open(obFolderName)
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
				port, serr := net.Stun()
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
	ctx.ConfigRoot = expPath
	ctx.LoadConfig = func(path string) (*config.Config, error) {
		return fsrepo.ConfigAt(expPath)
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
	if !x.Testnet {
		params = chaincfg.MainNetParams
	} else {
		params = chaincfg.TestNet3Params
	}
	maxFee, err := repo.GetMaxFee(path.Join(expPath, "config"))
	if err != nil {
		log.Error(err)
		return err
	}
	feeApi, err := repo.GetFeeAPI(path.Join(expPath, "config"))
	if err != nil {
		log.Error(err)
		return err
	}
	low, medium, high, err := repo.GetDefaultFees(path.Join(expPath, "config"))
	if err != nil {
		log.Error(err)
		return err
	}

	w3 := &lumberjack.Logger{
		Filename:   path.Join(expPath, "logs", "bitcoin.log"),
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     30, //days
	}
	bitcoinFile := logging.NewLogBackend(w3, "", 0)
	bitcoinFileFormatter := logging.NewBackendFormatter(bitcoinFile, fileLogFormat)
	ml := logging.MultiLogger(bitcoinFileFormatter)
	wallet := spvwallet.NewSPVWallet(mn, &params, maxFee, high, medium, low, feeApi, expPath, sqliteDB, "OpenBazaar", ml)
	if !x.DisableWallet {
		log.Info("Starting bitcoin wallet...")
		go wallet.Start()
	}

	// Offline messaging storage
	var storage sto.OfflineMessagingStorage
	if x.Storage == "self-hosted" || x.Storage == "" {
		storage = selfhosted.NewSelfHostedStorage(expPath, ctx)
	} else if x.Storage == "dropbox" {
		token, err := repo.GetDropboxApiToken(path.Join(expPath, "config"))
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

	// OpenBazaar node setup
	core.Node = &core.OpenBazaarNode{
		Context:        ctx,
		IpfsNode:       nd,
		RootHash:       ipath.Path(e.Value).String(),
		RepoPath:       expPath,
		Datastore:      sqliteDB,
		Wallet:         wallet,
		MessageStorage: storage,
	}

	var gwErrc <-chan error
	var cb <-chan bool
	if len(cfg.Addresses.Gateway) > 0 {
		var err error
		err, cb, gwErrc = serveHTTPGateway(core.Node)
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
			OBService := service.SetupOpenBazaarService(nd, core.Node.Broadcast, ctx, sqliteDB)
			core.Node.Service = OBService
			MR := net.NewMessageRetriever(sqliteDB, ctx, nd, OBService, 16, core.Node.SendOfflineAck)
			go MR.Run()
			core.Node.MessageRetriever = MR
			PR := net.NewPointerRepublisher(nd, sqliteDB)
			go PR.Run()
			core.Node.PointerRepublisher = PR
		}
		break
	}

	for err := range gwErrc {
		fmt.Println(err)
	}

	return nil
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

// serveHTTPGateway collects options, creates listener, prints status message and starts serving requests
func serveHTTPGateway(node *core.OpenBazaarNode) (error, <-chan bool, <-chan error) {

	cfg, err := node.Context.GetConfig()
	if err != nil {
		return nil, nil, nil
	}

	gatewayMaddr, err := ma.NewMultiaddr(cfg.Addresses.Gateway)
	if err != nil {
		return fmt.Errorf("serveHTTPGateway: invalid gateway address: %q (err: %s)", cfg.Addresses.Gateway, err), nil, nil
	}

	writable := cfg.Gateway.Writable

	gwLis, err := manet.Listen(gatewayMaddr)
	if err != nil {
		return fmt.Errorf("serveHTTPGateway: manet.Listen(%s) failed: %s", gatewayMaddr, err), nil, nil
	}
	// we might have listened to /tcp/0 - lets see what we are listing on
	gatewayMaddr = gwLis.Multiaddr()

	log.Infof("Gateway/API server listening on %s\n", gatewayMaddr)

	var opts = []corehttp.ServeOption{
		corehttp.MetricsCollectionOption("gateway"),
		corehttp.CommandsROOption(node.Context),
		corehttp.VersionOption(),
		corehttp.IPNSHostnameOption(),
		corehttp.GatewayOption(writable, cfg.Gateway.PathPrefixes),
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
		errc <- api.Serve(cb, node, node.Context, gwLis.NetListener(), opts...)
		close(errc)
	}()
	return nil, cb, errc
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
