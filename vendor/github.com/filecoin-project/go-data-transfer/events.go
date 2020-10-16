package datatransfer

import "time"

// EventCode is a name for an event that occurs on a data transfer channel
type EventCode int

const (
	// Open is an event occurs when a channel is first opened
	Open EventCode = iota

	// Accept is an event that emits when the data transfer is first accepted
	Accept

	// Restart is an event that emits when the data transfer is restarted
	Restart

	// DataReceived is emitted when data is received on the channel from a remote peer
	DataReceived

	// DataSent is emitted when data is sent on the channel to the remote peer
	DataSent

	// Cancel indicates one side has cancelled the transfer
	Cancel

	// Error is an event that emits when an error occurs in a data transfer
	Error

	// CleanupComplete emits when a request is cleaned up
	CleanupComplete

	// NewVoucher means we have a new voucher on this channel
	NewVoucher

	// NewVoucherResult means we have a new voucher result on this channel
	NewVoucherResult

	// PauseInitiator emits when the data sender pauses transfer
	PauseInitiator

	// ResumeInitiator emits when the data sender resumes transfer
	ResumeInitiator

	// PauseResponder emits when the data receiver pauses transfer
	PauseResponder

	// ResumeResponder emits when the data receiver resumes transfer
	ResumeResponder

	// FinishTransfer emits when the initiator has completed sending/receiving data
	FinishTransfer

	// ResponderCompletes emits when the initiator receives a message that the responder is finished
	ResponderCompletes

	// ResponderBeginsFinalization emits when the initiator receives a message that the responder is finilizing
	ResponderBeginsFinalization

	// BeginFinalizing emits when the responder completes its operations but awaits a response from the
	// initiator
	BeginFinalizing

	// Disconnected emits when we are not able to connect to the other party
	Disconnected

	// Complete is emitted when a data transfer is complete
	Complete

	// CompleteCleanupOnRestart is emitted when a data transfer channel is restarted to signal
	// that channels that were cleaning up should finish cleanup
	CompleteCleanupOnRestart
)

// Events are human readable names for data transfer events
var Events = map[EventCode]string{
	Open:                        "Open",
	Accept:                      "Accept",
	DataSent:                    "DataSent",
	DataReceived:                "DataReceived",
	Cancel:                      "Cancel",
	Error:                       "Error",
	CleanupComplete:             "CleanupComplete",
	NewVoucher:                  "NewVoucher",
	NewVoucherResult:            "NewVoucherResult",
	PauseInitiator:              "PauseInitiator",
	ResumeInitiator:             "ResumeInitiator",
	PauseResponder:              "PauseResponder",
	ResumeResponder:             "ResumeResponder",
	FinishTransfer:              "FinishTransfer",
	ResponderBeginsFinalization: "ResponderBeginsFinalization",
	ResponderCompletes:          "ResponderCompletes",
	BeginFinalizing:             "BeginFinalizing",
	Complete:                    "Complete",
	CompleteCleanupOnRestart:    "CompleteCleanupOnRestart",
}

// Event is a struct containing information about a data transfer event
type Event struct {
	Code      EventCode // What type of event it is
	Message   string    // Any clarifying information about the event
	Timestamp time.Time // when the event happened
}

// Subscriber is a callback that is called when events are emitted
type Subscriber func(event Event, channelState ChannelState)

// Unsubscribe is a function that gets called to unsubscribe from data transfer events
type Unsubscribe func()
