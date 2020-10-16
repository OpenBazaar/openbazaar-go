package market

import (
	"bytes"
	"encoding/binary"

	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

type StateSummary struct {
	ProposalIDs          []abi.DealID
	PendingProposalCount uint64
	DealStateCount       uint64
	LockTableCount       uint64
	DealOpEpochCount     uint64
	DealOpCount          uint64
}

// Checks internal invariants of market state.
func CheckStateInvariants(st *State, store adt.Store, balance abi.TokenAmount, currEpoch abi.ChainEpoch) (*StateSummary, *builtin.MessageAccumulator, error) {
	acc := &builtin.MessageAccumulator{}

	acc.Require(
		st.TotalClientLockedCollateral.GreaterThanEqual(big.Zero()),
		"negative total client locked collateral: %v", st.TotalClientLockedCollateral)

	acc.Require(
		st.TotalProviderLockedCollateral.GreaterThanEqual(big.Zero()),
		"negative total provider locked collateral: %v", st.TotalClientLockedCollateral)

	acc.Require(
		st.TotalClientStorageFee.GreaterThanEqual(big.Zero()),
		"negative total client storage fee: %v", st.TotalClientLockedCollateral)

	//
	// Proposals
	//

	allIDs := make(map[abi.DealID]struct{})
	proposalCids := make(map[cid.Cid]struct{})
	maxDealID := int64(-1)
	var proposalIDs []abi.DealID
	expectedDealOps := make(map[abi.DealID]struct{})

	proposals, err := adt.AsArray(store, st.Proposals)
	if err != nil {
		return nil, acc, err
	}
	var proposal DealProposal
	totalProposalCollateral := abi.NewTokenAmount(0)
	err = proposals.ForEach(&proposal, func(dealID int64) error {
		allIDs[abi.DealID(dealID)] = struct{}{}

		pcid, err := proposal.Cid()
		if err != nil {
			return err
		}

		if proposal.StartEpoch >= currEpoch {
			expectedDealOps[abi.DealID(dealID)] = struct{}{}
		}

		// keep some state
		proposalCids[pcid] = struct{}{}
		if dealID > maxDealID {
			maxDealID = dealID
		}
		proposalIDs = append(proposalIDs, abi.DealID(dealID))

		totalProposalCollateral = big.Sum(totalProposalCollateral, proposal.ClientCollateral, proposal.ProviderCollateral)

		acc.Require(proposal.Client.Protocol() == address.ID, "client address for deal %d is not an ID address", dealID)
		acc.Require(proposal.Provider.Protocol() == address.ID, "provider address for deal %d is not an ID address", dealID)

		return nil
	})
	if err != nil {
		return nil, acc, err
	}

	// next id should be higher than any existing deal
	acc.Require(int64(st.NextID) > maxDealID, "next id, %d, is not greater than highest id in proposals, %d", st.NextID, maxDealID)

	//
	// Deal States
	//

	dealStateCount := uint64(0)
	dealStates, err := adt.AsArray(store, st.States)
	if err != nil {
		return nil, acc, err
	}
	var dealState DealState
	err = dealStates.ForEach(&dealState, func(dealID int64) error {
		acc.Require(
			dealState.SectorStartEpoch >= 0,
			"deal %d state start epoch undefined: %v", dealID, dealState)

		acc.Require(
			dealState.LastUpdatedEpoch == epochUndefined || dealState.LastUpdatedEpoch >= dealState.SectorStartEpoch,
			"deal %d state last updated before sector start: %v", dealID, dealState)

		acc.Require(
			dealState.LastUpdatedEpoch == epochUndefined || dealState.LastUpdatedEpoch <= currEpoch,
			"deal %d last updated epoch %d after current %d", dealID, dealState.LastUpdatedEpoch, currEpoch)

		acc.Require(
			dealState.SlashEpoch == epochUndefined || dealState.SlashEpoch >= dealState.SectorStartEpoch,
			"deal %d state slashed before sector start: %v", dealID, dealState)

		acc.Require(
			dealState.SlashEpoch == epochUndefined || dealState.SlashEpoch <= currEpoch,
			"deal %d state slashed after current epoch %d: %v", dealID, currEpoch, dealState)

		_, found := allIDs[abi.DealID(dealID)]
		acc.Require(found, "deal proposal %d for deal state not found", dealID)

		dealStateCount++
		return nil
	})
	if err != nil {
		return nil, acc, err
	}

	//
	// Pending Proposals
	//

	pendingProposalCount := uint64(0)
	pendingProposals, err := adt.AsMap(store, st.PendingProposals)
	if err != nil {
		return nil, nil, err
	}
	var pendingProposal DealProposal
	err = pendingProposals.ForEach(&pendingProposal, func(key string) error {
		proposalCID, err := cid.Parse([]byte(key))
		if err != nil {
			return err
		}

		pcid, err := pendingProposal.Cid()
		if err != nil {
			return err
		}
		acc.Require(pcid.Equals(proposalCID), "pending proposal's key does not match its CID %v != %v", pcid, proposalCID)

		_, found := proposalCids[pcid]
		acc.Require(found, "pending proposal with cid %v not fond within proposals %v", pcid, pendingProposals)

		pendingProposalCount++
		return nil
	})
	if err != nil {
		return nil, acc, err
	}

	//
	// Escrow Table and Locked Table
	//

	escrowTable, err := adt.AsBalanceTable(store, st.EscrowTable)
	if err != nil {
		return nil, acc, err
	}

	lockTableCount := uint64(0)
	lockTable, err := adt.AsBalanceTable(store, st.LockedTable)
	if err != nil {
		return nil, acc, err
	}
	var lockedAmount abi.TokenAmount
	lockedTotal := abi.NewTokenAmount(0)
	err = (*adt.Map)(lockTable).ForEach(&lockedAmount, func(key string) error {
		addr, err := address.NewFromBytes([]byte(key))
		if err != nil {
			return err
		}
		lockedTotal = big.Add(lockedTotal, lockedAmount)

		// every entry in locked table should have a corresponding entry in escrow table that is at least as high
		escrowAmount, err := escrowTable.Get(addr)
		if err != nil {
			return err
		}
		acc.Require(escrowAmount.GreaterThanEqual(lockedAmount),
			"locked funds for %s, %s, greater than escrow amount, %s", addr, lockedAmount, escrowAmount)

		lockTableCount++
		return nil
	})
	if err != nil {
		return nil, acc, err
	}

	// lockTable total should be sum of client and provider locked plus client storage fee
	expectedLockTotal := big.Sum(st.TotalProviderLockedCollateral, st.TotalClientLockedCollateral, st.TotalClientStorageFee)
	acc.Require(lockedTotal.Equals(expectedLockTotal),
		"locked total, %s, does not sum to provider locked, %s, client locked, %s, and client storage fee, %s",
		lockedTotal, st.TotalProviderLockedCollateral, st.TotalClientLockedCollateral, st.TotalClientStorageFee)

	// assert escrow <= actor balance
	// lockTable item <= escrow item and escrowTotal <= balance implies lockTable total <= balance
	escrowTotal, err := escrowTable.Total()
	if err != nil {
		return nil, acc, err
	}
	acc.Require(escrowTotal.LessThanEqual(balance), "escrow total, %v, greater than actor balance, %v", escrowTotal, balance)
	acc.Require(escrowTotal.GreaterThanEqual(totalProposalCollateral), "escrow total, %v, lset than sum of proposal collateral, %v", escrowTotal, totalProposalCollateral)

	//
	// Deal Ops by Epoch
	//

	dealOpEpochCount := uint64(0)
	dealOpCount := uint64(0)
	dealOps, err := AsSetMultimap(store, st.DealOpsByEpoch)
	if err != nil {
		return nil, acc, err
	}

	// get into internals just to iterate through full data structure
	var setRoot cbg.CborCid
	err = dealOps.mp.ForEach(&setRoot, func(key string) error {
		epoch, err := binary.ReadUvarint(bytes.NewReader([]byte(key)))
		if err != nil {
			return errors.Wrapf(err, "deal ops has key that is not an int: %s", key)
		}

		dealOpEpochCount++
		return dealOps.ForEach(abi.ChainEpoch(epoch), func(id abi.DealID) error {
			_, found := allIDs[id]
			acc.Require(found, "deal op found for deal id %d with missing proposal at epoch %d", id, epoch)
			delete(expectedDealOps, id)
			dealOpCount++
			return nil
		})
	})
	if err != nil {
		return nil, acc, err
	}

	acc.Require(len(expectedDealOps) == 0, "missing deal ops for proposals: %v", expectedDealOps)

	return &StateSummary{
		ProposalIDs:          proposalIDs,
		PendingProposalCount: pendingProposalCount,
		DealStateCount:       dealStateCount,
		LockTableCount:       lockTableCount,
		DealOpEpochCount:     dealOpEpochCount,
		DealOpCount:          dealOpCount,
	}, acc, nil
}
