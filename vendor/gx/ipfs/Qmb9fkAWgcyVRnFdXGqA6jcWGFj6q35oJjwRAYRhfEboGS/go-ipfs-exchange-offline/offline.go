// package offline implements an object that implements the exchange
// interface but returns nil values to every request.
package offline

import (
	"context"

	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	exchange "gx/ipfs/QmWokDcQdSZCxrNxgaRzQDDBofALhActzNBaxRLtiRkUHg/go-ipfs-exchange-interface"
	blockstore "gx/ipfs/QmXjKkjMDTtXAiLBwstVexofB8LeruZmE2eBd85GwGFFLA/go-ipfs-blockstore"
	blocks "gx/ipfs/QmYYLnAzR28nAQ4U5MFniLprnktu6eTFKibeNt96V21EZK/go-block-format"
)

func Exchange(bs blockstore.Blockstore) exchange.Interface {
	return &offlineExchange{bs: bs}
}

// offlineExchange implements the Exchange interface but doesn't return blocks.
// For use in offline mode.
type offlineExchange struct {
	bs blockstore.Blockstore
}

// GetBlock returns nil to signal that a block could not be retrieved for the
// given key.
// NB: This function may return before the timeout expires.
func (e *offlineExchange) GetBlock(_ context.Context, k cid.Cid) (blocks.Block, error) {
	return e.bs.Get(k)
}

// HasBlock always returns nil.
func (e *offlineExchange) HasBlock(b blocks.Block) error {
	return e.bs.Put(b)
}

// Close always returns nil.
func (_ *offlineExchange) Close() error {
	// NB: exchange doesn't own the blockstore's underlying datastore, so it is
	// not responsible for closing it.
	return nil
}

func (e *offlineExchange) GetBlocks(ctx context.Context, ks []cid.Cid) (<-chan blocks.Block, error) {
	out := make(chan blocks.Block)
	go func() {
		defer close(out)
		var misses []cid.Cid
		for _, k := range ks {
			hit, err := e.bs.Get(k)
			if err != nil {
				misses = append(misses, k)
				// a long line of misses should abort when context is cancelled.
				select {
				// TODO case send misses down channel
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			select {
			case out <- hit:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func (e *offlineExchange) IsOnline() bool {
	return false
}
