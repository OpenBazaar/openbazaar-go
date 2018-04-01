package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/namesys"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/op/go-logging"
	"time"
)

var log = logging.MustGetLogger("repo")
var ErrRepoExists = errors.New("IPFS configuration file exists. Reinitializing would overwrite your keys. Use -f to force overwrite.")

func DoInit(repoRoot string, nBitsForKeypair int, testnet bool, password string, mnemonic string, creationDate time.Time, dbInit func(string, []byte, string, time.Time) error) error {
	nodeSchema, err := schema.NewCustomSchemaManager(schema.SchemaContext{
		DataPath:        repoRoot,
		TestModeEnabled: testnet,
		Mnemonic:        mnemonic,
	})
	if err != nil {
		return err
	}
	if nodeSchema.IsInitialized() {
		if err := nodeSchema.MigrateDatabase(); err != nil {
			return err
		}
		return ErrRepoExists
	}
	log.Infof("Initializing OpenBazaar node at %s\n", repoRoot)

	if err := checkWriteable(nodeSchema.DataPath()); err != nil {
		return err
	}
	if err := nodeSchema.BuildSchemaDirectories(); err != nil {
		return err
	}
	if err := nodeSchema.InitializeDatabase(); err != nil {
		return err
	}
	if err := nodeSchema.InitializeIPFSRepo(); err != nil {
		return err
	}
	return initializeIpnsKeyspace(repoRoot, nodeSchema.IdentityKey())
}

func checkWriteable(dir string) error {
	_, err := os.Stat(dir)
	if err == nil {
		// Directory exists, make sure we can write to it
		testfile := path.Join(dir, "test")
		fi, err := os.Create(testfile)
		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("%s is not writeable by the current user", dir)
			}
			return fmt.Errorf("Unexpected error while checking writeablility of repo root: %s", err)
		}
		fi.Close()
		return os.Remove(testfile)
	}

	if os.IsNotExist(err) {
		// Directory does not exist, check that we can create it
		return os.Mkdir(dir, 0775)
	}

	if os.IsPermission(err) {
		return fmt.Errorf("Cannot write to %s, incorrect permissions", err)
	}

	return err
}

func initializeIpnsKeyspace(repoRoot string, privKeyBytes []byte) error {
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
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

/* Returns the directory to store repo data in.
   It depends on the OS and whether or not we are on testnet. */
func GetRepoPath(isTestnet bool) (string, error) {
	paths, err := schema.NewCustomSchemaManager(schema.SchemaContext{
		TestModeEnabled: isTestnet,
	})
	if err != nil {
		return "", err
	}
	return paths.DataPath(), nil
}
