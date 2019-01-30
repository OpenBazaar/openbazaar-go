package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	bitswap "gx/ipfs/QmNkxFCmPtr2RQxjZNRCNryLud4L9wMEiBJsLgF14MqTHj/go-bitswap/network"
	"gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	"gx/ipfs/QmPpYHPRGVpSJTkQDQDwTYZ1cYUR2NM4HS6M3iAXi8aoUa/go-libp2p-kad-dht/opts"
	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	"gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	pstore "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
	"gx/ipfs/QmUDTcnDp2WssbmiDLC6aYurUeyt7QeRakHUQMxA2mZ5iB/go-libp2p"
	oniontp "gx/ipfs/QmVSfWChGxC5AkUhM6ZyZxbcBmZoPrUmrPuW6BnHU3YDA9/go-onion-transport"
	p2phost "gx/ipfs/QmdJfsSbKSZnMkfZ1kpopiyB9i3Hd6cp8VKWZmtWPa7Moc/go-libp2p-host"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	obnet "github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/ipfs/go-ipfs/core"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"golang.org/x/crypto/ssh/terminal"
	ipath "gx/ipfs/QmT3rzed1ppXefourpmoZ7tyVQfsGPQZ1pHDngLmCvXxd3/go-path"
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
	ipfscore.DHTOption = constructDHTRouting
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
	sqliteDB, err = InitializeRepo(repoPath, x.Password, x.Mnemonic, x.Testnet, creationDate, wallet.Bitcoin)
	if err != nil && err != repo.ErrRepoExists {
		return err
	}

	// If the database cannot be decrypted, exit
	if sqliteDB.Config().IsEncrypted() {
		sqliteDB.Close()
		fmt.Print("Database is encrypted, enter your password: ")
		// nolint:unconvert
		bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
		fmt.Println("")
		pw := string(bytePassword)
		sqliteDB, err = InitializeRepo(repoPath, pw, "", x.Testnet, time.Now(), wallet.Bitcoin)
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
		testnetBootstrapAddrs, err := schema.GetTestnetBootstrapAddrs(configFile)
		if err != nil {
			PrintError(err.Error())
			return err
		}
		cfg.Bootstrap = testnetBootstrapAddrs
		dhtopts.ProtocolDHT = "/openbazaar/kad/testnet/1.0.0"
		bitswap.ProtocolBitswap = "/openbazaar/bitswap/testnet/1.1.0"
	} else {
		bitswap.ProtocolBitswap = "/openbazaar/bitswap/1.1.0"
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
	torConfig, err := schema.GetTorConfig(configFile)
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
		} else {
			usingClearnet = true
		}
	}
	// Create Tor transport
	var (
		onionTransport *oniontp.OnionTransport
		torPw          = torConfig.Password
		torControl     = torConfig.TorControl
	)
	if usingTor {
		if torControl == "" {
			controlPort, err = obnet.GetTorControlPort()
			if err != nil {
				PrintError(err.Error())
				return err
			}
			torControl = "127.0.0.1:" + strconv.Itoa(controlPort)
		}

		if x.TorPassword != "" {
			torPw = x.TorPassword
		}
	}

	// Custom host option used if Tor is enabled
	defaultHostOption := func(ctx context.Context, id peer.ID, ps pstore.Peerstore, options ...libp2p.Option) (p2phost.Host, error) {
		pkey := ps.PrivKey(id)
		if pkey == nil {
			return nil, fmt.Errorf("missing private key for node ID: %s", id.Pretty())
		}
		options = append([]libp2p.Option{libp2p.Identity(pkey), libp2p.Peerstore(ps)}, options...)
		if usingTor {
			options = append(options, libp2p.Transport(oniontp.NewOnionTransportC("tcp4", torControl, torPw, nil, repoPath, (usingTor && usingClearnet))))
		}
		return libp2p.New(ctx, options...)
	}

	ncfg := &ipfscore.BuildCfg{
		Repo:   r,
		Online: true,
		ExtraOpts: map[string]bool{
			"mplex":  true,
			"ipnsps": true,
		},
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

	wg := new(sync.WaitGroup)
	wg.Add(10)
	pid, err := peer.IDB58Decode(identity.PeerID)
	if err != nil {
		PrintError(err.Error())
		return err
	}
	k, err := ipfs.Resolve(nd, pid, time.Minute, false)
	if err != nil || k == "" {
		PrintError(fmt.Sprintf("IPNS record for %s not found on network\n", identity.PeerID))
		return err
	}
	c, err := cid.Decode(k)
	if err != nil {
		PrintError(err.Error())
		return err
	}
	node, err := nd.DAG.Get(context.Background(), c)
	if err != nil {
		PrintError(err.Error())
		return err
	}
	links := node.Links()
	for _, l := range links {
		if l.Name == "listings" || l.Name == "ratings" || l.Name == "feed" || l.Name == "channel" || l.Name == "files" {
			go RestoreDirectory(repoPath, l.Name, nd, &l.Cid, wg)
		} else if l.Name == "images" {
			node, err := nd.DAG.Get(context.Background(), l.Cid)
			if err != nil {
				PrintError(err.Error())
				return err
			}
			ilinks := node.Links()
			for _, link := range ilinks {
				wg.Add(1)
				go RestoreDirectory(repoPath, path.Join("images", link.Name), nd, &link.Cid, wg)
			}
		}
	}

	go RestoreFile(repoPath, identity.PeerID, "profile.json", nd, wg)
	go RestoreFile(repoPath, identity.PeerID, "ratings.json", nd, wg)
	go RestoreFile(repoPath, identity.PeerID, "listings.json", nd, wg)
	go RestoreFile(repoPath, identity.PeerID, "following.json", nd, wg)
	go RestoreFile(repoPath, identity.PeerID, "followers.json", nd, wg)
	wg.Wait()
	fmt.Println("Finished")
	return nil
}

func RestoreFile(repoPath, peerID, filename string, n *core.IpfsNode, wg *sync.WaitGroup) {
	defer wg.Done()
	b, err := ipfs.ResolveThenCat(n, ipath.FromString(path.Join(peerID, filename)), time.Minute, false)
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
	node, err := nd.DAG.Get(context.Background(), *id)
	if err != nil {
		PrintError(err.Error())
		return
	}
	for _, l := range node.Links() {
		wg.Add(1)
		go func(link *ipld.Link) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()

			api := coreapi.NewCoreAPI(nd)
			pth, err := iface.ParsePath("/ipfs/" + link.Cid.String())
			if err != nil {
				PrintError(fmt.Sprintf("Error retrieving %s\n", path.Join(directory, link.Name)))
				return
			}

			r, err := api.Unixfs().Get(cctx, pth)
			if err != nil {
				PrintError(fmt.Sprintf("Error retrieving %s\n", path.Join(directory, link.Name)))
				return
			}

			fmt.Printf("Restoring %s/%s\n", directory, link.Name)
			f, err := os.Create(path.Join(repoPath, "root", directory, link.Name))
			if err != nil {
				PrintError(err.Error())
				return
			}
			_, err = io.Copy(f, r)
			if err != nil {
				PrintError(err.Error())
				return
			}
		}(l)
	}

}

func PrintError(e string) {
	os.Stderr.Write([]byte(e))
}
