package sessionrequestsplitter

import (
	"context"

	"gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
)

const (
	minReceivedToAdjustSplit = 2
	maxSplit                 = 16
	maxAcceptableDupes       = 0.4
	minDuplesToTryLessSplits = 0.2
	initialSplit             = 2
)

// PartialRequest is represents one slice of an over request split among peers
type PartialRequest struct {
	Peers []peer.ID
	Keys  []cid.Cid
}

type srsMessage interface {
	handle(srs *SessionRequestSplitter)
}

// SessionRequestSplitter track how many duplicate and unique blocks come in and
// uses that to determine how much to split up each set of wants among peers.
type SessionRequestSplitter struct {
	ctx      context.Context
	messages chan srsMessage

	// data, do not touch outside run loop
	receivedCount          int
	split                  int
	duplicateReceivedCount int
}

// New returns a new SessionRequestSplitter.
func New(ctx context.Context) *SessionRequestSplitter {
	srs := &SessionRequestSplitter{
		ctx:      ctx,
		messages: make(chan srsMessage, 10),
		split:    initialSplit,
	}
	go srs.run()
	return srs
}

// SplitRequest splits a request for the given cids one or more times among the
// given peers.
func (srs *SessionRequestSplitter) SplitRequest(peers []peer.ID, ks []cid.Cid) []*PartialRequest {
	resp := make(chan []*PartialRequest, 1)

	select {
	case srs.messages <- &splitRequestMessage{peers, ks, resp}:
	case <-srs.ctx.Done():
		return nil
	}
	select {
	case splitRequests := <-resp:
		return splitRequests
	case <-srs.ctx.Done():
		return nil
	}

}

// RecordDuplicateBlock records the fact that the session received a duplicate
// block and adjusts split factor as neccesary.
func (srs *SessionRequestSplitter) RecordDuplicateBlock() {
	select {
	case srs.messages <- &recordDuplicateMessage{}:
	case <-srs.ctx.Done():
	}
}

// RecordUniqueBlock records the fact that the session received unique block
// and adjusts the split factor as neccesary.
func (srs *SessionRequestSplitter) RecordUniqueBlock() {
	select {
	case srs.messages <- &recordUniqueMessage{}:
	case <-srs.ctx.Done():
	}
}

func (srs *SessionRequestSplitter) run() {
	for {
		select {
		case message := <-srs.messages:
			message.handle(srs)
		case <-srs.ctx.Done():
			return
		}
	}
}

func (srs *SessionRequestSplitter) duplicateRatio() float64 {
	return float64(srs.duplicateReceivedCount) / float64(srs.receivedCount)
}

type splitRequestMessage struct {
	peers []peer.ID
	ks    []cid.Cid
	resp  chan []*PartialRequest
}

func (s *splitRequestMessage) handle(srs *SessionRequestSplitter) {
	split := srs.split
	peers := s.peers
	ks := s.ks
	if len(peers) < split {
		split = len(peers)
	}
	peerSplits := splitPeers(peers, split)
	if len(ks) < split {
		split = len(ks)
	}
	keySplits := splitKeys(ks, split)
	splitRequests := make([]*PartialRequest, len(keySplits))
	for i := range splitRequests {
		splitRequests[i] = &PartialRequest{peerSplits[i], keySplits[i]}
	}
	s.resp <- splitRequests
}

type recordDuplicateMessage struct{}

func (r *recordDuplicateMessage) handle(srs *SessionRequestSplitter) {
	srs.receivedCount++
	srs.duplicateReceivedCount++
	if (srs.receivedCount > minReceivedToAdjustSplit) && (srs.duplicateRatio() > maxAcceptableDupes) && (srs.split < maxSplit) {
		srs.split++
	}
}

type recordUniqueMessage struct{}

func (r *recordUniqueMessage) handle(srs *SessionRequestSplitter) {
	srs.receivedCount++
	if (srs.split > 1) && (srs.duplicateRatio() < minDuplesToTryLessSplits) {
		srs.split--
	}

}
func splitKeys(ks []cid.Cid, split int) [][]cid.Cid {
	splits := make([][]cid.Cid, split)
	for i, c := range ks {
		pos := i % split
		splits[pos] = append(splits[pos], c)
	}
	return splits
}

func splitPeers(peers []peer.ID, split int) [][]peer.ID {
	splits := make([][]peer.ID, split)
	for i, p := range peers {
		pos := i % split
		splits[pos] = append(splits[pos], p)
	}
	return splits
}
