package market

import (
	"fmt"

	addr "github.com/filecoin-project/go-address"
	cbg "github.com/whyrusleeping/cbor-gen"
	xerrors "golang.org/x/xerrors"

	abi "github.com/filecoin-project/specs-actors/actors/abi"
	big "github.com/filecoin-project/specs-actors/actors/abi/big"
	builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	verifreg "github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
	vmr "github.com/filecoin-project/specs-actors/actors/runtime"
	exitcode "github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	. "github.com/filecoin-project/specs-actors/actors/util"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
)

type Actor struct{}

type Runtime = vmr.Runtime

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.AddBalance,
		3:                         a.WithdrawBalance,
		4:                         a.PublishStorageDeals,
		5:                         a.VerifyDealsForActivation,
		6:                         a.ActivateDeals,
		7:                         a.OnMinerSectorsTerminate,
		8:                         a.ComputeDataCommitment,
		9:                         a.CronTick,
	}
}

var _ abi.Invokee = Actor{}

////////////////////////////////////////////////////////////////////////////////
// Actor methods
////////////////////////////////////////////////////////////////////////////////

func (a Actor) Constructor(rt Runtime, _ *adt.EmptyValue) *adt.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	emptyArray, err := adt.MakeEmptyArray(adt.AsStore(rt)).Root()
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to create storage market state: %v", err)
	}

	emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to create storage market state: %v", err)
	}

	emptyMSet, err := MakeEmptySetMultimap(adt.AsStore(rt)).Root()
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to create storage market state: %v", err)
	}

	st := ConstructState(emptyArray, emptyMap, emptyMSet)
	rt.State().Create(st)
	return nil
}

type WithdrawBalanceParams struct {
	ProviderOrClientAddress addr.Address
	Amount                  abi.TokenAmount
}

// Attempt to withdraw the specified amount from the balance held in escrow.
// If less than the specified amount is available, yields the entire available balance.
func (a Actor) WithdrawBalance(rt Runtime, params *WithdrawBalanceParams) *adt.EmptyValue {
	if params.Amount.LessThan(big.Zero()) {
		rt.Abortf(exitcode.ErrIllegalArgument, "negative amount %v", params.Amount)
	}

	nominal, recipient := escrowAddress(rt, params.ProviderOrClientAddress)

	amountSlashedTotal := abi.NewTokenAmount(0)
	amountExtracted := abi.NewTokenAmount(0)
	var st State
	rt.State().Transaction(&st, func() interface{} {
		// The withdrawable amount might be slightly less than nominal
		// depending on whether or not all relevant entries have been processed
		// by cron

		minBalance := st.GetLockedBalance(rt, nominal)

		et, err := adt.AsBalanceTable(adt.AsStore(rt), st.EscrowTable)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "load escrow table: %v", err)
		}
		ex, err := et.SubtractWithMinimum(nominal, params.Amount, minBalance)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "subtract form escrow table: %v", err)
		}

		etc, err := et.Root()
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to flush escrow table: %w", err)
		}
		st.EscrowTable = etc
		amountExtracted = ex
		return nil
	})

	if amountSlashedTotal.GreaterThan(big.Zero()) {
		_, code := rt.Send(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, amountSlashedTotal)
		builtin.RequireSuccess(rt, code, "failed to burn slashed funds")
	}

	_, code := rt.Send(recipient, builtin.MethodSend, nil, amountExtracted)
	builtin.RequireSuccess(rt, code, "failed to send funds")
	return nil
}

// Deposits the received value into the balance held in escrow.
func (a Actor) AddBalance(rt Runtime, providerOrClientAddress *addr.Address) *adt.EmptyValue {
	msgValue := rt.Message().ValueReceived()
	if msgValue.LessThan(big.Zero()) {
		rt.Abortf(exitcode.ErrIllegalArgument, "add balance called with negative value %v", msgValue)
	}

	nominal, _ := escrowAddress(rt, *providerOrClientAddress)

	var st State
	rt.State().Transaction(&st, func() interface{} {

		err := st.AddEscrowBalance(adt.AsStore(rt), nominal, msgValue)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "adding to escrow table: %v", err)
		}

		// ensure there is an entry in the locked table
		err = st.AddLockedBalance(adt.AsStore(rt), nominal, big.Zero())
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalArgument, "adding to locked table: %v", err)
		}

		return nil
	})
	return nil
}

type PublishStorageDealsParams struct {
	Deals []ClientDealProposal
}

