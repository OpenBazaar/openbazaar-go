package market

import (
	"bytes"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/ipfs/go-cid"
	xerrors "golang.org/x/xerrors"

	. "github.com/filecoin-project/specs-actors/v2/actors/util"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

const epochUndefined = abi.ChainEpoch(-1)

// Market mutations
// add / rm balance
// pub deal (always provider)
// activate deal (miner)
// end deal (miner terminate, expire(no activation))

// BalanceLockingReason is the reason behind locking an amount.
type BalanceLockingReason int

const (
	ClientCollateral BalanceLockingReason = iota
	ClientStorageFee
	ProviderCollateral
)

type State struct {
	Proposals cid.Cid // AMT[DealID]DealProposal
	States    cid.Cid // AMT[DealID]DealState

	// PendingProposals tracks dealProposals that have not yet reached their deal start date.
	// We track them here to ensure that miners can't publish the same deal proposal twice
	PendingProposals cid.Cid // HAMT[DealCid]DealProposal

	// Total amount held in escrow, indexed by actor address (including both locked and unlocked amounts).
	EscrowTable cid.Cid // BalanceTable

	// Amount locked, indexed by actor address.
	// Note: the amounts in this table do not affect the overall amount in escrow:
	// only the _portion_ of the total escrow amount that is locked.
	LockedTable cid.Cid // BalanceTable

	NextID abi.DealID

	// Metadata cached for efficient iteration over deals.
	DealOpsByEpoch cid.Cid // SetMultimap, HAMT[epoch]Set
	LastCron       abi.ChainEpoch

	// Total Client Collateral that is locked -> unlocked when deal is terminated
	TotalClientLockedCollateral abi.TokenAmount
	// Total Provider Collateral that is locked -> unlocked when deal is terminated
	TotalProviderLockedCollateral abi.TokenAmount
	// Total storage fee that is locked in escrow -> unlocked when payments are made
	TotalClientStorageFee abi.TokenAmount
}

func ConstructState(emptyArrayCid, emptyMapCid, emptyMSetCid cid.Cid) *State {
	return &State{
		Proposals:        emptyArrayCid,
		States:           emptyArrayCid,
		PendingProposals: emptyMapCid,
		EscrowTable:      emptyMapCid,
		LockedTable:      emptyMapCid,
		NextID:           abi.DealID(0),
		DealOpsByEpoch:   emptyMSetCid,
		LastCron:         abi.ChainEpoch(-1),

		TotalClientLockedCollateral:   abi.NewTokenAmount(0),
		TotalProviderLockedCollateral: abi.NewTokenAmount(0),
		TotalClientStorageFee:         abi.NewTokenAmount(0),
	}
}

////////////////////////////////////////////////////////////////////////////////
// Deal state operations
////////////////////////////////////////////////////////////////////////////////

func (m *marketStateMutation) updatePendingDealState(rt Runtime, state *DealState, deal *DealProposal, epoch abi.ChainEpoch) (amountSlashed abi.TokenAmount, nextEpoch abi.ChainEpoch, removeDeal bool) {
	amountSlashed = abi.NewTokenAmount(0)

	everUpdated := state.LastUpdatedEpoch != epochUndefined
	everSlashed := state.SlashEpoch != epochUndefined

	Assert(!everUpdated || (state.LastUpdatedEpoch <= epoch)) // if the deal was ever updated, make sure it didn't happen in the future

	// This would be the case that the first callback somehow triggers before it is scheduled to
	// This is expected not to be able to happen
	if deal.StartEpoch > epoch {
		return amountSlashed, epochUndefined, false
	}

	paymentEndEpoch := deal.EndEpoch
	if everSlashed {
		AssertMsg(epoch >= state.SlashEpoch, "current epoch less than slash epoch")
		Assert(state.SlashEpoch <= deal.EndEpoch)
		paymentEndEpoch = state.SlashEpoch
	} else if epoch < paymentEndEpoch {
		paymentEndEpoch = epoch
	}

	paymentStartEpoch := deal.StartEpoch
	if everUpdated && state.LastUpdatedEpoch > paymentStartEpoch {
		paymentStartEpoch = state.LastUpdatedEpoch
	}

	numEpochsElapsed := paymentEndEpoch - paymentStartEpoch

	{
		// Process deal payment for the elapsed epochs.
		totalPayment := big.Mul(big.NewInt(int64(numEpochsElapsed)), deal.StoragePricePerEpoch)

		// the transfer amount can be less than or equal to zero if a deal is slashed before or at the deal's start epoch.
		if totalPayment.GreaterThan(big.Zero()) {
			m.transferBalance(rt, deal.Client, deal.Provider, totalPayment)
		}
	}

	if everSlashed {
		// unlock client collateral and locked storage fee
		paymentRemaining := dealGetPaymentRemaining(deal, state.SlashEpoch)

		// unlock remaining storage fee
		if err := m.unlockBalance(deal.Client, paymentRemaining, ClientStorageFee); err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to unlock remaining client storage fee: %s", err)
		}
		// unlock client collateral
		if err := m.unlockBalance(deal.Client, deal.ClientCollateral, ClientCollateral); err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to unlock client collateral: %s", err)
		}

		// slash provider collateral
		amountSlashed = deal.ProviderCollateral
		if err := m.slashBalance(deal.Provider, amountSlashed, ProviderCollateral); err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "slashing balance: %s", err)
		}

		return amountSlashed, epochUndefined, true
	}

	if epoch >= deal.EndEpoch {
		m.processDealExpired(rt, deal, state)
		return amountSlashed, epochUndefined, true
	}

	// We're explicitly not inspecting the end epoch and may process a deal's expiration late, in order to prevent an outsider
	// from loading a cron tick by activating too many deals with the same end epoch.
	nextEpoch = epoch + DealUpdatesInterval

	return amountSlashed, nextEpoch, false
}

