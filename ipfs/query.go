package ipfs

import (
	"context"

	routing "gx/ipfs/QmSY3nkMNLzh9GdbFKK5tT7YMfLpf52iUZ8ZRkr29MJaa5/go-libp2p-kad-dht"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
)

// Query returns the closest peers known for peerID
func Query(dht *routing.IpfsDHT, peerID string) ([]peer.ID, error) {
	id, err := peer.IDB58Decode(peerID)
	if err != nil {
		return nil, err
	}
	ch, err := dht.GetClosestPeers(context.Background(), string(id))
	if err != nil {
		return nil, err
	}

	var closestPeers []peer.ID
	for p := range ch {
		closestPeers = append(closestPeers, p)
	}
	return closestPeers, nil
}
