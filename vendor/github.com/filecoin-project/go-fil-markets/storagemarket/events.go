package storagemarket

// ClientEvent is an event that happens in the client's deal state machine
type ClientEvent uint64

const (
	// ClientEventOpen indicates a new deal was started
	ClientEventOpen ClientEvent = iota

	// ClientEventEnsureFundsFailed happens when attempting to ensure the client has enough funds available fails
	ClientEventEnsureFundsFailed

	// ClientEventFundingInitiated happens when a client has sent a message adding funds to its balance
	ClientEventFundingInitiated

	// ClientEventFundsEnsured happens when a client successfully ensures it has funds for a deal
	ClientEventFundsEnsured

	// ClientEventWriteProposalFailed indicates an attempt to send a deal proposal to a provider failed
	ClientEventWriteProposalFailed

	// ClientEventInitiateDataTransfer happens when a a client is ready to transfer data to a provider
	ClientEventInitiateDataTransfer

	// ClientEventDataTransferInitiated happens when piece data transfer has started
	ClientEventDataTransferInitiated

	// ClientEventDataTransferComplete happens when piece data transfer has been completed
	ClientEventDataTransferComplete

	// ClientEventWaitForDealState happens when the client needs to continue waiting for an actionable deal state
	ClientEventWaitForDealState

	// ClientEventDataTransferFailed happens the client can't initiate a push data transfer to the provider
	ClientEventDataTransferFailed

	// ClientEventReadResponseFailed means a network error occurred reading a deal response
	ClientEventReadResponseFailed

	// ClientEventResponseVerificationFailed means a response was not verified
	ClientEventResponseVerificationFailed

	// ClientEventResponseDealDidNotMatch means a response was sent for the wrong deal
	ClientEventResponseDealDidNotMatch

	// ClientEventUnexpectedDealState means a response was sent but the state wasn't what we expected
	ClientEventUnexpectedDealState

	// ClientEventStreamCloseError happens when an attempt to close a deals stream fails
	ClientEventStreamCloseError

	// ClientEventDealRejected happens when the provider does not accept a deal
	ClientEventDealRejected

	// ClientEventDealAccepted happens when a client receives a response accepting a deal from a provider
	ClientEventDealAccepted

	// ClientEventDealPublishFailed happens when a client cannot verify a deal was published
	ClientEventDealPublishFailed

	// ClientEventDealPublished happens when a deal is successfully published
	ClientEventDealPublished

	// ClientEventDealActivationFailed happens when a client cannot verify a deal was activated
	ClientEventDealActivationFailed

	// ClientEventDealActivated happens when a deal is successfully activated
	ClientEventDealActivated

	// ClientEventDealCompletionFailed happens when a client cannot verify a deal expired or was slashed
	ClientEventDealCompletionFailed

	// ClientEventDealExpired happens when a deal expires
	ClientEventDealExpired

	// ClientEventDealSlashed happens when a deal is slashed
	ClientEventDealSlashed

	// ClientEventFailed happens when a deal terminates in failure
	ClientEventFailed

	// ClientEventRestart is used to resume the deal after a state machine shutdown
	ClientEventRestart
)

