package datatransfer

import (
	"context"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipld/go-ipld-prime"
	peer "github.com/libp2p/go-libp2p-core/peer"
)

// EventsHandler are semantic data transfer events that happen as a result of graphsync hooks
type EventsHandler interface {
	// OnChannelOpened is called when we ask the other peer to send us data on the
	// given channel ID
	// return values are:
	// - nil = this channel is recognized
	// - error = ignore incoming data for this channel
	OnChannelOpened(chid ChannelID) error
	// OnResponseReceived is called when we receive a response to a request
	// - nil = continue receiving data
	// - error = cancel this request
	OnResponseReceived(chid ChannelID, msg Response) error
	// OnDataReceive is called when we receive data for the given channel ID
	// return values are:
	// - nil = proceed with sending data
	// - error = cancel this request
	// - err == ErrPause - pause this request
	OnDataReceived(chid ChannelID, link ipld.Link, size uint64) error
	// OnDataSent is called when we send data for the given channel ID
	// return values are:
	// message = data transfer message along with data
	// err = error
	// - nil = proceed with sending data
	// - error = cancel this request
	// - err == ErrPause - pause this request
	OnDataSent(chid ChannelID, link ipld.Link, size uint64) (Message, error)
	// OnRequestReceived is called when we receive a new request to send data
	// for the given channel ID
	// return values are:
	// message = data transfer message along with reply
	// err = error
	// - nil = proceed with sending data
	// - error = cancel this request
	// - err == ErrPause - pause this request (only for new requests)
	// - err == ErrResume - resume this request (only for update requests)
	OnRequestReceived(chid ChannelID, msg Request) (Response, error)
	// OnResponseCompleted is called when we finish sending data for the given channel ID
	// Error returns are logged but otherwise have not effect
	OnChannelCompleted(chid ChannelID, success bool) error

	// OnRequestTimedOut is called when a request we opened (with the given channel Id) to receive data times out.
	// Error returns are logged but otherwise have no effect
	OnRequestTimedOut(ctx context.Context, chid ChannelID) error

	// OnRequestDisconnected is called when a network error occurs in a graphsync request
	// or we appear to stall while receiving data
	OnRequestDisconnected(ctx context.Context, chid ChannelID) error
}

/*
Transport defines the interface for a transport layer for data
transfer. Where the data transfer manager will coordinate setting up push and
pull requests, validation, etc, the transport layer is responsible for moving
data back and forth, and may be medium specific. For example, some transports
may have the ability to pause and resume requests, while others may not.
Some may support individual data events, while others may only support message
events. Some transport layers may opt to use the actual data transfer network
protocols directly while others may be able to encode messages in their own
data protocol.

Transport is the minimum interface that must be satisfied to serve as a datatransfer
transport layer. Transports must be able to open (open is always called by the receiving peer)
and close channels, and set at an event handler */
type Transport interface {
	// OpenChannel initiates an outgoing request for the other peer to send data
	// to us on this channel
	// Note: from a data transfer symantic standpoint, it doesn't matter if the
	// request is push or pull -- OpenChannel is called by the party that is
	// intending to receive data
	OpenChannel(ctx context.Context,
		dataSender peer.ID,
		channelID ChannelID,
		root ipld.Link,
		stor ipld.Node,
		doNotSendCids []cid.Cid,
		msg Message) error

	// CloseChannel closes the given channel
	CloseChannel(ctx context.Context, chid ChannelID) error
	// SetEventHandler sets the handler for events on channels
	SetEventHandler(events EventsHandler) error
	// CleanupChannel is called on the otherside of a cancel - removes any associated
	// data for the channel
	CleanupChannel(chid ChannelID)
}

// PauseableTransport is a transport that can also pause and resume channels
type PauseableTransport interface {
	Transport
	// PauseChannel paused the given channel ID
	PauseChannel(ctx context.Context,
		chid ChannelID,
	) error
	// ResumeChannel resumes the given channel
	ResumeChannel(ctx context.Context,
		msg Message,
		chid ChannelID,
	) error
}
