package channels

import (
	"errors"
	"sync"
	"sync/atomic"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	peer "github.com/libp2p/go-libp2p-core/peer"
)

// channel represents all the parameters for a single data transfer
type channel struct {
	// an identifier for this channel shared by request and responder, set by requester through protocol
	transferID datatransfer.TransferID
	// base CID for the piece being transferred
	baseCid cid.Cid
	// portion of Piece to return, specified by an IPLD selector
	selector ipld.Node
	// used to verify this channel
	voucher datatransfer.Voucher
	// the party that is sending the data (not who initiated the request)
	sender peer.ID
	// the party that is receiving the data (not who initiated the request)
	recipient peer.ID
	// expected amount of data to be transferred
	totalSize uint64
}

// NewChannel makes a new channel
func NewChannel(transferID datatransfer.TransferID, baseCid cid.Cid,
	selector ipld.Node,
	voucher datatransfer.Voucher,
	sender peer.ID,
	recipient peer.ID,
	totalSize uint64) datatransfer.Channel {
	return channel{transferID, baseCid, selector, voucher, sender, recipient, totalSize}
}

// TransferID returns the transfer id for this channel
func (c channel) TransferID() datatransfer.TransferID { return c.transferID }

// BaseCID returns the CID that is at the root of this data transfer
func (c channel) BaseCID() cid.Cid { return c.baseCid }

// Selector returns the IPLD selector for this data transfer (represented as
// an IPLD node)
func (c channel) Selector() ipld.Node { return c.selector }

// Voucher returns the voucher for this data transfer
func (c channel) Voucher() datatransfer.Voucher { return c.voucher }

// Sender returns the peer id for the node that is sending data
func (c channel) Sender() peer.ID { return c.sender }

// Recipient returns the peer id for the node that is receiving data
func (c channel) Recipient() peer.ID { return c.recipient }

// TotalSize returns the total size for the data being transferred
func (c channel) TotalSize() uint64 { return c.totalSize }

// ChannelState is immutable channel data plus mutable state
type ChannelState struct {
	datatransfer.Channel
	// total bytes sent from this node (0 if receiver)
	sent uint64
	// total bytes received by this node (0 if sender)
	received uint64
}

// EmptyChannelState is the zero value for channel state, meaning not present
var EmptyChannelState = ChannelState{}

// Sent returns the number of bytes sent
func (c ChannelState) Sent() uint64 { return c.sent }

// Received returns the number of bytes received
func (c ChannelState) Received() uint64 { return c.received }

type internalChannel struct {
	datatransfer.Channel
	sent     *uint64
	received *uint64
}

// ErrNotFound is returned when a channel cannot be found with a given channel ID
var ErrNotFound = errors.New("No channel for this channel ID")

// ErrWrongType is returned when a caller attempts to change the type of implementation data after setting it
var ErrWrongType = errors.New("Cannot change type of implementation specific data after setting it")

// Channels is a thread safe list of channels
type Channels struct {
	channelsLk sync.RWMutex
	channels   map[datatransfer.ChannelID]internalChannel
}

// New returns a new thread safe list of channels
func New() *Channels {
	return &Channels{
		sync.RWMutex{},
		make(map[datatransfer.ChannelID]internalChannel),
	}
}

// CreateNew creates a new channel id and channel state and saves to channels.
// returns error if the channel exists already.
func (c *Channels) CreateNew(tid datatransfer.TransferID, baseCid cid.Cid, selector ipld.Node, voucher datatransfer.Voucher, initiator, dataSender, dataReceiver peer.ID) (datatransfer.ChannelID, error) {
	chid := datatransfer.ChannelID{Initiator: initiator, ID: tid}
	c.channelsLk.Lock()
	defer c.channelsLk.Unlock()
	_, ok := c.channels[chid]
	if ok {
		return chid, errors.New("tried to create channel but it already exists")
	}
	c.channels[chid] = internalChannel{Channel: NewChannel(0, baseCid, selector, voucher, dataSender, dataReceiver, 0), sent: new(uint64), received: new(uint64)}
	return chid, nil
}

// InProgress returns a list of in progress channels
func (c *Channels) InProgress() map[datatransfer.ChannelID]datatransfer.ChannelState {
	c.channelsLk.RLock()
	defer c.channelsLk.RUnlock()
	channelsCopy := make(map[datatransfer.ChannelID]datatransfer.ChannelState, len(c.channels))
	for channelID, internalChannel := range c.channels {
		channelsCopy[channelID] = ChannelState{
			internalChannel.Channel, atomic.LoadUint64(internalChannel.sent), atomic.LoadUint64(internalChannel.received),
		}
	}
	return channelsCopy
}

// GetByID searches for a channel in the slice of channels with id `chid`.
// Returns datatransfer.EmptyChannelState if there is no channel with that id
func (c *Channels) GetByID(chid datatransfer.ChannelID) (datatransfer.ChannelState, error) {
	c.channelsLk.RLock()
	internalChannel, ok := c.channels[chid]
	c.channelsLk.RUnlock()
	if !ok {
		return EmptyChannelState, ErrNotFound
	}
	return ChannelState{
		internalChannel.Channel, atomic.LoadUint64(internalChannel.sent), atomic.LoadUint64(internalChannel.received),
	}, nil
}

// IncrementSent increments the total sent on the given channel by the given amount (returning
// the new total)
func (c *Channels) IncrementSent(chid datatransfer.ChannelID, delta uint64) (uint64, error) {
	c.channelsLk.RLock()
	channel, ok := c.channels[chid]
	c.channelsLk.RUnlock()
	if !ok {
		return 0, ErrNotFound
	}
	return atomic.AddUint64(channel.sent, delta), nil
}

// IncrementReceived increments the total received on the given channel by the given amount (returning
// the new total)
func (c *Channels) IncrementReceived(chid datatransfer.ChannelID, delta uint64) (uint64, error) {
	c.channelsLk.RLock()
	channel, ok := c.channels[chid]
	c.channelsLk.RUnlock()
	if !ok {
		return 0, ErrNotFound
	}
	return atomic.AddUint64(channel.received, delta), nil
}
