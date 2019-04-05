package peermanager

import (
	"context"
	"sync"

	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"
	bsmsg "gx/ipfs/QmcSPuzpSbVLU6UHU4e5PwZpm4fHbCn5SbNR5ZNL6Mj63G/go-bitswap/message"
	wantlist "gx/ipfs/QmcSPuzpSbVLU6UHU4e5PwZpm4fHbCn5SbNR5ZNL6Mj63G/go-bitswap/wantlist"

	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
)

var log = logging.Logger("bitswap")

var (
	metricsBuckets = []float64{1 << 6, 1 << 10, 1 << 14, 1 << 18, 1<<18 + 15, 1 << 22}
)

// PeerQueue provides a queer of messages to be sent for a single peer.
type PeerQueue interface {
	AddMessage(entries []bsmsg.Entry, ses uint64)
	Startup()
	AddWantlist(initialWants *wantlist.SessionTrackedWantlist)
	Shutdown()
}

// PeerQueueFactory provides a function that will create a PeerQueue.
type PeerQueueFactory func(ctx context.Context, p peer.ID) PeerQueue

type peerMessage interface {
	handle(pm *PeerManager)
}

type peerQueueInstance struct {
	refcnt int
	pq     PeerQueue
}

// PeerManager manages a pool of peers and sends messages to peers in the pool.
type PeerManager struct {
	// peerQueues -- interact through internal utility functions get/set/remove/iterate
	peerQueues   map[peer.ID]*peerQueueInstance
	peerQueuesLk sync.RWMutex

	createPeerQueue PeerQueueFactory
	ctx             context.Context
}

// New creates a new PeerManager, given a context and a peerQueueFactory.
func New(ctx context.Context, createPeerQueue PeerQueueFactory) *PeerManager {
	return &PeerManager{
		peerQueues:      make(map[peer.ID]*peerQueueInstance),
		createPeerQueue: createPeerQueue,
		ctx:             ctx,
	}
}

// ConnectedPeers returns a list of peers this PeerManager is managing.
func (pm *PeerManager) ConnectedPeers() []peer.ID {
	pm.peerQueuesLk.RLock()
	defer pm.peerQueuesLk.RUnlock()
	peers := make([]peer.ID, 0, len(pm.peerQueues))
	for p := range pm.peerQueues {
		peers = append(peers, p)
	}
	return peers
}

// Connected is called to add a new peer to the pool, and send it an initial set
// of wants.
func (pm *PeerManager) Connected(p peer.ID, initialWants *wantlist.SessionTrackedWantlist) {
	pm.peerQueuesLk.Lock()

	pq := pm.getOrCreate(p)

	if pq.refcnt == 0 {
		pq.pq.AddWantlist(initialWants)
	}

	pq.refcnt++

	pm.peerQueuesLk.Unlock()
}

// Disconnected is called to remove a peer from the pool.
func (pm *PeerManager) Disconnected(p peer.ID) {
	pm.peerQueuesLk.Lock()
	pq, ok := pm.peerQueues[p]

	if !ok {
		pm.peerQueuesLk.Unlock()
		return
	}

	pq.refcnt--
	if pq.refcnt > 0 {
		pm.peerQueuesLk.Unlock()
		return
	}

	delete(pm.peerQueues, p)
	pm.peerQueuesLk.Unlock()

	pq.pq.Shutdown()

}

// SendMessage is called to send a message to all or some peers in the pool;
// if targets is nil, it sends to all.
func (pm *PeerManager) SendMessage(entries []bsmsg.Entry, targets []peer.ID, from uint64) {
	if len(targets) == 0 {
		pm.peerQueuesLk.RLock()
		for _, p := range pm.peerQueues {
			p.pq.AddMessage(entries, from)
		}
		pm.peerQueuesLk.RUnlock()
	} else {
		for _, t := range targets {
			pm.peerQueuesLk.Lock()
			pqi := pm.getOrCreate(t)
			pm.peerQueuesLk.Unlock()
			pqi.pq.AddMessage(entries, from)
		}
	}
}

func (pm *PeerManager) getOrCreate(p peer.ID) *peerQueueInstance {
	pqi, ok := pm.peerQueues[p]
	if !ok {
		pq := pm.createPeerQueue(pm.ctx, p)
		pq.Startup()
		pqi = &peerQueueInstance{0, pq}
		pm.peerQueues[p] = pqi
	}
	return pqi
}
