package main

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
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"

	"bufio"
	"crypto/rand"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"crypto/sha256"
	"encoding/hex"
	bstk "github.com/OpenBazaar/go-blockstackclient"
	"github.com/OpenBazaar/openbazaar-go/api"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"github.com/OpenBazaar/openbazaar-go/bitcoin/bitcoind"
	"github.com/OpenBazaar/openbazaar-go/bitcoin/exchange"
	lis "github.com/OpenBazaar/openbazaar-go/bitcoin/listeners"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	obns "github.com/OpenBazaar/openbazaar-go/namesys"
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
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/base58"
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
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	lockfile "github.com/ipfs/go-ipfs/repo/fsrepo/lock"
	"github.com/ipfs/go-ipfs/thirdparty/ds-help"
	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/go-homedir"
	"github.com/natefinch/lumberjack"
	"github.com/op/go-logging"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/proxy"
	routing "gx/ipfs/QmPR2JzfKd9poHx9XBhzoFeBBC31ZM3W5iUPKJZWyaoZZm/go-libp2p-routing"
	pstore "gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	metrics "gx/ipfs/QmQbh3Rb7KM37As3vkHYnEFnzkVXNCP8EYGtHz6g2fXk14/go-libp2p-metrics"
	"gx/ipfs/QmQq9YzmdFdWNTDdArueGyD7L5yyiRQigrRHJnTGkxcEjT/go-libp2p-interface-pnet"
	p2pbhost "gx/ipfs/QmRQ76P5dgvxTujhfPsCRAG83rC15jgb1G9bKLuomuC6dQ/go-libp2p/p2p/host/basic"
	addrutil "gx/ipfs/QmVJGsPeK3vwtEyyTxpCs47yjBYMmYsAhEouPDF3Gb2eK3/go-addr-util"
	oniontp "gx/ipfs/QmVYZ6jGE4uogWAZK2w8PrKWDEKMvYaQWTSXWCbYJLEuKs/go-onion-transport"
	swarm "gx/ipfs/QmWpJ4y2vxJ6GZpPfQbpVpQxAYS3UeR6AKNbAHxw7wN3qw/go-libp2p-swarm"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	smux "gx/ipfs/QmY9JXR3FupnYAYJWK9aMr9bCpqWKcToQ1tz8DVGTrHpHw/go-stream-muxer"
	p2phost "gx/ipfs/QmaSxYRuMq4pkpBBG2CYaRrPx2z7NmMVEs34b9g61biQA6/go-libp2p-host"
	recpb "gx/ipfs/QmbxkgUceEcuSZ4ZdBA3x74VUDSSYjHYmmeEqkjxbtZ6Jg/go-libp2p-record/pb"
	dht "gx/ipfs/Qmcjua7379qzY63PJ5a8w3mDteHZppiX2zo6vFeaqjVcQi/go-libp2p-kad-dht"
	dhtutil "gx/ipfs/Qmcjua7379qzY63PJ5a8w3mDteHZppiX2zo6vFeaqjVcQi/go-libp2p-kad-dht/util"
	"syscall"
	"time"
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
	Password           string `short:"p" long:"password" description:"the encryption password if the database is to be encrypted"`
	DataDir            string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Mnemonic           string `short:"m" long:"mnemonic" description:"specify a mnemonic seed to use to derive the keychain"`
	Testnet            bool   `short:"t" long:"testnet" description:"use the test network"`
	Force              bool   `short:"f" long:"force" description:"force overwrite existing repo (dangerous!)"`
	WalletCreationDate string `short:"w" long:"walletcreationdate" description:"specify the date the seed was created. if omitted the wallet will sync from the oldest checkpoint."`
}
type Status struct {
	DataDir string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Testnet bool   `short:"t" long:"testnet" description:"use the test network"`
}
type SetAPICreds struct {
	DataDir string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Testnet bool   `short:"t" long:"testnet" description:"config file is for testnet node"`
}
type Start struct {
	Password             string   `short:"p" long:"password" description:"the encryption password if the database is encrypted"`
	Testnet              bool     `short:"t" long:"testnet" description:"use the test network"`
	Regtest              bool     `short:"r" long:"regtest" description:"run in regression test mode"`
	LogLevel             string   `short:"l" long:"loglevel" description:"set the logging level [debug, info, notice, warning, error, critical]"`
	AllowIP              []string `short:"a" long:"allowip" description:"only allow API connections from these IPs"`
	STUN                 bool     `short:"s" long:"stun" description:"use stun on µTP IPv4"`
	DataDir              string   `short:"d" long:"datadir" description:"specify the data directory to be used"`
	AuthCookie           string   `short:"c" long:"authcookie" description:"turn on API authentication and use this specific cookie"`
	UserAgent            string   `short:"u" long:"useragent" description:"add a custom user-agent field"`
	TorPassword          string   `long:"torpassword" description:"Set the tor control password. This will override the tor password in the config."`
	Tor                  bool     `long:"tor" description:"Automatically configure the daemon to run as a Tor hidden service and use Tor exclusively. Requires Tor to be running."`
	DualStack            bool     `long:"dualstack" description:"Automatically configure the daemon to run as a Tor hidden service IN ADDITION to using the clear internet. Requires Tor to be running. WARNING: this mode is not private"`
	DisableWallet        bool     `long:"disablewallet" description:"disable the wallet functionality of the node"`
	DisableExchangeRates bool     `long:"disableexchangerates" description:"disable the exchange rate service to prevent api queries"`
	Storage              string   `long:"storage" description:"set the outgoing message storage option [self-hosted, dropbox] default=self-hosted"`
}
type Opts struct {
	Version bool `short:"v" long:"version" description:"Print the version number and exit"`
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
var setAPICreds SetAPICreds
var status Status
var opts Opts

var parser = flags.NewParser(&opts, flags.Default)

var ErrNoGateways = errors.New("No gateway addresses configured")

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			log.Noticef("Received %s\n", sig)
			log.Info("OpenBazaar Server shutting down...")
			if core.Node != nil {
				core.OfflineMessageWaitGroup.Wait()
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
	parser.AddCommand("status",
		"get the repo status",
		"Returns the status of the repo ― Uninitialized, Encrypted, Decrypted. Also returns whether Tor is available.",
		&status)
	parser.AddCommand("setapicreds",
		"set API credentials",
		"The API password field in the config file takes a SHA256 hash of the password. This command will generate the hash for you and save it to the config file.",
		&setAPICreds)
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
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(core.VERSION)
		return
	}
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

func (x *SetAPICreds) Execute(args []string) error {
	// Set repo path
	repoPath, err := getRepoPath(x.Testnet)
	if err != nil {
		return err
	}
	if x.DataDir != "" {
		repoPath = x.DataDir
	}
	r, err := fsrepo.Open(repoPath)
	if err != nil {
		log.Error(err)
		return err
	}
	configFile, err := ioutil.ReadFile(path.Join(repoPath, "config"))
	if err != nil {
		return err
	}
	apiCfg, err := repo.GetAPIConfig(configFile)
	if err != nil {
		log.Error(err)
		return err
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter username: ")
	username, _ := reader.ReadString('\n')

	var pw string
	for {
		fmt.Print("Enter a veerrrry strong password: ")
		bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
		fmt.Println("")
		resp := string(bytePassword)
		if len(resp) < 8 {
			fmt.Println("You call that a password? Try again.")
		} else if resp != "" {
			pw = resp
			break
		} else {
			fmt.Println("Seriously, enter a password.")
		}
	}
	for {
		fmt.Print("Confirm your password: ")
		bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
		fmt.Println("")
		resp := string(bytePassword)
		if resp == pw {
			break
		} else {
			fmt.Println("Quit effin around. Try again.")
		}
	}
	pw = strings.Replace(pw, "'", "''", -1)
	if strings.Contains(username, "\r\n") {
		apiCfg.Username = strings.Replace(username, "\r\n", "", -1)
	} else if strings.Contains(username, "\n") {
		apiCfg.Username = strings.Replace(username, "\n", "", -1)
	}
	apiCfg.Authenticated = true
	h := sha256.Sum256([]byte(pw))
	apiCfg.Password = hex.EncodeToString(h[:])
	if len(apiCfg.AllowedIPs) == 0 {
		apiCfg.AllowedIPs = []string{}
	}

	if err := r.SetConfigKey("JSON-API", apiCfg); err != nil {
		return err
	}
	return nil
}

func (x *Status) Execute(args []string) error {
	// Set repo path
	repoPath, err := getRepoPath(x.Testnet)
	if err != nil {
		return err
	}
	if x.DataDir != "" {
		repoPath = x.DataDir
	}
	torAvailable := false
	_, err = obnet.GetTorControlPort()
	if err == nil {
		torAvailable = true
	}
	if fsrepo.IsInitialized(repoPath) {
		sqliteDB, err := db.Create(repoPath, "", x.Testnet)
		if err != nil {
			return err
			os.Exit(1)
		}
		defer sqliteDB.Close()
		if sqliteDB.Config().IsEncrypted() {
			if !torAvailable {
				fmt.Println("Initialized - Encrypted")
				os.Exit(30)
			} else {
				fmt.Println("Initialized - Encrypted")
				fmt.Println("Tor Available")
				os.Exit(31)
			}
		} else {
			if !torAvailable {
				fmt.Println("Initialized - Not Encrypted")
				os.Exit(20)
			} else {
				fmt.Println("Initialized - Not Encrypted")
				fmt.Println("Tor Available")
				os.Exit(21)
			}
		}
	} else {
		if !torAvailable {
			fmt.Println("Not initialized")
			os.Exit(10)
		} else {
			fmt.Println("Not initialized")
			fmt.Println("Tor Available")
			os.Exit(11)
		}
	}
	return nil
}

func (x *Init) Execute(args []string) error {
	// Set repo path
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
	creationDate := time.Now()
	if x.WalletCreationDate != "" {
		creationDate, err = time.Parse(time.RFC3339, x.WalletCreationDate)
		if err != nil {
			return errors.New("Wallet creation date timestamp must be in RFC3339 format")
		}
	}

	_, err = initializeRepo(repoPath, x.Password, x.Mnemonic, x.Testnet, creationDate)
	if err == repo.ErrRepoExists && x.Force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Force overwriting the db will destroy your existing keys and history. Are you really, really sure you want to continue? (y/n): ")
		resp, _ := reader.ReadString('\n')
		if strings.ToLower(resp) == "y\n" || strings.ToLower(resp) == "yes\n" {
			os.RemoveAll(repoPath)
			_, err = initializeRepo(repoPath, x.Password, x.Mnemonic, x.Testnet, creationDate)
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

	if x.Tor && x.DualStack {
		return errors.New("Invalid combination of tor and dual stack modes")
	}

	isTestnet := false
	if x.Testnet || x.Regtest {
		isTestnet = true
	}

	// Set repo path
	repoPath, err := getRepoPath(isTestnet)
	if err != nil {
		return err
	}
	if x.DataDir != "" {
		repoPath = x.DataDir
	}

	repoLockFile := filepath.Join(repoPath, lockfile.LockFile)
	os.Remove(repoLockFile)

	sqliteDB, err := initializeRepo(repoPath, x.Password, "", isTestnet, time.Now())
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
	backendStdout := logging.NewLogBackend(os.Stdout, "", 0)
	backendFile := logging.NewLogBackend(w, "", 0)
	backendStdoutFormatter := logging.NewBackendFormatter(backendStdout, stdoutLogFormat)
	backendFileFormatter := logging.NewBackendFormatter(backendFile, fileLogFormat)
	logging.SetBackend(backendFileFormatter, backendStdoutFormatter)

	ipfslogging.LdJSONFormatter()
	w2 := &lumberjack.Logger{
		Filename:   path.Join(repoPath, "logs", "ipfs.log"),
		MaxSize:    10, // Megabytes
		MaxBackups: 3,
		MaxAge:     30, // Days
	}
	ipfslogging.Output(w2)()

	// If the database cannot be decrypted, exit
	if sqliteDB.Config().IsEncrypted() {
		sqliteDB.Close()
		fmt.Print("Database is encrypted, enter your password: ")
		bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
		fmt.Println("")
		pw := string(bytePassword)
		sqliteDB, err = initializeRepo(repoPath, pw, "", isTestnet, time.Now())
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
	gatewayUrlStrings, err := repo.GetCrosspostGateway(configFile)
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
	if x.Testnet {
		testnetBootstrapAddrs, err := repo.GetTestnetBootstrapAddrs(configFile)
		if err != nil {
			log.Error(err)
			return err
		}
		cfg.Bootstrap = testnetBootstrapAddrs
		dht.ProtocolDHT = "/openbazaar/kad/testnet/1.0.0"
		bitswap.ProtocolBitswap = "/openbazaar/bitswap/testnet/1.1.0"
		service.ProtocolOpenBazaar = "/openbazaar/app/testnet/1.0.0"

		gatewayUrlStrings = []string{}
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
	if x.Regtest && strings.ToLower(walletCfg.Type) == "spvwallet" && walletCfg.TrustedPeer == "" {
		return errors.New("Trusted peer must be set if using regtest with the spvwallet")
	}

	w3 := &lumberjack.Logger{
		Filename:   path.Join(repoPath, "logs", "bitcoin.log"),
		MaxSize:    10, // Megabytes
		MaxBackups: 3,
		MaxAge:     30, // Days
	}
	bitcoinFile := logging.NewLogBackend(w3, "", 0)
	bitcoinFileFormatter := logging.NewBackendFormatter(bitcoinFile, fileLogFormat)
	ml := logging.MultiLogger(bitcoinFileFormatter)
	var cryptoWallet wallet.Wallet
	switch strings.ToLower(walletCfg.Type) {
	case "spvwallet":
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
	case "bitcoind":
		if walletCfg.Binary == "" {
			return errors.New("The path to the bitcoind binary must be specified in the config file when using bitcoind")
		}
		usetor := false
		if usingTor && !usingClearnet {
			usetor = true
		}
		cryptoWallet = bitcoind.NewBitcoindWallet(mn, &params, repoPath, walletCfg.TrustedPeer, walletCfg.Binary, walletCfg.RPCUser, walletCfg.RPCPassword, usetor, controlPort)
	default:
		log.Fatal("Unknown wallet type")
	}

	// Crosspost gateway
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

	// Offline messaging storage
	var storage sto.OfflineMessagingStorage
	if x.Storage == "self-hosted" || x.Storage == "" {
		storage = selfhosted.NewSelfHostedStorage(repoPath, ctx, gatewayUrls, torDialer)
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

	var exchangeRates bitcoin.ExchangeRates
	if !x.DisableExchangeRates {
		exchangeRates = exchange.NewBitcoinPriceFetcher(torDialer)
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
		Context:           ctx,
		IpfsNode:          nd,
		RootHash:          ipath.Path(e.Value).String(),
		RepoPath:          repoPath,
		Datastore:         sqliteDB,
		Wallet:            cryptoWallet,
		MessageStorage:    storage,
		NameSystem:        ns,
		ExchangeRates:     exchangeRates,
		CrosspostGateways: gatewayUrls,
		TorDialer:         torDialer,
		UserAgent:         core.USERAGENT,
		BanManager:        bm,
	}

	if len(cfg.Addresses.Gateway) <= 0 {
		return ErrNoGateways
	}
	if (apiConfig.SSL && apiConfig.SSLCert == "") || (apiConfig.SSL && apiConfig.SSLKey == "") {
		return errors.New("SSL cert and key files must be set when SSL is enabled")
	}

	gateway, err := newHTTPGateway(core.Node, authCookie, *apiConfig)
	if err != nil {
		log.Error(err)
		return err
	}

	go func() {
		<-ipfscore.DefaultBootstrapConfig.DoneChan
		core.Node.Service = service.New(core.Node, ctx, sqliteDB)
		MR := ret.NewMessageRetriever(sqliteDB, ctx, nd, bm, core.Node.Service, 14, torDialer, core.Node.CrosspostGateways, core.Node.SendOfflineAck)
		go MR.Run()
		core.Node.MessageRetriever = MR
		PR := rep.NewPointerRepublisher(nd, sqliteDB, core.Node.IsModerator)
		go PR.Run()
		core.Node.PointerRepublisher = PR
		if !x.DisableWallet {
			MR.Wait()
			TL := lis.NewTransactionListener(core.Node.Datastore, core.Node.Broadcast, core.Node.Wallet)
			WL := lis.NewWalletListener(core.Node.Datastore, core.Node.Broadcast)
			cryptoWallet.AddTransactionListener(TL.OnTransactionReceived)
			cryptoWallet.AddTransactionListener(WL.OnTransactionReceived)
			log.Info("Starting bitcoin wallet")
			su := bitcoin.NewStatusUpdater(cryptoWallet, core.Node.Broadcast, nd.Context())
			go su.Start()
			go cryptoWallet.Start()
		}
		core.Node.UpdateFollow()
		core.Node.SeedNode()
	}()

	// Start gateway
	err = gateway.Serve()
	if err != nil {
		log.Error(err)
	}

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
	w4 := &lumberjack.Logger{
		Filename:   path.Join(node.RepoPath, "logs", "api.log"),
		MaxSize:    10, // Megabytes
		MaxBackups: 3,
		MaxAge:     30, // Days
	}
	apiFile := logging.NewLogBackend(w4, "", 0)
	apiFileFormatter := logging.NewBackendFormatter(apiFile, fileLogFormat)
	ml := logging.MultiLogger(apiFileFormatter)

	return api.NewGateway(node, authCookie, gwLis.NetListener(), config, ml, opts...)
}

/* Returns the directory to store repo data in.
   It depends on the OS and whether or not we are on testnet. */
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
		directoryName += "-testnet"
	}

	// Join the path and directory name, then expand the home path
	fullPath, err := homedir.Expand(filepath.Join(path, directoryName))
	if err != nil {
		return "", err
	}

	// Return the shortest lexical representation of the path
	return filepath.Clean(fullPath), nil
}

var DHTOption ipfscore.RoutingOption = constructDHTRouting

func constructDHTRouting(ctx context.Context, host p2phost.Host, dstore ipfsrepo.Datastore) (routing.IpfsRouting, error) {
	dhtRouting := dht.NewDHT(ctx, host, dstore)
	dhtRouting.Validator[ipfscore.IpnsValidatorTag] = namesys.IpnsRecordValidator
	dhtRouting.Selector[ipfscore.IpnsValidatorTag] = namesys.IpnsSelectorFunc
	return dhtRouting, nil
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
	fmt.Println("OpenBazaar Server v" + core.VERSION + " starting...")
}
