package ipfs

import (
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/notifications"
	peer "gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
)

func Query(ctx commands.Context, peerID string) ([]peer.ID, error) {
	var peers []peer.ID
	args := []string{"dht", "query", peerID}
	req, cmd, err := NewRequest(ctx, args)
	if err != nil {
		return peers, err
	}
	res := commands.NewResponse(req)
	cmd.Run(req, res)
	resp := res.Output()
	if res.Error() != nil {
		log.Error(res.Error())
		return peers, res.Error()
	}
	peerChan := resp.(<-chan interface{})
	peerMap := make(map[string]peer.ID)
	for p := range peerChan {
		peerMap[p.(*notifications.QueryEvent).ID.Pretty()] = p.(*notifications.QueryEvent).ID
		if len(p.(*notifications.QueryEvent).Responses) > 0 {
			for _, r := range p.(*notifications.QueryEvent).Responses {
				peerMap[r.ID.Pretty()] = r.ID
			}
		}
	}
	for _, v := range peerMap {
		peers = append(peers, v)
	}
	return peers, nil
}
