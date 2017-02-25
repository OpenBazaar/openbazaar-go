package main

import (
	"context"
	"errors"
	"fmt"
	ipfslogging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	manet "gx/ipfs/QmT6Cp31887FpAc25z25YHgpFJohZedrYLWPPspRtj1Brp/go-multiaddr-net"
	ma "gx/ipfs/QmUAQaWbKxGCUTuoQVvvicbQNZ9APF5pDGWyAZSe93AtKH/go-multiaddr"
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
	"github.com/OpenBazaar/go-onion-transport"
	"github.com/OpenBazaar/openbazaar-go/api"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"github.com/OpenBazaar/openbazaar-go/bitcoin/bitcoind"
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
	"github.com/btcsuite/btcutil/base58"
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
	dht "github.com/ipfs/go-ipfs/routing/dht"
	"github.com/ipfs/go-ipfs/thirdparty/ds-help"
	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/go-homedir"
	"github.com/natefinch/lumberjack"
	"github.com/op/go-logging"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/proxy"
	p2phost "gx/ipfs/QmPsRtodRuBUir32nz5v4zuSBTSszrR1d3fA6Ahb6eaejj/go-libp2p-host"
	addrutil "gx/ipfs/QmVDnc2zvyQm8LhT72n22THcshvH7j3qPMnhvjerQER62T/go-addr-util"
	metrics "gx/ipfs/QmY2otvyPM2sTaDsczo7Yuosg98sUMCJ9qx1gpPaAPTS9B/go-libp2p-metrics"
	"gx/ipfs/Qmbiq2d2ZMi34A6V22kNY3b4GgPGFztmRCQZ931TJkYWp7/go-libp2p-swarm"
	recpb "gx/ipfs/QmdM4ohF7cr4MvAECVeD3hRA3HtZrk1ngaek4n8ojVT87h/go-libp2p-record/pb"
	p2pbhost "gx/ipfs/QmdzDdLZ7nj133QvNHypyS9Y39g35bMFk5DJ2pmX7YqtKU/go-libp2p/p2p/host/basic"
	pstore "gx/ipfs/QmeXj9VAjmYQZxpmVz7VzccbJrpmr8qkCDSjfVNsPTWTYU/go-libp2p-peerstore"
	smux "gx/ipfs/QmeZBgYBHvxMukGK5ojg28BCNLB9SeXqT7XXg6o7r2GbJy/go-stream-muxer"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
	"syscall"
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
	Tor                  bool     `long:"tor" description:"Automatically configure the daemon to run as a Tor hidden service and use Tor exclusively. Requires Tor to be running."`
	DualStack            bool     `long:"dualstack" description:"Automatically configure the daemon to run as a Tor hidden service IN ADDITION to using the clear internet. Requires Tor to be running. WARNING: this mode is not private"`
	DisableWallet        bool     `long:"disablewallet" description:"disable the wallet functionality of the node"`
	DisableExchangeRates bool     `long:"disableexchangerates" description:"disable the exchange rate service to prevent api queries"`
	Storage              string   `long:"storage" description:"set the outgoing message storage option [self-hosted, dropbox] default=self-hosted"`
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

var parser = flags.NewParser(nil, flags.Default)

var ErrNoGateways = errors.New("No gateway addresses configured")

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
	apiCfg, err := repo.GetAPIConfig(path.Join(repoPath, "config"))
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
	h := sha256.Sum256([]byte(pw))
	apiCfg.Password = hex.EncodeToString(h[:])

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

	sqliteDB, err := initializeRepo(repoPath, x.Password, "", isTestnet)
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
		log.Error("Invalid database encryption password")
		os.Exit(3)
	}

	// Create authentication cookie
	var authCookie http.Cookie
	authCookie.Name = "OpenBazaar_Auth_Cookie"
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
		defer f.Close()
		cookie := cookiePrefix + authCookie.Value
		_, werr := f.Write([]byte(cookie))
		if werr != nil {
			log.Error(werr)
			return werr
		}
	} else {
		if string(cookie)[:len(cookiePrefix)] != cookiePrefix {
			return errors.New("Invalid authentication cookie. Delete it to generate a new one.")
		}
		split := strings.SplitAfter(string(cookie), cookiePrefix)
		authCookie.Value = split[1]
	}

	// Create user-agent file
	userAgentBytes := []byte(core.USERAGENT)
	ioutil.WriteFile(path.Join(repoPath, "root", "user_agent"), userAgentBytes, os.ModePerm)

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

	onionAddr, err := obnet.MaybeCreateHiddenServiceKey(repoPath)
	if err != nil {
		log.Error(err)
		return err
	}
	onionAddrString := "/onion/" + onionAddr + ":4003"
	if x.Tor {
		// Only use Tor swarm address for Tor-only nodes
		cfg.Addresses.Swarm = []string{}
		cfg.Addresses.Swarm = append(cfg.Addresses.Swarm, onionAddrString)
	} else if x.DualStack {
		// Add Tor swarm address for dualstack nodes
		cfg.Addresses.Swarm = append(cfg.Addresses.Swarm, onionAddrString)
	}
	// Iterate over our address and process them as needed
	var onionTransport *torOnion.OnionTransport
	var torDialer proxy.Dialer
	var usingTor, usingClearnet bool
	var controlPort int
	for i, addr := range cfg.Addresses.Swarm {
		m, _ := ma.NewMultiaddr(addr)
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
			torConfig, err := repo.GetTorConfig(path.Join(repoPath, "config"))
			if err != nil {
				log.Error(err)
				return err
			}
			torControl := torConfig.TorControl
			if torControl == "" {
				controlPort, err = obnet.GetTorControlPort()
				if err != nil {
					log.Error(err)
					return err
				}
				torControl = "127.0.0.1:" + strconv.Itoa(controlPort)
			}
			auth := &proxy.Auth{Password: torConfig.Password}
			onionTransport, err = torOnion.NewOnionTransport("tcp4", torControl, auth, repoPath)
			if err != nil {
				log.Error(err)
				return err
			}
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
	// If we're only using Tor set the proxy dialer
	if usingTor && !usingClearnet {
		log.Notice("Using Tor exclusively")
		torDialer, err = onionTransport.TorDialer()
		if err != nil {
			log.Error(err)
			return err
		}
	}

	// Custom host option used if Tor is enabled
	defaultHostOption := func(ctx context.Context, id peer.ID, ps pstore.Peerstore, bwr metrics.Reporter, fs []*net.IPNet, tpt smux.Transport) (p2phost.Host, error) {

		// No addresses to begin with. we'll start later.
		swrm, err := swarm.NewSwarmWithProtector(ctx, nil, id, ps, nil, tpt, bwr)
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
			host = p2pbhost.New(network, p2pbhost.NATPortMap, bwr)
		}

		return host, nil
	}

	ncfg := &ipfscore.BuildCfg{
		Repo:   r,
		Online: true,
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
	ipnsConfig, err := nd.Repo.GetConfigKey("Ipns")
	if err != nil {
		log.Error(err)
		return err
	}
	qs := ipnsConfig.(map[string]interface{})["QuerySize"].(float64)
	if qs <= 20 {
		dht.QuerySize = int(qs)
	} else {
		dht.QuerySize = 20
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
	walletCfg, err := repo.GetWalletConfig(path.Join(repoPath, "config"))
	if err != nil {
		log.Error(err)
		return err
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
	var wallet bitcoin.BitcoinWallet
	if strings.ToLower(walletCfg.Type) == "spvwallet" {
		wallet, err = spvwallet.NewSPVWallet(mn, &params, uint64(walletCfg.MaxFee), uint64(walletCfg.LowFeeDefault), uint64(walletCfg.MediumFeeDefault), uint64(walletCfg.HighFeeDefault), walletCfg.FeeAPI, repoPath, sqliteDB, "OpenBazaar", walletCfg.TrustedPeer, torDialer, ml)
		if err != nil {
			log.Error(err)
			return err
		}
	} else if strings.ToLower(walletCfg.Type) == "bitcoind" {
		if walletCfg.Binary == "" {
			return errors.New("The path to the bitcoind binary must be specified in the config file when using bitcoind")
		}
		usetor := false
		if usingTor && !usingClearnet {
			usetor = true
		}
		wallet = bitcoind.NewBitcoindWallet(mn, &params, repoPath, walletCfg.TrustedPeer, walletCfg.Binary, walletCfg.RPCUser, walletCfg.RPCPassword, usetor, controlPort)
	} else {
		log.Fatal("Unknown wallet type")
	}

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

	// Authenticated gateway
	apiConfig, err := repo.GetAPIConfig(path.Join(repoPath, "config"))
	if err != nil {
		log.Error(err)
		return err
	}
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
	if addr != "127.0.0.1" && wallet.Params().Name == chaincfg.MainNetParams.Name && apiConfig.Enabled {
		apiConfig.Authenticated = true
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

	var exchangeRates bitcoin.ExchangeRates
	if !x.DisableExchangeRates {
		exchangeRates = exchange.NewBitcoinPriceFetcher(torDialer)
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
		Resolver:          bstk.NewBlockStackClient(resolverUrl, torDialer),
		ExchangeRates:     exchangeRates,
		CrosspostGateways: gatewayUrls,
		TorDialer:         torDialer,
		UserAgent:         core.USERAGENT,
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
		core.Node.Service = service.New(core.Node, ctx, sqliteDB)
		MR := ret.NewMessageRetriever(sqliteDB, ctx, nd, core.Node.Service, 16, torDialer, core.Node.SendOfflineAck)
		go MR.Run()
		core.Node.MessageRetriever = MR
		PR := rep.NewPointerRepublisher(nd, sqliteDB, core.Node.IsModerator)
		go PR.Run()
		core.Node.PointerRepublisher = PR
		if !x.DisableWallet {
			MR.Wait()
			TL := lis.NewTransactionListener(core.Node.Datastore, core.Node.Broadcast, core.Node.Wallet.Params())
			wallet.AddTransactionListener(TL.OnTransactionReceived)
			log.Info("Starting bitcoin wallet")
			go wallet.Start()
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

func initializeRepo(dataDir, password, mnemonic string, testnet bool) (*db.SQLiteDatastore, error) {
	// Database
	sqliteDB, err := db.Create(dataDir, password, testnet)
	if err != nil {
		return sqliteDB, err
	}

	// Initialize the IPFS repo if it does not already exist
	err = repo.DoInit(dataDir, 4096, testnet, password, mnemonic, sqliteDB.Config().Init)
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
		corehttp.GatewayOption(node.Resolver, config.Authenticated, authCookie, config.Username, config.Password, cfg.Gateway.Writable, "/ipfs", "/ipns"),
	}

	if len(cfg.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
	}

	if err != nil {
		return nil, fmt.Errorf("newHTTPGateway: ConstructNode() failed: %s", err)
	}

	// Create and return an API gateway
	return api.NewGateway(node, authCookie, gwLis.NetListener(), config, opts...)
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
