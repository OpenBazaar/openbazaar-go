package ipfs

import (
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"

	"github.com/ipfs/go-ipfs/core"
)

func ConnectedPeers(n *core.IpfsNode) []peer.ID {
	return n.PeerHost.Network().Peers()
}
