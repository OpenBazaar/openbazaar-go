// package bitswap implements the IPFS exchange interface with the BitSwap
// bilateral exchange protocol.
package bitswap

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	decision "gx/ipfs/QmNkxFCmPtr2RQxjZNRCNryLud4L9wMEiBJsLgF14MqTHj/go-bitswap/decision"
	bsmsg "gx/ipfs/QmNkxFCmPtr2RQxjZNRCNryLud4L9wMEiBJsLgF14MqTHj/go-bitswap/message"
	bsnet "gx/ipfs/QmNkxFCmPtr2RQxjZNRCNryLud4L9wMEiBJsLgF14MqTHj/go-bitswap/network"
	notifications "gx/ipfs/QmNkxFCmPtr2RQxjZNRCNryLud4L9wMEiBJsLgF14MqTHj/go-bitswap/notifications"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	exchange "gx/ipfs/QmR1nncPsZR14A4hWr39mq8Lm7BGgS68bHVT9nop8NpWEM/go-ipfs-exchange-interface"
	delay "gx/ipfs/QmRJVNatYJwTAHgdSM1Xef9QVQ1Ch3XHdmcrykjP5Y4soL/go-ipfs-delay"
	flags "gx/ipfs/QmRMGdC6HKdLsPDABL9aXPDidrpmEHzJqFWSvshkbn9Hj8/go-ipfs-flags"
	blocks "gx/ipfs/QmRcHuYzAyswytBuMF78rj3LTChYszomRFXNg4685ZN1WM/go-block-format"
	process "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	procctx "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess/context"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
	blockstore "gx/ipfs/QmcDDgAXDbpDUpadCJKLr49KYR4HuL7T8Z1dZTHt6ixsoR/go-ipfs-blockstore"
	metrics "gx/ipfs/QmekzFM3hPZjTjUFGTABdQkEnQ3PTiMstY198PwSFr5w1Q/go-metrics-interface"
)

var log = logging.Logger("bitswap")

const (
	// maxProvidersPerRequest specifies the maximum number of providers desired
	// from the network. This value is specified because the network streams
	// results.
	// TODO: if a 'non-nice' strategy is implemented, consider increasing this value
	maxProvidersPerRequest = 3
	findProviderDelay      = 1 * time.Second
	providerRequestTimeout = time.Second * 10
	provideTimeout         = time.Second * 15
	sizeBatchRequestChan   = 32
	// kMaxPriority is the max priority as defined by the bitswap protocol
	kMaxPriority = math.MaxInt32
)

var (
	HasBlockBufferSize    = 256
	provideKeysBufferSize = 2048
	provideWorkerMax      = 512

	// the 1<<18+15 is to observe old file chunks that are 1<<18 + 14 in size
	metricsBuckets = []float64{1 << 6, 1 << 10, 1 << 14, 1 << 18, 1<<18 + 15, 1 << 22}
)

func init() {
	if flags.LowMemMode {
		HasBlockBufferSize = 64
		provideKeysBufferSize = 512
		provideWorkerMax = 16
	}
}

var rebroadcastDelay = delay.Fixed(time.Minute)

// New initializes a BitSwap instance that communicates over the provided
// BitSwapNetwork. This function registers the returned instance as the network
// delegate.
// Runs until context is cancelled.
func New(parent context.Context, network bsnet.BitSwapNetwork,
	bstore blockstore.Blockstore) exchange.Interface {

	// important to use provided parent context (since it may include important
	// loggable data). It's probably not a good idea to allow bitswap to be
	// coupled to the concerns of the ipfs daemon in this way.
	//
	// FIXME(btc) Now that bitswap manages itself using a process, it probably
	// shouldn't accept a context anymore. Clients should probably use Close()
	// exclusively. We should probably find another way to share logging data
	ctx, cancelFunc := context.WithCancel(parent)
	ctx = metrics.CtxSubScope(ctx, "bitswap")
	dupHist := metrics.NewCtx(ctx, "recv_dup_blocks_bytes", "Summary of duplicate"+
		" data blocks recived").Histogram(metricsBuckets)
	allHist := metrics.NewCtx(ctx, "recv_all_blocks_bytes", "Summary of all"+
		" data blocks recived").Histogram(metricsBuckets)

	notif := notifications.New()
	px := process.WithTeardown(func() error {
		notif.Shutdown()
		return nil
	})

	bs := &Bitswap{
		blockstore:    bstore,
		notifications: notif,
		engine:        decision.NewEngine(ctx, bstore), // TODO close the engine with Close() method
		network:       network,
		findKeys:      make(chan *blockRequest, sizeBatchRequestChan),
		process:       px,
		newBlocks:     make(chan cid.Cid, HasBlockBufferSize),
		provideKeys:   make(chan cid.Cid, provideKeysBufferSize),
		wm:            NewWantManager(ctx, network),
		counters:      new(counters),

		dupMetric: dupHist,
		allMetric: allHist,
	}
	go bs.wm.Run()
	network.SetDelegate(bs)

	// Start up bitswaps async worker routines
	bs.startWorkers(px, ctx)

	// bind the context and process.
	// do it over here to avoid closing before all setup is done.
	go func() {
		<-px.Closing() // process closes first
		cancelFunc()
	}()
	procctx.CloseAfterContext(px, ctx) // parent cancelled first

	return bs
}

