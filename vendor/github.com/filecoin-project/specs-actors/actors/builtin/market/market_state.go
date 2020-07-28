package market

import (
	"bytes"
	"fmt"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/ipfs/go-cid"
	xerrors "golang.org/x/xerrors"

	abi "github.com/filecoin-project/specs-actors/actors/abi"
	big "github.com/filecoin-project/specs-actors/actors/abi/big"
	exitcode "github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	. "github.com/filecoin-project/specs-actors/actors/util"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
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

	// PendingProposals tracks proposals that have not yet reached their deal start date.
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

func (st *State) updatePendingDealState(rt Runtime, state *DealState, deal *DealProposal, dealID abi.DealID, et, lt *adt.BalanceTable, epoch abi.ChainEpoch) (abi.TokenAmount, abi.ChainEpoch) {
	amountSlashed := abi.NewTokenAmount(0)

	everUpdated := state.LastUpdatedEpoch != epochUndefined
	everSlashed := state.SlashEpoch != epochUndefined

	Assert(!everUpdated || (state.LastUpdatedEpoch <= epoch)) // if the deal was ever updated, make sure it didn't happen in the future

	// This would be the case that the first callback somehow triggers before it is scheduled to
	// This is expected not to be able to happen
	if deal.StartEpoch > epoch {
		return amountSlashed, epochUndefined
	}

	dealEnd := deal.EndEpoch
	if everSlashed {
		Assert(state.SlashEpoch <= dealEnd)
		dealEnd = state.SlashEpoch
	}

	elapsedStart := deal.StartEpoch
	if everUpdated && state.LastUpdatedEpoch > elapsedStart {
		elapsedStart = state.LastUpdatedEpoch
	}

	elapsedEnd := dealEnd
	if epoch < elapsedEnd {
		elapsedEnd = epoch
	}

	numEpochsElapsed := elapsedEnd - elapsedStart

	{
		// Process deal payment for the elapsed epochs.
		totalPayment := big.Mul(big.NewInt(int64(numEpochsElapsed)), deal.StoragePricePerEpoch)

		st.transferBalance(rt, deal.Client, deal.Provider, totalPayment)
	}

	if everSlashed {
		// unlock client collateral and locked storage fee
		paymentRemaining := dealGetPaymentRemaining(deal, state.SlashEpoch)

		// unlock remaining storage fee
		if err := st.unlockBalance(lt, deal.Client, paymentRemaining, ClientStorageFee); err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to unlock remaining client storage fee: %s", err)
		}
		// unlock client collateral
		if err := st.unlockBalance(lt, deal.Client, deal.ClientCollateral, ClientCollateral); err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to unlock client collateral: %s", err)
		}

		// slash provider collateral
		amountSlashed = deal.ProviderCollateral
		if err := st.slashBalance(et, lt, deal.Provider, amountSlashed, ProviderCollateral); err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "slashing balance: %s", err)
		}

		st.deleteDeal(rt, dealID)
		return amountSlashed, epochUndefined
	}

	if epoch >= deal.EndEpoch {
		st.processDealExpired(rt, deal, state, lt, dealID)
		return amountSlashed, epochUndefined
	}

	next := epoch + DealUpdatesInterval
	if next > deal.EndEpoch {
		next = deal.EndEpoch
	}

	return amountSlashed, next
}

func (st *State) mutateDealProposals(rt Runtime, f func(*DealArray)) {
	proposals, err := AsDealProposalArray(adt.AsStore(rt), st.Proposals)
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to load deal proposals array: %s", err)
	}

	f(proposals)

	rcid, err := proposals.Root()
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "flushing deal proposals set failed: %s", err)
	}

	st.Proposals = rcid
}

func (st *State) mutateDealStates(store adt.Store, f func(*DealMetaArray)) error {
	states, err := AsDealStateArray(store, st.States)
	if err != nil {
		return fmt.Errorf("failed to load deal states array: %w", err)
	}

	f(states)

	scid, err := states.Root()
	if err != nil {
		return fmt.Errorf("flushing deal states set failed: %w", err)
	}

	st.States = scid
	return nil
}

func (st *State) deleteDeal(rt Runtime, dealID abi.DealID) {
	st.mutateDealProposals(rt, func(proposals *DealArray) {
		if err := proposals.Delete(uint64(dealID)); err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to delete deal: %v", err)
		}
	})

	if err := st.mutateDealStates(adt.AsStore(rt), func(states *DealMetaArray) {
		if err := states.Delete(dealID); err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to delete deal state: %v", err)
		}
	}); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to delete deal state: %v", err)
	}
}

