package merkledag

import (
	"context"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
)

// ComboService implements ipld.DAGService, using 'Read' for all fetch methods,
// and 'Write' for all methods that add new objects.
type ComboService struct {
	Read  ipld.NodeGetter
	Write ipld.DAGService
}

var _ ipld.DAGService = (*ComboService)(nil)

// Add writes a new node using the Write DAGService.
func (cs *ComboService) Add(ctx context.Context, nd ipld.Node) error {
	return cs.Write.Add(ctx, nd)
}

// AddMany adds nodes using the Write DAGService.
func (cs *ComboService) AddMany(ctx context.Context, nds []ipld.Node) error {
	return cs.Write.AddMany(ctx, nds)
}

// Get fetches a node using the Read DAGService.
func (cs *ComboService) Get(ctx context.Context, c cid.Cid) (ipld.Node, error) {
	return cs.Read.Get(ctx, c)
}

// GetMany fetches nodes using the Read DAGService.
func (cs *ComboService) GetMany(ctx context.Context, cids []cid.Cid) <-chan *ipld.NodeOption {
	return cs.Read.GetMany(ctx, cids)
}

// Remove deletes a node using the Write DAGService.
func (cs *ComboService) Remove(ctx context.Context, c cid.Cid) error {
	return cs.Write.Remove(ctx, c)
}

// RemoveMany deletes nodes using the Write DAGService.
func (cs *ComboService) RemoveMany(ctx context.Context, cids []cid.Cid) error {
	return cs.Write.RemoveMany(ctx, cids)
}
