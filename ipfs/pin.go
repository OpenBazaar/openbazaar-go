package ipfs

import (
	"context"

	"github.com/ipfs/go-ipfs/core/coreapi"

	coreiface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"
	options "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core/options"

	"github.com/ipfs/go-ipfs/core"
)

/* Recursively un-pin a directory given its hash.
   This will allow it to be garbage collected. */
func UnPinDir(n *core.IpfsNode, rootHash string) error {
	api, err := coreapi.NewCoreAPI(n)
	if err != nil {
		return err
	}
	p, err := coreiface.ParsePath("/ipfs/" + rootHash)
	if err != nil {
		return err
	}

	rp, err := api.ResolvePath(context.Background(), p)
	if err != nil {
		return err
	}

	return api.Pin().Rm(context.Background(), rp, options.Pin.RmRecursive(true))
}