type PublishStorageDealsReturn struct {
	IDs []abi.DealID
}

// Publish a new set of storage deals (not yet included in a sector).
func (a Actor) PublishStorageDeals(rt Runtime, params *PublishStorageDealsParams) *PublishStorageDealsReturn {

	// Deal message must have a From field identical to the provider of all the deals.
	// This allows us to retain and verify only the client's signature in each deal proposal itself.
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)
	if len(params.Deals) == 0 {
		rt.Abortf(exitcode.ErrIllegalArgument, "empty deals parameter")
	}

	// All deals should have the same provider so get worker once
	providerRaw := params.Deals[0].Proposal.Provider
	provider, ok := rt.ResolveAddress(providerRaw)
	if !ok {
		rt.Abortf(exitcode.ErrNotFound, "failed to resolve provider address %v", providerRaw)
	}

	_, worker := builtin.RequestMinerControlAddrs(rt, provider)
	if worker != rt.Message().Caller() {
		rt.Abortf(exitcode.ErrForbidden, "caller is not provider %v", provider)
	}

	for _, deal := range params.Deals {
		// Check VerifiedClient allowed cap and deduct PieceSize from cap.
		// Either the DealSize is within the available DataCap of the VerifiedClient
		// or this message will fail. We do not allow a deal that is partially verified.
		if deal.Proposal.VerifiedDeal {
			_, code := rt.Send(
				builtin.VerifiedRegistryActorAddr,
				builtin.MethodsVerifiedRegistry.UseBytes,
				&verifreg.UseBytesParams{
					Address:  deal.Proposal.Client,
					DealSize: big.NewIntUnsigned(uint64(deal.Proposal.PieceSize)),
				},
				abi.NewTokenAmount(0),
			)
			builtin.RequireSuccess(rt, code, "failed to add verified deal for client: %v", deal.Proposal.Client)
		}
	}

	var newDealIds []abi.DealID
	var st State
	rt.State().Transaction(&st, func() interface{} {
		proposals, err := AsDealProposalArray(adt.AsStore(rt), st.Proposals)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to load proposals array: %s", err)
		}

		pending, err := adt.AsMap(adt.AsStore(rt), st.PendingProposals)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load pending proposals map: %s", err)

		dealOps, err := AsSetMultimap(adt.AsStore(rt), st.DealOpsByEpoch)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to load deal ids set: %s", err)
		}

		// All storage proposals will be added in an atomic transaction; this operation will be unrolled if any of them fails.
		for di, deal := range params.Deals {
			validateDeal(rt, deal)

			if deal.Proposal.Provider != provider && deal.Proposal.Provider != providerRaw {
				rt.Abortf(exitcode.ErrIllegalArgument, "cannot publish deals from different providers at the same time")
			}

			client, ok := rt.ResolveAddress(deal.Proposal.Client)
			if !ok {
				rt.Abortf(exitcode.ErrNotFound, "failed to resolve client address %v", deal.Proposal.Client)
			}
			// Normalise provider and client addresses in the proposal stored on chain (after signature verification).
			deal.Proposal.Provider = provider
			deal.Proposal.Client = client

			st.lockBalanceOrAbort(rt, client, deal.Proposal.ClientCollateral, ClientCollateral)
			st.lockBalanceOrAbort(rt, client, deal.Proposal.TotalStorageFee(), ClientStorageFee)
			st.lockBalanceOrAbort(rt, provider, deal.Proposal.ProviderCollateral, ProviderCollateral)

			id := st.generateStorageDealID()

			pcid, err := deal.Proposal.Cid()
			if err != nil {
				rt.Abortf(exitcode.ErrIllegalArgument, "failed to take cid of proposal %d: %s", di, err)
			}

			has, err := pending.Get(adt.CidKey(pcid), nil)
			if err != nil {
				rt.Abortf(exitcode.ErrIllegalState, "failed to check for existence of deal proposal: %v", err)
			}

			if has {
				rt.Abortf(exitcode.ErrIllegalArgument, "cannot publish duplicate deals")
			}

			if err := pending.Put(adt.CidKey(pcid), &deal.Proposal); err != nil {
				rt.Abortf(exitcode.ErrIllegalState, "set deal in pending: %v", err)
			}

			if err := proposals.Set(id, &deal.Proposal); err != nil {
				rt.Abortf(exitcode.ErrIllegalState, "set deal: %v", err)
			}

			if err := dealOps.Put(deal.Proposal.StartEpoch, id); err != nil {
				rt.Abortf(exitcode.ErrIllegalState, "set deal in ops set: %v", err)
			}

			newDealIds = append(newDealIds, id)
		}
		pendc, err := pending.Root()
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to flush pending proposal set: %w", err)
		}
		st.PendingProposals = pendc

		propc, err := proposals.Root()
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to flush proposal set: %w", err)
		}
		st.Proposals = propc

		dipc, err := dealOps.Root()
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to flush deal ids map: %w", err)
		}

		st.DealOpsByEpoch = dipc
		return nil
	})

	return &PublishStorageDealsReturn{newDealIds}
}