// Deal start deadline elapsed without appearing in a proven sector.
// Slash a portion of provider's collateral, and unlock remaining collaterals
// for both provider and client.
func (m *marketStateMutation) processDealInitTimedOut(rt Runtime, deal *DealProposal) abi.TokenAmount {
	if err := m.unlockBalance(deal.Client, deal.TotalStorageFee(), ClientStorageFee); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failure unlocking client storage fee: %s", err)
	}
	if err := m.unlockBalance(deal.Client, deal.ClientCollateral, ClientCollateral); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failure unlocking client collateral: %s", err)
	}

	amountSlashed := CollateralPenaltyForDealActivationMissed(deal.ProviderCollateral)
	amountRemaining := big.Sub(deal.ProviderBalanceRequirement(), amountSlashed)

	if err := m.slashBalance(deal.Provider, amountSlashed, ProviderCollateral); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to slash balance: %s", err)
	}

	if err := m.unlockBalance(deal.Provider, amountRemaining, ProviderCollateral); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to unlock deal provider balance: %s", err)
	}

	return amountSlashed
}

// Normal expiration. Unlock collaterals for both provider and client.
func (m *marketStateMutation) processDealExpired(rt Runtime, deal *DealProposal, state *DealState) {
	Assert(state.SectorStartEpoch != epochUndefined)

	// Note: payment has already been completed at this point (_rtProcessDealPaymentEpochsElapsed)
	if err := m.unlockBalance(deal.Provider, deal.ProviderCollateral, ProviderCollateral); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed unlocking deal provider balance: %s", err)
	}

	if err := m.unlockBalance(deal.Client, deal.ClientCollateral, ClientCollateral); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed unlocking deal client balance: %s", err)
	}
}

func (m *marketStateMutation) generateStorageDealID() abi.DealID {
	ret := m.nextDealId
	m.nextDealId = m.nextDealId + abi.DealID(1)
	return ret
}

////////////////////////////////////////////////////////////////////////////////
// State utility functions
////////////////////////////////////////////////////////////////////////////////

func dealProposalIsInternallyValid(rt Runtime, proposal ClientDealProposal) error {
	// Note: we do not verify the provider signature here, since this is implicit in the
	// authenticity of the on-chain message publishing the deal.
	buf := bytes.Buffer{}
	err := proposal.Proposal.MarshalCBOR(&buf)
	if err != nil {
		return xerrors.Errorf("proposal signature verification failed to marshal proposal: %w", err)
	}
	err = rt.VerifySignature(proposal.ClientSignature, proposal.Proposal.Client, buf.Bytes())
	if err != nil {
		return xerrors.Errorf("signature proposal invalid: %w", err)
	}
	return nil
}

func dealGetPaymentRemaining(deal *DealProposal, slashEpoch abi.ChainEpoch) abi.TokenAmount {
	Assert(slashEpoch <= deal.EndEpoch)

	// Payments are always for start -> end epoch irrespective of when the deal is slashed.
	if slashEpoch < deal.StartEpoch {
		slashEpoch = deal.StartEpoch
	}

	durationRemaining := deal.EndEpoch - slashEpoch
	Assert(durationRemaining >= 0)

	return big.Mul(big.NewInt(int64(durationRemaining)), deal.StoragePricePerEpoch)
}

// MarketStateMutationPermission is the mutation permission on a state field
type MarketStateMutationPermission int

const (
	// Invalid means NO permission
	Invalid MarketStateMutationPermission = iota
	// ReadOnlyPermission allows reading but not mutating the field
	ReadOnlyPermission
	// WritePermission allows mutating the field
	WritePermission
)

type marketStateMutation struct {
	st    *State
	store adt.Store

	proposalPermit MarketStateMutationPermission
	dealProposals  *DealArray

	statePermit MarketStateMutationPermission
	dealStates  *DealMetaArray

	escrowPermit MarketStateMutationPermission
	escrowTable  *adt.BalanceTable

	pendingPermit MarketStateMutationPermission
	pendingDeals  *adt.Map

	dpePermit    MarketStateMutationPermission
	dealsByEpoch *SetMultimap

	lockedPermit                  MarketStateMutationPermission
	lockedTable                   *adt.BalanceTable
	totalClientLockedCollateral   abi.TokenAmount
	totalProviderLockedCollateral abi.TokenAmount
	totalClientStorageFee         abi.TokenAmount

	nextDealId abi.DealID
}

