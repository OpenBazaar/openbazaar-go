package ipfs

import (
	"context"
	"gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	"sync"
	"time"

	"gx/ipfs/QmPJNbVw8o3ohC43ppSXyNXwYKsWShG4zygnirHptfbHri/go-merkledag"

	"github.com/ipfs/go-ipfs/core"
)

// This function takes a Cid directory object and walks it returning each linked cid in the graph
func FetchGraph(n *core.IpfsNode, id *cid.Cid) ([]cid.Cid, error) {
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

func RemoveAll(nd *core.IpfsNode, peerID string, quorum uint) error {
	pid, err := peer.IDB58Decode(peerID)
	if err != nil {
		return nil
	}
	hash, err := Resolve(nd, pid, time.Minute*5, quorum, true)
	if err != nil {
		return err
	}
	c, err := cid.Decode(hash)
	if err != nil {
		return err
	}
	graph, err := FetchGraph(nd, &c)
	if err != nil {
		return err
	}
	for _, id := range graph {
		err = nd.DAG.Remove(context.Background(), id)
		if err != nil {
			log.Error(err)
		}
	}
	return nil
}