type VerifyDealsForActivationParams struct {
	DealIDs      []abi.DealID
	SectorExpiry abi.ChainEpoch
	SectorStart  abi.ChainEpoch
}

type VerifyDealsForActivationReturn struct {
	DealWeight         abi.DealWeight
	VerifiedDealWeight abi.DealWeight
}

// Verify that a given set of storage deals is valid for a sector currently being PreCommitted
// and return DealWeight of the set of storage deals given.
// The weight is defined as the sum, over all deals in the set, of the product of deal size and duration.
func (A Actor) VerifyDealsForActivation(rt Runtime, params *VerifyDealsForActivationParams) *VerifyDealsForActivationReturn {
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)
	minerAddr := rt.Message().Caller()

	var st State
	rt.State().Readonly(&st)
	store := adt.AsStore(rt)

	dealWeight, verifiedWeight, err := ValidateDealsForActivation(&st, store, params.DealIDs, minerAddr, params.SectorExpiry, params.SectorStart)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to validate proposals for activation")

	return &VerifyDealsForActivationReturn{
		DealWeight:         dealWeight,
		VerifiedDealWeight: verifiedWeight,
	}
}

type ActivateDealsParams struct {
	DealIDs      []abi.DealID
	SectorExpiry abi.ChainEpoch
}

// Verify that a given set of storage deals is valid for a sector currently being ProveCommitted,
// update the market's internal state accordingly.
func (a Actor) ActivateDeals(rt Runtime, params *ActivateDealsParams) *adt.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)
	minerAddr := rt.Message().Caller()
	currEpoch := rt.CurrEpoch()

	var st State
	store := adt.AsStore(rt)

	// Update deal states.
	rt.State().Transaction(&st, func() interface{} {
		_, _, err := ValidateDealsForActivation(&st, store, params.DealIDs, minerAddr, params.SectorExpiry, currEpoch)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to validate proposals for activation")

		states, err := AsDealStateArray(store, st.States)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load deal states")

		pending, err := adt.AsMap(adt.AsStore(rt), st.PendingProposals)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "load pending %v", err)

		proposals, err := AsDealProposalArray(adt.AsStore(rt), st.Proposals)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "load proposals %v", err)

		for _, dealID := range params.DealIDs {
			// This construction could be replaced with a single "update deal state" state method, possibly batched
			// over all deal ids at once.
			_, found, err := states.Get(dealID)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get state for dealId %d", dealID)
			if found {
				rt.Abortf(exitcode.ErrIllegalArgument, "deal %d already included in another sector", dealID)
			}

			proposal, found, err := proposals.Get(dealID)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load proposal for dealId %d", dealID)
			if !found {
				rt.Abortf(exitcode.ErrNotFound, "dealId %d not found", dealID)
			}

			propc, err := proposal.Cid()
			if err != nil {
				rt.Abortf(exitcode.ErrIllegalState, "get proposal cid %v", err)
			}

			has, err := pending.Get(adt.CidKey(propc), nil)
			if err != nil {
				rt.Abortf(exitcode.ErrIllegalState, "no pending proposal for  %v", err)
			}

			if !has {
				rt.Abortf(exitcode.ErrIllegalState, "tried to active deal that was not in the pending set (%s)", propc)
			}

			err = states.Set(dealID, &DealState{
				SectorStartEpoch: currEpoch,
				LastUpdatedEpoch: epochUndefined,
				SlashEpoch:       epochUndefined,
			})
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to set deal state %d", dealID)
		}

		st.States, err = states.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush deal states")
		return nil
	})

	return nil
}

type ComputeDataCommitmentParams struct {
	DealIDs    []abi.DealID
	SectorType abi.RegisteredSealProof
}

