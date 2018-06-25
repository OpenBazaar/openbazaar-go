package ipfs

import (
	"github.com/ipfs/go-ipfs/core"
	"context"
	"github.com/OpenBazaar/go-ipfs/core/corerepo"
)

/* Recursively un-pin a directory given its hash.
   This will allow it to be garbage collected. */
func UnPinDir(n *core.IpfsNode, rootHash string) error {
	_, err := corerepo.Unpin(n, context.Background(), []string{"/ipfs/"+rootHash}, true)
	return err
}
