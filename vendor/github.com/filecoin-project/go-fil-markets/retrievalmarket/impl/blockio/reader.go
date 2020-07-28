package blockio

import (
	"bytes"
	"context"
	"io"

	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
)

// BlockReader is any data struct that can be read block by block
type BlockReader interface {
	// ReadBlock reads data from a single block. Data is nil
	// for intermediate nodes
	ReadBlock(context.Context) (retrievalmarket.Block, bool, error)
}

// SelectorBlockReader reads an ipld data structure in individual blocks
// allowing the next block to be read and then advancing no further
type SelectorBlockReader struct {
	root      ipld.Link
	selector  ipld.Node
	loader    ipld.Loader
	traverser *Traverser
}

// NewSelectorBlockReader returns a new Block reader starting at the given
// root and using the given loader
func NewSelectorBlockReader(root ipld.Link, sel ipld.Node, loader ipld.Loader) BlockReader {
	return &SelectorBlockReader{root, sel, loader, nil}
}

// ReadBlock reads the next block in the IPLD traversal
func (sr *SelectorBlockReader) ReadBlock(ctx context.Context) (retrievalmarket.Block, bool, error) {

	if sr.traverser == nil {
		sr.traverser = NewTraverser(sr.root, sr.selector)
		sr.traverser.Start(ctx)
	}
	lnk, lnkCtx := sr.traverser.CurrentRequest(ctx)
	reader, err := sr.loader(lnk, lnkCtx)
	if err != nil {
		sr.traverser.Error(ctx, err)
		return retrievalmarket.EmptyBlock, false, err
	}
	var buf bytes.Buffer
	_, err = io.Copy(&buf, reader)
	if err != nil {
		sr.traverser.Error(ctx, err)
		return retrievalmarket.EmptyBlock, false, err
	}
	block := retrievalmarket.Block{
		Data:   buf.Bytes(),
		Prefix: lnk.(cidlink.Link).Cid.Prefix().Bytes(),
	}
	err = sr.traverser.Advance(ctx, &buf)
	return block, sr.traverser.IsComplete(ctx), err
}
