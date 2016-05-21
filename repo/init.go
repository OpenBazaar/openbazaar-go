package repo

import (
	"io"
	"fmt"
	"errors"
	"os"
	"path"
	_ "github.com/mattn/go-sqlite3"
	"github.com/op/go-logging"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/namesys"
	"github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	"github.com/pebbe/zmq4"
)

var log = logging.MustGetLogger("repo")

var ErrRepoExists = errors.New(`ipfs configuration file already exists!
Reinitializing would overwrite your keys.
(use -f to force overwrite)
`)

func DoInit(out io.Writer, repoRoot string, nBitsForKeypair int) error {
	log.Infof("initializing openbazaar node at %s\n", repoRoot)

	if err := maybeCreateOBDirectories(repoRoot); err != nil {
		return err
	}

	if fsrepo.IsInitialized(repoRoot) {
		return ErrRepoExists
	}

	if err := checkWriteable(repoRoot); err != nil {
		return err
	}

	conf, err := config.Init(out, nBitsForKeypair)
	if err != nil {
		return err
	}
	conf.Discovery.MDNS.Enabled = false
	conf.Addresses.API = ""
	conf.Ipns.RecordLifetime = "7d"
	conf.Ipns.RepublishPeriod = "24h"
	conf.Addresses.Swarm = append(conf.Addresses.Swarm, "/ip4/0.0.0.0/udp/4001/utp")
	conf.Addresses.Swarm = append(conf.Addresses.Swarm, "/ip6/::/udp/4001/utp")

	if err := fsrepo.Init(repoRoot, conf); err != nil {
		return err
	}

	if err := addConfigExtensions(repoRoot); err != nil {
		return err
	}

	return initializeIpnsKeyspace(repoRoot)
}

func maybeCreateOBDirectories(repoRoot string) error {
	if err := os.MkdirAll(path.Join(repoRoot, "node"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(path.Join(repoRoot, "node", "listings"), os.ModePerm); err != nil {
		return err
	}

	if err := os.MkdirAll(path.Join(repoRoot, "purchases", "unfunded"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(path.Join(repoRoot, "purchases", "in progress"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(path.Join(repoRoot, "purchases", "trade receipts"), os.ModePerm); err != nil {
		return err
	}

	if err := os.MkdirAll(path.Join(repoRoot, "sales", "unfunded"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(path.Join(repoRoot, "sales", "in progress"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(path.Join(repoRoot, "sales", "trade receipts"), os.ModePerm); err != nil {
		return err
	}

	if err := os.MkdirAll(path.Join(repoRoot, "cases"), os.ModePerm); err != nil {
		return err
	}
	return nil
}

func checkWriteable(dir string) error {
	_, err := os.Stat(dir)
	if err == nil {
		// dir exists, make sure we can write to it
		testfile := path.Join(dir, "test")
		fi, err := os.Create(testfile)
		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("%s is not writeable by the current user", dir)
			}
			return fmt.Errorf("unexpected error while checking writeablility of repo root: %s", err)
		}
		fi.Close()
		return os.Remove(testfile)
	}

	if os.IsNotExist(err) {
		// dir doesnt exist, check that we can create it
		return os.Mkdir(dir, 0775)
	}

	if os.IsPermission(err) {
		return fmt.Errorf("cannot write to %s, incorrect permissions", err)
	}

	return err
}

func initializeIpnsKeyspace(repoRoot string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}

	nd, err := core.NewNode(ctx, &core.BuildCfg{Repo: r})
	if err != nil {
		return err
	}
	defer nd.Close()

	err = nd.SetupOfflineRouting()
	if err != nil {
		return err
	}

	return namesys.InitializeKeyspace(ctx, nd.DAG, nd.Namesys, nd.Pinning, nd.PrivateKey)
}

func addConfigExtensions(repoRoot string) error {
	r, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}
	type Server struct {
		Url       string
		PublicKey []byte
	}
	type LibbitcoinServers struct {
		Mainnet  []Server
		Testnet  []Server
	}
	ls := &LibbitcoinServers{
		Mainnet: []Server{
			Server{Url: "tcp://libbitcoin1.openbazaar.org:9091", PublicKey: []byte{}},
			Server{Url: "tcp://libbitcoin3.openbazaar.org:9091", PublicKey: []byte{}},
			Server{Url: "tcp://obelisk.airbitz.co:9091", PublicKey: []byte{}},
		},
		Testnet: []Server {
			Server{Url: "tcp://libbitcoin2.openbazaar.org:9091", PublicKey: []byte(zmq4.Z85decode("baihZB[vT(dcVCwkhYLAzah<t2gJ>{3@k?+>T&^3"))},
			Server{Url: "tcp://libbitcoin4.openbazaar.org:9091", PublicKey: []byte(zmq4.Z85decode("<Z&{.=LJSPySefIKgCu99w.L%b^6VvuVp0+pbnOM"))},
		},
	}
	if err := extendConfigFile(r, "LibbitcoinServers", ls); err != nil {
		return err
	}
	if err := extendConfigFile(r, "Resolver", "https://resolver.onename.com/"); err != nil {
		return err
	}
	if err := r.Close(); err != nil {
		return err
	}
	return nil
}