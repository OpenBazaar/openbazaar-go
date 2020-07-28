// Package testnodes contains stubbed implementations of the StorageProviderNode
// and StorageClientNode interface to simulate communications with a filecoin node
package testnodes

import (
	"context"
	"errors"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
)

// Below fake node implementations

// StorageMarketState represents a state for the storage market that can be inspected
// - methods on the provider nodes will affect this state
type StorageMarketState struct {
	TipSetToken  shared.TipSetToken
	Epoch        abi.ChainEpoch
	DealID       abi.DealID
	Balances     map[address.Address]abi.TokenAmount
	StorageDeals map[address.Address][]storagemarket.StorageDeal
	Providers    []*storagemarket.StorageProviderInfo
}

// NewStorageMarketState returns a new empty state for the storage market
func NewStorageMarketState() *StorageMarketState {
	return &StorageMarketState{
		Epoch:        0,
		DealID:       0,
		Balances:     map[address.Address]abi.TokenAmount{},
		StorageDeals: map[address.Address][]storagemarket.StorageDeal{},
		Providers:    nil,
	}
}

// AddFunds adds funds for a given address in the storage market
func (sma *StorageMarketState) AddFunds(addr address.Address, amount abi.TokenAmount) {
	if existing, ok := sma.Balances[addr]; ok {
		sma.Balances[addr] = big.Add(existing, amount)
	} else {
		sma.Balances[addr] = amount
	}
}

// Balance returns the balance of a given address in the market
func (sma *StorageMarketState) Balance(addr address.Address) storagemarket.Balance {
	if existing, ok := sma.Balances[addr]; ok {
		return storagemarket.Balance{Locked: big.NewInt(0), Available: existing}
	}
	return storagemarket.Balance{Locked: big.NewInt(0), Available: big.NewInt(0)}
}

// Deals returns all deals in the current state
func (sma *StorageMarketState) Deals(addr address.Address) []storagemarket.StorageDeal {
	if existing, ok := sma.StorageDeals[addr]; ok {
		return existing
	}
	return nil
}

// StateKey returns a state key with the storage market states set Epoch
func (sma *StorageMarketState) StateKey() (shared.TipSetToken, abi.ChainEpoch) {
	return sma.TipSetToken, sma.Epoch
}

// AddDeal adds a deal to the current state of the storage market
func (sma *StorageMarketState) AddDeal(deal storagemarket.StorageDeal) (shared.TipSetToken, abi.ChainEpoch) {
	for _, addr := range []address.Address{deal.Client, deal.Provider} {
		if existing, ok := sma.StorageDeals[addr]; ok {
			sma.StorageDeals[addr] = append(existing, deal)
		} else {
			sma.StorageDeals[addr] = []storagemarket.StorageDeal{deal}
		}
	}

	return sma.StateKey()
}

// FakeCommonNode implements common methods for the storage & client node adapters
// where responses are stubbed
type FakeCommonNode struct {
	SMState                    *StorageMarketState
	AddFundsCid                cid.Cid
	EnsureFundsError           error
	VerifySignatureFails       bool
	GetBalanceError            error
	GetChainHeadError          error
	SignBytesError             error
	DealCommittedSyncError     error
	DealCommittedAsyncError    error
	WaitForDealCompletionError error
	OnDealExpiredError         error
	OnDealSlashedError         error
	OnDealSlashedEpoch         abi.ChainEpoch

	WaitForMessageBlocks    bool
	WaitForMessageError     error
	WaitForMessageExitCode  exitcode.ExitCode
	WaitForMessageRetBytes  []byte
	WaitForMessageNodeError error
	WaitForMessageCalls     []cid.Cid
}

// GetChainHead returns the state id in the storage market state
func (n *FakeCommonNode) GetChainHead(ctx context.Context) (shared.TipSetToken, abi.ChainEpoch, error) {
	if n.GetChainHeadError == nil {
		key, epoch := n.SMState.StateKey()
		return key, epoch, nil
	}

	return []byte{}, 0, n.GetChainHeadError
}

// AddFunds adds funds to the given actor in the storage market state
func (n *FakeCommonNode) AddFunds(ctx context.Context, addr address.Address, amount abi.TokenAmount) (cid.Cid, error) {
	n.SMState.AddFunds(addr, amount)
	return n.AddFundsCid, nil
}

