package repo

import (
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/ipfs/go-ipfs/core"
)

var (
	getObjectFromIPFSCache   = map[string]getObjectFromIPFSCacheEntry{}
	getObjectFromIPFSCacheMu = sync.Mutex{}
)

type getObjectFromIPFSCacheEntry struct {
	bytes   []byte
	created time.Time
}

// GetObjectFromIPFS gets the requested name from ipfs or the local cache
func GetObjectFromIPFS(n *core.IpfsNode, p peer.ID, name string, maxCacheLen time.Duration) ([]byte, error) {
	getObjectFromIPFSCacheMu.Lock()
	defer getObjectFromIPFSCacheMu.Unlock()

	fetchAndUpdateCache := func() ([]byte, error) {
		objBytes, err := fetchObjectFromIPFS(n, p, name)
		if err != nil {
			return nil, err
		}

		getObjectFromIPFSCache[getIPFSCacheKey(p, name)] = getObjectFromIPFSCacheEntry{
			bytes:   objBytes,
			created: time.Now(),
		}

		return objBytes, nil
	}

	entry, ok := getObjectFromIPFSCache[getIPFSCacheKey(p, name)]
	if !ok || entry.created.Add(maxCacheLen).Before(time.Now()) {
		return fetchAndUpdateCache()
	}

	// Update cache in background after a successful read
	go func() {
		getObjectFromIPFSCacheMu.Lock()
		defer getObjectFromIPFSCacheMu.Unlock()
		_, err := fetchAndUpdateCache()
		if err != nil {
			log.Error(err)
		}
	}()

	return entry.bytes, nil
}

// fetchObjectFromIPFS gets the requested object from ipfs
func fetchObjectFromIPFS(n *core.IpfsNode, p peer.ID, name string) ([]byte, error) {
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

func getIPFSCacheKey(p peer.ID, name string) string {
	return p.Pretty() + "|" + name
}
