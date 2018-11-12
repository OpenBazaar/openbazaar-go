package decision

import (
	"sync"
	"time"

	wantlist "gx/ipfs/QmNkxFCmPtr2RQxjZNRCNryLud4L9wMEiBJsLgF14MqTHj/go-bitswap/wantlist"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	pq "gx/ipfs/QmZUbTDJ39JpvtFCSubiWeUTQRvMA1tVE5RZCJrY4oeAsC/go-ipfs-pq"
)

type peerRequestQueue interface {
	// Pop returns the next peerRequestTask. Returns nil if the peerRequestQueue is empty.
	Pop() *peerRequestTask
	Push(to peer.ID, entries ...*wantlist.Entry)
	Remove(k cid.Cid, p peer.ID)

	// NB: cannot expose simply expose taskQueue.Len because trashed elements
	// may exist. These trashed elements should not contribute to the count.
}

func newPRQ() *prq {
	return &prq{
		taskMap:  make(map[taskEntryKey]*peerRequestTask),
		partners: make(map[peer.ID]*activePartner),
		frozen:   make(map[peer.ID]*activePartner),
		pQueue:   pq.New(partnerCompare),
	}
}

// verify interface implementation
var _ peerRequestQueue = &prq{}

// TODO: at some point, the strategy needs to plug in here
// to help decide how to sort tasks (on add) and how to select
// tasks (on getnext). For now, we are assuming a dumb/nice strategy.
type prq struct {
	lock     sync.Mutex
	pQueue   pq.PQ
	taskMap  map[taskEntryKey]*peerRequestTask
	partners map[peer.ID]*activePartner

	frozen map[peer.ID]*activePartner
}

// Push currently adds a new peerRequestTask to the end of the list
func (tl *prq) Push(to peer.ID, entries ...*wantlist.Entry) {
	tl.lock.Lock()
	defer tl.lock.Unlock()
	partner, ok := tl.partners[to]
	if !ok {
		partner = newActivePartner()
		tl.pQueue.Push(partner)
		tl.partners[to] = partner
	}

	partner.activelk.Lock()
	defer partner.activelk.Unlock()

	var priority int
	newEntries := make([]*wantlist.Entry, 0, len(entries))
	for _, entry := range entries {
		if partner.activeBlocks.Has(entry.Cid) {
			continue
		}
		if task, ok := tl.taskMap[taskEntryKey{to, entry.Cid}]; ok {
			if entry.Priority > task.Priority {
				task.Priority = entry.Priority
				partner.taskQueue.Update(task.index)
			}
			continue
		}
		if entry.Priority > priority {
			priority = entry.Priority
		}
		newEntries = append(newEntries, entry)
	}

	if len(newEntries) == 0 {
		return
	}

	task := &peerRequestTask{
		Entries: newEntries,
		Target:  to,
		created: time.Now(),
		Done: func(e []*wantlist.Entry) {
			tl.lock.Lock()
			for _, entry := range e {
				partner.TaskDone(entry.Cid)
			}
			tl.pQueue.Update(partner.Index())
			tl.lock.Unlock()
		},
	}
	task.Priority = priority
	partner.taskQueue.Push(task)
	for _, entry := range newEntries {
		tl.taskMap[taskEntryKey{to, entry.Cid}] = task
	}
	partner.requests += len(newEntries)
	tl.pQueue.Update(partner.Index())
}

// Pop 'pops' the next task to be performed. Returns nil if no task exists.
func (tl *prq) Pop() *peerRequestTask {
	tl.lock.Lock()
	defer tl.lock.Unlock()
	if tl.pQueue.Len() == 0 {
		return nil
	}
	partner := tl.pQueue.Pop().(*activePartner)

	var out *peerRequestTask
	for partner.taskQueue.Len() > 0 && partner.freezeVal == 0 {
		out = partner.taskQueue.Pop().(*peerRequestTask)

		newEntries := make([]*wantlist.Entry, 0, len(out.Entries))
		for _, entry := range out.Entries {
			delete(tl.taskMap, taskEntryKey{out.Target, entry.Cid})
			if entry.Trash {
				continue
			}
			partner.requests--
			partner.StartTask(entry.Cid)
			newEntries = append(newEntries, entry)
		}
		if len(newEntries) > 0 {
			out.Entries = newEntries
		} else {
			out = nil // discarding tasks that have been removed
			continue
		}
		break // and return |out|
	}

	tl.pQueue.Push(partner)
	return out
}

