package storagemarket

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/filecoin-project/go-fil-markets/filestore"
)

//go:generate cbor-gen-for ClientDeal MinerDeal Balance SignedStorageAsk StorageAsk StorageDeal DataRef ProviderDealState

// DealProtocolID is the ID for the libp2p protocol for proposing storage deals.
const DealProtocolID = "/fil/storage/mk/1.0.1"

// AskProtocolID is the ID for the libp2p protocol for querying miners for their current StorageAsk.
const AskProtocolID = "/fil/storage/ask/1.0.1"

// DealStatusProtocolID is the ID for the libp2p protocol for querying miners for the current status of a deal.
const DealStatusProtocolID = "/fil/storage/status/1.0.1"

// Balance represents a current balance of funds in the StorageMarketActor.
type Balance struct {
	Locked    abi.TokenAmount
	Available abi.TokenAmount
}

// StorageAsk defines the parameters by which a miner will choose to accept or
// reject a deal. Note: making a storage deal proposal which matches the miner's
// ask is a precondition, but not sufficient to ensure the deal is accepted (the
// storage provider may run its own decision logic).
type StorageAsk struct {
	// Price per GiB / Epoch
	Price abi.TokenAmount

	MinPieceSize abi.PaddedPieceSize
	MaxPieceSize abi.PaddedPieceSize
	Miner        address.Address
	Timestamp    abi.ChainEpoch
	Expiry       abi.ChainEpoch
	SeqNo        uint64
}

// SignedStorageAsk is an ask signed by the miner's private key
type SignedStorageAsk struct {
	Ask       *StorageAsk
	Signature *crypto.Signature
}

// SignedStorageAskUndefined represents the empty value for SignedStorageAsk
var SignedStorageAskUndefined = SignedStorageAsk{}

// StorageAskOption allows custom configuration of a storage ask
type StorageAskOption func(*StorageAsk)

// MinPieceSize configures a minimum piece size of a StorageAsk
func MinPieceSize(minPieceSize abi.PaddedPieceSize) StorageAskOption {
	return func(sa *StorageAsk) {
		sa.MinPieceSize = minPieceSize
	}
}

// MaxPieceSize configures maxiumum piece size of a StorageAsk
func MaxPieceSize(maxPieceSize abi.PaddedPieceSize) StorageAskOption {
	return func(sa *StorageAsk) {
		sa.MaxPieceSize = maxPieceSize
	}
}

// StorageAskUndefined represents an empty value for StorageAsk
var StorageAskUndefined = StorageAsk{}

// MinerDeal is the local state tracked for a deal by a StorageProvider
type MinerDeal struct {
	market.ClientDealProposal
	ProposalCid   cid.Cid
	AddFundsCid   *cid.Cid
	PublishCid    *cid.Cid
	Miner         peer.ID
	Client        peer.ID
	State         StorageDealStatus
	PiecePath     filestore.Path
	MetadataPath  filestore.Path
	SlashEpoch    abi.ChainEpoch
	FastRetrieval bool
	Message       string

	Ref *DataRef

	DealID abi.DealID
}

// ClientDeal is the local state tracked for a deal by a StorageClient
type ClientDeal struct {
	market.ClientDealProposal
	ProposalCid    cid.Cid
	AddFundsCid    *cid.Cid
	State          StorageDealStatus
	Miner          peer.ID
	MinerWorker    address.Address
	DealID         abi.DealID
	DataRef        *DataRef
	Message        string
	PublishMessage *cid.Cid
	SlashEpoch     abi.ChainEpoch
	PollRetryCount uint64
	PollErrorCount uint64
	FastRetrieval  bool
}

// StorageDeal is a local combination of a proposal and a current deal state
type StorageDeal struct {
	market.DealProposal
	market.DealState
}

// StorageProviderInfo describes on chain information about a StorageProvider
// (use QueryAsk to determine more specific deal parameters)
type StorageProviderInfo struct {
	Address    address.Address // actor address
	Owner      address.Address
	Worker     address.Address // signs messages
	SectorSize uint64
	PeerID     peer.ID
	Addrs      []ma.Multiaddr
}

// ProposeStorageDealResult returns the result for a proposing a deal
type ProposeStorageDealResult struct {
	ProposalCid cid.Cid
}

const (
	// TTGraphsync means data for a deal will be transferred by graphsync
	TTGraphsync = "graphsync"

	// TTManual means data for a deal will be transferred manually and imported
	// on the provider
	TTManual = "manual"
)

// DataRef is a reference for how data will be transferred for a given storage deal
type DataRef struct {
	TransferType string
	Root         cid.Cid

	PieceCid  *cid.Cid              // Optional for non-manual transfer, will be recomputed from the data if not given
	PieceSize abi.UnpaddedPieceSize // Optional for non-manual transfer, will be recomputed from the data if not given
}

// ProviderDealState represents a Provider's current state of a deal
type ProviderDealState struct {
	State         StorageDealStatus
	Message       string
	Proposal      *market.DealProposal
	ProposalCid   *cid.Cid
	AddFundsCid   *cid.Cid
	PublishCid    *cid.Cid
	DealID        abi.DealID
	FastRetrieval bool
}
