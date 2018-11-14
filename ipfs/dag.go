package ipfs

import (
	"context"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	"gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	"gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	"sync"
	"time"

	"github.com/ipfs/go-ipfs/core"
	"gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag"
)

// This function takes a Cid directory object and walks it returning each linked cid in the graph
func FetchGraph(n *core.IpfsNode, id cid.Cid) ([]cid.Cid, error) {
	dag := merkledag.NewDAGService(n.Blocks)
	var ret []cid.Cid
	l := new(sync.Mutex)
	m := make(map[string]bool)
	ctx := context.Background()
	m[id.String()] = true
	for {
		if len(m) == 0 {
			break
		}
		for k := range m {
			c, err := cid.Decode(k)
			if err != nil {
				return ret, err
			}
			ret = append(ret, c)
			links, err := dag.GetLinks(ctx, c)
			if err != nil {
				return ret, err
			}
			l.Lock()
			delete(m, k)
			for _, link := range links {
				m[link.Cid.String()] = true
			}
			l.Unlock()
		}
	}
	return ret, nil
}

func RemoveAll(nd *core.IpfsNode, peerID string) error {


	pid, err := peer.IDB58Decode(peerID)
	if err != nil {
		return nil
	}
	hash, err := Resolve(nd, pid, time.Minute*5, true)
	if err != nil {
		return err
	}

	//c, err := cid.Decode(hash)
	//if err != nil {
	//	return err
	//}
	h, _ := mh.FromB58String(hash)
	t := cid.NewCidV0(h)

	graph, err := FetchGraph(nd, t)
	if err != nil {
		return err
	}
	for _, id := range graph {
		nd.DAG.Remove(context.Background(), id)
	}
	return nil
}
