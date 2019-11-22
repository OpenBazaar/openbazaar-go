package ipfs

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	ipath "gx/ipfs/QmQAgv6Gaoe2tQpcabqwKXKChp2MZ7i3UXv9DqTTaxCaTR/go-path"
	ds "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	ipnspb "gx/ipfs/QmUwMnKKjH3JwGKNVZ3TcP37W93xzqNA4ECFFiMo6sXkkc/go-ipns/pb"
	nameopts "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core/options/namesys"
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	"gx/ipfs/QmddjPSGZb3ieihSseFeCfVRpZzcqczPNsD2DvarSwnjJB/gogo-protobuf/proto"
	"gx/ipfs/QmfVj3x4D6Jkq9SEoi5n2NmoUomLwoeiwnYz2KQa15wRw6/base32"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/namesys"
)

const (
	persistentCacheDbPrefix = "/ipns/persistentcache/"
	pubkeyCacheDbPrefix     = "/pubkey/"
	ipnsCacheDbPrefix       = "/ipns/"
)

// Resolve an IPNS record. This is a multi-step process.
// If the usecache flag is provided we will attempt to load the record from the database. If
// it succeeds we will update the cache in a separate goroutine.
//
// If we need to actually get a record from the network the IPNS namesystem will first check to
// see if it is subscribed to the name with pubsub. If so, it will return cache from the database.
// If not, it will subscribe to the name and proceed to a DHT query to find the record.
// If the DHT query returns nothing it will finally attempt to return from cache.
// All subsequent resolves will return from cache as the pubsub will update the cache in real time
// as new records are published.
func Resolve(n *core.IpfsNode, p peer.ID, timeout time.Duration, quorum uint, usecache bool) (string, error) {
	if usecache {
		pth, err := getFromDatastore(n.Repo.Datastore(), p)
		if err == nil {
			// Update the cache in background
			go func() {
				pth, err := resolve(n, p, timeout, quorum)
				if err != nil {
					return
				}
				if n.Identity != p {
					if err := putToDatastoreCache(n.Repo.Datastore(), p, pth); err != nil {
						log.Error("Error putting IPNS record to datastore: %s", err.Error())
					}
				}
			}()
			return pth.Segments()[1], nil
		}
	}
	pth, err := resolve(n, p, timeout, quorum)
	if err != nil {
		// Resolving fail. See if we have it in the db.
		pth, err := getFromDatastore(n.Repo.Datastore(), p)
		if err != nil {
			return "", err
		}
		return pth.Segments()[1], nil
	}
	// Resolving succeeded. Update the cache.
	if n.Identity != p {
		if err := putToDatastoreCache(n.Repo.Datastore(), p, pth); err != nil {
			log.Error("Error putting IPNS record to datastore: %s", err.Error())
		}
	}
	return pth.Segments()[1], nil
}

func resolve(n *core.IpfsNode, p peer.ID, timeout time.Duration, quorum uint) (ipath.Path, error) {
	cctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// TODO [cp]: we should load the record count from our config and set it here. We'll need a
	// migration for this.
	pth, err := n.Namesys.Resolve(cctx, obIPNSCacheKey(p.Pretty()), nameopts.DhtRecordCount(quorum))
	if err != nil {
		return pth, err
	}
	return pth, nil
}

func ResolveAltRoot(n *core.IpfsNode, p peer.ID, altRoot string, timeout time.Duration) (string, error) {
	cctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pth, err := n.Namesys.Resolve(cctx, obIPNSCacheKey(p.Pretty()+":"+altRoot))
	if err != nil {
		return "", err
	}
	return pth.Segments()[1], nil
}

// getFromDatastore looks in two places in the database for a record. First is
// under the /ipns/<peerID> key which is sometimes used by the DHT. The value
// returned by this location is a serialized protobuf record. The second is
// under /ipns/persistentcache/<peerID> which returns only the value (the path)
// inside the protobuf record.
func getFromDatastore(datastore ds.Datastore, p peer.ID) (ipath.Path, error) {
	rec, err := GetCachedIPNSRecord(datastore, p)
	if err == nil {
		return ipath.ParsePath(string(rec.Value))
	}

	pth, err := datastore.Get(persistentCacheKey(p))
	if err != nil {
		if err == ds.ErrNotFound {
			return "", namesys.ErrResolveFailed
		}
		return "", fmt.Errorf("getting cached ipns path: %s", err.Error())
	}
	return ipath.ParsePath(string(pth))
}

func putToDatastoreCache(datastore ds.Datastore, p peer.ID, pth ipath.Path) error {
	return datastore.Put(persistentCacheKey(p), []byte(pth.String()))
}

// GetCachedIPNSRecord retrieves the full IPNSEntry from the provided datastore if present
func GetCachedIPNSRecord(store ds.Datastore, id peer.ID) (*ipnspb.IpnsEntry, error) {
	ival, err := store.Get(nativeIPNSRecordCacheKey(id))
	if err != nil {
		return nil, fmt.Errorf("getting cached ipns record: %s", err.Error())
	}
	rec := new(ipnspb.IpnsEntry)
	err = proto.Unmarshal(ival, rec)
	if err != nil {
		log.Errorf("failed parsing cached record for peer (%s): %s", id.Pretty(), err.Error())
		log.Debug(debug.Stack())
		return nil, fmt.Errorf("parsing cached ipns record: %s", err.Error())
	}
	return rec, nil
}

// DeleteCachedIPNSRecord removes the cached record associated with the provided peer.ID
func DeleteCachedIPNSRecord(store ds.Datastore, id peer.ID) error {
	return store.Delete(nativeIPNSRecordCacheKey(id))
}

// PutCachedPubkey persists the pubkey using the appropriate key prefix
// from the provided datastore
func PutCachedPubkey(store ds.Datastore, peerID string, pubkey []byte) error {
	return store.Put(pubkeyCacheKey(peerID), pubkey)
}

// GetCachedPubkey retrieves the pubkey using the appropriate key prefix
// from the provided Datastore
func GetCachedPubkey(store ds.Datastore, peerID string) ([]byte, error) {
	return store.Get(pubkeyCacheKey(peerID))
}

func pubkeyCacheKey(id string) ds.Key {
	return ds.NewKey(pubkeyCacheDbPrefix + id)
}

func persistentCacheKey(id peer.ID) ds.Key {
	return ds.NewKey(persistentCacheDbPrefix + base32.RawStdEncoding.EncodeToString([]byte(id)))
}

// nativeIPNSRecordCacheKey applies native IPFS key: "/ipns/" + encoded(id)
func nativeIPNSRecordCacheKey(id peer.ID) ds.Key {
	return namesys.IpnsDsKey(id)
}

// obIPNSCacheKey applies custom IPNS prefix key: "/ipns/" + id
func obIPNSCacheKey(id string) string {
	return ipnsCacheDbPrefix + id
}