func (a Actor) ComputeDataCommitment(rt Runtime, params *ComputeDataCommitmentParams) *cbg.CborCid {
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)

	pieces := make([]abi.PieceInfo, 0)
	var st State
	rt.State().Transaction(&st, func() interface{} {
		for _, dealID := range params.DealIDs {
			deal := st.mustGetDeal(rt, dealID)
			pieces = append(pieces, abi.PieceInfo{
				PieceCID: deal.PieceCID,
				Size:     deal.PieceSize,
			})
		}
		return nil
	})

	commd, err := rt.Syscalls().ComputeUnsealedSectorCID(params.SectorType, pieces)
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalArgument, "failed to compute unsealed sector CID: %s", err)
	}

	return (*cbg.CborCid)(&commd)
}

type OnMinerSectorsTerminateParams struct {
	DealIDs []abi.DealID
}

// Terminate a set of deals in response to their containing sector being terminated.
// Slash provider collateral, refund client collateral, and refund partial unpaid escrow
// amount to client.
func (a Actor) OnMinerSectorsTerminate(rt Runtime, params *OnMinerSectorsTerminateParams) *adt.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)
	minerAddr := rt.Message().Caller()

	var st State
	rt.State().Transaction(&st, func() interface{} {
		proposals, err := AsDealProposalArray(adt.AsStore(rt), st.Proposals)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "load proposals: %v", err)
		}

		states, err := AsDealStateArray(adt.AsStore(rt), st.States)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "load states: %v", err)
		}

		for _, dealID := range params.DealIDs {
			deal, found, err := proposals.Get(dealID)
			if err != nil {
				rt.Abortf(exitcode.ErrIllegalState, "get deal: %v", err)
			}
			// deal could have terminated and hence deleted before the sector is terminated.
			// we should simply continue instead of aborting execution here if a deal is not found.
			if !found {
				continue
			}

			Assert(deal.Provider == minerAddr)

			// do not slash expired deals
			if deal.EndEpoch <= rt.CurrEpoch() {
				continue
			}

			state, found, err := states.Get(dealID)
			if err != nil {
				rt.Abortf(exitcode.ErrIllegalState, "get deal: %v", err)
			}
			if !found {
				rt.Abortf(exitcode.ErrIllegalState, "no state found for deal in sector being terminated")
			}

			// mark the deal for slashing here.
			// actual releasing of locked funds for the client and slashing of provider collateral happens in CronTick.
			state.SlashEpoch = rt.CurrEpoch()

			if err := states.Set(dealID, state); err != nil {
				rt.Abortf(exitcode.ErrIllegalState, "set deal: %v", err)
			}
		}

		st.States, err = states.Root()
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to flush states: %s", err)
		}
		return nil
	})
	return nil
}

