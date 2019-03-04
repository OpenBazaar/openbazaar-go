package ipfs

import (
	"context"

	routing "gx/ipfs/QmPpYHPRGVpSJTkQDQDwTYZ1cYUR2NM4HS6M3iAXi8aoUa/go-libp2p-kad-dht"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
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
