package bitswap

import (
	"context"
	"fmt"
	"time"

	notifications "gx/ipfs/QmNkxFCmPtr2RQxjZNRCNryLud4L9wMEiBJsLgF14MqTHj/go-bitswap/notifications"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	lru "gx/ipfs/QmQjMHF8ptRgx4E57UFMiT4YM6kqaJeYxZ1MCDX23aw4rK/golang-lru"
	blocks "gx/ipfs/QmRcHuYzAyswytBuMF78rj3LTChYszomRFXNg4685ZN1WM/go-block-format"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	loggables "gx/ipfs/QmVrDtvvQCUeMZaY9UFkae6c85kdQ1GvVEhPrjPTdjxRLv/go-libp2p-loggables"
	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
)

const activeWantsLimit = 16

// Session holds state for an individual bitswap transfer operation.
// This allows bitswap to make smarter decisions about who to send wantlist
// info to, and who to request blocks from
type Session struct {
	ctx            context.Context
	tofetch        *cidQueue
	activePeers    map[peer.ID]struct{}
	activePeersArr []peer.ID

	bs           *Bitswap
	incoming     chan blkRecv
	newReqs      chan []cid.Cid
	cancelKeys   chan []cid.Cid
	interestReqs chan interestReq

	interest  *lru.Cache
	liveWants map[cid.Cid]time.Time

	tick          *time.Timer
	baseTickDelay time.Duration

	latTotal time.Duration
	fetchcnt int

	notif notifications.PubSub

	uuid logging.Loggable

	id  uint64
	tag string
}

// NewSession creates a new bitswap session whose lifetime is bounded by the
// given context
func (bs *Bitswap) NewSession(ctx context.Context) *Session {
	s := &Session{
		activePeers:   make(map[peer.ID]struct{}),
		liveWants:     make(map[cid.Cid]time.Time),
		newReqs:       make(chan []cid.Cid),
		cancelKeys:    make(chan []cid.Cid),
		tofetch:       newCidQueue(),
		interestReqs:  make(chan interestReq),
		ctx:           ctx,
		bs:            bs,
		incoming:      make(chan blkRecv),
		notif:         notifications.New(),
		uuid:          loggables.Uuid("GetBlockRequest"),
		baseTickDelay: time.Millisecond * 500,
		id:            bs.getNextSessionID(),
	}

	s.tag = fmt.Sprint("bs-ses-", s.id)

	cache, _ := lru.New(2048)
	s.interest = cache

	bs.sessLk.Lock()
	bs.sessions = append(bs.sessions, s)
	bs.sessLk.Unlock()

	go s.run(ctx)

	return s
}

func (bs *Bitswap) removeSession(s *Session) {
	s.notif.Shutdown()

	live := make([]cid.Cid, 0, len(s.liveWants))
	for c := range s.liveWants {
		live = append(live, c)
	}
	bs.CancelWants(live, s.id)

	bs.sessLk.Lock()
	defer bs.sessLk.Unlock()
	for i := 0; i < len(bs.sessions); i++ {
		if bs.sessions[i] == s {
			bs.sessions[i] = bs.sessions[len(bs.sessions)-1]
			bs.sessions = bs.sessions[:len(bs.sessions)-1]
			return
		}
	}
}

type blkRecv struct {
	from peer.ID
	blk  blocks.Block
}

func (s *Session) receiveBlockFrom(from peer.ID, blk blocks.Block) {
	select {
	case s.incoming <- blkRecv{from: from, blk: blk}:
	case <-s.ctx.Done():
	}
}

type interestReq struct {
	c    cid.Cid
	resp chan bool
}

// TODO: PERF: this is using a channel to guard a map access against race
// conditions. This is definitely much slower than a mutex, though its unclear
// if it will actually induce any noticeable slowness. This is implemented this
// way to avoid adding a more complex set of mutexes around the liveWants map.
// note that in the average case (where this session *is* interested in the
// block we received) this function will not be called, as the cid will likely
// still be in the interest cache.
func (s *Session) isLiveWant(c cid.Cid) bool {
	resp := make(chan bool, 1)
	select {
	case s.interestReqs <- interestReq{
		c:    c,
		resp: resp,
	}:
	case <-s.ctx.Done():
		return false
	}

	select {
	case want := <-resp:
		return want
	case <-s.ctx.Done():
		return false
	}
}

func (s *Session) interestedIn(c cid.Cid) bool {
	return s.interest.Contains(c) || s.isLiveWant(c)
}

const provSearchDelay = time.Second * 10

func (s *Session) addActivePeer(p peer.ID) {
	if _, ok := s.activePeers[p]; !ok {
		s.activePeers[p] = struct{}{}
		s.activePeersArr = append(s.activePeersArr, p)

		cmgr := s.bs.network.ConnectionManager()
		cmgr.TagPeer(p, s.tag, 10)
	}
}

func (s *Session) resetTick() {
	if s.latTotal == 0 {
		s.tick.Reset(provSearchDelay)
	} else {
		avLat := s.latTotal / time.Duration(s.fetchcnt)
		s.tick.Reset(s.baseTickDelay + (3 * avLat))
	}
}