// ClientEvents maps client event codes to string names
var ClientEvents = map[ClientEvent]string{
	ClientEventOpen:                       "ClientEventOpen",
	ClientEventEnsureFundsFailed:          "ClientEventEnsureFundsFailed",
	ClientEventFundingInitiated:           "ClientEventFundingInitiated",
	ClientEventFundsEnsured:               "ClientEventFundsEnsured",
	ClientEventWriteProposalFailed:        "ClientEventWriteProposalFailed",
	ClientEventInitiateDataTransfer:       "ClientEventInitiateDataTransfer",
	ClientEventDataTransferInitiated:      "ClientEventDataTransferInitiated",
	ClientEventDataTransferComplete:       "ClientEventDataTransferComplete",
	ClientEventWaitForDealState:           "ClientEventWaitForDealState",
	ClientEventDataTransferFailed:         "ClientEventDataTransferFailed",
	ClientEventReadResponseFailed:         "ClientEventReadResponseFailed",
	ClientEventResponseVerificationFailed: "ClientEventResponseVerificationFailed",
	ClientEventResponseDealDidNotMatch:    "ClientEventResponseDealDidNotMatch",
	ClientEventUnexpectedDealState:        "ClientEventUnexpectedDealState",
	ClientEventStreamCloseError:           "ClientEventStreamCloseError",
	ClientEventDealRejected:               "ClientEventDealRejected",
	ClientEventDealAccepted:               "ClientEventDealAccepted",
	ClientEventDealPublishFailed:          "ClientEventDealPublishFailed",
	ClientEventDealPublished:              "ClientEventDealPublished",
	ClientEventDealActivationFailed:       "ClientEventDealActivationFailed",
	ClientEventDealActivated:              "ClientEventDealActivated",
	ClientEventDealCompletionFailed:       "ClientEventDealCompletionFailed",
	ClientEventDealExpired:                "ClientEventDealExpired",
	ClientEventDealSlashed:                "ClientEventDealSlashed",
	ClientEventFailed:                     "ClientEventFailed",
	ClientEventRestart:                    "ClientEventRestart",
}

// ProviderEvent is an event that happens in the provider's deal state machine
type ProviderEvent uint64

const (
	// ProviderEventOpen indicates a new deal proposal has been received
	ProviderEventOpen ProviderEvent = iota

	// ProviderEventNodeErrored indicates an error happened talking to the node implementation
	ProviderEventNodeErrored

	// ProviderEventDealDeciding happens when a deal is being decided on by the miner
	ProviderEventDealDeciding

	// ProviderEventDealRejected happens when a deal proposal is rejected for not meeting criteria
	ProviderEventDealRejected

	// ProviderEventRejectionSent happens after a deal proposal rejection has been sent to the client
	ProviderEventRejectionSent

	// ProviderEventDealAccepted happens when a deal is accepted based on provider criteria
	ProviderEventDealAccepted

	// ProviderEventInsufficientFunds indicates not enough funds available for a deal
	ProviderEventInsufficientFunds

	// ProviderEventFundingInitiated indicates provider collateral funding has been initiated
	ProviderEventFundingInitiated

	// ProviderEventFunded indicates provider collateral has appeared in the storage market balance
	ProviderEventFunded

	// ProviderEventDataTransferFailed happens when an error occurs transferring data
	ProviderEventDataTransferFailed

	// ProviderEventDataRequested happens when a provider requests data from a client
	ProviderEventDataRequested

	// ProviderEventDataTransferInitiated happens when a data transfer starts
	ProviderEventDataTransferInitiated

	// ProviderEventDataTransferCompleted happens when a data transfer is successful
	ProviderEventDataTransferCompleted

	// ProviderEventManualDataReceived happens when data is received manually for an offline deal
	ProviderEventManualDataReceived

	// ProviderEventDataVerificationFailed happens when an error occurs validating deal data
	ProviderEventDataVerificationFailed

	// ProviderEventVerifiedData happens when received data is verified as matching the pieceCID in a deal proposal
	ProviderEventVerifiedData

	// ProviderEventSendResponseFailed happens when a response cannot be sent to a deal
	ProviderEventSendResponseFailed

	// ProviderEventDealPublishInitiated happens when a provider has sent a PublishStorageDeals message to the chain
	ProviderEventDealPublishInitiated

	// ProviderEventDealPublished happens when a deal is successfully published
	ProviderEventDealPublished

	// ProviderEventDealPublishError happens when PublishStorageDeals returns a non-ok exit code
	ProviderEventDealPublishError

	// ProviderEventFileStoreErrored happens when an error occurs accessing the filestore
	ProviderEventFileStoreErrored

	// ProviderEventDealHandoffFailed happens when an error occurs handing off a deal with OnDealComplete
	ProviderEventDealHandoffFailed

	// ProviderEventDealHandedOff happens when a deal is successfully handed off to the node for processing in a sector
	ProviderEventDealHandedOff

	// ProviderEventDealActivationFailed happens when an error occurs activating a deal
	ProviderEventDealActivationFailed

	// ProviderEventUnableToLocatePiece happens when an attempt to learn the location of a piece from
	// the node fails
	ProviderEventUnableToLocatePiece

	// ProviderEventDealActivated happens when a deal is successfully activated and commited to a sector
	ProviderEventDealActivated

	// ProviderEventPieceStoreErrored happens when an attempt to save data in the piece store errors
	ProviderEventPieceStoreErrored

	// ProviderEventReadMetadataErrored happens when an error occurs reading recorded piece metadata
	ProviderEventReadMetadataErrored

	// ProviderEventPieceRecorded happens when a piece is successfully recorded
	ProviderEventPieceRecorded

	// ProviderEventDealCompletionFailed happens when a miner cannot verify a deal expired or was slashed
	ProviderEventDealCompletionFailed

	// ProviderEventDealExpired happens when a deal expires
	ProviderEventDealExpired

	// ProviderEventDealSlashed happens when a deal is slashed
	ProviderEventDealSlashed

	// ProviderEventFailed indicates a deal has failed and should no longer be processed
	ProviderEventFailed

	// ProviderEventRestart is used to resume the deal after a state machine shutdown
	ProviderEventRestart
)

