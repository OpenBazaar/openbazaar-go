package repo

import (
	"errors"
	"fmt"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/namesys"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/op/go-logging"
	"github.com/pebbe/zmq4"
	"github.com/tyler-smith/go-bip39"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	"io"
	"os"
	"path"
)

var log = logging.MustGetLogger("repo")

var ErrRepoExists = errors.New(`ipfs configuration file already exists!
Reinitializing would overwrite your keys.
(use -f to force overwrite)
`)

func DoInit(out io.Writer, repoRoot string, nBitsForKeypair int, testnet bool, password string, dbInit func(string, []byte, string) error) error {
	if err := maybeCreateOBDirectories(repoRoot); err != nil {
		return err
	}

	if fsrepo.IsInitialized(repoRoot) {
		return ErrRepoExists
	}

	if err := checkWriteable(repoRoot); err != nil {
		return err
	}

	conf, err := initConfig(out)
	if err != nil {
		return err
	}

	mnemonic, err := createMnemonic()
	if err != nil {
		return err
	}

	seed := bip39.NewSeed(mnemonic, "Secret Passphrase")
	fmt.Printf("generating %d-bit RSA keypair...", nBitsForKeypair)
	identityKey, err := ipfs.IdentityKeyFromSeed(seed, nBitsForKeypair)
	if err != nil {
		return err
	}
	fmt.Printf("done\n")

	identity, err := ipfs.IdentityFromKey(identityKey)
	if err != nil {
		return err
	}

	log.Infof("initializing openbazaar node at %s\n", repoRoot)
	if err := fsrepo.Init(repoRoot, conf); err != nil {
		return err
	}
	conf.Identity = identity

	if err := addConfigExtensions(repoRoot, testnet); err != nil {
		return err
	}

	if err := dbInit(mnemonic, identityKey, password); err != nil {
		return err
	}

	return initializeIpnsKeyspace(repoRoot, identityKey)
}

func maybeCreateOBDirectories(repoRoot string) error {
	if err := os.MkdirAll(path.Join(repoRoot, "root"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(path.Join(repoRoot, "root", "listings"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(path.Join(repoRoot, "root", "feed"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(path.Join(repoRoot, "root", "channel"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(path.Join(repoRoot, "outbox"), os.ModePerm); err != nil {
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

func initializeIpnsKeyspace(repoRoot string, privKeyBytes []byte) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}
	cfg, err := r.Config()
	if err != nil {
		log.Error(err)
		return err
	}
	identity, err := ipfs.IdentityFromKey(privKeyBytes)
	if err != nil {
		return err
	}

	cfg.Identity = identity
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

func addConfigExtensions(repoRoot string, testnet bool) error {
	r, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}
	type Server struct {
		Url       string
		PublicKey []byte
	}
	var ls []Server
	if !testnet {
		ls = []Server{
			{Url: "tcp://libbitcoin1.openbazaar.org:9091", PublicKey: []byte{}},
			{Url: "tcp://libbitcoin3.openbazaar.org:9091", PublicKey: []byte{}},
			{Url: "tcp://obelisk.airbitz.co:9091", PublicKey: []byte{}},
		}
	} else {
		ls = []Server{
			{Url: "tcp://libbitcoin2.openbazaar.org:9091", PublicKey: []byte(zmq4.Z85decode("baihZB[vT(dcVCwkhYLAzah<t2gJ>{3@k?+>T&^3"))},
			{Url: "tcp://libbitcoin4.openbazaar.org:9091", PublicKey: []byte(zmq4.Z85decode("<Z&{.=LJSPySefIKgCu99w.L%b^6VvuVp0+pbnOM"))},
		}
	}
	if err := extendConfigFile(r, "LibbitcoinServers", ls); err != nil {
		return err
	}
	if err := extendConfigFile(r, "Resolver", "https://resolver.onename.com/"); err != nil {
		return err
	}
	if err := extendConfigFile(r, "Dropbox-api-token", ""); err != nil {
		return err
	}
	if err := r.Close(); err != nil {
		return err
	}
	return nil
}

func createMnemonic() (string, error) {
	entropy, err := bip39.NewEntropy(128)
	if err != nil {
		return "", err
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", err
	}
	return mnemonic, nil
}
