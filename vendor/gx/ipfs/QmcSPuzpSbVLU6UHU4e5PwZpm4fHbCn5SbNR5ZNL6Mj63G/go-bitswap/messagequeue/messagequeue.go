package messagequeue

import (
	"context"
	"sync"
	"time"

	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"
	bsmsg "gx/ipfs/QmcSPuzpSbVLU6UHU4e5PwZpm4fHbCn5SbNR5ZNL6Mj63G/go-bitswap/message"
	bsnet "gx/ipfs/QmcSPuzpSbVLU6UHU4e5PwZpm4fHbCn5SbNR5ZNL6Mj63G/go-bitswap/network"
	wantlist "gx/ipfs/QmcSPuzpSbVLU6UHU4e5PwZpm4fHbCn5SbNR5ZNL6Mj63G/go-bitswap/wantlist"
)

var log = logging.Logger("bitswap")

const maxRetries = 10

// MessageNetwork is any network that can connect peers and generate a message
// sender.
type MessageNetwork interface {
	ConnectTo(context.Context, peer.ID) error
	NewMessageSender(context.Context, peer.ID) (bsnet.MessageSender, error)
}

type request interface {
	handle(mq *MessageQueue)
}

// MessageQueue implements queue of want messages to send to peers.
type MessageQueue struct {
	ctx     context.Context
	p       peer.ID
	network MessageNetwork

	newRequests      chan request
	outgoingMessages chan bsmsg.BitSwapMessage
	done             chan struct{}

	// do not touch out of run loop
	wl          *wantlist.SessionTrackedWantlist
	nextMessage bsmsg.BitSwapMessage
	sender      bsnet.MessageSender
}

type messageRequest struct {
	entries []bsmsg.Entry
	ses     uint64
}

type wantlistRequest struct {
	wl *wantlist.SessionTrackedWantlist
}

// New creats a new MessageQueue.
func New(ctx context.Context, p peer.ID, network MessageNetwork) *MessageQueue {
	return &MessageQueue{
		ctx:              ctx,
		wl:               wantlist.NewSessionTrackedWantlist(),
		network:          network,
		p:                p,
		newRequests:      make(chan request, 16),
		outgoingMessages: make(chan bsmsg.BitSwapMessage),
		done:             make(chan struct{}),
	}
}

// AddMessage adds new entries to an outgoing message for a given session.
func (mq *MessageQueue) AddMessage(entries []bsmsg.Entry, ses uint64) {
	select {
	case mq.newRequests <- newMessageRequest(entries, ses):
	case <-mq.ctx.Done():
	}
}

// AddWantlist adds a complete session tracked want list to a message queue
func (mq *MessageQueue) AddWantlist(initialWants *wantlist.SessionTrackedWantlist) {
	wl := wantlist.NewSessionTrackedWantlist()
	initialWants.CopyWants(wl)

	select {
	case mq.newRequests <- &wantlistRequest{wl}:
	case <-mq.ctx.Done():
	}
}

// Startup starts the processing of messages, and creates an initial message
// based on the given initial wantlist.
func (mq *MessageQueue) Startup() {
	go mq.runQueue()
	go mq.sendMessages()
}

// Shutdown stops the processing of messages for a message queue.
func (mq *MessageQueue) Shutdown() {
	close(mq.done)
}

func (mq *MessageQueue) runQueue() {
	outgoingMessages := func() chan bsmsg.BitSwapMessage {
		if mq.nextMessage == nil {
			return nil
		}
		return mq.outgoingMessages
	}

	for {
		select {
		case newRequest := <-mq.newRequests:
			newRequest.handle(mq)
		case outgoingMessages() <- mq.nextMessage:
			mq.nextMessage = nil
		case <-mq.done:
			if mq.sender != nil {
				mq.sender.Close()
			}
			return
		case <-mq.ctx.Done():
			if mq.sender != nil {
				mq.sender.Reset()
			}
			return
		}
	}
}

// We allocate a bunch of these so use a pool.
var messageRequestPool = sync.Pool{
	New: func() interface{} {
		return new(messageRequest)
	},
}

