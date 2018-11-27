package ipfs

import (
	"context"
	"gx/ipfs/QmS73grfbWgWrNztd8Lns9GCG3jjRNDfcPYg2VYQzKDZSt/go-ipfs-ds-help"
	"gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	"gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	"time"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/namesys"
	"gx/ipfs/QmT3rzed1ppXefourpmoZ7tyVQfsGPQZ1pHDngLmCvXxd3/go-path"
	pb "gx/ipfs/QmaRFtZhVAwXBk4Z3zEsvjScH9fjsDZmhXfa1Gm8eMb9cg/go-ipns/pb"
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
func Resolve(n *core.IpfsNode, p peer.ID, timeout time.Duration, usecache bool) (string, error) {
	if usecache {
		pth, err := getFromDatastore(n.Repo.Datastore(), "/ipns/"+p.Pretty())
		if err == nil {
			// Update the cache in background
			go resolve(n, p, timeout)
			return pth.Segments()[1], nil
		}
	}
	return resolve(n, p, timeout)
}

func resolve(n *core.IpfsNode, p peer.ID, timeout time.Duration) (string, error) {
	cctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pth, err := n.Namesys.Resolve(cctx, "/ipns/"+p.Pretty())
	if err != nil {
		return "", err
	}
	return pth.Segments()[1], nil
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

func getFromDatastore(datastore ds.Datastore, name string) (path.Path, error) {
	// resolve to what we may already have in the datastore
	dsval, err := datastore.Get(dshelp.NewKeyFromBinary([]byte(name)))
	if err != nil {
		if err == ds.ErrNotFound {
			return "", namesys.ErrResolveFailed
		}
		return "", err
	}

	data := dsval
	entry := new(pb.IpnsEntry)

	err = proto.Unmarshal(data, entry)
	if err != nil {
		return "", err
	}

	value, err := path.ParsePath(string(entry.GetValue()))
	return value, err
}