// ProviderEvents maps provider event codes to string names
var ProviderEvents = map[ProviderEvent]string{
	ProviderEventOpen:                   "ProviderEventOpen",
	ProviderEventNodeErrored:            "ProviderEventNodeErrored",
	ProviderEventDealRejected:           "ProviderEventDealRejected",
	ProviderEventRejectionSent:          "ProviderEventRejectionSent",
	ProviderEventDealAccepted:           "ProviderEventDealAccepted",
	ProviderEventDealDeciding:           "ProviderEventDealDeciding",
	ProviderEventInsufficientFunds:      "ProviderEventInsufficientFunds",
	ProviderEventFundingInitiated:       "ProviderEventFundingInitiated",
	ProviderEventFunded:                 "ProviderEventFunded",
	ProviderEventDataTransferFailed:     "ProviderEventDataTransferFailed",
	ProviderEventDataRequested:          "ProviderEventDataRequested",
	ProviderEventDataTransferInitiated:  "ProviderEventDataTransferInitiated",
	ProviderEventDataTransferCompleted:  "ProviderEventDataTransferCompleted",
	ProviderEventManualDataReceived:     "ProviderEventManualDataReceived",
	ProviderEventDataVerificationFailed: "ProviderEventDataVerificationFailed",
	ProviderEventVerifiedData:           "ProviderEventVerifiedData",
	ProviderEventSendResponseFailed:     "ProviderEventSendResponseFailed",
	ProviderEventDealPublishInitiated:   "ProviderEventDealPublishInitiated",
	ProviderEventDealPublished:          "ProviderEventDealPublished",
	ProviderEventDealPublishError:       "ProviderEventDealPublishError",
	ProviderEventFileStoreErrored:       "ProviderEventFileStoreErrored",
	ProviderEventDealHandoffFailed:      "ProviderEventDealHandoffFailed",
	ProviderEventDealHandedOff:          "ProviderEventDealHandedOff",
	ProviderEventDealActivationFailed:   "ProviderEventDealActivationFailed",
	ProviderEventUnableToLocatePiece:    "ProviderEventUnableToLocatePiece",
	ProviderEventDealActivated:          "ProviderEventDealActivated",
	ProviderEventPieceStoreErrored:      "ProviderEventPieceStoreErrored",
	ProviderEventReadMetadataErrored:    "ProviderEventReadMetadataErrored",
	ProviderEventPieceRecorded:          "ProviderEventPieceRecorded",
	ProviderEventDealCompletionFailed:   "ProviderEventDealCompletionFailed",
	ProviderEventDealExpired:            "ProviderEventDealExpired",
	ProviderEventDealSlashed:            "ProviderEventDealSlashed",
	ProviderEventFailed:                 "ProviderEventFailed",
	ProviderEventRestart:                "ProviderEventRestart",
}
