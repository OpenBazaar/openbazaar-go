package ipfs

import (
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"github.com/ipfs/go-ipfs/core"
	routing "gx/ipfs/QmRaVcGchmC1stHHK7YhcgEuTk5k1JiGS568pfYWMgT91H/go-libp2p-kad-dht"
	"errors"
	"context"
)

func Query(n *core.IpfsNode, peerID string) ([]peer.ID, error) {
	dht, ok := n.Routing.(*routing.IpfsDHT)
	if !ok {
		return nil, errors.New("routing is not type IpfsDHT")
	}
	id, err := peer.IDB58Decode(peerID)
	if err != nil {
		return nil, err
	}

	ch, err := dht.GetClosestPeers(context.Background(), string(id))
	if err != nil {
		return nil, err
	}
	var closestPeers []peer.ID
	events := make(chan struct{})
	go func() {
		defer close(events)
		for p := range ch {
			closestPeers = append(closestPeers, p)
		}
	}()
	<- events
	return  closestPeers, nil
}
