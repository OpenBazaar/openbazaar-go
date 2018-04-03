package ipfs

import (
	"github.com/ipfs/go-ipfs/commands"
	coreCmds "github.com/ipfs/go-ipfs/core/commands"
	"time"
	"context"
	"gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
)

// Publish a signed IPNS record to our Peer ID
func Resolve(ctx commands.Context, hash string, timeout time.Duration) (string, error) {
	args := []string{"name", "resolve", hash}
	req, cmd, err := NewRequestWithTimeout(ctx, args, timeout)
	if err != nil {
		return "", err
	}
	res := commands.NewResponse(req)
	cmd.Run(req, res)
	resp := res.Output()
	if res.Error() != nil {
		log.Error(res.Error(), hash)
		return "", res.Error()
	}
	returnedVal := resp.(*coreCmds.ResolvedPath)
	return returnedVal.Path.Segments()[1], nil
}

func ResolveAltRoot(ctx commands.Context, p peer.ID, altRoot string, timeout time.Duration) (string, error) {
	nd, err := ctx.ConstructNode()
	if err != nil {
		return "", err
	}
	cctx, _ := context.WithTimeout(context.Background(), timeout)
	pth, err := nd.Namesys.Resolve(cctx, "/ipns/"+p.Pretty()+":"+altRoot)
	if err != nil {
		return "", err
	}
	return pth.Segments()[1], nil
}