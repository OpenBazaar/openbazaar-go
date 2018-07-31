package core

import (
	"github.com/OpenBazaar/openbazaar-go/net/repointer"
)

// StartPointerRepublisher - setup republisher for IPNS
func (n *OpenBazaarNode) StartPointerRepublisher() {
	n.PointerRepublisher = net.NewPointerRepublisher(n.IpfsNode, n.Datastore, n.PushNodes, n.IsModerator)
	go n.PointerRepublisher.Run()
}
