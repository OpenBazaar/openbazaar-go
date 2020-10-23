package storeutil

import (
	"bytes"
	"fmt"
	"io"

	blocks "github.com/ipfs/go-block-format"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	ipld "github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
)

// LoaderForBlockstore returns an IPLD Loader function compatible with graphsync
// from an IPFS blockstore
func LoaderForBlockstore(bs bstore.Blockstore) ipld.Loader {
	return func(lnk ipld.Link, lnkCtx ipld.LinkContext) (io.Reader, error) {
		asCidLink, ok := lnk.(cidlink.Link)
		if !ok {
			return nil, fmt.Errorf("Unsupported Link Type")
		}
		block, err := bs.Get(asCidLink.Cid)
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(block.RawData()), nil
	}
}

// StorerForBlockstore returns an IPLD Storer function compatible with graphsync
// from an IPFS blockstore
func StorerForBlockstore(bs bstore.Blockstore) ipld.Storer {
	return func(lnkCtx ipld.LinkContext) (io.Writer, ipld.StoreCommitter, error) {
		var buffer bytes.Buffer
		committer := func(lnk ipld.Link) error {
			asCidLink, ok := lnk.(cidlink.Link)
			if !ok {
				return fmt.Errorf("Unsupported Link Type")
			}
			block, err := blocks.NewBlockWithCid(buffer.Bytes(), asCidLink.Cid)
			if err != nil {
				return err
			}
			return bs.Put(block)
		}
		return &buffer, committer, nil
	}
}
