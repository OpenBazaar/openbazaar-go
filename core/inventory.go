package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/ipfs/go-ipfs/namesys"
	ipfsPath "github.com/ipfs/go-ipfs/path"
	"gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"io/ioutil"
	"os"
	"path"
	"time"
)

func (n *OpenBazaarNode) PublishInventory(inventory interface{}) error {
	inv, err := json.MarshalIndent(inventory, "", "    ")
	if err != nil {
		return err
	}
	h := sha256.Sum256(inv)

	tmpPath := path.Join(n.RepoPath, hex.EncodeToString(h[:])+".json")
	err = ioutil.WriteFile(tmpPath, inv, os.ModePerm)
	if err != nil {
		return err
	}
	hash, err := ipfs.AddFile(n.Context, tmpPath)
	if err != nil {
		return err
	}
	err = os.Remove(tmpPath)
	if err != nil {
		return err
	}

	return ipfs.PublishAltRoot(n.Context, "inventory", ipfsPath.FromString("/ipfs/"+hash), time.Now().Add(namesys.DefaultPublishLifetime))
}

func (n *OpenBazaarNode) GetInventory(p peer.ID) ([]byte, error) {
	root, err := ipfs.ResolveAltRoot(n.Context, p, "inventory", time.Minute)
	if err != nil {
		return nil, err
	}
	return ipfs.Cat(n.Context, root, time.Minute)
}
