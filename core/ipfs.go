package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/ipfs/go-ipfs/namesys"
	ipfsPath "github.com/ipfs/go-ipfs/path"
)

var getModelFromIPFSCache = map[string]getModelFromIPFSCacheEntry{}

type getModelFromIPFSCacheEntry struct {
	bytes   []byte
	created time.Time
}

// PublishModelToIPFS writes the given data to IPFS labeled as the given model
// name
func (n *OpenBazaarNode) PublishModelToIPFS(model string, data interface{}) error {
	serializedData, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return err
	}
	h := sha256.Sum256(serializedData)

	tmpPath := path.Join(n.RepoPath, hex.EncodeToString(h[:])+".json")
	err = ioutil.WriteFile(tmpPath, serializedData, os.ModePerm)
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

	return ipfs.PublishAltRoot(n.Context, n.IpfsNode, model, ipfsPath.FromString("/ipfs/"+hash), time.Now().Add(namesys.DefaultPublishLifetime))
}

// GetModelFromIPFS gets the requested model from ipfs or the local cache
func (n *OpenBazaarNode) GetModelFromIPFS(p peer.ID, model string, maxCacheLen time.Duration) ([]byte, error) {
	entry, ok := getModelFromIPFSCache[getIPFSCacheKey(p, model)]
	if !ok {
		return n.fetchModelFromIPFS(p, model)
	}

	if entry.created.Add(maxCacheLen).Before(time.Now()) {
		return n.fetchModelFromIPFS(p, model)
	}

	return entry.bytes, nil
}

// fetchModelFromIPFS gets the requested model from ipfs
func (n *OpenBazaarNode) fetchModelFromIPFS(p peer.ID, model string) ([]byte, error) {
	root, err := ipfs.ResolveAltRoot(n.Context, p, model, time.Minute)
	if err != nil {
		return nil, err
	}
	bytes, err := ipfs.Cat(n.Context, root, time.Minute)
	if err != nil {
		return nil, err
	}
	getModelFromIPFSCache[getIPFSCacheKey(p, model)] = getModelFromIPFSCacheEntry{
		bytes:   bytes,
		created: time.Now(),
	}

	return bytes, nil
}

func getIPFSCacheKey(p peer.ID, model string) string {
	return p.Pretty() + "|" + model
}
