package storagemarket

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/go-fil-markets/shared"
)

// ClientSubscriber is a callback that is run when events are emitted on a StorageClient
type ClientSubscriber func(event ClientEvent, deal ClientDeal)

// StorageClient is a client interface for making storage deals with a StorageProvider
type StorageClient interface {

	// Start initializes deal processing on a StorageClient and restarts
	// in progress deals
	Start(ctx context.Context) error

	// OnReady registers a listener for when the client comes on line
	OnReady(shared.ReadyFunc)

	// Stop ends deal processing on a StorageClient
	Stop() error

	// ListProviders queries chain state and returns active storage providers
	ListProviders(ctx context.Context) (<-chan StorageProviderInfo, error)

	// ListLocalDeals lists deals initiated by this storage client
	ListLocalDeals(ctx context.Context) ([]ClientDeal, error)

	// GetLocalDeal lists deals that are in progress or rejected
	GetLocalDeal(ctx context.Context, cid cid.Cid) (ClientDeal, error)

	// GetAsk returns the current ask for a storage provider
	GetAsk(ctx context.Context, info StorageProviderInfo) (*StorageAsk, error)

	// GetProviderDealState queries a provider for the current state of a client's deal
	GetProviderDealState(ctx context.Context, proposalCid cid.Cid) (*ProviderDealState, error)

	// ProposeStorageDeal initiates deal negotiation with a Storage Provider
	ProposeStorageDeal(ctx context.Context, params ProposeStorageDealParams) (*ProposeStorageDealResult, error)

	// GetPaymentEscrow returns the current funds available for deal payment
	GetPaymentEscrow(ctx context.Context, addr address.Address) (Balance, error)

	// AddStorageCollateral adds storage collateral
	AddPaymentEscrow(ctx context.Context, addr address.Address, amount abi.TokenAmount) error

	// SubscribeToEvents listens for events that happen related to storage deals on a provider
	SubscribeToEvents(subscriber ClientSubscriber) shared.Unsubscribe
}
