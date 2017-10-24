package cmd

import (
	"context"
	"fmt"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	"net"
	"os"
	"path"
	"strconv"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	obnet "github.com/OpenBazaar/openbazaar-go/net"
	"github.com/ipfs/go-ipfs/commands"
	ipfscore "github.com/ipfs/go-ipfs/core"
	bitswap "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	"github.com/ipfs/go-ipfs/namesys"
	"github.com/ipfs/go-ipfs/repo/config"
	"io/ioutil"
	"strings"

	"bufio"
	"errors"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/ipfs/go-ipfs/core/coreunix"
	ipfspath "github.com/ipfs/go-ipfs/path"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/proxy"
	"gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	ipld "gx/ipfs/QmPN7cwmpcc4DWXb4KTB9dNAJgjuPY69h3npsMfhRrQL9c/go-ipld-format"
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
	"sync"
	"syscall"
	"time"
)

type Restore struct {
	Password           string `short:"p" long:"password" description:"the encryption password if the database is encrypted"`
	DataDir            string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Testnet            bool   `short:"t" long:"testnet" description:"use the test network"`
	TorPassword        string `long:"torpassword" description:"Set the tor control password. This will override the tor password in the config."`
	Tor                bool   `long:"tor" description:"Automatically configure the daemon to run as a Tor hidden service and use Tor exclusively. Requires Tor to be running."`
	Mnemonic           string `short:"m" long:"mnemonic" description:"specify a mnemonic seed to use to derive the keychain"`
	WalletCreationDate string `short:"w" long:"walletcreationdate" description:"specify the date the seed was created. if omitted the wallet will sync from the oldest checkpoint."`
}