// Deal start deadline elapsed without appearing in a proven sector.
// Delete deal, slash a portion of provider's collateral, and unlock remaining collaterals
// for both provider and client.
func (st *State) processDealInitTimedOut(rt Runtime, et, lt *adt.BalanceTable, dealID abi.DealID, deal *DealProposal, state *DealState) abi.TokenAmount {
	Assert(state.SectorStartEpoch == epochUndefined)

	if err := st.unlockBalance(lt, deal.Client, deal.TotalStorageFee(), ClientStorageFee); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failure unlocking client storage fee: %s", err)
	}
	if err := st.unlockBalance(lt, deal.Client, deal.ClientCollateral, ClientCollateral); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failure unlocking client collateral: %s", err)
	}

	amountSlashed := collateralPenaltyForDealActivationMissed(deal.ProviderCollateral)
	amountRemaining := big.Sub(deal.ProviderBalanceRequirement(), amountSlashed)

	if err := st.slashBalance(et, lt, deal.Provider, amountSlashed, ProviderCollateral); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to slash balance: %s", err)
	}

	if err := st.unlockBalance(lt, deal.Provider, amountRemaining, ProviderCollateral); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to unlock deal provider balance: %s", err)
	}

	st.deleteDeal(rt, dealID)
	return amountSlashed
}

// Normal expiration. Delete deal and unlock collaterals for both miner and client.
func (st *State) processDealExpired(rt Runtime, deal *DealProposal, state *DealState, lt *adt.BalanceTable, dealID abi.DealID) {
	Assert(state.SectorStartEpoch != epochUndefined)

	// Note: payment has already been completed at this point (_rtProcessDealPaymentEpochsElapsed)
	if err := st.unlockBalance(lt, deal.Provider, deal.ProviderCollateral, ProviderCollateral); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed unlocking deal provider balance: %s", err)
	}

	if err := st.unlockBalance(lt, deal.Client, deal.ClientCollateral, ClientCollateral); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed unlocking deal client balance: %s", err)
	}

	st.deleteDeal(rt, dealID)
}

func (st *State) generateStorageDealID() abi.DealID {
	ret := st.NextID
	st.NextID = st.NextID + abi.DealID(1)
	return ret
}

////////////////////////////////////////////////////////////////////////////////
// Balance table operations
////////////////////////////////////////////////////////////////////////////////

func (st *State) MutateBalanceTable(s adt.Store, c *cid.Cid, f func(t *adt.BalanceTable) error) error {
	t, err := adt.AsBalanceTable(s, *c)
	if err != nil {
		return err
	}

	if err := f(t); err != nil {
		return err
	}

	rc, err := t.Root()
	if err != nil {
		return err
	}

	*c = rc
	return nil
}

func (st *State) AddEscrowBalance(s adt.Store, a addr.Address, amount abi.TokenAmount) error {
	return st.MutateBalanceTable(s, &st.EscrowTable, func(et *adt.BalanceTable) error {
		err := et.AddCreate(a, amount)
		if err != nil {
			return xerrors.Errorf("failed to add %s to balance table: %w", a, err)
		}
		return nil
	})
}

func (st *State) AddLockedBalance(s adt.Store, a addr.Address, amount abi.TokenAmount) error {
	return st.MutateBalanceTable(s, &st.LockedTable, func(lt *adt.BalanceTable) error {
		err := lt.AddCreate(a, amount)
		if err != nil {
			return err
		}
		return nil
	})
}

func (st *State) GetEscrowBalance(rt Runtime, a addr.Address) abi.TokenAmount {
	bt, err := adt.AsBalanceTable(adt.AsStore(rt), st.EscrowTable)
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "get escrow balance: %v", err)
	}
	ret, err := bt.Get(a)
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "get escrow balance: %v", err)
	}
	return ret
}

func (st *State) GetLockedBalance(rt Runtime, a addr.Address) abi.TokenAmount {
	lt, err := adt.AsBalanceTable(adt.AsStore(rt), st.LockedTable)
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "get locked balance: %v", err)
	}
	ret, err := lt.Get(a)
	if _, ok := err.(adt.ErrNotFound); ok {
		rt.Abortf(exitcode.ErrInsufficientFunds, "failed to get locked balance: %v", err)
	}
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "get locked balance: %v", err)
	}
	return ret
}

func (st *State) maybeLockBalance(rt Runtime, addr addr.Address, amount abi.TokenAmount) error {
	Assert(amount.GreaterThanEqual(big.Zero()))

	prevLocked := st.GetLockedBalance(rt, addr)
	escrowBalance := st.GetEscrowBalance(rt, addr)
	if big.Add(prevLocked, amount).GreaterThan(st.GetEscrowBalance(rt, addr)) {
		return xerrors.Errorf("not enough balance to lock for addr %s: %s <  %s + %s", addr, escrowBalance, prevLocked, amount)
	}

	return st.MutateBalanceTable(adt.AsStore(rt), &st.LockedTable, func(lt *adt.BalanceTable) error {
		err := lt.Add(addr, amount)
		if err != nil {
			return xerrors.Errorf("adding locked balance: %w", err)
		}
		return nil
	})
}

