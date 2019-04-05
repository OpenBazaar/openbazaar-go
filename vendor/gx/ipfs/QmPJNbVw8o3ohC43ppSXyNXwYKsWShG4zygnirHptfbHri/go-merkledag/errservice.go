package merkledag

import (
	"context"

	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	ipld "gx/ipfs/QmZ6nzCLwGLVfRzYLpD7pW6UNuBDKEcA2imJtVpbEx2rxy/go-ipld-format"
)

// ErrorService implements ipld.DAGService, returning 'Err' for every call.
type ErrorService struct {
	Err error
}

var _ ipld.DAGService = (*ErrorService)(nil)

// Add returns the cs.Err.
func (cs *ErrorService) Add(ctx context.Context, nd ipld.Node) error {
	return cs.Err
}

// AddMany returns the cs.Err.
func (cs *ErrorService) AddMany(ctx context.Context, nds []ipld.Node) error {
	return cs.Err
}

// Get returns the cs.Err.
func (cs *ErrorService) Get(ctx context.Context, c cid.Cid) (ipld.Node, error) {
	return nil, cs.Err
}

// GetMany many returns the cs.Err.
func (cs *ErrorService) GetMany(ctx context.Context, cids []cid.Cid) <-chan *ipld.NodeOption {
	ch := make(chan *ipld.NodeOption)
	close(ch)
	return ch
}

// Remove returns the cs.Err.
func (cs *ErrorService) Remove(ctx context.Context, c cid.Cid) error {
	return cs.Err
}

// RemoveMany returns the cs.Err.
func (cs *ErrorService) RemoveMany(ctx context.Context, cids []cid.Cid) error {
	return cs.Err
}
