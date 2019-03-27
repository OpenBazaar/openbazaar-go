package wantmanager

import (
	"context"
	"math"

	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"
	bsmsg "gx/ipfs/QmcSPuzpSbVLU6UHU4e5PwZpm4fHbCn5SbNR5ZNL6Mj63G/go-bitswap/message"
	wantlist "gx/ipfs/QmcSPuzpSbVLU6UHU4e5PwZpm4fHbCn5SbNR5ZNL6Mj63G/go-bitswap/wantlist"

	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	metrics "gx/ipfs/QmekzFM3hPZjTjUFGTABdQkEnQ3PTiMstY198PwSFr5w1Q/go-metrics-interface"
)

var log = logging.Logger("bitswap")

const (
	// maxPriority is the max priority as defined by the bitswap protocol
	maxPriority = math.MaxInt32
)

// PeerHandler sends changes out to the network as they get added to the wantlist
// managed by the WantManager.
type PeerHandler interface {
	Disconnected(p peer.ID)
	Connected(p peer.ID, initialWants *wantlist.SessionTrackedWantlist)
	SendMessage(entries []bsmsg.Entry, targets []peer.ID, from uint64)
}

type wantMessage interface {
	handle(wm *WantManager)
}

// WantManager manages a global want list. It tracks two seperate want lists -
// one for all wants, and one for wants that are specifically broadcast to the
// internet.
type WantManager struct {
	// channel requests to the run loop
	// to get predictable behavior while running this in a go routine
	// having only one channel is neccesary, so requests are processed serially
	wantMessages chan wantMessage

	// synchronized by Run loop, only touch inside there
	wl   *wantlist.SessionTrackedWantlist
	bcwl *wantlist.SessionTrackedWantlist

	ctx    context.Context
	cancel func()

	peerHandler   PeerHandler
	wantlistGauge metrics.Gauge
}

// New initializes a new WantManager for a given context.
func New(ctx context.Context) *WantManager {
	ctx, cancel := context.WithCancel(ctx)
	wantlistGauge := metrics.NewCtx(ctx, "wantlist_total",
		"Number of items in wantlist.").Gauge()
	return &WantManager{
		wantMessages:  make(chan wantMessage, 10),
		wl:            wantlist.NewSessionTrackedWantlist(),
		bcwl:          wantlist.NewSessionTrackedWantlist(),
		ctx:           ctx,
		cancel:        cancel,
		wantlistGauge: wantlistGauge,
	}
}

// SetDelegate specifies who will send want changes out to the internet.
func (wm *WantManager) SetDelegate(peerHandler PeerHandler) {
	wm.peerHandler = peerHandler
}

// WantBlocks adds the given cids to the wantlist, tracked by the given session.
func (wm *WantManager) WantBlocks(ctx context.Context, ks []cid.Cid, peers []peer.ID, ses uint64) {
	log.Infof("want blocks: %s", ks)
	wm.addEntries(ctx, ks, peers, false, ses)
}

// CancelWants removes the given cids from the wantlist, tracked by the given session.
func (wm *WantManager) CancelWants(ctx context.Context, ks []cid.Cid, peers []peer.ID, ses uint64) {
	wm.addEntries(context.Background(), ks, peers, true, ses)
}

// IsWanted returns whether a CID is currently wanted.
func (wm *WantManager) IsWanted(c cid.Cid) bool {
	resp := make(chan bool, 1)
	select {
	case wm.wantMessages <- &isWantedMessage{c, resp}:
	case <-wm.ctx.Done():
		return false
	}
	select {
	case wanted := <-resp:
		return wanted
	case <-wm.ctx.Done():
		return false
	}
}

// CurrentWants returns the list of current wants.
func (wm *WantManager) CurrentWants() []wantlist.Entry {
	resp := make(chan []wantlist.Entry, 1)
	select {
	case wm.wantMessages <- &currentWantsMessage{resp}:
	case <-wm.ctx.Done():
		return nil
	}
	select {
	case wantlist := <-resp:
		return wantlist
	case <-wm.ctx.Done():
		return nil
	}
}

// CurrentBroadcastWants returns the current list of wants that are broadcasts.
func (wm *WantManager) CurrentBroadcastWants() []wantlist.Entry {
	resp := make(chan []wantlist.Entry, 1)
	select {
	case wm.wantMessages <- &currentBroadcastWantsMessage{resp}:
	case <-wm.ctx.Done():
		return nil
	}
	select {
	case wl := <-resp:
		return wl
	case <-wm.ctx.Done():
		return nil
	}
}