// TODO: all these balance table mutations need to happen at the top level and be batched (no flushing after each!)
// https://github.com/filecoin-project/specs-actors/issues/464
func (st *State) unlockBalance(lt *adt.BalanceTable, addr addr.Address, amount abi.TokenAmount, lockReason BalanceLockingReason) error {
	Assert(amount.GreaterThanEqual(big.Zero()))

	err := lt.MustSubtract(addr, amount)
	if err != nil {
		return xerrors.Errorf("subtracting from locked balance: %v", err)
	}

	switch lockReason {
	case ClientCollateral:
		st.TotalClientLockedCollateral = big.Sub(st.TotalClientLockedCollateral, amount)
	case ClientStorageFee:
		st.TotalClientStorageFee = big.Sub(st.TotalClientStorageFee, amount)
	case ProviderCollateral:
		st.TotalProviderLockedCollateral = big.Sub(st.TotalProviderLockedCollateral, amount)
	}

	return nil
}

// move funds from locked in client to available in provider
func (st *State) transferBalance(rt Runtime, fromAddr addr.Address, toAddr addr.Address, amount abi.TokenAmount) {
	Assert(amount.GreaterThanEqual(big.Zero()))

	et, err := adt.AsBalanceTable(adt.AsStore(rt), st.EscrowTable)
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "loading escrow table: %s", err)
	}
	lt, err := adt.AsBalanceTable(adt.AsStore(rt), st.LockedTable)
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "loading locked balance table: %s", err)
	}

	if err := et.MustSubtract(fromAddr, amount); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "subtract from escrow: %v", err)
	}

	if err := st.unlockBalance(lt, fromAddr, amount, ClientStorageFee); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "subtract from locked: %v", err)
	}

	if err := et.Add(toAddr, amount); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "add to escrow: %v", err)
	}

	ltc, err := lt.Root()
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to flush locked table: %s", err)
	}
	etc, err := et.Root()
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to flush escrow table: %s", err)
	}

	st.LockedTable = ltc
	st.EscrowTable = etc
}

func (st *State) slashBalance(et, lt *adt.BalanceTable, addr addr.Address, amount abi.TokenAmount, reason BalanceLockingReason) error {
	Assert(amount.GreaterThanEqual(big.Zero()))

	if err := et.MustSubtract(addr, amount); err != nil {
		return xerrors.Errorf("subtract from escrow: %v", err)
	}

	return st.unlockBalance(lt, addr, amount, reason)
}

////////////////////////////////////////////////////////////////////////////////
// Method utility functions
////////////////////////////////////////////////////////////////////////////////

func (st *State) mustGetDeal(rt Runtime, dealID abi.DealID) *DealProposal {
	proposals, err := AsDealProposalArray(adt.AsStore(rt), st.Proposals)
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "get proposal: %v", err)
	}

	proposal, found, err := proposals.Get(dealID)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load proposal for dealId %d", dealID)
	if !found {
		rt.Abortf(exitcode.ErrNotFound, "dealId %d not found", dealID)
	}

	return proposal
}

func (st *State) lockBalanceOrAbort(rt Runtime, addr addr.Address, amount abi.TokenAmount, reason BalanceLockingReason) {
	if err := st.maybeLockBalance(rt, addr, amount); err != nil {
		rt.Abortf(exitcode.ErrInsufficientFunds, "Insufficient funds available to lock: %s", err)
	}

	switch reason {
	case ClientCollateral:
		st.TotalClientLockedCollateral = big.Add(st.TotalClientLockedCollateral, amount)
	case ClientStorageFee:
		st.TotalClientStorageFee = big.Add(st.TotalClientStorageFee, amount)
	case ProviderCollateral:
		st.TotalProviderLockedCollateral = big.Add(st.TotalProviderLockedCollateral, amount)
	}
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
	err = rt.Syscalls().VerifySignature(proposal.ClientSignature, proposal.Proposal.Client, buf.Bytes())
	if err != nil {
		return xerrors.Errorf("signature proposal invalid: %w", err)
	}
	return nil
}

func dealGetPaymentRemaining(deal *DealProposal, epoch abi.ChainEpoch) abi.TokenAmount {
	Assert(epoch <= deal.EndEpoch)

	durationRemaining := deal.EndEpoch - (epoch - 1)
	Assert(durationRemaining > 0)

	return big.Mul(big.NewInt(int64(durationRemaining)), deal.StoragePricePerEpoch)
}
