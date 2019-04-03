package bitswap

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	bsmsg "gx/ipfs/QmcSPuzpSbVLU6UHU4e5PwZpm4fHbCn5SbNR5ZNL6Mj63G/go-bitswap/message"
	bsnet "gx/ipfs/QmcSPuzpSbVLU6UHU4e5PwZpm4fHbCn5SbNR5ZNL6Mj63G/go-bitswap/network"

	mocknet "gx/ipfs/QmRxk6AUaGaKCfzS1xSNRojiAPd7h2ih8GuCdjJBF3Y6GK/go-libp2p/p2p/net/mock"
	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	delay "gx/ipfs/QmUe1WCHkQaz4UeNKiHDUBV2T6i9prc3DniqyHPXyfGaUq/go-ipfs-delay"
	testutil "gx/ipfs/QmWapVoHjtKhn4MhvKNoPTkJKADFGACfXPFnt7combwp5W/go-testutil"
	ifconnmgr "gx/ipfs/QmXa6sgzUvP5bgF5CyyV36bZYv5VDRwttggQYUPvFybLVd/go-libp2p-interface-connmgr"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	routing "gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing"
	mockrouting "gx/ipfs/QmZ22s3UgNi5vvYNH79jWJ63NPyQGiv4mdNaWCz4WKqMTZ/go-ipfs-routing/mock"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"
)

var log = logging.Logger("bstestnet")

func VirtualNetwork(rs mockrouting.Server, d delay.D) Network {
	return &network{
		latencies:          make(map[peer.ID]map[peer.ID]time.Duration),
		clients:            make(map[peer.ID]*receiverQueue),
		delay:              d,
		routingserver:      rs,
		isRateLimited:      false,
		rateLimitGenerator: nil,
		conns:              make(map[string]struct{}),
	}
}

type RateLimitGenerator interface {
	NextRateLimit() float64
}

func RateLimitedVirtualNetwork(rs mockrouting.Server, d delay.D, rateLimitGenerator RateLimitGenerator) Network {
	return &network{
		latencies:          make(map[peer.ID]map[peer.ID]time.Duration),
		rateLimiters:       make(map[peer.ID]map[peer.ID]*mocknet.RateLimiter),
		clients:            make(map[peer.ID]*receiverQueue),
		delay:              d,
		routingserver:      rs,
		isRateLimited:      true,
		rateLimitGenerator: rateLimitGenerator,
		conns:              make(map[string]struct{}),
	}
}

type network struct {
	mu                 sync.Mutex
	latencies          map[peer.ID]map[peer.ID]time.Duration
	rateLimiters       map[peer.ID]map[peer.ID]*mocknet.RateLimiter
	clients            map[peer.ID]*receiverQueue
	routingserver      mockrouting.Server
	delay              delay.D
	isRateLimited      bool
	rateLimitGenerator RateLimitGenerator
	conns              map[string]struct{}
}

type message struct {
	from       peer.ID
	msg        bsmsg.BitSwapMessage
	shouldSend time.Time
}

// receiverQueue queues up a set of messages to be sent, and sends them *in
// order* with their delays respected as much as sending them in order allows
// for
type receiverQueue struct {
	receiver *networkClient
	queue    []*message
	active   bool
	lk       sync.Mutex
}

func (n *network) Adapter(p testutil.Identity) bsnet.BitSwapNetwork {
	n.mu.Lock()
	defer n.mu.Unlock()

	client := &networkClient{
		local:   p.ID(),
		network: n,
		routing: n.routingserver.Client(p),
	}
	n.clients[p.ID()] = &receiverQueue{receiver: client}
	return client
}

func (n *network) HasPeer(p peer.ID) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	_, found := n.clients[p]
	return found
}

// TODO should this be completely asynchronous?
// TODO what does the network layer do with errors received from services?
func (n *network) SendMessage(
	ctx context.Context,
	from peer.ID,
	to peer.ID,
	mes bsmsg.BitSwapMessage) error {

	n.mu.Lock()
	defer n.mu.Unlock()

	latencies, ok := n.latencies[from]
	if !ok {
		latencies = make(map[peer.ID]time.Duration)
		n.latencies[from] = latencies
	}

	latency, ok := latencies[to]
	if !ok {
		latency = n.delay.NextWaitTime()
		latencies[to] = latency
	}

	var bandwidthDelay time.Duration
	if n.isRateLimited {
		rateLimiters, ok := n.rateLimiters[from]
		if !ok {
			rateLimiters = make(map[peer.ID]*mocknet.RateLimiter)
			n.rateLimiters[from] = rateLimiters
		}

		rateLimiter, ok := rateLimiters[to]
		if !ok {
			rateLimiter = mocknet.NewRateLimiter(n.rateLimitGenerator.NextRateLimit())
			rateLimiters[to] = rateLimiter
		}

		size := mes.ToProtoV1().Size()
		bandwidthDelay = rateLimiter.Limit(size)
	} else {
		bandwidthDelay = 0
	}

	receiver, ok := n.clients[to]
	if !ok {
		return errors.New("cannot locate peer on network")
	}

	// nb: terminate the context since the context wouldn't actually be passed
	// over the network in a real scenario

	msg := &message{
		from:       from,
		msg:        mes,
		shouldSend: time.Now().Add(latency).Add(bandwidthDelay),
	}
	receiver.enqueue(msg)

	return nil
}

