package ipfs

import (
	"github.com/ipfs/go-ipfs/commands"
	coreCmds "github.com/ipfs/go-ipfs/core/commands"
	"time"
)

const ResolveTimeout = 30 * time.Second

// Publish a signed IPNS record to our Peer ID
func Resolve(ctx commands.Context, hash string) (string, error) {
	args := []string{"name", "resolve", hash}
	req, cmd, err := NewRequestWithTimeout(ctx, args, ResolveTimeout)
	if err != nil {
		return "", err
	}
	res := commands.NewResponse(req)
	cmd.Run(req, res)
	resp := res.Output()
	if res.Error() != nil {
		log.Error(res.Error())
		return "", res.Error()
	}
	returnedVal := resp.(*coreCmds.ResolvedPath)
	return returnedVal.Path.Segments()[1], nil
}