func (x *Restore) Execute(args []string) error {
	reader := bufio.NewReader(os.Stdin)
	if x.Mnemonic == "" {
		fmt.Print("This command will override any current user data. Do you want to continue? (y/n): ")
	} else {
		fmt.Print("This command will override any current user data as well as destroy your existing keys and history. Are you really, really sure you want to continue? (y/n): ")
	}
	yn, _ := reader.ReadString('\n')
	if !(strings.ToLower(yn) == "y\n" || strings.ToLower(yn) == "yes\n" || strings.ToLower(yn)[:1] == "y") {
		return nil
	}

	// Set repo path
	repoPath, err := repo.GetRepoPath(x.Testnet)
	if err != nil {
		return err
	}
	if x.DataDir != "" {
		repoPath = x.DataDir
	}

	// Initialize repo if they included a mnemonic
	creationDate := time.Now()
	var sqliteDB *db.SQLiteDatastore
	if x.Mnemonic != "" {
		if x.WalletCreationDate != "" {
			creationDate, err = time.Parse(time.RFC3339, x.WalletCreationDate)
			if err != nil {
				return errors.New("Wallet creation date timestamp must be in RFC3339 format")
			}
		}
		os.RemoveAll(repoPath)
	}
	sqliteDB, err = InitializeRepo(repoPath, x.Password, x.Mnemonic, x.Testnet, creationDate)
	if err != nil && err != repo.ErrRepoExists {
		return err
	}

	// If the database cannot be decrypted, exit
	if sqliteDB.Config().IsEncrypted() {
		sqliteDB.Close()
		fmt.Print("Database is encrypted, enter your password: ")
		bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
		fmt.Println("")
		pw := string(bytePassword)
		sqliteDB, err = InitializeRepo(repoPath, pw, "", x.Testnet, time.Now())
		if err != nil && err != repo.ErrRepoExists {
			return err
		}
		if sqliteDB.Config().IsEncrypted() {
			PrintError("Invalid password")
			os.Exit(3)
		}
	}
	// Get current identity
	identityKey, err := sqliteDB.Config().GetIdentityKey()
	if err != nil {
		PrintError(err.Error())
		return err
	}
	identity, err := ipfs.IdentityFromKey(identityKey)
	if err != nil {
		return err
	}

	// IPFS node setup
	r, err := fsrepo.Open(repoPath)
	if err != nil {
		PrintError(err.Error())
		return err
	}
	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := r.Config()
	if err != nil {
		PrintError(err.Error())
		return err
	}

	// Setup testnet
	configFile, err := ioutil.ReadFile(path.Join(repoPath, "config"))
	if err != nil {
		PrintError(err.Error())
		return err
	}
	if x.Testnet {
		testnetBootstrapAddrs, err := repo.GetTestnetBootstrapAddrs(configFile)
		if err != nil {
			PrintError(err.Error())
			return err
		}
		cfg.Bootstrap = testnetBootstrapAddrs
		dht.ProtocolDHT = "/openbazaar/kad/testnet/1.0.0"
		bitswap.ProtocolBitswap = "/openbazaar/bitswap/testnet/1.1.0"
	}

	cfg.Identity = identity

	// Tor configuration
	onionAddr, err := obnet.MaybeCreateHiddenServiceKey(repoPath)
	if err != nil {
		log.Error(err)
		return err
	}
	onionAddrString := "/onion/" + onionAddr + ":4003"
	if x.Tor {
		cfg.Addresses.Swarm = []string{}
		cfg.Addresses.Swarm = append(cfg.Addresses.Swarm, onionAddrString)
	}
	torConfig, err := repo.GetTorConfig(configFile)
	if err != nil {
		PrintError(err.Error())
		return err
	}
	var usingTor, usingClearnet bool
	var controlPort int
	for _, addr := range cfg.Addresses.Swarm {
		m, err := ma.NewMultiaddr(addr)
		if err != nil {
			PrintError(err.Error())
			return err
		}
		p := m.Protocols()
		if p[0].Name == "onion" {
			usingTor = true
			addrutil.SupportedTransportStrings = append(addrutil.SupportedTransportStrings, "/onion")
			t, err := ma.ProtocolsWithString("/onion")
			if err != nil {
				PrintError(err.Error())
				return err
			}
			addrutil.SupportedTransportProtocols = append(addrutil.SupportedTransportProtocols, t)
			if err != nil {
				PrintError(err.Error())
				return err
			}
		} else {
			usingClearnet = true
		}
	}
	// Create Tor transport
	var onionTransport *oniontp.OnionTransport
	if usingTor {
		torControl := torConfig.TorControl
		if torControl == "" {
			controlPort, err = obnet.GetTorControlPort()
			if err != nil {
				PrintError(err.Error())
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
			PrintError(err.Error())
			return err
		}
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
		DNSResolver: nil,
		Routing:     DHTOption,
	}
	if onionTransport != nil {
		ncfg.Host = defaultHostOption
	}
	fmt.Println("Starting node...")
	nd, err := ipfscore.NewNode(cctx, ncfg)
	if err != nil {
		PrintError(err.Error())
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

	<-dht.DefaultBootstrapConfig.DoneChan
	wg := new(sync.WaitGroup)
	wg.Add(10)
	k, err := ipfs.Resolve(ctx, identity.PeerID)
	if err != nil || k == "" {
		PrintError(fmt.Sprintf("IPNS record for %s not found on network\n", identity.PeerID))
		return err
	}
	c, err := cid.Decode(k)
	if err != nil {
		PrintError(err.Error())
		return err
	}
	links, err := nd.DAG.GetLinks(context.Background(), c)
	if err != nil {
		PrintError(err.Error())
		return err
	}
	for _, l := range links {
		if l.Name == "listings" || l.Name == "ratings" || l.Name == "feed" || l.Name == "channel" || l.Name == "files" {
			go RestoreDirectory(repoPath, l.Name, nd, l.Cid, wg)
		} else if l.Name == "images" {
			ilinks, err := nd.DAG.GetLinks(context.Background(), l.Cid)
			if err != nil {
				PrintError(err.Error())
				return err
			}
			for _, link := range ilinks {
				wg.Add(1)
				go RestoreDirectory(repoPath, path.Join("images", link.Name), nd, link.Cid, wg)
			}
		}
	}

	go RestoreFile(repoPath, identity.PeerID, "profile.json", ctx, wg)
	go RestoreFile(repoPath, identity.PeerID, "ratings.json", ctx, wg)
	go RestoreFile(repoPath, identity.PeerID, "listings.json", ctx, wg)
	go RestoreFile(repoPath, identity.PeerID, "following.json", ctx, wg)
	go RestoreFile(repoPath, identity.PeerID, "followers.json", ctx, wg)
	wg.Wait()
	fmt.Println("Finished")
	return nil
}

func RestoreFile(repoPath, peerID, filename string, ctx commands.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	b, err := ipfs.ResolveThenCat(ctx, ipfspath.FromString(path.Join(peerID, filename)))
	if err != nil {
		PrintError(fmt.Sprintf("Failed to find %s\n", filename))
	} else {
		fmt.Printf("Restoring %s\n", filename)
		err := ioutil.WriteFile(path.Join(repoPath, "root", filename), b, os.ModePerm)
		if err != nil {
			PrintError(err.Error())
		}
	}
}

func RestoreDirectory(repoPath, directory string, nd *ipfscore.IpfsNode, id *cid.Cid, wg *sync.WaitGroup) {
	defer wg.Done()
	links, err := nd.DAG.GetLinks(context.Background(), id)
	if err != nil {
		PrintError(err.Error())
		return
	}
	for _, l := range links {
		wg.Add(1)
		go func(link *ipld.Link) {
			defer wg.Done()
			cctx, _ := context.WithTimeout(context.Background(), time.Second*30)
			r, err := coreunix.Cat(cctx, nd, "/ipfs/"+link.Cid.String())
			if err != nil {
				PrintError(fmt.Sprintf("Error retrieving %s\n", path.Join(directory, l.Name)))
				return
			}
			fmt.Printf("Restoring %s/%s\n", directory, link.Name)
			f, err := os.Create(path.Join(repoPath, "root", directory, link.Name))
			if err != nil {
				PrintError(err.Error())
				return
			}
			r.WriteTo(f)
		}(l)
	}

}

func PrintError(e string) {
	os.Stderr.Write([]byte(e))
}
