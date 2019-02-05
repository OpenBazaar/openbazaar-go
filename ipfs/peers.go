package ipfs

import (
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"

	"github.com/ipfs/go-ipfs/core"
)

func ConnectedPeers(n *core.IpfsNode) []peer.ID {
	return n.PeerHost.Network().Peers()
}