func (a Actor) CronTick(rt Runtime, params *adt.EmptyValue) *adt.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.CronActorAddr)
	amountSlashed := big.Zero()

	var timedOutVerifiedDeals []*DealProposal

	var st State
	rt.State().Transaction(&st, func() interface{} {
		dbe, err := AsSetMultimap(adt.AsStore(rt), st.DealOpsByEpoch)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to load deal opts set: %s", err)
		}

		updatesNeeded := make(map[abi.ChainEpoch][]abi.DealID)

		states, err := AsDealStateArray(adt.AsStore(rt), st.States)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "get state state: %v", err)
		}

		et, err := adt.AsBalanceTable(adt.AsStore(rt), st.EscrowTable)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "loading escrow table: %s", err)
		}

		lt, err := adt.AsBalanceTable(adt.AsStore(rt), st.LockedTable)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "loading locked balance table: %s", err)
		}

		pending, err := adt.AsMap(adt.AsStore(rt), st.PendingProposals)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "loading pending proposals map: %s", err)
		}

		for i := st.LastCron + 1; i <= rt.CurrEpoch(); i++ {
			if err := dbe.ForEach(i, func(dealID abi.DealID) error {
				state, found, err := states.Get(dealID)
				if err != nil {
					rt.Abortf(exitcode.ErrIllegalState, "failed to get deal: %d", dealID)
				}

				if !found {
					return nil
				}

				deal := st.mustGetDeal(rt, dealID)
				dcid, err := deal.Cid()
				if err != nil {
					return xerrors.Errorf("failed to get cid for deal proposal: %w", err)
				}
				if err := pending.Delete(adt.CidKey(dcid)); err != nil {
					rt.Abortf(exitcode.ErrIllegalState, "failed to delete pending proposal: %v", err)
				}

				if state.SectorStartEpoch == epochUndefined {
					// Not yet appeared in proven sector; check for timeout.
					AssertMsg(rt.CurrEpoch() >= deal.StartEpoch, "if sector start is not set, we must be in a timed out state")

					slashed := st.processDealInitTimedOut(rt, et, lt, dealID, deal, state)
					if !slashed.IsZero() {
						amountSlashed = big.Add(amountSlashed, slashed)
					}
					if deal.VerifiedDeal {
						timedOutVerifiedDeals = append(timedOutVerifiedDeals, deal)
					}
					return nil
				}

				slashAmount, nextEpoch := st.updatePendingDealState(rt, state, deal, dealID, et, lt, rt.CurrEpoch())
				if !slashAmount.IsZero() {
					amountSlashed = big.Add(amountSlashed, slashAmount)
				}

				if nextEpoch != epochUndefined {
					Assert(nextEpoch > rt.CurrEpoch())

					// TODO: can we avoid having this field?
					// https://github.com/filecoin-project/specs-actors/issues/463
					state.LastUpdatedEpoch = rt.CurrEpoch()

					if err := states.Set(dealID, state); err != nil {
						rt.Abortf(exitcode.ErrPlaceholder, "failed to get deal: %v", err)
					}

					updatesNeeded[nextEpoch] = append(updatesNeeded[nextEpoch], dealID)
				}

				return nil
			}); err != nil {
				rt.Abortf(exitcode.ErrIllegalState, "failed to iterate deals for epoch: %s", err)
			}
			if err := dbe.RemoveAll(i); err != nil {
				rt.Abortf(exitcode.ErrIllegalState, "failed to delete deals from set: %s", err)
			}
		}

		// NB: its okay that we're doing a 'random' golang map iteration here
		// because HAMTs and AMTs are insertion order independent, the same set of
		// data inserted will always produce the same structure, no matter the order
		for epoch, deals := range updatesNeeded {
			if err := dbe.PutMany(epoch, deals); err != nil {
				rt.Abortf(exitcode.ErrIllegalState, "failed to reinsert deal IDs into epoch set: %s", err)
			}
		}

		ndbec, err := dbe.Root()
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to get root of deals by epoch set: %s", err)
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

		st.DealOpsByEpoch = ndbec

		st.LastCron = rt.CurrEpoch()

		return nil
	})

	for _, d := range timedOutVerifiedDeals {
		_, code := rt.Send(
			builtin.VerifiedRegistryActorAddr,
			builtin.MethodsVerifiedRegistry.RestoreBytes,
			&verifreg.RestoreBytesParams{
				Address:  d.Client,
				DealSize: big.NewIntUnsigned(uint64(d.PieceSize)),
			},
			abi.NewTokenAmount(0),
		)

		builtin.RequireSuccess(rt, code, "failed to restore bytes for verified client: %v", d.Client)
	}

	_, e := rt.Send(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, amountSlashed)
	builtin.RequireSuccess(rt, e, "expected send to burnt funds actor to succeed")
	return nil
}

//
// Exported functions
//

// Validates a collection of deal proposals for activation, and returns their combined weight,
// split into regular deal weight and verified deal weight.
func ValidateDealsForActivation(st *State, store adt.Store, dealIDs []abi.DealID, minerAddr addr.Address,
	sectorExpiry, currEpoch abi.ChainEpoch) (big.Int, big.Int, error) {

	proposals, err := AsDealProposalArray(store, st.Proposals)
	if err != nil {
		return big.Int{}, big.Int{}, fmt.Errorf("failed to load proposals: %w", err)
	}

	totalDealSpaceTime := big.Zero()
	totalVerifiedSpaceTime := big.Zero()
	for _, dealID := range dealIDs {
		proposal, found, err := proposals.Get(dealID)
		if err != nil {
			return big.Int{}, big.Int{}, fmt.Errorf("failed to load deal %d: %w", dealID, err)
		}
		if !found {
			return big.Int{}, big.Int{}, fmt.Errorf("dealId %d not found", dealID)
		}
		if err = validateDealCanActivate(proposal, minerAddr, sectorExpiry, currEpoch); err != nil {
			return big.Int{}, big.Int{}, fmt.Errorf("cannot activate deal %d: %w", dealID, err)
		}

		// Compute deal weight
		dealSpaceTime := DealWeight(proposal)
		if proposal.VerifiedDeal {
			totalVerifiedSpaceTime = big.Add(totalVerifiedSpaceTime, dealSpaceTime)
		} else {
			totalDealSpaceTime = big.Add(totalDealSpaceTime, dealSpaceTime)
		}
	}
	return totalDealSpaceTime, totalVerifiedSpaceTime, nil
}

