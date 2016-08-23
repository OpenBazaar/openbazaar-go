package ipfs

import "github.com/ipfs/go-ipfs/commands"

const PeersTimeout = 30

func ConnectedPeers(ctx commands.Context) ([]string, error) {
	args := []string{"swarm", "peers"}
	req, cmd, err := NewRequest(ctx, args, PeersTimeout)
	if err != nil {
		return nil, err
	}
	res := commands.NewResponse(req)
	cmd.Run(req, res)
	if res.Error() != nil {
		return nil, res.Error()
	}
	return res.Output().([]string), nil
}
