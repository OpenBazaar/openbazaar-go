package ipfs

import (
	"context"

	"github.com/ipfs/go-ipfs/core/coreapi"

	"github.com/ipfs/go-ipfs/core"
	corerepo "github.com/ipfs/go-ipfs/core/corerepo"
)

/* Recursively un-pin a directory given its hash.
   This will allow it to be garbage collected. */
func UnPinDir(n *core.IpfsNode, rootHash string) error {
	api := coreapi.NewCoreAPI(n)
	_, err := corerepo.Unpin(n, api, context.Background(), []string{"/ipfs/" + rootHash}, true)
	return err
}
