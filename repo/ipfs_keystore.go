package repo

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
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/namesys"
	ipfsPath "github.com/ipfs/go-ipfs/path"
)

var getObjectFromIPFSCache = map[string]getObjectFromIPFSCacheEntry{}

type getObjectFromIPFSCacheEntry struct {
	bytes   []byte
	created time.Time
}

// PublishObjectToIPFS writes the given data to IPFS labeled as the given name
func PublishObjectToIPFS(ctx commands.Context, ipfsNode *core.IpfsNode, tempDir string, name string, data interface{}) error {
	serializedData, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return err
	}
	h := sha256.Sum256(serializedData)

	tmpPath := path.Join(tempDir, hex.EncodeToString(h[:])+".json")
	err = ioutil.WriteFile(tmpPath, serializedData, os.ModePerm)
	if err != nil {
		return err
	}
	hash, err := ipfs.AddFile(ctx, tmpPath)
	if err != nil {
		return err
	}
	err = os.Remove(tmpPath)
	if err != nil {
		return err
	}

	return ipfs.PublishAltRoot(ctx, ipfsNode, name, ipfsPath.FromString("/ipfs/"+hash), time.Now().Add(namesys.DefaultPublishLifetime))
}

// GetObjectFromIPFS gets the requested name from ipfs or the local cache
func GetObjectFromIPFS(ctx commands.Context, p peer.ID, name string, maxCacheLen time.Duration) ([]byte, error) {
	entry, ok := getObjectFromIPFSCache[getIPFSCacheKey(p, name)]
	if !ok {
		return fetchObjectFromIPFS(ctx, p, name)
	}

	if entry.created.Add(maxCacheLen).Before(time.Now()) {
		return fetchObjectFromIPFS(ctx, p, name)
	}

	return entry.bytes, nil
}

// fetchObjectFromIPFS gets the requested object from ipfs
func fetchObjectFromIPFS(ctx commands.Context, p peer.ID, name string) ([]byte, error) {
	root, err := ipfs.ResolveAltRoot(ctx, p, name, time.Minute)
	if err != nil {
		return nil, err
	}
	bytes, err := ipfs.Cat(ctx, root, time.Minute)
	if err != nil {
		return nil, err
	}
	getObjectFromIPFSCache[getIPFSCacheKey(p, name)] = getObjectFromIPFSCacheEntry{
		bytes:   bytes,
		created: time.Now(),
	}

	return bytes, nil
}

func getIPFSCacheKey(p peer.ID, name string) string {
	return p.Pretty() + "|" + name
}