type networkClient struct {
	local peer.ID
	bsnet.Receiver
	network *network
	routing routing.IpfsRouting
	stats   bsnet.NetworkStats
}

func (nc *networkClient) SendMessage(
	ctx context.Context,
	to peer.ID,
	message bsmsg.BitSwapMessage) error {
	if err := nc.network.SendMessage(ctx, nc.local, to, message); err != nil {
		return err
	}
	atomic.AddUint64(&nc.stats.MessagesSent, 1)
	return nil
}

func (nc *networkClient) Stats() bsnet.NetworkStats {
	return bsnet.NetworkStats{
		MessagesRecvd: atomic.LoadUint64(&nc.stats.MessagesRecvd),
		MessagesSent:  atomic.LoadUint64(&nc.stats.MessagesSent),
	}
}

// FindProvidersAsync returns a channel of providers for the given key.
func (nc *networkClient) FindProvidersAsync(ctx context.Context, k cid.Cid, max int) <-chan peer.ID {

	// NB: this function duplicates the PeerInfo -> ID transformation in the
	// bitswap network adapter. Not to worry. This network client will be
	// deprecated once the ipfsnet.Mock is added. The code below is only
	// temporary.

	out := make(chan peer.ID)
	go func() {
		defer close(out)
		providers := nc.routing.FindProvidersAsync(ctx, k, max)
		for info := range providers {
			select {
			case <-ctx.Done():
			case out <- info.ID:
			}
		}
	}()
	return out
}

func (nc *networkClient) ConnectionManager() ifconnmgr.ConnManager {
	return &ifconnmgr.NullConnMgr{}
}

type messagePasser struct {
	net    *networkClient
	target peer.ID
	local  peer.ID
	ctx    context.Context
}

func (mp *messagePasser) SendMsg(ctx context.Context, m bsmsg.BitSwapMessage) error {
	return mp.net.SendMessage(ctx, mp.target, m)
}

func (mp *messagePasser) Close() error {
	return nil
}

func (mp *messagePasser) Reset() error {
	return nil
}

func (n *networkClient) NewMessageSender(ctx context.Context, p peer.ID) (bsnet.MessageSender, error) {
	return &messagePasser{
		net:    n,
		target: p,
		local:  n.local,
		ctx:    ctx,
	}, nil
}

// Provide provides the key to the network.
func (nc *networkClient) Provide(ctx context.Context, k cid.Cid) error {
	return nc.routing.Provide(ctx, k, true)
}

func (nc *networkClient) SetDelegate(r bsnet.Receiver) {
	nc.Receiver = r
}

func (nc *networkClient) ConnectTo(_ context.Context, p peer.ID) error {
	nc.network.mu.Lock()

	otherClient, ok := nc.network.clients[p]
	if !ok {
		nc.network.mu.Unlock()
		return errors.New("no such peer in network")
	}

	tag := tagForPeers(nc.local, p)
	if _, ok := nc.network.conns[tag]; ok {
		nc.network.mu.Unlock()
		log.Warning("ALREADY CONNECTED TO PEER (is this a reconnect? test lib needs fixing)")
		return nil
	}
	nc.network.conns[tag] = struct{}{}
	nc.network.mu.Unlock()

	// TODO: add handling for disconnects

	otherClient.receiver.PeerConnected(nc.local)
	nc.Receiver.PeerConnected(p)
	return nil
}

func (rq *receiverQueue) enqueue(m *message) {
	rq.lk.Lock()
	defer rq.lk.Unlock()
	rq.queue = append(rq.queue, m)
	if !rq.active {
		rq.active = true
		go rq.process()
	}
}

func (rq *receiverQueue) Swap(i, j int) {
	rq.queue[i], rq.queue[j] = rq.queue[j], rq.queue[i]
}

func (rq *receiverQueue) Len() int {
	return len(rq.queue)
}

func (rq *receiverQueue) Less(i, j int) bool {
	return rq.queue[i].shouldSend.UnixNano() < rq.queue[j].shouldSend.UnixNano()
}

func (rq *receiverQueue) process() {
	for {
		rq.lk.Lock()
		sort.Sort(rq)
		if len(rq.queue) == 0 {
			rq.active = false
			rq.lk.Unlock()
			return
		}
		m := rq.queue[0]
		if time.Until(m.shouldSend).Seconds() < 0.1 {
			rq.queue = rq.queue[1:]
			rq.lk.Unlock()
			time.Sleep(time.Until(m.shouldSend))
			atomic.AddUint64(&rq.receiver.stats.MessagesRecvd, 1)
			rq.receiver.ReceiveMessage(context.TODO(), m.from, m.msg)
		} else {
			rq.lk.Unlock()
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func tagForPeers(a, b peer.ID) string {
	if a < b {
		return string(a + b)
	}
	return string(b + a)
}
