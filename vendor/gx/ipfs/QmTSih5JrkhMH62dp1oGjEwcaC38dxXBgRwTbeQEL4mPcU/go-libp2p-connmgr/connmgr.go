package connmgr

import (
	"context"
	"sort"
	"sync"
	"time"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	"gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	"gx/ipfs/QmWRvjn5BHMLCGkf48Hk1LDc4W72RPA9H59AAVCXmn9esJ/go-libp2p-interface-connmgr"
	inet "gx/ipfs/QmXuRkCR7BNQa9uqfpTiFWsTQLzmTWYg91Ja1w95gnqb6u/go-libp2p-net"
	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
)

var log = logging.Logger("connmgr")

// BasicConnMgr is a ConnManager that trims connections whenever the count exceeds the
// high watermark. New connections are given a grace period before they're subject
// to trimming. Trims are automatically run on demand, only if the time from the
// previous trim is higher than 10 seconds. Furthermore, trims can be explicitly
// requested through the public interface of this struct (see TrimOpenConns).
//
// See configuration parameters in NewConnManager.
type BasicConnMgr struct {
	highWater int
	lowWater  int

	gracePeriod time.Duration

	peers     map[peer.ID]*peerInfo
	connCount int

	lk sync.Mutex

	lastTrim time.Time
}

var _ ifconnmgr.ConnManager = (*BasicConnMgr)(nil)

// NewConnManager creates a new BasicConnMgr with the provided params:
// * lo and hi are watermarks governing the number of connections that'll be maintained.
//   When the peer count exceeds the 'high watermark', as many peers will be pruned (and
//   their connections terminated) until 'low watermark' peers remain.
// * grace is the amount of time a newly opened connection is given before it becomes
//   subject to pruning.
func NewConnManager(low, hi int, grace time.Duration) *BasicConnMgr {
	return &BasicConnMgr{
		highWater:   hi,
		lowWater:    low,
		gracePeriod: grace,
		peers:       make(map[peer.ID]*peerInfo),
	}
}

// peerInfo stores metadata for a given peer.
type peerInfo struct {
	tags  map[string]int // value for each tag
	value int            // cached sum of all tag values

	conns map[inet.Conn]time.Time // start time of each connection

	firstSeen time.Time // timestamp when we began tracking this peer.
}

// TrimOpenConns closes the connections of as many peers as needed to make the peer count
// equal the low watermark. Peers are sorted in ascending order based on their total value,
// pruning those peers with the lowest scores first, as long as they are not within their
// grace period.
func (cm *BasicConnMgr) TrimOpenConns(ctx context.Context) {
	defer log.EventBegin(ctx, "connCleanup").Done()
	for _, c := range cm.getConnsToClose(ctx) {
		log.Info("closing conn: ", c.RemotePeer())
		log.Event(ctx, "closeConn", c.RemotePeer())
		c.Close()
	}
}

// getConnsToClose runs the heuristics described in TrimOpenConns and returns the
// connections to close.
func (cm *BasicConnMgr) getConnsToClose(ctx context.Context) []inet.Conn {
	cm.lk.Lock()
	defer cm.lk.Unlock()

	if cm.lowWater == 0 || cm.highWater == 0 {
		// disabled
		return nil
	}
	now := time.Now()
	cm.lastTrim = now

	if len(cm.peers) < cm.lowWater {
		log.Info("open connection count below limit")
		return nil
	}

	var infos []*peerInfo

	for _, inf := range cm.peers {
		infos = append(infos, inf)
	}

	// Sort peers according to their value.
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].value < infos[j].value
	})

	closeCount := len(infos) - cm.lowWater
	toclose := infos[:closeCount]

	// 2x number of peers we're disconnecting from because we may have more
	// than one connection per peer. Slightly over allocating isn't an issue
	// as this is a very short-lived array.
	closed := make([]inet.Conn, 0, len(toclose)*2)

	for _, inf := range toclose {
		// TODO: should we be using firstSeen or the time associated with the connection itself?
		if inf.firstSeen.Add(cm.gracePeriod).After(now) {
			continue
		}

		// TODO: if a peer has more than one connection, maybe only close one?
		for c := range inf.conns {
			// TODO: probably don't want to always do this in a goroutine
			closed = append(closed, c)
		}
	}

	return closed
}

// GetTagInfo is called to fetch the tag information associated with a given
// peer, nil is returned if p refers to an unknown peer.
func (cm *BasicConnMgr) GetTagInfo(p peer.ID) *ifconnmgr.TagInfo {
	cm.lk.Lock()
	defer cm.lk.Unlock()

	pi, ok := cm.peers[p]
	if !ok {
		return nil
	}

	out := &ifconnmgr.TagInfo{
		FirstSeen: pi.firstSeen,
		Value:     pi.value,
		Tags:      make(map[string]int),
		Conns:     make(map[string]time.Time),
	}

	for t, v := range pi.tags {
		out.Tags[t] = v
	}
	for c, t := range pi.conns {
		out.Conns[c.RemoteMultiaddr().String()] = t
	}

	return out
}

