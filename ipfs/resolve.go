package ipfs

import (
	"context"
	ds "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"

	"time"

	ipath "gx/ipfs/QmQAgv6Gaoe2tQpcabqwKXKChp2MZ7i3UXv9DqTTaxCaTR/go-path"
	ipnspb "gx/ipfs/QmUwMnKKjH3JwGKNVZ3TcP37W93xzqNA4ECFFiMo6sXkkc/go-ipns/pb"
	"gx/ipfs/QmddjPSGZb3ieihSseFeCfVRpZzcqczPNsD2DvarSwnjJB/gogo-protobuf/proto"
	"gx/ipfs/QmfVj3x4D6Jkq9SEoi5n2NmoUomLwoeiwnYz2KQa15wRw6/base32"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/namesys"

	nameopts "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core/options/namesys"
)

const (
	persistentCacheDbPrefix = "/ipns/persistentcache/"
)

func ipnsCacheDsKey(id peer.ID) ds.Key {
	return ds.NewKey(persistentCacheDbPrefix + base32.RawStdEncoding.EncodeToString([]byte(id)))
}

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
		pth, err := getIPFSPath(n.Repo.Datastore(), p)
		if err == nil {
			// Update the cache in background
			go func() {
				pth, err := resolve(n, p, timeout, quorum)
				if err != nil {
					return
				}
				if n.Identity != p {
					if err := putIPFSPath(n.Repo.Datastore(), p, pth); err != nil {
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
		pth, err := getIPFSPath(n.Repo.Datastore(), p)
		if err != nil {
			return "", err
		}
		return pth.Segments()[1], nil
	}
	// Resolving succeeded. Update the cache.
	if n.Identity != p {
		if err := putIPFSPath(n.Repo.Datastore(), p, pth); err != nil {
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
	pth, err := n.Namesys.Resolve(cctx, "/ipns/"+p.Pretty(), nameopts.DhtRecordCount(quorum))
	if err != nil {
		return pth, err
	}
	return pth, nil
}

func ResolveAltRoot(n *core.IpfsNode, p peer.ID, altRoot string, timeout time.Duration) (string, error) {
	cctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pth, err := n.Namesys.Resolve(cctx, "/ipns/"+p.Pretty()+":"+altRoot)
	if err != nil {
		return "", err
	}
	return pth.Segments()[1], nil
}

func getIPNSRecord(datastore ds.Datastore, p peer.ID) (*ipnspb.IpnsEntry, error) {
	valBytes, err := datastore.Get(namesys.IpnsDsKey(p))
	if err != nil {
		return nil, err
	}

	ipnsEntry := new(ipnspb.IpnsEntry)
	if err := proto.Unmarshal(valBytes, ipnsEntry); err != nil {
		return nil, err
	}
	return ipnsEntry, nil
}

// GetIPNSRecord will look in the datastore shared by the DHT under the /ipfs/<peerID>
// key. The value in this namespace contains a serialized protobuf record which is
// returned if present or an error otherwise.
func GetIPNSRecord(n *core.IpfsNode, p peer.ID) (*ipnspb.IpnsEntry, error) {
	var (
		entry, err = getIPNSRecord(n.Repo.Datastore(), p)
		localID    = n.Identity
	)
	if err != nil {
		return nil, err
	}

	// Deserialize the record and check for the presence of a pubkey. If the
	// record doesn't have one we'll inject it in
	if p == localID {
		if len(entry.PubKey) == 0 {
			entry.PubKey, err = n.PrivateKey.GetPublic().Bytes()
			if err != nil {
				return nil, err
			}
		}
	}

	return entry, nil
}

// getIPFSPath looks in two places in the database for a record. First is
// under the /ipfs/<peerID> key which is sometimes used by the DHT. The value
// returned by this location is a serialized protobuf record. The second is
// under /ipfs/persistentcache/<peerID> which returns only the value (the path)
// inside the protobuf record.
func getIPFSPath(datastore ds.Datastore, p peer.ID) (ipath.Path, error) {
	entry, err := getIPNSRecord(datastore, p)
	if err == nil {
		return ipath.ParsePath(string(entry.Value))
	}

	pth, err := datastore.Get(ipnsCacheDsKey(p))
	if err != nil {
		if err == ds.ErrNotFound {
			return "", namesys.ErrResolveFailed
		}
		return "", err
	}
	return ipath.ParsePath(string(pth))
}

// putIPFSPath places the path into the datastore under /ipfs/persistentcache/<peerID>
// for later retreival by getIPFSPath
func putIPFSPath(datastore ds.Datastore, p peer.ID, pth ipath.Path) error {
	return datastore.Put(ipnsCacheDsKey(p), []byte(pth.String()))
}
