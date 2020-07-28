package storagemarket

// StorageDealStatus is the local status of a StorageDeal.
// Note: this status has meaning in the context of this module only - it is not
// recorded on chain
type StorageDealStatus = uint64

const (
	// StorageDealUnknown means the current status of a deal is undefined
	StorageDealUnknown = StorageDealStatus(iota)

	// StorageDealProposalNotFound is a status returned in responses when the deal itself cannot
	// be located
	StorageDealProposalNotFound

	// StorageDealProposalRejected is returned by a StorageProvider when it chooses not to accept
	// a DealProposal
	StorageDealProposalRejected

	// StorageDealProposalAccepted indicates an intent to accept a storage deal proposal
	StorageDealProposalAccepted

	// StorageDealStaged means a deal has been published and data is ready to be put into a sector
	StorageDealStaged

	// StorageDealSealing means a deal is in a sector that is being sealed
	StorageDealSealing

	// StorageDealRecordPiece means a deal is in a sealed sector and the piece
	// is being added to the piece store
	StorageDealRecordPiece

	// StorageDealActive means a deal is in a sealed sector and the miner is proving the data
	// for the deal
	StorageDealActive

	// StorageDealExpired means a deal has passed its final epoch and is expired
	StorageDealExpired

	// StorageDealSlashed means the deal was in a sector that got slashed from failing to prove
	StorageDealSlashed

	// StorageDealRejecting means the Provider has rejected the deal, and will send a rejection response
	StorageDealRejecting

	// StorageDealFailing means something has gone wrong in a deal. Once data is cleaned up the deal will finalize on
	// StorageDealError
	StorageDealFailing

	// StorageDealFundsEnsured means we've deposited funds as necessary to create a deal, ready to move forward
	StorageDealFundsEnsured

	// StorageDealCheckForAcceptance means the client is waiting for a provider to seal and publish a deal
	StorageDealCheckForAcceptance

	// StorageDealValidating means the provider is validating that deal parameters are good for a proposal
	StorageDealValidating

	// StorageDealAcceptWait means the provider is running any custom decision logic to decide whether or not to accept the deal
	StorageDealAcceptWait

	// StorageDealStartDataTransfer means data transfer is beginning
	StorageDealStartDataTransfer

	// StorageDealTransferring means data is being sent from the client to the provider via the data transfer module
	StorageDealTransferring

	// StorageDealWaitingForData indicates either a manual transfer
	// or that the provider has not received a data transfer request from the client
	StorageDealWaitingForData

	// StorageDealVerifyData means data has been transferred and we are attempting to verify it against the PieceCID
	StorageDealVerifyData

	// StorageDealEnsureProviderFunds means that provider is making sure it has adequate funds for the deal in the StorageMarketActor
	StorageDealEnsureProviderFunds

	// StorageDealEnsureClientFunds means that client is making sure it has adequate funds for the deal in the StorageMarketActor
	StorageDealEnsureClientFunds

	// StorageDealProviderFunding means that the provider has deposited funds in the StorageMarketActor and it is waiting
	// to see the funds appear in its balance
	StorageDealProviderFunding

	// StorageDealClientFunding means that the client has deposited funds in the StorageMarketActor and it is waiting
	// to see the funds appear in its balance
	StorageDealClientFunding

	// StorageDealPublish means the deal is ready to be published on chain
	StorageDealPublish

	// StorageDealPublishing means the deal has been published but we are waiting for it to appear on chain
	StorageDealPublishing

	// StorageDealError means the deal has failed due to an error, and no further updates will occur
	StorageDealError
)

// DealStates maps StorageDealStatus codes to string names
var DealStates = map[StorageDealStatus]string{
	StorageDealUnknown:             "StorageDealUnknown",
	StorageDealProposalNotFound:    "StorageDealProposalNotFound",
	StorageDealProposalRejected:    "StorageDealProposalRejected",
	StorageDealProposalAccepted:    "StorageDealProposalAccepted",
	StorageDealAcceptWait:          "StorageDealAcceptWait",
	StorageDealStartDataTransfer:   "StorageDealStartDataTransfer",
	StorageDealStaged:              "StorageDealStaged",
	StorageDealSealing:             "StorageDealSealing",
	StorageDealActive:              "StorageDealActive",
	StorageDealExpired:             "StorageDealExpired",
	StorageDealSlashed:             "StorageDealSlashed",
	StorageDealRejecting:           "StorageDealRejecting",
	StorageDealFailing:             "StorageDealFailing",
	StorageDealFundsEnsured:        "StorageDealFundsEnsured",
	StorageDealCheckForAcceptance:  "StorageDealCheckForAcceptance",
	StorageDealValidating:          "StorageDealValidating",
	StorageDealTransferring:        "StorageDealTransferring",
	StorageDealWaitingForData:      "StorageDealWaitingForData",
	StorageDealVerifyData:          "StorageDealVerifyData",
	StorageDealEnsureProviderFunds: "StorageDealEnsureProviderFunds",
	StorageDealEnsureClientFunds:   "StorageDealEnsureClientFunds",
	StorageDealProviderFunding:     "StorageDealProviderFunding",
	StorageDealClientFunding:       "StorageDealClientFunding",
	StorageDealPublish:             "StorageDealPublish",
	StorageDealPublishing:          "StorageDealPublishing",
	StorageDealError:               "StorageDealError",
}