////////////////////////////////////////////////////////////////////////////////
// Checks
////////////////////////////////////////////////////////////////////////////////

func validateDealCanActivate(proposal *DealProposal, minerAddr addr.Address, sectorExpiration, currEpoch abi.ChainEpoch) error {
	if proposal.Provider != minerAddr {
		return fmt.Errorf("proposal has provider %v, must be %v", proposal.Provider, minerAddr)
	}
	if currEpoch > proposal.StartEpoch {
		return fmt.Errorf("proposal start epoch %d has already elapsed at %d", proposal.StartEpoch, currEpoch)
	}
	if proposal.EndEpoch > sectorExpiration {
		return fmt.Errorf("proposal expiration %d exceeds sector expiration %d", proposal.EndEpoch, sectorExpiration)
	}
	return nil
}

func validateDeal(rt Runtime, deal ClientDealProposal) {
	if err := dealProposalIsInternallyValid(rt, deal); err != nil {
		rt.Abortf(exitcode.ErrIllegalArgument, "Invalid deal proposal: %s", err)
	}

	proposal := deal.Proposal

	if proposal.EndEpoch <= proposal.StartEpoch {
		rt.Abortf(exitcode.ErrIllegalArgument, "proposal end before proposal start")
	}

	if rt.CurrEpoch() > proposal.StartEpoch {
		rt.Abortf(exitcode.ErrIllegalArgument, "Deal start epoch has already elapsed.")
	}

	minDuration, maxDuration := dealDurationBounds(proposal.PieceSize)
	if proposal.Duration() < minDuration || proposal.Duration() > maxDuration {
		rt.Abortf(exitcode.ErrIllegalArgument, "Deal duration out of bounds.")
	}

	minPrice, maxPrice := dealPricePerEpochBounds(proposal.PieceSize, proposal.Duration())
	if proposal.StoragePricePerEpoch.LessThan(minPrice) || proposal.StoragePricePerEpoch.GreaterThan(maxPrice) {
		rt.Abortf(exitcode.ErrIllegalArgument, "Storage price out of bounds.")
	}

	minProviderCollateral, maxProviderCollateral := dealProviderCollateralBounds(proposal.PieceSize, proposal.Duration())
	if proposal.ProviderCollateral.LessThan(minProviderCollateral) || proposal.ProviderCollateral.GreaterThan(maxProviderCollateral) {
		rt.Abortf(exitcode.ErrIllegalArgument, "Provider collateral out of bounds.")
	}

	minClientCollateral, maxClientCollateral := dealClientCollateralBounds(proposal.PieceSize, proposal.Duration())
	if proposal.ClientCollateral.LessThan(minClientCollateral) || proposal.ClientCollateral.GreaterThan(maxClientCollateral) {
		rt.Abortf(exitcode.ErrIllegalArgument, "Client collateral out of bounds.")
	}
}

// Resolves a provider or client address to the canonical form against which a balance should be held, and
// the designated recipient address of withdrawals (which is the same, for simple account parties).
func escrowAddress(rt Runtime, addr addr.Address) (nominal addr.Address, recipient addr.Address) {
	// Resolve the provided address to the canonical form against which the balance is held.
	nominal, ok := rt.ResolveAddress(addr)
	if !ok {
		rt.Abortf(exitcode.ErrIllegalArgument, "failed to resolve address %v", addr)
	}

	codeID, ok := rt.GetActorCodeCID(nominal)
	if !ok {
		rt.Abortf(exitcode.ErrIllegalArgument, "no code for address %v", nominal)
	}

	if codeID.Equals(builtin.StorageMinerActorCodeID) {
		// Storage miner actor entry; implied funds recipient is the associated owner address.
		ownerAddr, workerAddr := builtin.RequestMinerControlAddrs(rt, nominal)
		rt.ValidateImmediateCallerIs(ownerAddr, workerAddr)
		return nominal, ownerAddr
	}

	// Ordinary account-style actor entry; funds recipient is just the entry address itself.
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)
	return nominal, nominal
}