func (s *Session) run(ctx context.Context) {
	s.tick = time.NewTimer(provSearchDelay)
	newpeers := make(chan peer.ID, 16)
	for {
		select {
		case blk := <-s.incoming:
			s.tick.Stop()

			if blk.from != "" {
				s.addActivePeer(blk.from)
			}

			s.receiveBlock(ctx, blk.blk)

			s.resetTick()
		case keys := <-s.newReqs:
			for _, k := range keys {
				s.interest.Add(k, nil)
			}
			if len(s.liveWants) < activeWantsLimit {
				toadd := activeWantsLimit - len(s.liveWants)
				if toadd > len(keys) {
					toadd = len(keys)
				}

				now := keys[:toadd]
				keys = keys[toadd:]

				s.wantBlocks(ctx, now)
			}
			for _, k := range keys {
				s.tofetch.Push(k)
			}
		case keys := <-s.cancelKeys:
			s.cancel(keys)

		case <-s.tick.C:
			live := make([]cid.Cid, 0, len(s.liveWants))
			now := time.Now()
			for c := range s.liveWants {
				live = append(live, c)
				s.liveWants[c] = now
			}

			// Broadcast these keys to everyone we're connected to
			s.bs.wm.WantBlocks(ctx, live, nil, s.id)

			if len(live) > 0 {
				go func(k cid.Cid) {
					// TODO: have a task queue setup for this to:
					// - rate limit
					// - manage timeouts
					// - ensure two 'findprovs' calls for the same block don't run concurrently
					// - share peers between sessions based on interest set
					for p := range s.bs.network.FindProvidersAsync(ctx, k, 10) {
						newpeers <- p
					}
				}(live[0])
			}
			s.resetTick()
		case p := <-newpeers:
			s.addActivePeer(p)
		case lwchk := <-s.interestReqs:
			lwchk.resp <- s.cidIsWanted(lwchk.c)
		case <-ctx.Done():
			s.tick.Stop()
			s.bs.removeSession(s)

			cmgr := s.bs.network.ConnectionManager()
			for _, p := range s.activePeersArr {
				cmgr.UntagPeer(p, s.tag)
			}
			return
		}
	}
}

func (s *Session) cidIsWanted(c cid.Cid) bool {
	_, ok := s.liveWants[c]
	if !ok {
		ok = s.tofetch.Has(c)
	}

	return ok
}

func (s *Session) receiveBlock(ctx context.Context, blk blocks.Block) {
	c := blk.Cid()
	if s.cidIsWanted(c) {
		tval, ok := s.liveWants[c]
		if ok {
			s.latTotal += time.Since(tval)
			delete(s.liveWants, c)
		} else {
			s.tofetch.Remove(c)
		}
		s.fetchcnt++
		s.notif.Publish(blk)

		if next := s.tofetch.Pop(); next.Defined() {
			s.wantBlocks(ctx, []cid.Cid{next})
		}
	}
}

func (s *Session) wantBlocks(ctx context.Context, ks []cid.Cid) {
	now := time.Now()
	for _, c := range ks {
		s.liveWants[c] = now
	}
	s.bs.wm.WantBlocks(ctx, ks, s.activePeersArr, s.id)
}

func (s *Session) cancel(keys []cid.Cid) {
	for _, c := range keys {
		s.tofetch.Remove(c)
	}
}

func (s *Session) cancelWants(keys []cid.Cid) {
	select {
	case s.cancelKeys <- keys:
	case <-s.ctx.Done():
	}
}

func (s *Session) fetch(ctx context.Context, keys []cid.Cid) {
	select {
	case s.newReqs <- keys:
	case <-ctx.Done():
	case <-s.ctx.Done():
	}
}

// GetBlocks fetches a set of blocks within the context of this session and
// returns a channel that found blocks will be returned on. No order is
// guaranteed on the returned blocks.
func (s *Session) GetBlocks(ctx context.Context, keys []cid.Cid) (<-chan blocks.Block, error) {
	ctx = logging.ContextWithLoggable(ctx, s.uuid)
	return getBlocksImpl(ctx, keys, s.notif, s.fetch, s.cancelWants)
}

// GetBlock fetches a single block
func (s *Session) GetBlock(parent context.Context, k cid.Cid) (blocks.Block, error) {
	return getBlock(parent, k, s.GetBlocks)
}

type cidQueue struct {
	elems []cid.Cid
	eset  *cid.Set
}

func newCidQueue() *cidQueue {
	return &cidQueue{eset: cid.NewSet()}
}

func (cq *cidQueue) Pop() cid.Cid {
	for {
		if len(cq.elems) == 0 {
			return cid.Cid{}
		}

		out := cq.elems[0]
		cq.elems = cq.elems[1:]

		if cq.eset.Has(out) {
			cq.eset.Remove(out)
			return out
		}
	}
}

func (cq *cidQueue) Push(c cid.Cid) {
	if cq.eset.Visit(c) {
		cq.elems = append(cq.elems, c)
	}
}

func (cq *cidQueue) Remove(c cid.Cid) {
	cq.eset.Remove(c)
}

func (cq *cidQueue) Has(c cid.Cid) bool {
	return cq.eset.Has(c)
}

func (cq *cidQueue) Len() int {
	return cq.eset.Len()
}
