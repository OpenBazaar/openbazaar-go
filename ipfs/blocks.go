package ipfs

import (
	key "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/commands"
)

func BlockstoreHas(ctx commands.Context, hash string) (bool, error) {
	node, err := ctx.GetNode()
	if err != nil {
		return false, err
	}
	k := key.B58KeyDecode(hash)
	return node.Blockstore.Has(k)
}