// Bitswap instances implement the bitswap protocol.
type Bitswap struct {
	// the peermanager manages sending messages to peers in a way that
	// wont block bitswap operation
	wm *WantManager

	// the engine is the bit of logic that decides who to send which blocks to
	engine *decision.Engine

	// network delivers messages on behalf of the session
	network bsnet.BitSwapNetwork

	// blockstore is the local database
	// NB: ensure threadsafety
	blockstore blockstore.Blockstore

	// notifications engine for receiving new blocks and routing them to the
	// appropriate user requests
	notifications notifications.PubSub

	// findKeys sends keys to a worker to find and connect to providers for them
	findKeys chan *blockRequest
	// newBlocks is a channel for newly added blocks to be provided to the
	// network.  blocks pushed down this channel get buffered and fed to the
	// provideKeys channel later on to avoid too much network activity
	newBlocks chan cid.Cid
	// provideKeys directly feeds provide workers
	provideKeys chan cid.Cid

	process process.Process

	// Counters for various statistics
	counterLk sync.Mutex
	counters  *counters

	// Metrics interface metrics
	dupMetric metrics.Histogram
	allMetric metrics.Histogram

	// Sessions
	sessions []*Session
	sessLk   sync.Mutex

	sessID   uint64
	sessIDLk sync.Mutex
}

type counters struct {
	blocksRecvd    uint64
	dupBlocksRecvd uint64
	dupDataRecvd   uint64
	blocksSent     uint64
	dataSent       uint64
	dataRecvd      uint64
	messagesRecvd  uint64
}

type blockRequest struct {
	Cid cid.Cid
	Ctx context.Context
}

// GetBlock attempts to retrieve a particular block from peers within the
// deadline enforced by the context.
func (bs *Bitswap) GetBlock(parent context.Context, k cid.Cid) (blocks.Block, error) {
	return getBlock(parent, k, bs.GetBlocks)
}

func (bs *Bitswap) WantlistForPeer(p peer.ID) []cid.Cid {
	var out []cid.Cid
	for _, e := range bs.engine.WantlistForPeer(p) {
		out = append(out, e.Cid)
	}
	return out
}

func (bs *Bitswap) LedgerForPeer(p peer.ID) *decision.Receipt {
	return bs.engine.LedgerForPeer(p)
}

