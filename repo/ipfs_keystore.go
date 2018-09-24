package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/namesys"
	ipfsPath "github.com/ipfs/go-ipfs/path"
)

// PublishObjectToIPFS writes the given data to IPFS labeled as the given name
func PublishObjectToIPFS(ipfsNode *core.IpfsNode, tempDir string, name string, data interface{}) (string, error) {
	serializedData, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(serializedData)

	tmpPath := path.Join(tempDir, hex.EncodeToString(h[:])+".json")
	err = ioutil.WriteFile(tmpPath, serializedData, os.ModePerm)
	if err != nil {
		return "", err
	}
	hash, err := ipfs.AddFile(ipfsNode, tmpPath)
	if err != nil {
		return "", err
	}
	err = os.Remove(tmpPath)
	if err != nil {
		return "", err
	}

	return hash, ipfs.PublishAltRoot(ipfsNode, name, ipfsPath.FromString("/ipfs/"+hash), time.Now().Add(namesys.DefaultPublishLifetime))
}

// GetObjectFromIPFS gets the requested name from ipfs
func GetObjectFromIPFS(n *core.IpfsNode, p peer.ID, name string, maxCacheLen time.Duration) ([]byte, error) {
	root, err := ipfs.ResolveAltRoot(n, p, name, time.Minute)
	if err != nil {
		return nil, err
	}
	bytes, err := ipfs.Cat(n, root, time.Minute)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}
