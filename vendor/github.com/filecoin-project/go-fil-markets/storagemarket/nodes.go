package storagemarket

import (
	"context"
	"io"

	"github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
)

// DealSectorCommittedCallback is a callback that runs when a sector is committed
type DealSectorCommittedCallback func(err error)

// DealExpiredCallback is a callback that runs when a deal expires
type DealExpiredCallback func(err error)

// DealSlashedCallback is a callback that runs when a deal gets slashed
type DealSlashedCallback func(slashEpoch abi.ChainEpoch, err error)

// StorageCommon are common interfaces provided by a filecoin Node to both StorageClient and StorageProvider
type StorageCommon interface {

	// GetChainHead returns a tipset token for the current chain head
	GetChainHead(ctx context.Context) (shared.TipSetToken, abi.ChainEpoch, error)

	// Adds funds with the StorageMinerActor for a storage participant.  Used by both providers and clients.
	AddFunds(ctx context.Context, addr address.Address, amount abi.TokenAmount) (cid.Cid, error)

	// EnsureFunds ensures that a storage market participant has a certain amount of available funds
	// If additional funds are needed, they will be sent from the 'wallet' address, and a cid for the
	// corresponding chain message is returned
	EnsureFunds(ctx context.Context, addr, wallet address.Address, amount abi.TokenAmount, tok shared.TipSetToken) (cid.Cid, error)

	// GetBalance returns locked/unlocked for a storage participant.  Used by both providers and clients.
	GetBalance(ctx context.Context, addr address.Address, tok shared.TipSetToken) (Balance, error)

	// VerifySignature verifies a given set of data was signed properly by a given address's private key
	VerifySignature(ctx context.Context, signature crypto.Signature, signer address.Address, plaintext []byte, tok shared.TipSetToken) (bool, error)

	// WaitForMessage waits until a message appears on chain. If it is already on chain, the callback is called immediately
	WaitForMessage(ctx context.Context, mcid cid.Cid, onCompletion func(exitcode.ExitCode, []byte, error) error) error

	// SignsBytes signs the given data with the given address's private key
	SignBytes(ctx context.Context, signer address.Address, b []byte) (*crypto.Signature, error)

	// OnDealSectorCommitted waits for a deal's sector to be sealed and proved, indicating the deal is active
	OnDealSectorCommitted(ctx context.Context, provider address.Address, dealID abi.DealID, cb DealSectorCommittedCallback) error

	// OnDealExpiredOrSlashed registers callbacks to be called when the deal expires or is slashed
	OnDealExpiredOrSlashed(ctx context.Context, dealID abi.DealID, onDealExpired DealExpiredCallback, onDealSlashed DealSlashedCallback) error
}

// StorageProviderNode are node dependencies for a StorageProvider
type StorageProviderNode interface {
	StorageCommon

	// PublishDeals publishes a deal on chain, returns the message cid, but does not wait for message to appear
	PublishDeals(ctx context.Context, deal MinerDeal) (cid.Cid, error)

	// ListProviderDeals lists all deals on chain associated with a storage provider
	ListProviderDeals(ctx context.Context, addr address.Address, tok shared.TipSetToken) ([]StorageDeal, error)

	// OnDealComplete is called when a deal is complete and on chain, and data has been transferred and is ready to be added to a sector
	OnDealComplete(ctx context.Context, deal MinerDeal, pieceSize abi.UnpaddedPieceSize, pieceReader io.Reader) error

	// GetMinerWorkerAddress returns the worker address associated with a miner
	GetMinerWorkerAddress(ctx context.Context, addr address.Address, tok shared.TipSetToken) (address.Address, error)

	// LocatePieceForDealWithinSector looks up a given dealID in the miners sectors, and returns its sectorID and location
	LocatePieceForDealWithinSector(ctx context.Context, dealID abi.DealID, tok shared.TipSetToken) (sectorID uint64, offset uint64, length uint64, err error)

	// GetDataCap gets the current data cap for addr
	GetDataCap(ctx context.Context, addr address.Address, tok shared.TipSetToken) (verifreg.DataCap, error)
}

// StorageClientNode are node dependencies for a StorageClient
type StorageClientNode interface {
	StorageCommon

	// ListClientDeals lists all on-chain deals associated with a storage client
	ListClientDeals(ctx context.Context, addr address.Address, tok shared.TipSetToken) ([]StorageDeal, error)

	// GetStorageProviders returns information about known miners
	ListStorageProviders(ctx context.Context, tok shared.TipSetToken) ([]*StorageProviderInfo, error)

	// ValidatePublishedDeal verifies a deal is published on chain and returns the dealID
	ValidatePublishedDeal(ctx context.Context, deal ClientDeal) (abi.DealID, error)

	// SignProposal signs a DealProposal
	SignProposal(ctx context.Context, signer address.Address, proposal market.DealProposal) (*market.ClientDealProposal, error)

	// GetDefaultWalletAddress returns the address for this client
	GetDefaultWalletAddress(ctx context.Context) (address.Address, error)

	// ValidateAskSignature verifies a the signature is valid for a given SignedStorageAsk
	ValidateAskSignature(ctx context.Context, ask *SignedStorageAsk, tok shared.TipSetToken) (bool, error)

	// GetMinerInfo returns info for a single miner with the given address
	GetMinerInfo(ctx context.Context, maddr address.Address, tok shared.TipSetToken) (*StorageProviderInfo, error)
}
