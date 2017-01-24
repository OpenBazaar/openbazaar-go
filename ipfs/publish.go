package ipfs

import (
	"errors"

	"github.com/ipfs/go-ipfs/commands"
	coreCmds "github.com/ipfs/go-ipfs/core/commands"
)

var pubErr = errors.New(`Name publish failed`)

// Publish a signed IPNS record to our Peer ID
func Publish(ctx commands.Context, hash string) (string, error) {
	args := []string{"name", "publish", "/ipfs/" + hash}
	req, cmd, err := NewRequest(ctx, args)
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
	returnedVal := resp.(*coreCmds.IpnsEntry).Value
	if returnedVal != "/ipfs/"+hash {
		return "", pubErr
	}
	log.Infof("Published %s to IPNS", hash)
	return returnedVal, nil
}
