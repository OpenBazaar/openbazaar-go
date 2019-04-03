package sessionpeermanager

import (
	"context"
	"fmt"
	"math/rand"

	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"

	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
)

var log = logging.Logger("bitswap")

const (
	maxOptimizedPeers = 32
	reservePeers      = 2
)

// PeerTagger is an interface for tagging peers with metadata
type PeerTagger interface {
	TagPeer(peer.ID, string, int)
	UntagPeer(p peer.ID, tag string)
}

// PeerProviderFinder is an interface for finding providers
type PeerProviderFinder interface {
	FindProvidersAsync(context.Context, cid.Cid) <-chan peer.ID
}

type peerMessage interface {
	handle(spm *SessionPeerManager)
}

// SessionPeerManager tracks and manages peers for a session, and provides
// the best ones to the session
type SessionPeerManager struct {
	ctx            context.Context
	tagger         PeerTagger
	providerFinder PeerProviderFinder
	tag            string
	id             uint64

	peerMessages chan peerMessage

	// do not touch outside of run loop
	activePeers         map[peer.ID]bool
	unoptimizedPeersArr []peer.ID
	optimizedPeersArr   []peer.ID
}

// New creates a new SessionPeerManager
func New(ctx context.Context, id uint64, tagger PeerTagger, providerFinder PeerProviderFinder) *SessionPeerManager {
	spm := &SessionPeerManager{
		id:             id,
		ctx:            ctx,
		tagger:         tagger,
		providerFinder: providerFinder,
		peerMessages:   make(chan peerMessage, 16),
		activePeers:    make(map[peer.ID]bool),
	}

	spm.tag = fmt.Sprint("bs-ses-", id)

	go spm.run(ctx)
	return spm
}

// RecordPeerResponse records that a peer received a block, and adds to it
// the list of peers if it wasn't already added
func (spm *SessionPeerManager) RecordPeerResponse(p peer.ID, k cid.Cid) {

	// at the moment, we're just adding peers here
	// in the future, we'll actually use this to record metrics
	select {
	case spm.peerMessages <- &peerResponseMessage{p}:
	case <-spm.ctx.Done():
	}
}

// RecordPeerRequests records that a given set of peers requested the given cids
func (spm *SessionPeerManager) RecordPeerRequests(p []peer.ID, ks []cid.Cid) {
	// at the moment, we're not doing anything here
	// soon we'll use this to track latency by peer
}

// GetOptimizedPeers returns the best peers available for a session
func (spm *SessionPeerManager) GetOptimizedPeers() []peer.ID {
	// right now this just returns all peers, but soon we might return peers
	// ordered by optimization, or only a subset
	resp := make(chan []peer.ID, 1)
	select {
	case spm.peerMessages <- &peerReqMessage{resp}:
	case <-spm.ctx.Done():
		return nil
	}

	select {
	case peers := <-resp:
		return peers
	case <-spm.ctx.Done():
		return nil
	}
}

// FindMorePeers attempts to find more peers for a session by searching for
// providers for the given Cid
func (spm *SessionPeerManager) FindMorePeers(ctx context.Context, c cid.Cid) {
	go func(k cid.Cid) {
		for p := range spm.providerFinder.FindProvidersAsync(ctx, k) {

			select {
			case spm.peerMessages <- &peerFoundMessage{p}:
			case <-ctx.Done():
			case <-spm.ctx.Done():
			}
		}
	}(c)
}

func (spm *SessionPeerManager) run(ctx context.Context) {
	for {
		select {
		case pm := <-spm.peerMessages:
			pm.handle(spm)
		case <-ctx.Done():
			spm.handleShutdown()
			return
		}
	}
}

func (spm *SessionPeerManager) tagPeer(p peer.ID) {
	spm.tagger.TagPeer(p, spm.tag, 10)
}

func (spm *SessionPeerManager) insertOptimizedPeer(p peer.ID) {
	if len(spm.optimizedPeersArr) >= (maxOptimizedPeers - reservePeers) {
		tailPeer := spm.optimizedPeersArr[len(spm.optimizedPeersArr)-1]
		spm.optimizedPeersArr = spm.optimizedPeersArr[:len(spm.optimizedPeersArr)-1]
		spm.unoptimizedPeersArr = append(spm.unoptimizedPeersArr, tailPeer)
	}

	spm.optimizedPeersArr = append([]peer.ID{p}, spm.optimizedPeersArr...)
}

func (spm *SessionPeerManager) removeOptimizedPeer(p peer.ID) {
	for i := 0; i < len(spm.optimizedPeersArr); i++ {
		if spm.optimizedPeersArr[i] == p {
			spm.optimizedPeersArr = append(spm.optimizedPeersArr[:i], spm.optimizedPeersArr[i+1:]...)
			return
		}
	}
}

func (spm *SessionPeerManager) removeUnoptimizedPeer(p peer.ID) {
	for i := 0; i < len(spm.unoptimizedPeersArr); i++ {
		if spm.unoptimizedPeersArr[i] == p {
			spm.unoptimizedPeersArr[i] = spm.unoptimizedPeersArr[len(spm.unoptimizedPeersArr)-1]
			spm.unoptimizedPeersArr = spm.unoptimizedPeersArr[:len(spm.unoptimizedPeersArr)-1]
			return
		}
	}
}

type peerFoundMessage struct {
	p peer.ID
}

func (pfm *peerFoundMessage) handle(spm *SessionPeerManager) {
	p := pfm.p
	if _, ok := spm.activePeers[p]; !ok {
		spm.activePeers[p] = false
		spm.unoptimizedPeersArr = append(spm.unoptimizedPeersArr, p)
		spm.tagPeer(p)
	}
}

type peerResponseMessage struct {
	p peer.ID
}

func (prm *peerResponseMessage) handle(spm *SessionPeerManager) {

	p := prm.p
	isOptimized, ok := spm.activePeers[p]
	if !ok {
		spm.activePeers[p] = true
		spm.tagPeer(p)
	} else {
		if isOptimized {
			spm.removeOptimizedPeer(p)
		} else {
			spm.activePeers[p] = true
			spm.removeUnoptimizedPeer(p)
		}
	}
	spm.insertOptimizedPeer(p)
}

type peerReqMessage struct {
	resp chan<- []peer.ID
}

func (prm *peerReqMessage) handle(spm *SessionPeerManager) {
	randomOrder := rand.Perm(len(spm.unoptimizedPeersArr))
	maxPeers := len(spm.unoptimizedPeersArr) + len(spm.optimizedPeersArr)
	if maxPeers > maxOptimizedPeers {
		maxPeers = maxOptimizedPeers
	}

	extraPeers := make([]peer.ID, maxPeers-len(spm.optimizedPeersArr))
	for i := range extraPeers {
		extraPeers[i] = spm.unoptimizedPeersArr[randomOrder[i]]
	}
	prm.resp <- append(spm.optimizedPeersArr, extraPeers...)
}

func (spm *SessionPeerManager) handleShutdown() {
	for p := range spm.activePeers {
		spm.tagger.UntagPeer(p, spm.tag)
	}
}