func newMessageRequest(entries []bsmsg.Entry, session uint64) *messageRequest {
	mr := messageRequestPool.Get().(*messageRequest)
	mr.entries = entries
	mr.ses = session
	return mr
}

func returnMessageRequest(mr *messageRequest) {
	*mr = messageRequest{}
	messageRequestPool.Put(mr)
}

func (mr *messageRequest) handle(mq *MessageQueue) {
	mq.addEntries(mr.entries, mr.ses)
	returnMessageRequest(mr)
}

func (wr *wantlistRequest) handle(mq *MessageQueue) {
	initialWants := wr.wl
	initialWants.CopyWants(mq.wl)
	if initialWants.Len() > 0 {
		if mq.nextMessage == nil {
			mq.nextMessage = bsmsg.New(false)
		}
		for _, e := range initialWants.Entries() {
			mq.nextMessage.AddEntry(e.Cid, e.Priority)
		}
	}
}

func (mq *MessageQueue) addEntries(entries []bsmsg.Entry, ses uint64) {
	for _, e := range entries {
		if e.Cancel {
			if mq.wl.Remove(e.Cid, ses) {
				if mq.nextMessage == nil {
					mq.nextMessage = bsmsg.New(false)
				}
				mq.nextMessage.Cancel(e.Cid)
			}
		} else {
			if mq.wl.Add(e.Cid, e.Priority, ses) {
				if mq.nextMessage == nil {
					mq.nextMessage = bsmsg.New(false)
				}
				mq.nextMessage.AddEntry(e.Cid, e.Priority)
			}
		}
	}
}

func (mq *MessageQueue) sendMessages() {
	for {
		select {
		case nextMessage := <-mq.outgoingMessages:
			mq.sendMessage(nextMessage)
		case <-mq.done:
			return
		case <-mq.ctx.Done():
			return
		}
	}
}

func (mq *MessageQueue) sendMessage(message bsmsg.BitSwapMessage) {

	err := mq.initializeSender()
	if err != nil {
		log.Infof("cant open message sender to peer %s: %s", mq.p, err)
		// TODO: cant connect, what now?
		return
	}

	for i := 0; i < maxRetries; i++ { // try to send this message until we fail.
		if mq.attemptSendAndRecovery(message) {
			return
		}
	}
}

func (mq *MessageQueue) initializeSender() error {
	if mq.sender != nil {
		return nil
	}
	nsender, err := openSender(mq.ctx, mq.network, mq.p)
	if err != nil {
		return err
	}
	mq.sender = nsender
	return nil
}

func (mq *MessageQueue) attemptSendAndRecovery(message bsmsg.BitSwapMessage) bool {
	err := mq.sender.SendMsg(mq.ctx, message)
	if err == nil {
		return true
	}

	log.Infof("bitswap send error: %s", err)
	mq.sender.Reset()
	mq.sender = nil

	select {
	case <-mq.done:
		return true
	case <-mq.ctx.Done():
		return true
	case <-time.After(time.Millisecond * 100):
		// wait 100ms in case disconnect notifications are still propogating
		log.Warning("SendMsg errored but neither 'done' nor context.Done() were set")
	}

	err = mq.initializeSender()
	if err != nil {
		log.Infof("couldnt open sender again after SendMsg(%s) failed: %s", mq.p, err)
		// TODO(why): what do we do now?
		// I think the *right* answer is to probably put the message we're
		// trying to send back, and then return to waiting for new work or
		// a disconnect.
		return true
	}

	// TODO: Is this the same instance for the remote peer?
	// If its not, we should resend our entire wantlist to them
	/*
		if mq.sender.InstanceID() != mq.lastSeenInstanceID {
			wlm = mq.getFullWantlistMessage()
		}
	*/
	return false
}

func openSender(ctx context.Context, network MessageNetwork, p peer.ID) (bsnet.MessageSender, error) {
	// allow ten minutes for connections this includes looking them up in the
	// dht dialing them, and handshaking
	conctx, cancel := context.WithTimeout(ctx, time.Minute*10)
	defer cancel()

	err := network.ConnectTo(conctx, p)
	if err != nil {
		return nil, err
	}

	nsender, err := network.NewMessageSender(ctx, p)
	if err != nil {
		return nil, err
	}

	return nsender, nil
}
