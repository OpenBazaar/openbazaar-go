package ipfs

import (
	"context"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/merkledag"
	"gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	"sync"
	"time"
)

// This function takes a Cid directory object and walks it returning each linked cid in the graph
func FetchGraph(dag merkledag.DAGService, id *cid.Cid) ([]cid.Cid, error) {
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
			ret = append(ret, *c)
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

func RemoveAll(ctx commands.Context, peerID string) error {
	hash, err := Resolve(ctx, peerID, time.Minute*5)
	if err != nil {
		return err
	}
	c, err := cid.Decode(hash)
	if err != nil {
		return err
	}
	nd, err := ctx.GetNode()
	if err != nil {
		return err
	}
	graph, err := FetchGraph(nd.DAG, c)
	if err != nil {
		return err
	}
	for _, id := range graph {
		ctx := context.Background()
		n, err := nd.DAG.Get(ctx, &id)
		if err != nil {
			continue
		}
		nd.DAG.Remove(n)
	}
	return nil
}