// GetBlocks returns a channel where the caller may receive blocks that
// correspond to the provided |keys|. Returns an error if BitSwap is unable to
// begin this request within the deadline enforced by the context.
//
// NB: Your request remains open until the context expires. To conserve
// resources, provide a context with a reasonably short deadline (ie. not one
// that lasts throughout the lifetime of the server)
func (bs *Bitswap) GetBlocks(ctx context.Context, keys []cid.Cid) (<-chan blocks.Block, error) {
	if len(keys) == 0 {
		out := make(chan blocks.Block)
		close(out)
		return out, nil
	}

	select {
	case <-bs.process.Closing():
		return nil, errors.New("bitswap is closed")
	default:
	}
	promise := bs.notifications.Subscribe(ctx, keys...)

	for _, k := range keys {
		log.Event(ctx, "Bitswap.GetBlockRequest.Start", k)
	}

	mses := bs.getNextSessionID()

	bs.wm.WantBlocks(ctx, keys, nil, mses)

	remaining := cid.NewSet()
	for _, k := range keys {
		remaining.Add(k)
	}

	out := make(chan blocks.Block)
	go func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer close(out)
		defer func() {
			// can't just defer this call on its own, arguments are resolved *when* the defer is created
			bs.CancelWants(remaining.Keys(), mses)
		}()
		findProvsDelay := time.NewTimer(findProviderDelay)
		defer findProvsDelay.Stop()

		findProvsDelayCh := findProvsDelay.C
		req := &blockRequest{
			Cid: keys[0],
			Ctx: ctx,
		}

		var findProvsReqCh chan<- *blockRequest

		for {
			select {
			case <-findProvsDelayCh:
				// NB: Optimization. Assumes that providers of key[0] are likely to
				// be able to provide for all keys. This currently holds true in most
				// every situation. Later, this assumption may not hold as true.
				findProvsReqCh = bs.findKeys
				findProvsDelayCh = nil
			case findProvsReqCh <- req:
				findProvsReqCh = nil
			case blk, ok := <-promise:
				if !ok {
					return
				}

				// No need to find providers now.
				findProvsDelay.Stop()
				findProvsDelayCh = nil
				findProvsReqCh = nil

				bs.CancelWants([]cid.Cid{blk.Cid()}, mses)
				remaining.Remove(blk.Cid())
				select {
				case out <- blk:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

func (bs *Bitswap) getNextSessionID() uint64 {
	bs.sessIDLk.Lock()
	defer bs.sessIDLk.Unlock()
	bs.sessID++
	return bs.sessID
}

// CancelWant removes a given key from the wantlist
func (bs *Bitswap) CancelWants(cids []cid.Cid, ses uint64) {
	if len(cids) == 0 {
		return
	}
	bs.wm.CancelWants(context.Background(), cids, nil, ses)
}

// HasBlock announces the existence of a block to this bitswap service. The
// service will potentially notify its peers.
func (bs *Bitswap) HasBlock(blk blocks.Block) error {
	return bs.receiveBlockFrom(blk, "")
}

// TODO: Some of this stuff really only needs to be done when adding a block
// from the user, not when receiving it from the network.
// In case you run `git blame` on this comment, I'll save you some time: ask
// @whyrusleeping, I don't know the answers you seek.
func (bs *Bitswap) receiveBlockFrom(blk blocks.Block, from peer.ID) error {
	select {
	case <-bs.process.Closing():
		return errors.New("bitswap is closed")
	default:
	}

	err := bs.blockstore.Put(blk)
	if err != nil {
		log.Errorf("Error writing block to datastore: %s", err)
		return err
	}

	// NOTE: There exists the possiblity for a race condition here.  If a user
	// creates a node, then adds it to the dagservice while another goroutine
	// is waiting on a GetBlock for that object, they will receive a reference
	// to the same node. We should address this soon, but i'm not going to do
	// it now as it requires more thought and isnt causing immediate problems.
	bs.notifications.Publish(blk)

	k := blk.Cid()
	ks := []cid.Cid{k}
	for _, s := range bs.SessionsForBlock(k) {
		s.receiveBlockFrom(from, blk)
		bs.CancelWants(ks, s.id)
	}

	bs.engine.AddBlock(blk)

	select {
	case bs.newBlocks <- blk.Cid():
		// send block off to be reprovided
	case <-bs.process.Closing():
		return bs.process.Close()
	}
	return nil
}

// SessionsForBlock returns a slice of all sessions that may be interested in the given cid
func (bs *Bitswap) SessionsForBlock(c cid.Cid) []*Session {
	bs.sessLk.Lock()
	defer bs.sessLk.Unlock()

	var out []*Session
	for _, s := range bs.sessions {
		if s.interestedIn(c) {
			out = append(out, s)
		}
	}
	return out
}

func (bs *Bitswap) ReceiveMessage(ctx context.Context, p peer.ID, incoming bsmsg.BitSwapMessage) {
	atomic.AddUint64(&bs.counters.messagesRecvd, 1)

	// This call records changes to wantlists, blocks received,
	// and number of bytes transfered.
	bs.engine.MessageReceived(p, incoming)
	// TODO: this is bad, and could be easily abused.
	// Should only track *useful* messages in ledger

	iblocks := incoming.Blocks()

	if len(iblocks) == 0 {
		return
	}

	wg := sync.WaitGroup{}
	for _, block := range iblocks {
		wg.Add(1)
		go func(b blocks.Block) { // TODO: this probably doesnt need to be a goroutine...
			defer wg.Done()

			bs.updateReceiveCounters(b)

			log.Debugf("got block %s from %s", b, p)

			if err := bs.receiveBlockFrom(b, p); err != nil {
				log.Warningf("ReceiveMessage recvBlockFrom error: %s", err)
			}
			log.Event(ctx, "Bitswap.GetBlockRequest.End", b.Cid())
		}(block)
	}
	wg.Wait()
}

var ErrAlreadyHaveBlock = errors.New("already have block")

func (bs *Bitswap) updateReceiveCounters(b blocks.Block) {
	blkLen := len(b.RawData())
	has, err := bs.blockstore.Has(b.Cid())
	if err != nil {
		log.Infof("blockstore.Has error: %s", err)
		return
	}

	bs.allMetric.Observe(float64(blkLen))
	if has {
		bs.dupMetric.Observe(float64(blkLen))
	}

	bs.counterLk.Lock()
	defer bs.counterLk.Unlock()
	c := bs.counters

	c.blocksRecvd++
	c.dataRecvd += uint64(len(b.RawData()))
	if has {
		c.dupBlocksRecvd++
		c.dupDataRecvd += uint64(blkLen)
	}
}

// Connected/Disconnected warns bitswap about peer connections
func (bs *Bitswap) PeerConnected(p peer.ID) {
	bs.wm.Connected(p)
	bs.engine.PeerConnected(p)
}

// Connected/Disconnected warns bitswap about peer connections
func (bs *Bitswap) PeerDisconnected(p peer.ID) {
	bs.wm.Disconnected(p)
	bs.engine.PeerDisconnected(p)
}

func (bs *Bitswap) ReceiveError(err error) {
	log.Infof("Bitswap ReceiveError: %s", err)
	// TODO log the network error
	// TODO bubble the network error up to the parent context/error logger
}

func (bs *Bitswap) Close() error {
	return bs.process.Close()
}

func (bs *Bitswap) GetWantlist() []cid.Cid {
	entries := bs.wm.wl.Entries()
	out := make([]cid.Cid, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Cid)
	}
	return out
}

func (bs *Bitswap) IsOnline() bool {
	return true
}
