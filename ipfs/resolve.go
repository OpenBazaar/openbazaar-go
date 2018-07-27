package ipfs

import (
	"context"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/namesys"
	pb "github.com/ipfs/go-ipfs/namesys/pb"
	path "github.com/ipfs/go-ipfs/path"
	dshelp "gx/ipfs/QmTmqJGRQfuH8eKWD1FjThwPRipt1QhqJQNZ8MpzmfAAxo/go-ipfs-ds-help"
	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"time"
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
	cctx, _ := context.WithTimeout(context.Background(), timeout)
	pth, err := n.Namesys.Resolve(cctx, "/ipns/"+p.Pretty())
	if err != nil {
		return "", err
	}
	return pth.Segments()[1], nil
}

func ResolveAltRoot(n *core.IpfsNode, p peer.ID, altRoot string, timeout time.Duration) (string, error) {
	cctx, _ := context.WithTimeout(context.Background(), timeout)
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

	data := dsval.([]byte)
	entry := new(pb.IpnsEntry)

	err = proto.Unmarshal(data, entry)
	if err != nil {
		return "", err
	}

	value, err := path.ParsePath(string(entry.GetValue()))
	return value, err
}
