package ipld

import (
	"context"
	"fmt"

	block "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/specs-actors/actors/util/adt"
)

type BlockStoreInMemory struct {
	data map[cid.Cid]block.Block
}

func NewBlockStoreInMemory() *BlockStoreInMemory {
	return &BlockStoreInMemory{make(map[cid.Cid]block.Block)}
}

func (mb *BlockStoreInMemory) Get(c cid.Cid) (block.Block, error) {
	d, ok := mb.data[c]
	if ok {
		return d, nil
	}
	return nil, fmt.Errorf("not found")
}

func (mb *BlockStoreInMemory) Put(b block.Block) error {
	mb.data[b.Cid()] = b
	return nil
}

// Creates a new, empty IPLD store in memory.
func NewADTStore(ctx context.Context) adt.Store {
	return adt.WrapStore(ctx, cbor.NewCborStore(NewBlockStoreInMemory()))

}
