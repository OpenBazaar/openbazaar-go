package repo

import (
	"io"
	"fmt"
	"errors"
	"os"
	"path"
	"github.com/op/go-logging"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/namesys"
	"github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

)

var log = logging.MustGetLogger("repo")

var ErrRepoExists = errors.New(`ipfs configuration file already exists!
Reinitializing would overwrite your keys.
(use -f to force overwrite)
`)

func DoInit(out io.Writer, repoRoot string, force bool, nBitsForKeypair int) error {
	log.Infof("initializing openbazaar node at %s\n", repoRoot)

	if err := maybeCreateOBDirectories(repoRoot); err != nil {
		return err
	}

	if fsrepo.IsInitialized(repoRoot) && !force {
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

	if fsrepo.IsInitialized(repoRoot) {
		if err := fsrepo.Remove(repoRoot); err != nil {
			return err
		}
	}

	if err := fsrepo.Init(repoRoot, conf); err != nil {
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