// EnsureFunds adds funds to the given actor in the storage market state to ensure it has at least the given amount
func (n *FakeCommonNode) EnsureFunds(ctx context.Context, addr, wallet address.Address, amount abi.TokenAmount, tok shared.TipSetToken) (cid.Cid, error) {
	if n.EnsureFundsError == nil {
		balance := n.SMState.Balance(addr)
		if balance.Available.LessThan(amount) {
			return n.AddFunds(ctx, addr, big.Sub(amount, balance.Available))
		}
	}

	return cid.Undef, n.EnsureFundsError
}

// WaitForMessage simulates waiting for a message to appear on chain
func (n *FakeCommonNode) WaitForMessage(ctx context.Context, mcid cid.Cid, onCompletion func(exitcode.ExitCode, []byte, error) error) error {
	n.WaitForMessageCalls = append(n.WaitForMessageCalls, mcid)

	if n.WaitForMessageError != nil {
		return n.WaitForMessageError
	}

	if n.WaitForMessageBlocks {
		// just leave the test node in this state to simulate a long operation
		return nil
	}

	return onCompletion(n.WaitForMessageExitCode, n.WaitForMessageRetBytes, n.WaitForMessageNodeError)
}

// GetBalance returns the funds in the storage market state
func (n *FakeCommonNode) GetBalance(ctx context.Context, addr address.Address, tok shared.TipSetToken) (storagemarket.Balance, error) {
	if n.GetBalanceError == nil {
		return n.SMState.Balance(addr), nil
	}
	return storagemarket.Balance{}, n.GetBalanceError
}

// VerifySignature just always returns true, for now
func (n *FakeCommonNode) VerifySignature(ctx context.Context, signature crypto.Signature, addr address.Address, data []byte, tok shared.TipSetToken) (bool, error) {
	return !n.VerifySignatureFails, nil
}

// SignBytes simulates signing data by returning a test signature
func (n *FakeCommonNode) SignBytes(ctx context.Context, signer address.Address, b []byte) (*crypto.Signature, error) {
	if n.SignBytesError == nil {
		return shared_testutil.MakeTestSignature(), nil
	}
	return nil, n.SignBytesError
}

// OnDealSectorCommitted returns immediately, and returns stubbed errors
func (n *FakeCommonNode) OnDealSectorCommitted(ctx context.Context, provider address.Address, dealID abi.DealID, cb storagemarket.DealSectorCommittedCallback) error {
	if n.DealCommittedSyncError == nil {
		cb(n.DealCommittedAsyncError)
	}
	return n.DealCommittedSyncError
}

// OnDealExpiredOrSlashed simulates waiting for a deal to be expired or slashed, but provides stubbed behavior
func (n *FakeCommonNode) OnDealExpiredOrSlashed(ctx context.Context, dealID abi.DealID, onDealExpired storagemarket.DealExpiredCallback, onDealSlashed storagemarket.DealSlashedCallback) error {
	if n.WaitForDealCompletionError != nil {
		return n.WaitForDealCompletionError
	}

	if n.OnDealSlashedError != nil {
		onDealSlashed(abi.ChainEpoch(0), n.OnDealSlashedError)
		return nil
	}

	if n.OnDealExpiredError != nil {
		onDealExpired(n.OnDealExpiredError)
		return nil
	}

	if n.OnDealSlashedEpoch == 0 {
		onDealExpired(nil)
		return nil
	}

	onDealSlashed(n.OnDealSlashedEpoch, nil)
	return nil
}

var _ storagemarket.StorageCommon = (*FakeCommonNode)(nil)

// FakeClientNode is a node adapter for a storage client whose responses
// are stubbed
type FakeClientNode struct {
	FakeCommonNode
	ClientAddr              address.Address
	MinerAddr               address.Address
	WorkerAddr              address.Address
	ValidationError         error
	ValidatePublishedDealID abi.DealID
	ValidatePublishedError  error
}

// ListClientDeals just returns the deals in the storage market state
func (n *FakeClientNode) ListClientDeals(ctx context.Context, addr address.Address, tok shared.TipSetToken) ([]storagemarket.StorageDeal, error) {
	return n.SMState.Deals(addr), nil
}

// ListStorageProviders lists the providers in the storage market state
func (n *FakeClientNode) ListStorageProviders(ctx context.Context, tok shared.TipSetToken) ([]*storagemarket.StorageProviderInfo, error) {
	return n.SMState.Providers, nil
}

// ValidatePublishedDeal always succeeds
func (n *FakeClientNode) ValidatePublishedDeal(ctx context.Context, deal storagemarket.ClientDeal) (abi.DealID, error) {
	return n.ValidatePublishedDealID, n.ValidatePublishedError
}