// Remove removes a task from the queue
func (tl *prq) Remove(k cid.Cid, p peer.ID) {
	tl.lock.Lock()
	t, ok := tl.taskMap[taskEntryKey{p, k}]
	if ok {
		for _, entry := range t.Entries {
			if entry.Cid.Equals(k) {
				// remove the task "lazily"
				// simply mark it as trash, so it'll be dropped when popped off the
				// queue.
				entry.Trash = true
				break
			}
		}

		// having canceled a block, we now account for that in the given partner
		partner := tl.partners[p]
		partner.requests--

		// we now also 'freeze' that partner. If they sent us a cancel for a
		// block we were about to send them, we should wait a short period of time
		// to make sure we receive any other in-flight cancels before sending
		// them a block they already potentially have
		if partner.freezeVal == 0 {
			tl.frozen[p] = partner
		}

		partner.freezeVal++
		tl.pQueue.Update(partner.index)
	}
	tl.lock.Unlock()
}

func (tl *prq) fullThaw() {
	tl.lock.Lock()
	defer tl.lock.Unlock()

	for id, partner := range tl.frozen {
		partner.freezeVal = 0
		delete(tl.frozen, id)
		tl.pQueue.Update(partner.index)
	}
}

func (tl *prq) thawRound() {
	tl.lock.Lock()
	defer tl.lock.Unlock()

	for id, partner := range tl.frozen {
		partner.freezeVal -= (partner.freezeVal + 1) / 2
		if partner.freezeVal <= 0 {
			delete(tl.frozen, id)
		}
		tl.pQueue.Update(partner.index)
	}
}

type peerRequestTask struct {
	Entries  []*wantlist.Entry
	Priority int
	Target   peer.ID

	// A callback to signal that this task has been completed
	Done func([]*wantlist.Entry)

	// created marks the time that the task was added to the queue
	created time.Time
	index   int // book-keeping field used by the pq container
}

// Index implements pq.Elem
func (t *peerRequestTask) Index() int {
	return t.index
}

// SetIndex implements pq.Elem
func (t *peerRequestTask) SetIndex(i int) {
	t.index = i
}

// taskEntryKey is a key identifying a task.
type taskEntryKey struct {
	p peer.ID
	k cid.Cid
}

// FIFO is a basic task comparator that returns tasks in the order created.
var FIFO = func(a, b *peerRequestTask) bool {
	return a.created.Before(b.created)
}

// V1 respects the target peer's wantlist priority. For tasks involving
// different peers, the oldest task is prioritized.
var V1 = func(a, b *peerRequestTask) bool {
	if a.Target == b.Target {
		return a.Priority > b.Priority
	}
	return FIFO(a, b)
}

func wrapCmp(f func(a, b *peerRequestTask) bool) func(a, b pq.Elem) bool {
	return func(a, b pq.Elem) bool {
		return f(a.(*peerRequestTask), b.(*peerRequestTask))
	}
}

type activePartner struct {

	// Active is the number of blocks this peer is currently being sent
	// active must be locked around as it will be updated externally
	activelk sync.Mutex
	active   int

	activeBlocks *cid.Set

	// requests is the number of blocks this peer is currently requesting
	// request need not be locked around as it will only be modified under
	// the peerRequestQueue's locks
	requests int

	// for the PQ interface
	index int

	freezeVal int

	// priority queue of tasks belonging to this peer
	taskQueue pq.PQ
}

func newActivePartner() *activePartner {
	return &activePartner{
		taskQueue:    pq.New(wrapCmp(V1)),
		activeBlocks: cid.NewSet(),
	}
}

// partnerCompare implements pq.ElemComparator
// returns true if peer 'a' has higher priority than peer 'b'
func partnerCompare(a, b pq.Elem) bool {
	pa := a.(*activePartner)
	pb := b.(*activePartner)

	// having no blocks in their wantlist means lowest priority
	// having both of these checks ensures stability of the sort
	if pa.requests == 0 {
		return false
	}
	if pb.requests == 0 {
		return true
	}

	if pa.freezeVal > pb.freezeVal {
		return false
	}
	if pa.freezeVal < pb.freezeVal {
		return true
	}

	if pa.active == pb.active {
		// sorting by taskQueue.Len() aids in cleaning out trash entries faster
		// if we sorted instead by requests, one peer could potentially build up
		// a huge number of cancelled entries in the queue resulting in a memory leak
		return pa.taskQueue.Len() > pb.taskQueue.Len()
	}
	return pa.active < pb.active
}

// StartTask signals that a task was started for this partner
func (p *activePartner) StartTask(k cid.Cid) {
	p.activelk.Lock()
	p.activeBlocks.Add(k)
	p.active++
	p.activelk.Unlock()
}

// TaskDone signals that a task was completed for this partner
func (p *activePartner) TaskDone(k cid.Cid) {
	p.activelk.Lock()
	p.activeBlocks.Remove(k)
	p.active--
	if p.active < 0 {
		panic("more tasks finished than started!")
	}
	p.activelk.Unlock()
}

// Index implements pq.Elem
func (p *activePartner) Index() int {
	return p.index
}

// SetIndex implements pq.Elem
func (p *activePartner) SetIndex(i int) {
	p.index = i
}
