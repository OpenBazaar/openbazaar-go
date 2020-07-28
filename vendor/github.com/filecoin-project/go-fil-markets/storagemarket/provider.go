package storagemarket

import (
	"context"
	"io"

	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-cid"
)

// ProviderSubscriber is a callback that is run when events are emitted on a StorageProvider
type ProviderSubscriber func(event ProviderEvent, deal MinerDeal)

// StorageProvider provides an interface to the storage market for a single
// storage miner.
type StorageProvider interface {

	// Start initializes deal processing on a StorageProvider and restarts in progress deals.
	// It also registers the provider with a StorageMarketNetwork so it can receive incoming
	// messages on the storage market's libp2p protocols
	Start(ctx context.Context) error

	// Stop terminates processing of deals on a StorageProvider
	Stop() error

	// SetAsk configures the storage miner's ask with the provided price,
	// duration, and options. Any previously-existing ask is replaced.
	SetAsk(price abi.TokenAmount, duration abi.ChainEpoch, options ...StorageAskOption) error

	// GetAsk returns the storage miner's ask, or nil if one does not exist.
	GetAsk() *SignedStorageAsk

	// ListDeals lists on-chain deals associated with this storage provider
	ListDeals(ctx context.Context) ([]StorageDeal, error)

	// ListLocalDeals lists deals processed by this storage provider
	ListLocalDeals() ([]MinerDeal, error)

	// AddStorageCollateral adds storage collateral
	AddStorageCollateral(ctx context.Context, amount abi.TokenAmount) error

	// GetStorageCollateral returns the current collateral balance
	GetStorageCollateral(ctx context.Context) (Balance, error)

	// ImportDataForDeal manually imports data for an offline storage deal
	ImportDataForDeal(ctx context.Context, propCid cid.Cid, data io.Reader) error

	// SubscribeToEvents listens for events that happen related to storage deals on a provider
	SubscribeToEvents(subscriber ProviderSubscriber) shared.Unsubscribe
}
