package core

import (
	"github.com/OpenBazaar/openbazaar-go/net/retriever"
)

// StartMessageRetriever will collect the required options from the
// OpenBazaarNode and begin the MessageRetriever in the background
func (n *OpenBazaarNode) StartMessageRetriever() {
	config := net.MRConfig{
		Db:        n.Datastore,
		IPFSNode:  n.IpfsNode,
		BanManger: n.BanManager,
		Service:   n.Service,
		PrefixLen: 14,
		PushNodes: n.PushNodes,
		Dialer:    n.TorDialer,
		SendAck:   n.SendOfflineAck,
		SendError: n.SendError,
	}
	n.MessageRetriever = net.NewMessageRetriever(config)
	go n.MessageRetriever.Run()
}

// WaitForMessageRetrieverCompletion will return once the MessageRetriever
// has finished processing messages
func (n *OpenBazaarNode) WaitForMessageRetrieverCompletion() {
	if n.MessageRetriever == nil {
		return
	}
	n.MessageRetriever.Wait()
}
