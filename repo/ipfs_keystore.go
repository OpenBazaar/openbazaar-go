package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	u "gx/ipfs/QmSU6eubNdhXjFBJBSksTp8kv8YRub8mGAPv8tVJHmL2EU/go-ipfs-util"
	mh "gx/ipfs/QmU9a9NV9RdPNwZQDYd5uKsm6N6LJLSvLbywDDYFbaaC6P/go-multihash"
	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/gogo/protobuf/proto"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/namesys"
	ipnspb "github.com/ipfs/go-ipfs/namesys/pb"
	ipfsPath "github.com/ipfs/go-ipfs/path"
)

var (
	ErrInvalidCacheEntryType = errors.New("cache entry is not expected type")

	CachedKeystoreEntryTime = time.Hour * 24 * 7
)

type CacheStore interface {
	Get(ds.Key) (interface{}, error)
	Put(ds.Key, interface{}) error
}

// PublishObjectToIPFS writes the given data to IPFS labeled as the given name
func PublishObjectToIPFS(ctx commands.Context, ipfsNode *core.IpfsNode, tempDir string, name string, data interface{}) (string, error) {
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
	hash, err := ipfs.AddFile(ctx, tmpPath)
	if err != nil {
		return "", err
	}
	err = os.Remove(tmpPath)
	if err != nil {
		return "", err
	}

	return hash, ipfs.PublishAltRoot(ctx, ipfsNode, name, ipfsPath.FromString("/ipfs/"+hash), time.Now().Add(namesys.DefaultPublishLifetime))
}

// GetObjectFromIPFS gets the requested name from ipfs or the local cache
func GetObjectFromIPFS(ctx commands.Context, cacheStore CacheStore, p peer.ID, name string) ([]byte, error) {
	// loadFromIPNS retrieves the object by resolving the ipns name to the data
	loadFromIPNS := func() ([]byte, error) {
		fmt.Println("staring loadFromIPNS")
		root, err := ipfs.ResolveAltRoot(ctx, p, name, time.Minute)
		if err != nil {
			return nil, err
		}

		bytes, err := ipfs.Cat(ctx, root, time.Minute)
		if err != nil {
			return nil, err
		}

		return bytes, nil
	}

	if cacheStore == nil {
		return loadFromIPNS()
	}

	fmt.Println("fetching from cache...")

	// Check IPNS cache
	hash, err := mh.FromB58String(p.Pretty())
	if err != nil {
		return nil, err
	}

	ipnsKey := ds.NewKey("/ipns/" + string(hash) + ":" + name)
	val, err := cacheStore.Get(ipnsKey)
	if err != nil {
		fmt.Println("no ipns entry found")
		return loadFromIPNS()
	}

	fmt.Println("ipns cache found")

	// Validate entry data and EOL
	valBytes, ok := val.([]byte)
	if !ok {
		return nil, ErrInvalidCacheEntryType
	}

	entry := new(ipnspb.IpnsEntry)
	err = proto.Unmarshal(valBytes, entry)
	if err != nil {
		return nil, err
	}

	fmt.Println("checking EOL...")
	eol, ok := GetIPNSEntryEOL(entry)
	if !ok || eol.Before(time.Now()) {
		fmt.Println("EOL invalid/expired")
		return loadFromIPNS()
	}

	fmt.Println("ipns entry not expired")

	// Load IPFS hash directly
	ipfsHash, err := ipfsPath.ParsePath(string(entry.GetValue()))
	if err != nil {
		return nil, err
	}
	ipfsHashString := strings.TrimPrefix(ipfsHash.String(), "/ipfs/")

	objectBytes, err := ipfs.Cat(ctx, ipfsHashString, time.Minute)
	if err != nil {
		return nil, err
	}

	fmt.Println("loaded bytes:", string(objectBytes))

	// Pre-fetch latest data for next time
	go loadFromIPNS()

	// Update the entry EOL
	go func() {
		entry.Validity = []byte(u.FormatRFC3339(time.Now().Add(CachedKeystoreEntryTime)))
		v, err := proto.Marshal(entry)
		if err != nil {
			return
		}
		cacheStore.Put(ipnsKey, v)
	}()

	return objectBytes, nil
}

// GetIPNSEntryEOL extracts the EOL from the entry
func GetIPNSEntryEOL(e *ipnspb.IpnsEntry) (time.Time, bool) {
	if e.GetValidityType() == ipnspb.IpnsEntry_EOL {
		eol, err := u.ParseRFC3339(string(e.GetValidity()))
		if err != nil {
			return time.Time{}, false
		}
		return eol, true
	}
	return time.Time{}, false
}

func getIPFSCacheKey(p peer.ID, name string) string {
	return p.Pretty() + "|" + name
}