func (s *State) mutator(store adt.Store) *marketStateMutation {
	return &marketStateMutation{st: s, store: store}
}

func (m *marketStateMutation) build() (*marketStateMutation, error) {
	if m.proposalPermit != Invalid {
		proposals, err := AsDealProposalArray(m.store, m.st.Proposals)
		if err != nil {
			return nil, xerrors.Errorf("failed to load deal proposals: %w", err)
		}
		m.dealProposals = proposals
	}

	if m.statePermit != Invalid {
		states, err := AsDealStateArray(m.store, m.st.States)
		if err != nil {
			return nil, xerrors.Errorf("failed to load deal state: %w", err)
		}
		m.dealStates = states
	}

	if m.lockedPermit != Invalid {
		lt, err := adt.AsBalanceTable(m.store, m.st.LockedTable)
		if err != nil {
			return nil, xerrors.Errorf("failed to load locked table: %w", err)
		}
		m.lockedTable = lt
		m.totalClientLockedCollateral = m.st.TotalClientLockedCollateral.Copy()
		m.totalClientStorageFee = m.st.TotalClientStorageFee.Copy()
		m.totalProviderLockedCollateral = m.st.TotalProviderLockedCollateral.Copy()
	}

	if m.escrowPermit != Invalid {
		et, err := adt.AsBalanceTable(m.store, m.st.EscrowTable)
		if err != nil {
			return nil, xerrors.Errorf("failed to load escrow table: %w", err)
		}
		m.escrowTable = et
	}

	if m.pendingPermit != Invalid {
		pending, err := adt.AsMap(m.store, m.st.PendingProposals)
		if err != nil {
			return nil, xerrors.Errorf("failed to load pending proposals: %w", err)
		}
		m.pendingDeals = pending
	}

	if m.dpePermit != Invalid {
		dbe, err := AsSetMultimap(m.store, m.st.DealOpsByEpoch)
		if err != nil {
			return nil, xerrors.Errorf("failed to load deals by epoch: %w", err)
		}
		m.dealsByEpoch = dbe
	}

	m.nextDealId = m.st.NextID

	return m, nil
}

func (m *marketStateMutation) withDealProposals(permit MarketStateMutationPermission) *marketStateMutation {
	m.proposalPermit = permit
	return m
}

func (m *marketStateMutation) withDealStates(permit MarketStateMutationPermission) *marketStateMutation {
	m.statePermit = permit
	return m
}

func (m *marketStateMutation) withEscrowTable(permit MarketStateMutationPermission) *marketStateMutation {
	m.escrowPermit = permit
	return m
}

func (m *marketStateMutation) withLockedTable(permit MarketStateMutationPermission) *marketStateMutation {
	m.lockedPermit = permit
	return m
}

func (m *marketStateMutation) withPendingProposals(permit MarketStateMutationPermission) *marketStateMutation {
	m.pendingPermit = permit
	return m
}

func (m *marketStateMutation) withDealsByEpoch(permit MarketStateMutationPermission) *marketStateMutation {
	m.dpePermit = permit
	return m
}

func (m *marketStateMutation) commitState() error {
	var err error
	if m.proposalPermit == WritePermission {
		if m.st.Proposals, err = m.dealProposals.Root(); err != nil {
			return xerrors.Errorf("failed to flush deal dealProposals: %w", err)
		}
	}

	if m.statePermit == WritePermission {
		if m.st.States, err = m.dealStates.Root(); err != nil {
			return xerrors.Errorf("failed to flush deal states: %w", err)
		}
	}

	if m.lockedPermit == WritePermission {
		if m.st.LockedTable, err = m.lockedTable.Root(); err != nil {
			return xerrors.Errorf("failed to flush locked table: %w", err)
		}
		m.st.TotalClientLockedCollateral = m.totalClientLockedCollateral.Copy()
		m.st.TotalProviderLockedCollateral = m.totalProviderLockedCollateral.Copy()
		m.st.TotalClientStorageFee = m.totalClientStorageFee.Copy()
	}

	if m.escrowPermit == WritePermission {
		if m.st.EscrowTable, err = m.escrowTable.Root(); err != nil {
			return xerrors.Errorf("failed to flush escrow table: %w", err)
		}
	}

	if m.pendingPermit == WritePermission {
		if m.st.PendingProposals, err = m.pendingDeals.Root(); err != nil {
			return xerrors.Errorf("failed to flush pending deals: %w", err)
		}
	}

	if m.dpePermit == WritePermission {
		if m.st.DealOpsByEpoch, err = m.dealsByEpoch.Root(); err != nil {
			return xerrors.Errorf("failed to flush deals by epoch: %w", err)
		}
	}

	m.st.NextID = m.nextDealId
	return nil
}