// TagPeer is called to associate a string and integer with a given peer.
func (cm *BasicConnMgr) TagPeer(p peer.ID, tag string, val int) {
	cm.lk.Lock()
	defer cm.lk.Unlock()

	pi, ok := cm.peers[p]
	if !ok {
		log.Info("tried to tag conn from untracked peer: ", p)
		return
	}

	// Update the total value of the peer.
	pi.value += (val - pi.tags[tag])
	pi.tags[tag] = val
}

// UntagPeer is called to disassociate a string and integer from a given peer.
func (cm *BasicConnMgr) UntagPeer(p peer.ID, tag string) {
	cm.lk.Lock()
	defer cm.lk.Unlock()

	pi, ok := cm.peers[p]
	if !ok {
		log.Info("tried to remove tag from untracked peer: ", p)
		return
	}

	// Update the total value of the peer.
	pi.value -= pi.tags[tag]
	delete(pi.tags, tag)
}

// CMInfo holds the configuration for BasicConnMgr, as well as status data.
type CMInfo struct {
	// The low watermark, as described in NewConnManager.
	LowWater int

	// The high watermark, as described in NewConnManager.
	HighWater int

	// The timestamp when the last trim was triggered.
	LastTrim time.Time

	// The configured grace period, as described in NewConnManager.
	GracePeriod time.Duration

	// The current connection count.
	ConnCount int
}

// GetInfo returns the configuration and status data for this connection manager.
func (cm *BasicConnMgr) GetInfo() CMInfo {
	cm.lk.Lock()
	defer cm.lk.Unlock()

	return CMInfo{
		HighWater:   cm.highWater,
		LowWater:    cm.lowWater,
		LastTrim:    cm.lastTrim,
		GracePeriod: cm.gracePeriod,
		ConnCount:   cm.connCount,
	}
}

// Notifee returns a sink through which Notifiers can inform the BasicConnMgr when
// events occur. Currently, the notifee only reacts upon connection events
// {Connected, Disconnected}.
func (cm *BasicConnMgr) Notifee() inet.Notifiee {
	return (*cmNotifee)(cm)
}

type cmNotifee BasicConnMgr

func (nn *cmNotifee) cm() *BasicConnMgr {
	return (*BasicConnMgr)(nn)
}

// Connected is called by notifiers to inform that a new connection has been established.
// The notifee updates the BasicConnMgr to start tracking the connection. If the new connection
// count exceeds the high watermark, a trim may be triggered.
func (nn *cmNotifee) Connected(n inet.Network, c inet.Conn) {
	cm := nn.cm()

	cm.lk.Lock()
	defer cm.lk.Unlock()

	pinfo, ok := cm.peers[c.RemotePeer()]
	if !ok {
		pinfo = &peerInfo{
			firstSeen: time.Now(),
			tags:      make(map[string]int),
			conns:     make(map[inet.Conn]time.Time),
		}
		cm.peers[c.RemotePeer()] = pinfo
	}

	_, ok = pinfo.conns[c]
	if ok {
		log.Error("received connected notification for conn we are already tracking: ", c.RemotePeer())
		return
	}

	pinfo.conns[c] = time.Now()
	cm.connCount++

	if cm.connCount > nn.highWater {
		if cm.lastTrim.IsZero() || time.Since(cm.lastTrim) > time.Second*10 {
			go cm.TrimOpenConns(context.Background())
		}
	}
}

// Disconnected is called by notifiers to inform that an existing connection has been closed or terminated.
// The notifee updates the BasicConnMgr accordingly to stop tracking the connection, and performs housekeeping.
func (nn *cmNotifee) Disconnected(n inet.Network, c inet.Conn) {
	cm := nn.cm()

	cm.lk.Lock()
	defer cm.lk.Unlock()

	cinf, ok := cm.peers[c.RemotePeer()]
	if !ok {
		log.Error("received disconnected notification for peer we are not tracking: ", c.RemotePeer())
		return
	}

	_, ok = cinf.conns[c]
	if !ok {
		log.Error("received disconnected notification for conn we are not tracking: ", c.RemotePeer())
		return
	}

	delete(cinf.conns, c)
	cm.connCount--
	if len(cinf.conns) == 0 {
		delete(cm.peers, c.RemotePeer())
	}
}

// Listen is no-op in this implementation.
func (nn *cmNotifee) Listen(n inet.Network, addr ma.Multiaddr) {}

// ListenClose is no-op in this implementation.
func (nn *cmNotifee) ListenClose(n inet.Network, addr ma.Multiaddr) {}

// OpenedStream is no-op in this implementation.
func (nn *cmNotifee) OpenedStream(inet.Network, inet.Stream) {}

// ClosedStream is no-op in this implementation.
func (nn *cmNotifee) ClosedStream(inet.Network, inet.Stream) {}
