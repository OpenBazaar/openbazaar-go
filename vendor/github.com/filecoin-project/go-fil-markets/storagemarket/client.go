package storagemarket

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-cid"
)

// ClientSubscriber is a callback that is run when events are emitted on a StorageClient
type ClientSubscriber func(event ClientEvent, deal ClientDeal)

// StorageClient is a client interface for making storage deals with a StorageProvider
type StorageClient interface {

	// Start initializes deal processing on a StorageClient and restarts
	// in progress deals
	Start(ctx context.Context) error

	// Stop ends deal processing on a StorageClient
	Stop() error

	// ListProviders queries chain state and returns active storage providers
	ListProviders(ctx context.Context) (<-chan StorageProviderInfo, error)

	// ListDeals lists on-chain deals associated with this storage client
	ListDeals(ctx context.Context, addr address.Address) ([]StorageDeal, error)

	// ListLocalDeals lists deals initiated by this storage client
	ListLocalDeals(ctx context.Context) ([]ClientDeal, error)

	// GetLocalDeal lists deals that are in progress or rejected
	GetLocalDeal(ctx context.Context, cid cid.Cid) (ClientDeal, error)

	// GetAsk returns the current ask for a storage provider
	GetAsk(ctx context.Context, info StorageProviderInfo) (*SignedStorageAsk, error)

	// GetProviderDealState queries a provider for the current state of a client's deal
	GetProviderDealState(ctx context.Context, proposalCid cid.Cid) (*ProviderDealState, error)

	// ProposeStorageDeal initiates deal negotiation with a Storage Provider
	ProposeStorageDeal(ctx context.Context, addr address.Address, info *StorageProviderInfo, data *DataRef, startEpoch abi.ChainEpoch, endEpoch abi.ChainEpoch, price abi.TokenAmount, collateral abi.TokenAmount, rt abi.RegisteredSealProof, fastRetrieval bool, verifiedDeal bool) (*ProposeStorageDealResult, error)

	// GetPaymentEscrow returns the current funds available for deal payment
	GetPaymentEscrow(ctx context.Context, addr address.Address) (Balance, error)

	// AddStorageCollateral adds storage collateral
	AddPaymentEscrow(ctx context.Context, addr address.Address, amount abi.TokenAmount) error

	// SubscribeToEvents listens for events that happen related to storage deals on a provider
	SubscribeToEvents(subscriber ClientSubscriber) shared.Unsubscribe
}