// SignProposal signs a deal with a dummy signature
func (n *FakeClientNode) SignProposal(ctx context.Context, signer address.Address, proposal market.DealProposal) (*market.ClientDealProposal, error) {
	return &market.ClientDealProposal{
		Proposal:        proposal,
		ClientSignature: *shared_testutil.MakeTestSignature(),
	}, nil
}

// GetDefaultWalletAddress returns a stubbed ClientAddr
func (n *FakeClientNode) GetDefaultWalletAddress(ctx context.Context) (address.Address, error) {
	return n.ClientAddr, nil
}

// GetMinerInfo returns stubbed information for the first miner in storage market state
func (n *FakeClientNode) GetMinerInfo(ctx context.Context, maddr address.Address, tok shared.TipSetToken) (*storagemarket.StorageProviderInfo, error) {
	if len(n.SMState.Providers) == 0 {
		return nil, errors.New("Provider not found")
	}
	return n.SMState.Providers[0], nil
}

// ValidateAskSignature returns the stubbed validation error and a boolean value
// communicating the validity of the provided signature
func (n *FakeClientNode) ValidateAskSignature(ctx context.Context, ask *storagemarket.SignedStorageAsk, tok shared.TipSetToken) (bool, error) {
	return n.ValidationError == nil, n.ValidationError
}

var _ storagemarket.StorageClientNode = (*FakeClientNode)(nil)

// FakeProviderNode implements functions specific to the StorageProviderNode
type FakeProviderNode struct {
	FakeCommonNode
	MinerAddr                           address.Address
	MinerWorkerError                    error
	PieceLength                         uint64
	PieceSectorID                       uint64
	PublishDealID                       abi.DealID
	PublishDealsError                   error
	OnDealCompleteError                 error
	OnDealCompleteCalls                 []storagemarket.MinerDeal
	LocatePieceForDealWithinSectorError error
	DataCap                             verifreg.DataCap
	GetDataCapErr                       error
}

// PublishDeals simulates publishing a deal by adding it to the storage market state
func (n *FakeProviderNode) PublishDeals(ctx context.Context, deal storagemarket.MinerDeal) (cid.Cid, error) {
	if n.PublishDealsError == nil {
		sd := storagemarket.StorageDeal{
			DealProposal: deal.Proposal,
			DealState:    market.DealState{},
		}

		n.SMState.AddDeal(sd)

		return shared_testutil.GenerateCids(1)[0], nil
	}
	return cid.Undef, n.PublishDealsError
}

// ListProviderDeals returns the deals in the storage market state
func (n *FakeProviderNode) ListProviderDeals(ctx context.Context, addr address.Address, tok shared.TipSetToken) ([]storagemarket.StorageDeal, error) {
	return n.SMState.Deals(addr), nil
}

// OnDealComplete simulates passing of the deal to the storage miner, and does nothing
func (n *FakeProviderNode) OnDealComplete(ctx context.Context, deal storagemarket.MinerDeal, pieceSize abi.UnpaddedPieceSize, pieceReader io.Reader) error {
	n.OnDealCompleteCalls = append(n.OnDealCompleteCalls, deal)
	return n.OnDealCompleteError
}

// GetMinerWorkerAddress returns the address specified by MinerAddr
func (n *FakeProviderNode) GetMinerWorkerAddress(ctx context.Context, miner address.Address, tok shared.TipSetToken) (address.Address, error) {
	if n.MinerWorkerError == nil {
		return n.MinerAddr, nil
	}
	return address.Undef, n.MinerWorkerError
}

// LocatePieceForDealWithinSector returns stubbed data for a pieces location in a sector
func (n *FakeProviderNode) LocatePieceForDealWithinSector(ctx context.Context, dealID abi.DealID, tok shared.TipSetToken) (sectorID uint64, offset uint64, length uint64, err error) {
	if n.LocatePieceForDealWithinSectorError == nil {
		return n.PieceSectorID, 0, n.PieceLength, nil
	}
	return 0, 0, 0, n.LocatePieceForDealWithinSectorError
}

// GetDataCap gets the current data cap for addr
func (n *FakeProviderNode) GetDataCap(ctx context.Context, addr address.Address, tok shared.TipSetToken) (verifreg.DataCap, error) {
	return n.DataCap, n.GetDataCapErr
}

var _ storagemarket.StorageProviderNode = (*FakeProviderNode)(nil)