// WantCount returns the total count of wants.
func (wm *WantManager) WantCount() int {
	resp := make(chan int, 1)
	select {
	case wm.wantMessages <- &wantCountMessage{resp}:
	case <-wm.ctx.Done():
		return 0
	}
	select {
	case count := <-resp:
		return count
	case <-wm.ctx.Done():
		return 0
	}
}

// Connected is called when a new peer is connected
func (wm *WantManager) Connected(p peer.ID) {
	select {
	case wm.wantMessages <- &connectedMessage{p}:
	case <-wm.ctx.Done():
	}
}

// Disconnected is called when a peer is disconnected
func (wm *WantManager) Disconnected(p peer.ID) {
	select {
	case wm.wantMessages <- &disconnectedMessage{p}:
	case <-wm.ctx.Done():
	}
}

// Startup starts processing for the WantManager.
func (wm *WantManager) Startup() {
	go wm.run()
}

// Shutdown ends processing for the want manager.
func (wm *WantManager) Shutdown() {
	wm.cancel()
}

func (wm *WantManager) run() {
	// NOTE: Do not open any streams or connections from anywhere in this
	// event loop. Really, just don't do anything likely to block.
	for {
		select {
		case message := <-wm.wantMessages:
			message.handle(wm)
		case <-wm.ctx.Done():
			return
		}
	}
}

func (wm *WantManager) addEntries(ctx context.Context, ks []cid.Cid, targets []peer.ID, cancel bool, ses uint64) {
	entries := make([]bsmsg.Entry, 0, len(ks))
	for i, k := range ks {
		entries = append(entries, bsmsg.Entry{
			Cancel: cancel,
			Entry:  wantlist.NewRefEntry(k, maxPriority-i),
		})
	}
	select {
	case wm.wantMessages <- &wantSet{entries: entries, targets: targets, from: ses}:
	case <-wm.ctx.Done():
	case <-ctx.Done():
	}
}

type wantSet struct {
	entries []bsmsg.Entry
	targets []peer.ID
	from    uint64
}

func (ws *wantSet) handle(wm *WantManager) {
	// is this a broadcast or not?
	brdc := len(ws.targets) == 0

	// add changes to our wantlist
	for _, e := range ws.entries {
		if e.Cancel {
			if brdc {
				wm.bcwl.Remove(e.Cid, ws.from)
			}

			if wm.wl.Remove(e.Cid, ws.from) {
				wm.wantlistGauge.Dec()
			}
		} else {
			if brdc {
				wm.bcwl.AddEntry(e.Entry, ws.from)
			}
			if wm.wl.AddEntry(e.Entry, ws.from) {
				wm.wantlistGauge.Inc()
			}
		}
	}

	// broadcast those wantlist changes
	wm.peerHandler.SendMessage(ws.entries, ws.targets, ws.from)
}

type isWantedMessage struct {
	c    cid.Cid
	resp chan<- bool
}

func (iwm *isWantedMessage) handle(wm *WantManager) {
	_, isWanted := wm.wl.Contains(iwm.c)
	iwm.resp <- isWanted
}

type currentWantsMessage struct {
	resp chan<- []wantlist.Entry
}

func (cwm *currentWantsMessage) handle(wm *WantManager) {
	cwm.resp <- wm.wl.Entries()
}

type currentBroadcastWantsMessage struct {
	resp chan<- []wantlist.Entry
}

func (cbcwm *currentBroadcastWantsMessage) handle(wm *WantManager) {
	cbcwm.resp <- wm.bcwl.Entries()
}

type wantCountMessage struct {
	resp chan<- int
}

func (wcm *wantCountMessage) handle(wm *WantManager) {
	wcm.resp <- wm.wl.Len()
}

type connectedMessage struct {
	p peer.ID
}

func (cm *connectedMessage) handle(wm *WantManager) {
	wm.peerHandler.Connected(cm.p, wm.bcwl)
}

type disconnectedMessage struct {
	p peer.ID
}

func (dm *disconnectedMessage) handle(wm *WantManager) {
	wm.peerHandler.Disconnected(dm.p)
}
