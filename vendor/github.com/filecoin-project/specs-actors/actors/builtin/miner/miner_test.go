package miner_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	addr "github.com/filecoin-project/go-address"
	bitfield "github.com/filecoin-project/go-bitfield"
	cid "github.com/ipfs/go-cid"
	"github.com/minio/blake2b-simd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/filecoin-project/specs-actors/support/mock"
	tutil "github.com/filecoin-project/specs-actors/support/testing"
)

var testPid abi.PeerID
var testMultiaddrs []abi.Multiaddrs

// A balance for use in tests where the miner's low balance is not interesting.
var bigBalance = big.Mul(big.NewInt(1000), big.NewInt(1e18))

func init() {
	testPid = abi.PeerID("peerID")

	testMultiaddrs = []abi.Multiaddrs{
		{1},
		{2},
	}

	miner.SupportedProofTypes = map[abi.RegisteredSealProof]struct{}{
		abi.RegisteredSealProof_StackedDrg2KiBV1: {},
	}
}

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, miner.Actor{})
}

func TestConstruction(t *testing.T) {
	actor := miner.Actor{}
	owner := tutil.NewIDAddr(t, 100)
	worker := tutil.NewIDAddr(t, 101)
	workerKey := tutil.NewBLSAddr(t, 0)
	receiver := tutil.NewIDAddr(t, 1000)
	builder := mock.NewBuilder(context.Background(), receiver).
		WithActorType(owner, builtin.AccountActorCodeID).
		WithActorType(worker, builtin.AccountActorCodeID).
		WithHasher(blake2b.Sum256).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	t.Run("simple construction", func(t *testing.T) {
		rt := builder.Build(t)
		params := miner.ConstructorParams{
			OwnerAddr:     owner,
			WorkerAddr:    worker,
			SealProofType: abi.RegisteredSealProof_StackedDrg2KiBV1,
			PeerId:        testPid,
			Multiaddrs:    testMultiaddrs,
		}

		provingPeriodStart := abi.ChainEpoch(2386) // This is just set from running the code.
		rt.ExpectValidateCallerAddr(builtin.InitActorAddr)
		// Fetch worker pubkey.
		rt.ExpectSend(worker, builtin.MethodsAccount.PubkeyAddress, nil, big.Zero(), &workerKey, exitcode.Ok)
		// Register proving period cron.
		rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.EnrollCronEvent,
			makeProvingPeriodCronEventParams(t, provingPeriodStart-1), big.Zero(), nil, exitcode.Ok)
		ret := rt.Call(actor.Constructor, &params)

		assert.Nil(t, ret)
		rt.Verify()

		var st miner.State
		rt.GetState(&st)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		assert.Equal(t, params.OwnerAddr, info.Owner)
		assert.Equal(t, params.WorkerAddr, info.Worker)
		assert.Equal(t, params.PeerId, info.PeerId)
		assert.Equal(t, params.Multiaddrs, info.Multiaddrs)
		assert.Equal(t, abi.RegisteredSealProof_StackedDrg2KiBV1, info.SealProofType)
		assert.Equal(t, abi.SectorSize(2048), info.SectorSize)
		assert.Equal(t, uint64(2), info.WindowPoStPartitionSectors)
		assert.Equal(t, provingPeriodStart, st.ProvingPeriodStart)

		assert.Equal(t, big.Zero(), st.PreCommitDeposits)
		assert.Equal(t, big.Zero(), st.LockedFunds)
		assert.True(t, st.VestingFunds.Defined())
		assert.True(t, st.PreCommittedSectors.Defined())
		assertEmptyBitfield(t, st.NewSectors)
		assert.True(t, st.SectorExpirations.Defined())
		assert.True(t, st.Deadlines.Defined())
		assertEmptyBitfield(t, st.Faults)
		assert.True(t, st.FaultEpochs.Defined())
		assertEmptyBitfield(t, st.Recoveries)
		assertEmptyBitfield(t, st.PostSubmissions)

		var deadlines miner.Deadlines
		assert.True(t, rt.Store().Get(st.Deadlines, &deadlines))
		for i := uint64(0); i < miner.WPoStPeriodDeadlines; i++ {
			assertEmptyBitfield(t, deadlines.Due[i])
		}
	})
}

// Tests for fetching and manipulating miner addresses.
func TestControlAddresses(t *testing.T) {
	actor := newHarness(t, 0)
	builder := builderForHarness(actor)

	t.Run("get addresses", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		o, w := actor.controlAddresses(rt)
		assert.Equal(t, actor.owner, o)
		assert.Equal(t, actor.worker, w)
	})

	// TODO: test changing worker (with delay), changing peer id
	// https://github.com/filecoin-project/specs-actors/issues/479
}

// Test for sector precommitment and proving.
func TestCommitments(t *testing.T) {
	periodOffset := abi.ChainEpoch(100)

	// TODO more tests
	// - Concurrent attempts to upgrade the same CC sector (one should succeed)
	// - Insufficient funds for pre-commit, for prove-commit
	// - CC sector targeted for upgrade expires naturally before the upgrade is proven

	t.Run("valid precommit then provecommit", func(t *testing.T) {
		actor := newHarness(t, periodOffset)
		rt := builderForHarness(actor).
			WithBalance(bigBalance, big.Zero()).
			Build(t)
		precommitEpoch := periodOffset + 1
		rt.SetEpoch(precommitEpoch)
		actor.constructAndVerify(rt)
		deadline := actor.deadline(rt)

		// Make a good commitment for the proof to target.
		sectorNo := abi.SectorNumber(100)
		precommit := makePreCommit(sectorNo, precommitEpoch-1, deadline.PeriodEnd(), nil)
		actor.preCommitSector(rt, precommit)

		// assert precommit exists and meets expectations
		onChainPrecommit := actor.getPreCommit(rt, sectorNo)

		// expect precommit deposit to be initial pledge calculated at precommit time
		sectorSize, err := precommit.SealProof.SectorSize()
		require.NoError(t, err)

		// deal weights mocked by actor harness for market actor must be set in precommit onchain info
		assert.Equal(t, big.NewInt(int64(sectorSize/2)), onChainPrecommit.DealWeight)
		assert.Equal(t, big.NewInt(int64(sectorSize/2)), onChainPrecommit.VerifiedDealWeight)

		qaPower := miner.QAPowerForWeight(sectorSize, precommit.Expiration-precommitEpoch, onChainPrecommit.DealWeight, onChainPrecommit.VerifiedDealWeight)
		expectedDeposit := miner.InitialPledgeForPower(qaPower, actor.networkQAPower, actor.networkPledge, actor.epochReward, rt.TotalFilCircSupply())
		assert.Equal(t, expectedDeposit, onChainPrecommit.PreCommitDeposit)

		// expect total precommit deposit to equal our new deposit
		st := getState(rt)
		assert.Equal(t, expectedDeposit, st.PreCommitDeposits)

		// run prove commit logic
		rt.SetEpoch(precommitEpoch + miner.PreCommitChallengeDelay + 1)
		rt.SetBalance(big.Mul(big.NewInt(1000), big.NewInt(1e18)))
		actor.proveCommitSectorAndConfirm(rt, precommit, precommitEpoch, makeProveCommit(sectorNo), proveCommitConf{})
		st = getState(rt)

		// expect precommit to have been removed
		_, found, err := st.GetPrecommittedSector(rt.AdtStore(), sectorNo)
		require.NoError(t, err)
		require.False(t, found)

		// expect deposit to have been transferred to initial pledges
		expectedInitialPledge := expectedDeposit
		assert.Equal(t, big.Zero(), st.PreCommitDeposits)
		assert.Equal(t, expectedInitialPledge, st.InitialPledgeRequirement)

		// expect new onchain sector
		onChainSector := actor.getSector(rt, sectorNo)

		// expect deal weights to be transfered to on chain info
		assert.Equal(t, onChainPrecommit.DealWeight, onChainSector.DealWeight)
		assert.Equal(t, onChainPrecommit.VerifiedDealWeight, onChainSector.VerifiedDealWeight)

		// expect activation epoch to be precommit
		assert.Equal(t, precommitEpoch, onChainSector.Activation)

		// expect initial plege of sector to be set
		assert.Equal(t, expectedInitialPledge, onChainSector.InitialPledge)

		// expect locked initial pledge of sector to be the same as precommit deposit
		assert.Equal(t, expectedInitialPledge, st.LockedFunds)
	})

	t.Run("invalid pre-commit rejected", func(t *testing.T) {
		actor := newHarness(t, periodOffset)
		rt := builderForHarness(actor).
			WithBalance(bigBalance, big.Zero()).
			Build(t)
		precommitEpoch := periodOffset + 1
		rt.SetEpoch(precommitEpoch)
		actor.constructAndVerify(rt)
		deadline := actor.deadline(rt)
		challengeEpoch := precommitEpoch - 1

		oldSector := actor.commitAndProveSectors(rt, 1, 100, nil)[0]

		// Good commitment.
		actor.preCommitSector(rt, makePreCommit(101, challengeEpoch, deadline.PeriodEnd(), nil))

		// Duplicate pre-commit sector ID
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			actor.preCommitSector(rt, makePreCommit(101, challengeEpoch, deadline.PeriodEnd(), nil))
		})
		rt.Reset()

		// Sector ID already committed
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			actor.preCommitSector(rt, makePreCommit(oldSector.SectorNumber, challengeEpoch, deadline.PeriodEnd(), nil))
		})
		rt.Reset()

		// Bad seal proof type
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			pc := makePreCommit(101, challengeEpoch, deadline.PeriodEnd(), nil)
			pc.SealProof = abi.RegisteredSealProof_StackedDrg8MiBV1
			actor.preCommitSector(rt, pc)
		})
		rt.Reset()

		// Expires at current epoch
		rt.SetEpoch(deadline.PeriodEnd())
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			actor.preCommitSector(rt, makePreCommit(101, challengeEpoch, deadline.PeriodEnd(), nil))
		})
		rt.Reset()

		// Expires before current epoch
		expiration := deadline.PeriodEnd()
		rt.SetEpoch(expiration + 1)
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			actor.preCommitSector(rt, makePreCommit(101, challengeEpoch, deadline.PeriodEnd(), nil))
		})
		rt.Reset()

		// Expires not on period end
		expiration = deadline.PeriodEnd() - 1
		rt.SetEpoch(precommitEpoch)
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			actor.preCommitSector(rt, makePreCommit(101, challengeEpoch, expiration, nil))
		})
		rt.Reset()

		// Errors when expiry too far in the future
		rt.SetEpoch(precommitEpoch)
		expiration = deadline.PeriodEnd() + miner.WPoStProvingPeriod*(miner.MaxSectorExpirationExtension/miner.WPoStProvingPeriod+1)
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			actor.preCommitSector(rt, makePreCommit(101, challengeEpoch, deadline.PeriodEnd()-1, nil))
		})
	})

	t.Run("valid committed capacity upgrade", func(t *testing.T) {
		actor := newHarness(t, periodOffset)
		rt := builderForHarness(actor).
			WithBalance(bigBalance, big.Zero()).
			Build(t)
		actor.constructAndVerify(rt)

		// Commit a sector to upgrade
		oldSector := actor.commitAndProveSectors(rt, 1, 100, nil)[0]

		// Reduce the epoch reward so that a new sector's initial pledge would otherwise be lesser.
		actor.epochReward = big.Div(actor.epochReward, big.NewInt(2))

		challengeEpoch := rt.Epoch() - 1
		upgradeParams := makePreCommit(200, challengeEpoch, oldSector.Expiration, []abi.DealID{1})
		upgradeParams.ReplaceCapacity = true
		upgradeParams.ReplaceSector = oldSector.SectorNumber
		upgrade := actor.preCommitSector(rt, upgradeParams)

		// Check new pre-commit in state
		assert.True(t, upgrade.Info.ReplaceCapacity)
		assert.Equal(t, upgradeParams.ReplaceSector, upgrade.Info.ReplaceSector)
		// Require new sector's pledge to be at least that of the old sector.
		assert.Equal(t, oldSector.InitialPledge, upgrade.PreCommitDeposit)

		// Old sector is unchanged
		oldSectorAgain := actor.getSector(rt, oldSector.SectorNumber)
		assert.Equal(t, oldSector, oldSectorAgain)

		// Deposit and pledge as expected
		st := getState(rt)
		assert.Equal(t, st.PreCommitDeposits, upgrade.PreCommitDeposit)
		assert.Equal(t, st.InitialPledgeRequirement, oldSector.InitialPledge)
		assert.Equal(t, st.LockedFunds, oldSector.InitialPledge)

		// Prove new sector
		rt.SetEpoch(upgrade.PreCommitEpoch + miner.PreCommitChallengeDelay + 1)
		newSector := actor.proveCommitSectorAndConfirm(rt, &upgrade.Info, upgrade.PreCommitEpoch,
			makeProveCommit(upgrade.Info.SectorNumber), proveCommitConf{})

		// Both sectors have pledge
		st = getState(rt)
		assert.Equal(t, big.Zero(), st.PreCommitDeposits)
		assert.Equal(t, st.InitialPledgeRequirement, big.Add(oldSector.InitialPledge, newSector.InitialPledge))
		assert.Equal(t, st.LockedFunds, big.Add(oldSector.InitialPledge, newSector.InitialPledge))

		// The old sector's expiration has changed to the end of this proving period
		deadline := actor.deadline(rt)
		oldSectorAgain = actor.getSector(rt, oldSector.SectorNumber)
		assert.Equal(t, deadline.PeriodEnd(), oldSectorAgain.Expiration)

		// Both sectors are currently listed as new, because deadlines not yet assigned
		assertBfEqual(t, bitfield.NewFromSet([]uint64{100, 200}), st.NewSectors)

		// Roll forward to PP cron and expect old sector removed without penalty
		completeProvingPeriod(rt, actor, true, nil, []*miner.SectorOnChainInfo{oldSectorAgain})

		// The old sector is gone, only the new sector is assigned to a deadline.
		st = getState(rt)
		sectors := actor.collectSectors(rt)
		assert.Equal(t, 1, len(sectors))
		assert.Nil(t, sectors[oldSector.SectorNumber])
		assert.Equal(t, newSector, sectors[newSector.SectorNumber])

		expirations := actor.collectExpirations(rt)
		assert.Equal(t, 1, len(expirations))
		assert.Equal(t, []uint64{200}, expirations[newSector.Expiration])

		provingSet := actor.collectProvingSet(rt)
		assert.Equal(t, map[uint64]struct{}{200: {}}, provingSet)
		assertBfEqual(t, bitfield.NewFromSet([]uint64{}), st.NewSectors) // No new sectors

		// Old sector's pledge still locked (not penalized), but no longer contributes to minimum requirement.
		assert.Equal(t, st.InitialPledgeRequirement, newSector.InitialPledge)
		assert.Equal(t, st.LockedFunds, big.Add(oldSector.InitialPledge, newSector.InitialPledge))
	})

	t.Run("invalid committed capacity upgrade rejected", func(t *testing.T) {
		actor := newHarness(t, periodOffset)
		rt := builderForHarness(actor).
			WithBalance(bigBalance, big.Zero()).
			Build(t)
		actor.constructAndVerify(rt)

		// Commit sectors to target upgrade. The first has no deals, the second has a deal.
		oldSectors := actor.commitAndProveSectors(rt, 2, 100, [][]abi.DealID{nil, {10}})

		challengeEpoch := rt.Epoch() - 1
		upgradeParams := makePreCommit(200, challengeEpoch, oldSectors[0].Expiration, []abi.DealID{20})
		upgradeParams.ReplaceCapacity = true
		upgradeParams.ReplaceSector = oldSectors[0].SectorNumber

		{ // Must have deals
			params := *upgradeParams
			params.DealIDs = nil
			rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
				actor.preCommitSector(rt, &params)
			})
			rt.Reset()
		}
		{ // Old sector cannot have deals
			params := *upgradeParams
			params.ReplaceSector = oldSectors[1].SectorNumber
			rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
				actor.preCommitSector(rt, &params)
			})
			rt.Reset()
		}
		{ // Target sector must exist
			params := *upgradeParams
			params.ReplaceSector = 999
			rt.ExpectAbort(exitcode.ErrNotFound, func() {
				actor.preCommitSector(rt, &params)
			})
			rt.Reset()
		}
		{ // Expiration must not be sooner than target
			params := *upgradeParams
			params.Expiration = params.Expiration - miner.WPoStProvingPeriod
			rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
				actor.preCommitSector(rt, &params)
			})
			rt.Reset()
		}
		{ // Target must not be faulty
			params := *upgradeParams
			st := getState(rt)
			st.Faults.Set(uint64(params.ReplaceSector))
			rt.ReplaceState(st)
			rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
				actor.preCommitSector(rt, &params)
			})
			st.Faults = abi.NewBitField()
			rt.ReplaceState(st)
			rt.Reset()
		}

		// Demonstrate that the params are otherwise ok
		actor.preCommitSector(rt, upgradeParams)
		rt.Verify()
	})

	t.Run("faulty committed capacity sector not replaced", func(t *testing.T) {
		actor := newHarness(t, periodOffset)
		rt := builderForHarness(actor).
			WithBalance(bigBalance, big.Zero()).
			Build(t)
		actor.constructAndVerify(rt)

		// Commit a sector to target upgrade
		oldSector := actor.commitAndProveSectors(rt, 1, 100, nil)[0]

		// Complete proving period
		// June 2020: it is impossible to declare fault for a sector not yet assigned to a deadline
		completeProvingPeriod(rt, actor, true, nil, nil)

		// Pre-commit a sector to replace the existing one
		challengeEpoch := rt.Epoch() - 1
		upgradeParams := makePreCommit(200, challengeEpoch, oldSector.Expiration, []abi.DealID{20})
		upgradeParams.ReplaceCapacity = true
		upgradeParams.ReplaceSector = oldSector.SectorNumber

		upgrade := actor.preCommitSector(rt, upgradeParams)

		// Declare the old sector faulty
		_, qaPower := powerForSectors(actor.sectorSize, []*miner.SectorOnChainInfo{oldSector})
		fee := miner.PledgePenaltyForDeclaredFault(actor.epochReward, actor.networkQAPower, qaPower)
		actor.declareFaults(rt, actor.networkQAPower, fee, oldSector)

		rt.SetEpoch(upgrade.PreCommitEpoch + miner.PreCommitChallengeDelay + 1)
		// Proof is initially denied because the fault fee has reduced locked funds.
		rt.ExpectAbort(exitcode.ErrInsufficientFunds, func() {
			actor.proveCommitSectorAndConfirm(rt, &upgrade.Info, upgrade.PreCommitEpoch,
				makeProveCommit(upgrade.Info.SectorNumber), proveCommitConf{})
		})
		rt.Reset()

		// Prove the new sector
		actor.addLockedFund(rt, fee)
		newSector := actor.proveCommitSectorAndConfirm(rt, &upgrade.Info, upgrade.PreCommitEpoch,
			makeProveCommit(upgrade.Info.SectorNumber), proveCommitConf{})

		// The old sector's expiration has *not* changed
		oldSectorAgain := actor.getSector(rt, oldSector.SectorNumber)
		assert.Equal(t, oldSector.Expiration, oldSectorAgain.Expiration)

		// Roll forward to PP cron. The faulty old sector pays a fee, but is not terminated.
		completeProvingPeriod(rt, actor, true, []*miner.SectorOnChainInfo{oldSector}, nil)

		// Both sectors remain
		sectors := actor.collectSectors(rt)
		assert.Equal(t, 2, len(sectors))
		assert.Equal(t, oldSector, sectors[oldSector.SectorNumber])
		assert.Equal(t, newSector, sectors[newSector.SectorNumber])
		expirations := actor.collectExpirations(rt)
		assert.Equal(t, 1, len(expirations))
		assert.Equal(t, []uint64{100, 200}, expirations[newSector.Expiration])
	})

	t.Run("invalid proof rejected", func(t *testing.T) {
		actor := newHarness(t, periodOffset)
		rt := builderForHarness(actor).
			WithBalance(bigBalance, big.Zero()).
			Build(t)
		precommitEpoch := periodOffset + 1
		rt.SetEpoch(precommitEpoch)
		actor.constructAndVerify(rt)
		deadline := actor.deadline(rt)

		// Make a good commitment for the proof to target.
		sectorNo := abi.SectorNumber(100)
		precommit := makePreCommit(sectorNo, precommitEpoch-1, deadline.PeriodEnd(), nil)
		actor.preCommitSector(rt, precommit)

		// Sector pre-commitment missing.
		rt.SetEpoch(precommitEpoch + miner.PreCommitChallengeDelay + 1)
		rt.ExpectAbort(exitcode.ErrNotFound, func() {
			actor.proveCommitSectorAndConfirm(rt, precommit, precommitEpoch, makeProveCommit(sectorNo+1), proveCommitConf{})
		})
		rt.Reset()

		// Too late.
		rt.SetEpoch(precommitEpoch + miner.MaxSealDuration[precommit.SealProof] + 1)
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			actor.proveCommitSectorAndConfirm(rt, precommit, precommitEpoch, makeProveCommit(sectorNo), proveCommitConf{})
		})
		rt.Reset()

		// TODO: too early to prove sector
		// TODO: seal rand epoch too old
		// TODO: commitment expires before proof
		// https://github.com/filecoin-project/specs-actors/issues/479

		// Set the right epoch for all following tests
		rt.SetEpoch(precommitEpoch + miner.PreCommitChallengeDelay + 1)

		// Invalid deals (market ActivateDeals aborts)
		verifyDealsExit := make(map[abi.SectorNumber]exitcode.ExitCode)
		verifyDealsExit[precommit.SectorNumber] = exitcode.ErrIllegalArgument
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			actor.proveCommitSectorAndConfirm(rt, precommit, precommitEpoch, makeProveCommit(sectorNo), proveCommitConf{
				verifyDealsExit: verifyDealsExit,
			})
		})
		rt.Reset()

		// Invalid seal proof
		/* TODO: how should this test work?
		// https://github.com/filecoin-project/specs-actors/issues/479
		rt.ExpectAbort(exitcode.ErrIllegalState, func() {
			actor.proveCommitSectorAndConfirm(rt, precommit, precommitEpoch, makeProveCommit(sectorNo), proveCommitConf{
				verifySealErr: fmt.Errorf("for testing"),
			})
		})
		rt.Reset()
		*/

		// Good proof
		rt.SetBalance(big.Mul(big.NewInt(1000), big.NewInt(1e18)))
		actor.proveCommitSectorAndConfirm(rt, precommit, precommitEpoch, makeProveCommit(sectorNo), proveCommitConf{})
		st := getState(rt)
		// Verify new sectors
		newSectors, err := st.NewSectors.All(miner.SectorsMax)
		require.NoError(t, err)
		assert.Equal(t, []uint64{uint64(sectorNo)}, newSectors)
		// Verify pledge lock-up
		assert.True(t, st.LockedFunds.GreaterThan(big.Zero()))
		rt.Reset()

		// Duplicate proof (sector no-longer pre-committed)
		rt.ExpectAbort(exitcode.ErrNotFound, func() {
			actor.proveCommitSectorAndConfirm(rt, precommit, precommitEpoch, makeProveCommit(sectorNo), proveCommitConf{})
		})
		rt.Reset()
	})
}

func TestWindowPost(t *testing.T) {
	periodOffset := abi.ChainEpoch(100)
	actor := newHarness(t, periodOffset)
	precommitEpoch := abi.ChainEpoch(1)
	builder := builderForHarness(actor).
		WithEpoch(precommitEpoch).
		WithBalance(bigBalance, big.Zero())

	t.Run("test proof", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)
		store := rt.AdtStore()
		_ = actor.commitAndProveSectors(rt, 1, 100, nil)

		// Skip to end of proving period, cron adds sectors to proving set.
		actor.advancePastProvingPeriodWithCron(rt)
		st := getState(rt)

		// Iterate deadlines in the proving period, setting epoch to the first in each deadline.
		// Submit a window post for all partitions due at each deadline when necessary.
		deadline := actor.deadline(rt)
		for !deadline.PeriodElapsed() {
			st = getState(rt)
			deadlines, err := st.LoadDeadlines(store)
			require.NoError(t, err)

			infos, partitions := actor.computePartitions(rt, deadlines, deadline.Index)
			if len(infos) > 0 {
				actor.submitWindowPoSt(rt, deadline, partitions, infos, nil)
			}

			rt.SetEpoch(deadline.Close + 1)
			deadline = actor.deadline(rt)
		}

		// Oops, went one epoch too far, rewind to last epoch of last deadline window for the cron.
		rt.SetEpoch(rt.Epoch() - 1)

		empty, err := st.PostSubmissions.IsEmpty()
		require.NoError(t, err)
		assert.False(t, empty, "no post submission")
	})

	runTillFirstDeadline := func(rt *mock.Runtime) (*miner.DeadlineInfo, []*miner.SectorOnChainInfo, []uint64) {
		actor.constructAndVerify(rt)

		_ = actor.commitAndProveSectors(rt, 4, 100, nil)

		// Skip to end of proving period, cron adds sectors to proving set.
		actor.advancePastProvingPeriodWithCron(rt)
		st := getState(rt)

		deadlines, err := st.LoadDeadlines(rt.AdtStore())
		require.NoError(t, err)
		deadline := actor.deadline(rt)

		// advance to next dealine where we expect the first sectors to appear
		rt.SetEpoch(deadline.Close + 1)
		deadline = st.DeadlineInfo(rt.Epoch())

		infos, partitions := actor.computePartitions(rt, deadlines, deadline.Index)
		return deadline, infos, partitions
	}

	t.Run("successful recoveries recover power", func(t *testing.T) {
		rt := builder.Build(t)
		deadline, infos, partitions := runTillFirstDeadline(rt)
		st := getState(rt)

		// mark all sectors as recovered faults
		sectors := bitfield.New()
		for _, info := range infos {
			sectors.Set(uint64(info.SectorNumber))
		}
		err := st.AddFaults(rt.AdtStore(), &sectors, rt.Epoch())
		require.NoError(t, err)
		err = st.AddRecoveries(&sectors)
		require.NoError(t, err)
		rt.ReplaceState(st)

		rawPower, qaPower := miner.PowerForSectors(actor.sectorSize, infos)

		cfg := &poStConfig{
			expectedRawPowerDelta: rawPower,
			expectedQAPowerDelta:  qaPower,
			expectedPenalty:       big.Zero(),
			skipped:               bitfield.NewFromSet(nil),
		}

		actor.submitWindowPoSt(rt, deadline, partitions, infos, cfg)
	})

	t.Run("skipped faults are penalized and adjust power adjusted", func(t *testing.T) {
		rt := builder.Build(t)
		deadline, infos, partitions := runTillFirstDeadline(rt)

		// skip the first sector in the partition
		skipped := bitfield.NewFromSet([]uint64{uint64(infos[0].SectorNumber)})

		rawPower, qaPower := miner.PowerForSectors(actor.sectorSize, infos[:1])

		// expected penalty is the fee for an undeclared fault
		expectedPenalty := miner.PledgePenaltyForUndeclaredFault(actor.epochReward, actor.networkQAPower, qaPower)

		cfg := &poStConfig{
			skipped:               skipped,
			expectedRawPowerDelta: rawPower.Neg(),
			expectedQAPowerDelta:  qaPower.Neg(),
			expectedPenalty:       expectedPenalty,
		}

		actor.submitWindowPoSt(rt, deadline, partitions, infos, cfg)
	})

	t.Run("skipped recoveries are penalized and do not recover power", func(t *testing.T) {
		rt := builder.Build(t)
		deadline, infos, partitions := runTillFirstDeadline(rt)
		st := getState(rt)

		// mark all sectors as recovered faults
		sectors := bitfield.NewFromSet([]uint64{uint64(infos[0].SectorNumber)})
		err := st.AddFaults(rt.AdtStore(), sectors, rt.Epoch())
		require.NoError(t, err)
		err = st.AddRecoveries(sectors)
		require.NoError(t, err)
		rt.ReplaceState(st)

		_, qaPower := miner.PowerForSectors(actor.sectorSize, infos[:1])

		// skip the first sector in the partition
		skipped := bitfield.NewFromSet([]uint64{uint64(infos[0].SectorNumber)})
		// expected penalty is the fee for an undeclared fault
		expectedPenalty := miner.PledgePenaltyForUndeclaredFault(actor.epochReward, actor.networkQAPower, qaPower)

		cfg := &poStConfig{
			expectedRawPowerDelta: big.Zero(),
			expectedQAPowerDelta:  big.Zero(),
			expectedPenalty:       expectedPenalty,
			skipped:               skipped,
		}

		actor.submitWindowPoSt(rt, deadline, partitions, infos, cfg)
	})

	t.Run("skipping a fault from the wrong deadline is an error", func(t *testing.T) {
		rt := builder.Build(t)
		deadline, infos, partitions := runTillFirstDeadline(rt)
		st := getState(rt)

		// look ahead to next deadline to find a sector not in this deadline
		deadlines, err := st.LoadDeadlines(rt.AdtStore())
		require.NoError(t, err)
		nextDeadline := st.DeadlineInfo(deadline.Close + 1)
		nextInfos, _ := actor.computePartitions(rt, deadlines, nextDeadline.Index)

		_, qaPower := miner.PowerForSectors(actor.sectorSize, nextInfos[:1])

		// skip the first sector in the partition
		skipped := bitfield.NewFromSet([]uint64{uint64(nextInfos[0].SectorNumber)})
		// expected penalty is the fee for an undeclared fault
		expectedPenalty := miner.PledgePenaltyForUndeclaredFault(actor.epochReward, actor.networkQAPower, qaPower)

		cfg := &poStConfig{
			expectedRawPowerDelta: big.Zero(),
			expectedQAPowerDelta:  big.Zero(),
			expectedPenalty:       expectedPenalty,
			skipped:               skipped,
		}

		rt.ExpectAbortConstainsMessage(exitcode.ErrIllegalArgument, "skipped faults contains sectors not due in deadline", func() {
			actor.submitWindowPoSt(rt, deadline, partitions, infos, cfg)
		})
	})
}

func TestProveCommit(t *testing.T) {
	periodOffset := abi.ChainEpoch(100)
	actor := newHarness(t, periodOffset)
	builder := builderForHarness(actor).
		WithBalance(bigBalance, big.Zero())

	t.Run("aborts if sum of initial pledges exceeds locked funds", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		// prove one sector to establish collateral and locked funds
		actor.commitAndProveSectors(rt, 1, 100, nil)

		// preecommit another sector so we may prove it
		expiration := 100*miner.WPoStProvingPeriod + periodOffset - 1
		precommitEpoch := rt.Epoch() + 1
		rt.SetEpoch(precommitEpoch)
		precommit := makePreCommit(actor.nextSectorNo, rt.Epoch()-1, expiration, nil)
		actor.preCommitSector(rt, precommit)

		// alter lock funds to simulate vesting since last prove
		st := getState(rt)
		st.LockedFunds = big.Div(st.LockedFunds, big.NewInt(2))
		rt.ReplaceState(st)
		info := actor.getInfo(rt)

		rt.SetEpoch(precommitEpoch + miner.MaxSealDuration[info.SealProofType] - 1)
		rt.ExpectAbort(exitcode.ErrInsufficientFunds, func() {
			actor.proveCommitSectorAndConfirm(rt, precommit, precommitEpoch, makeProveCommit(actor.nextSectorNo), proveCommitConf{})
		})
		rt.Reset()

		// succeeds when locked fund satisfy initial pledge requirement
		st.LockedFunds = st.InitialPledgeRequirement
		rt.ReplaceState(st)
		actor.proveCommitSectorAndConfirm(rt, precommit, precommitEpoch, makeProveCommit(actor.nextSectorNo), proveCommitConf{})
	})

	t.Run ("drop invalid prove commit while processing valid one", func (t *testing.T) { 
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		// make two precommits
		expiration := 100*miner.WPoStProvingPeriod + periodOffset - 1
		precommitEpoch := rt.Epoch() + 1
		rt.SetEpoch(precommitEpoch)
		precommitA := makePreCommit(actor.nextSectorNo, rt.Epoch()-1, expiration, nil)
		actor.preCommitSector(rt, precommitA)
		sectorNoA := actor.nextSectorNo
		actor.nextSectorNo++
		precommitB := makePreCommit(actor.nextSectorNo, rt.Epoch()-1, expiration, nil)
		actor.preCommitSector(rt, precommitB)
		sectorNoB := actor.nextSectorNo

		// handle both prove commits in the same epoch 
		info := actor.getInfo(rt)
		rt.SetEpoch(precommitEpoch + miner.MaxSealDuration[info.SealProofType] - 1)

		actor.proveCommitSector(rt, precommitA, precommitEpoch, makeProveCommit(sectorNoA))
		actor.proveCommitSector(rt, precommitB, precommitEpoch, makeProveCommit(sectorNoB))

		conf := proveCommitConf {
			verifyDealsExit: map[abi.SectorNumber]exitcode.ExitCode{ 
				sectorNoA: exitcode.ErrIllegalArgument,
			},
		}
		actor.confirmSectorProofsValid(rt, conf, precommitEpoch, precommitA, precommitB)
	})
}

func TestProvingPeriodCron(t *testing.T) {
	periodOffset := abi.ChainEpoch(100)
	actor := newHarness(t, periodOffset)
	builder := builderForHarness(actor).
		WithBalance(bigBalance, big.Zero())

	t.Run("empty periods", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)
		st := getState(rt)
		assert.Equal(t, periodOffset, st.ProvingPeriodStart)

		// First cron invocation just before the first proving period starts.
		rt.SetEpoch(periodOffset - 1)
		secondCronEpoch := periodOffset + miner.WPoStProvingPeriod - 1
		actor.onProvingPeriodCron(rt, secondCronEpoch, false, nil, nil)
		// The proving period start isn't changed, because the period hadn't started yet.
		st = getState(rt)
		assert.Equal(t, periodOffset, st.ProvingPeriodStart)

		rt.SetEpoch(secondCronEpoch)
		actor.onProvingPeriodCron(rt, periodOffset+2*miner.WPoStProvingPeriod-1, false, nil, nil)
		// Proving period moves forward
		st = getState(rt)
		assert.Equal(t, periodOffset+miner.WPoStProvingPeriod, st.ProvingPeriodStart)
	})

	t.Run("first period gets randomness from previous epoch", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)
		st := getState(rt)

		sectorInfo := actor.commitAndProveSectors(rt, 1, 100, nil)

		// Flag new sectors to trigger request for randomness
		rt.Transaction(st, func() interface{} {
			st.NewSectors.Set(uint64(sectorInfo[0].SectorNumber))
			return nil
		})

		// First cron invocation just before the first proving period starts
		// requires randomness come from current epoch minus lookback
		rt.SetEpoch(periodOffset - 1)
		secondCronEpoch := periodOffset + miner.WPoStProvingPeriod - 1
		actor.onProvingPeriodCron(rt, secondCronEpoch, true, nil, nil)

		// cron invocation after the proving period starts, requires randomness come from end of proving period
		rt.SetEpoch(periodOffset)
		actor.advanceProvingPeriodWithoutFaults(rt)

		// triggers a new request for randomness
		rt.Transaction(st, func() interface{} {
			st.NewSectors.Set(uint64(sectorInfo[0].SectorNumber))
			return nil
		})

		thirdCronEpoch := secondCronEpoch + miner.WPoStProvingPeriod
		actor.onProvingPeriodCron(rt, thirdCronEpoch, true, nil, nil)
	})

	// TODO: test cron being called one epoch late because the scheduled epoch had no blocks.
}

func TestDeclareFaults(t *testing.T) {
	periodOffset := abi.ChainEpoch(100)
	actor := newHarness(t, periodOffset)
	builder := builderForHarness(actor).
		WithBalance(bigBalance, big.Zero())

	t.Run("declare fault pays fee", func(t *testing.T) {
		// Get sector into proving state
		rt := builder.Build(t)
		actor.constructAndVerify(rt)
		precommits := actor.commitAndProveSectors(rt, 1, 100, nil)

		// Skip to end of proving period, cron adds sectors to proving set.
		completeProvingPeriod(rt, actor, true, nil, nil)
		info := actor.getSector(rt, precommits[0].SectorNumber)

		// Declare the sector as faulted
		ss, err := info.SealProof.SectorSize()
		require.NoError(t, err)
		sectorQAPower := miner.QAPowerForSector(ss, info)
		totalQAPower := big.NewInt(1 << 52)
		fee := miner.PledgePenaltyForDeclaredFault(actor.epochReward, totalQAPower, sectorQAPower)

		actor.declareFaults(rt, totalQAPower, fee, info)
	})
}

func TestExtendSectorExpiration(t *testing.T) {
	periodOffset := abi.ChainEpoch(100)
	actor := newHarness(t, periodOffset)
	precommitEpoch := abi.ChainEpoch(1)
	builder := builderForHarness(actor).
		WithEpoch(precommitEpoch).
		WithBalance(bigBalance, big.Zero())

	commitSector := func(t *testing.T, rt *mock.Runtime) *miner.SectorOnChainInfo {
		actor.constructAndVerify(rt)
		sectorInfo := actor.commitAndProveSectors(rt, 1, 100, nil)
		return sectorInfo[0]
	}

	t.Run("rejects negative extension", func(t *testing.T) {
		rt := builder.Build(t)
		sector := commitSector(t, rt)
		// attempt to shorten epoch
		newExpiration := sector.Expiration - abi.ChainEpoch(miner.WPoStProvingPeriod)
		params := &miner.ExtendSectorExpirationParams{
			SectorNumber:  sector.SectorNumber,
			NewExpiration: newExpiration,
		}

		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			actor.extendSector(rt, sector, 0, params)
		})
	})

	t.Run("rejects extension to invalid epoch", func(t *testing.T) {
		rt := builder.Build(t)
		sector := commitSector(t, rt)

		// attempt to extend to an epoch that is not a multiple of the proving period + the commit epoch
		extension := 42*miner.WPoStProvingPeriod + 1
		newExpiration := sector.Expiration - abi.ChainEpoch(extension)
		params := &miner.ExtendSectorExpirationParams{
			SectorNumber:  sector.SectorNumber,
			NewExpiration: newExpiration,
		}

		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			actor.extendSector(rt, sector, extension, params)
		})
	})

	t.Run("rejects extension too far in future", func(t *testing.T) {
		rt := builder.Build(t)
		sector := commitSector(t, rt)

		// extend by even proving period after max
		rt.SetEpoch(sector.Expiration)
		extension := miner.WPoStProvingPeriod * (miner.MaxSectorExpirationExtension/miner.WPoStProvingPeriod + 1)
		newExpiration := rt.Epoch() + extension
		params := &miner.ExtendSectorExpirationParams{
			SectorNumber:  sector.SectorNumber,
			NewExpiration: newExpiration,
		}

		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			actor.extendSector(rt, sector, extension, params)
		})
	})

	t.Run("rejects extension past max for seal proof", func(t *testing.T) {
		rt := builder.Build(t)
		sector := commitSector(t, rt)
		rt.SetEpoch(sector.Expiration)

		maxLifetime := sector.SealProof.SectorMaximumLifetime()

		// extend sector until just below threshold
		extension := miner.WPoStProvingPeriod * (miner.MaxSectorExpirationExtension/miner.WPoStProvingPeriod - 1)
		expiration := rt.Epoch() + extension
		for ; expiration-sector.Activation < maxLifetime; expiration += extension {
			params := &miner.ExtendSectorExpirationParams{
				SectorNumber:  sector.SectorNumber,
				NewExpiration: expiration,
			}

			actor.extendSector(rt, sector, extension, params)
			rt.SetEpoch(expiration)
		}

		// next extension fails because it extends sector past max lifetime
		params := &miner.ExtendSectorExpirationParams{
			SectorNumber:  sector.SectorNumber,
			NewExpiration: expiration,
		}

		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			actor.extendSector(rt, sector, extension, params)
		})
	})

	t.Run("updates expiration with valid params", func(t *testing.T) {
		rt := builder.Build(t)
		oldSector := commitSector(t, rt)

		extension := 42 * miner.WPoStProvingPeriod
		newExpiration := oldSector.Expiration + extension
		params := &miner.ExtendSectorExpirationParams{
			SectorNumber:  oldSector.SectorNumber,
			NewExpiration: newExpiration,
		}

		actor.extendSector(rt, oldSector, extension, params)

		// assert sector expiration is set to the new value
		st := getState(rt)
		newSector := actor.getSector(rt, oldSector.SectorNumber)
		assert.Equal(t, newExpiration, newSector.Expiration)

		// assert that an expiration exists at the target epoch
		expirations, err := st.GetSectorExpirations(rt.AdtStore(), newExpiration)
		require.NoError(t, err)
		exists, err := expirations.IsSet(uint64(newSector.SectorNumber))
		require.NoError(t, err)
		assert.True(t, exists)

		// assert that the expiration has been removed from the old epoch
		expirations, err = st.GetSectorExpirations(rt.AdtStore(), oldSector.Expiration)
		require.NoError(t, err)
		exists, err = expirations.IsSet(uint64(newSector.SectorNumber))
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestTerminateSectors(t *testing.T) {
	periodOffset := abi.ChainEpoch(100)
	actor := newHarness(t, periodOffset)
	builder := builderForHarness(actor).
		WithBalance(bigBalance, big.Zero())

	commitSector := func(t *testing.T, rt *mock.Runtime) *miner.SectorOnChainInfo {
		actor.constructAndVerify(rt)
		precommitEpoch := abi.ChainEpoch(1)
		rt.SetEpoch(precommitEpoch)
		sectorInfo := actor.commitAndProveSectors(rt, 1, 100, nil)
		return sectorInfo[0]
	}

	t.Run("removes sector with correct accounting", func(t *testing.T) {
		rt := builder.Build(t)
		sector := commitSector(t, rt)

		{
			// Verify that a sector expiration was registered.
			st := getState(rt)
			expiration, err := st.GetSectorExpirations(rt.AdtStore(), sector.Expiration)
			require.NoError(t, err)
			expiringSectorNos, err := expiration.All(1)
			require.NoError(t, err)
			assert.Len(t, expiringSectorNos, 1)
			assert.Equal(t, sector.SectorNumber, abi.SectorNumber(expiringSectorNos[0]))
		}

		sectorSize, err := sector.SealProof.SectorSize()
		require.NoError(t, err)
		sectorPower := miner.QAPowerForSector(sectorSize, sector)
		sectorAge := rt.Epoch() - sector.Activation
		expectedFee := miner.PledgePenaltyForTermination(sector.InitialPledge, sectorAge, actor.epochReward, actor.networkQAPower, sectorPower)

		sectors := bitfield.New()
		sectors.Set(uint64(sector.SectorNumber))
		actor.terminateSectors(rt, &sectors, expectedFee)

		{
			st := getState(rt)

			// expect sector expiration to have been removed
			err = st.ForEachSectorExpiration(rt.AdtStore(), func(expiry abi.ChainEpoch, sectors *abi.BitField) error {
				assert.Fail(t, "did not expect to find a sector expiration, found expiration at %s", expiry)
				return nil
			})
			assert.NoError(t, err)

			// expect sector to have been removed
			_, found, err := st.GetSector(rt.AdtStore(), sector.SectorNumber)
			require.NoError(t, err)
			assert.False(t, found)

			// expect pledge requirement to have been decremented
			assert.Equal(t, big.Zero(), st.InitialPledgeRequirement)
		}
	})
}

func TestWithdrawBalance(t *testing.T) {
	periodOffset := abi.ChainEpoch(100)
	actor := newHarness(t, periodOffset)
	builder := builderForHarness(actor).
		WithBalance(bigBalance, big.Zero())

	t.Run("happy path withdraws funds", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		// withdraw 1% of balance
		actor.withdrawFunds(rt, big.Mul(big.NewInt(10), big.NewInt(1e18)))
	})

	t.Run("fails if miner is currently undercollateralized", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		// prove one sector to establish collateral and locked funds
		actor.commitAndProveSectors(rt, 1, 100, nil)

		// alter lock funds to simulate vesting since last prove
		st := getState(rt)
		st.LockedFunds = big.Div(st.LockedFunds, big.NewInt(2))
		rt.ReplaceState(st)

		// withdraw 1% of balance
		rt.ExpectAbort(exitcode.ErrInsufficientFunds, func() {
			actor.withdrawFunds(rt, big.Mul(big.NewInt(10), big.NewInt(1e18)))
		})
	})
}

func TestReportConsensusFault(t *testing.T) {
	periodOffset := abi.ChainEpoch(100)
	actor := newHarness(t, periodOffset)
	builder := builderForHarness(actor).
		WithBalance(bigBalance, big.Zero())

	rt := builder.Build(t)
	actor.constructAndVerify(rt)
	precommitEpoch := abi.ChainEpoch(1)
	rt.SetEpoch(precommitEpoch)
	dealIDs := [][]abi.DealID{{1, 2}, {3, 4}}
	sectorInfo := actor.commitAndProveSectors(rt, 2, 10, dealIDs)
	_ = sectorInfo

	params := &miner.ReportConsensusFaultParams{
		BlockHeader1:     nil,
		BlockHeader2:     nil,
		BlockHeaderExtra: nil,
	}

	// miner should send a single call to terminate the deals for all its sectors
	allDeals := []abi.DealID{}
	for _, ids := range dealIDs {
		allDeals = append(allDeals, ids...)
	}
	actor.reportConsensusFault(rt, addr.TestAddress, params, allDeals)
}
func TestAddLockedFund(t *testing.T) {

	periodOffset := abi.ChainEpoch(1808)
	actor := newHarness(t, periodOffset)

	builder := builderForHarness(actor).
		WithBalance(bigBalance, big.Zero())

	t.Run("funds vest", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)
		st := getState(rt)
		store := rt.AdtStore()

		// Nothing vesting to start
		vestingFunds, err := adt.AsArray(store, st.VestingFunds)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), vestingFunds.Length())
		assert.Equal(t, big.Zero(), st.LockedFunds)

		// Lock some funds with AddLockedFund
		amt := abi.NewTokenAmount(600_000)
		actor.addLockedFund(rt, amt)
		st = getState(rt)
		newVestingFunds, err := adt.AsArray(store, st.VestingFunds)
		require.NoError(t, err)
		require.Equal(t, uint64(7), newVestingFunds.Length()) // 1 day steps over 1 week

		// Vested FIL pays out on epochs with expected offset
		lockedEntry := abi.NewTokenAmount(0)
		expectedOffset := periodOffset % miner.PledgeVestingSpec.Quantization
		err = newVestingFunds.ForEach(&lockedEntry, func(k int64) error {
			assert.Equal(t, int64(expectedOffset), k%int64(miner.PledgeVestingSpec.Quantization))
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, amt, st.LockedFunds)

	})

}

type actorHarness struct {
	a miner.Actor
	t testing.TB

	receiver addr.Address // The miner actor's own address
	owner    addr.Address
	worker   addr.Address
	key      addr.Address

	sealProofType abi.RegisteredSealProof
	sectorSize    abi.SectorSize
	partitionSize uint64
	periodOffset  abi.ChainEpoch
	nextSectorNo  abi.SectorNumber

	epochReward     abi.TokenAmount
	networkPledge   abi.TokenAmount
	networkRawPower abi.StoragePower
	networkQAPower  abi.StoragePower
}

func newHarness(t testing.TB, provingPeriodOffset abi.ChainEpoch) *actorHarness {
	sealProofType := abi.RegisteredSealProof_StackedDrg2KiBV1
	sectorSize, err := sealProofType.SectorSize()
	require.NoError(t, err)
	partitionSectors, err := sealProofType.WindowPoStPartitionSectors()
	require.NoError(t, err)
	owner := tutil.NewIDAddr(t, 100)
	worker := tutil.NewIDAddr(t, 101)
	workerKey := tutil.NewBLSAddr(t, 0)
	receiver := tutil.NewIDAddr(t, 1000)
	reward := big.Mul(big.NewIntUnsigned(100), big.NewIntUnsigned(1e18))
	return &actorHarness{
		t:        t,
		receiver: receiver,
		owner:    owner,
		worker:   worker,
		key:      workerKey,

		sealProofType: sealProofType,
		sectorSize:    sectorSize,
		partitionSize: partitionSectors,
		periodOffset:  provingPeriodOffset,
		nextSectorNo:  100,

		epochReward:     reward,
		networkPledge:   big.Mul(reward, big.NewIntUnsigned(1000)),
		networkRawPower: abi.NewStoragePower(1 << 50),
		networkQAPower:  abi.NewStoragePower(1 << 50),
	}
}

func (h *actorHarness) constructAndVerify(rt *mock.Runtime) {
	params := miner.ConstructorParams{
		OwnerAddr:     h.owner,
		WorkerAddr:    h.worker,
		SealProofType: h.sealProofType,
		PeerId:        testPid,
	}

	rt.ExpectValidateCallerAddr(builtin.InitActorAddr)
	// Fetch worker pubkey.
	rt.ExpectSend(h.worker, builtin.MethodsAccount.PubkeyAddress, nil, big.Zero(), &h.key, exitcode.Ok)
	// Register proving period cron.
	nextProvingPeriodEnd := h.periodOffset - 1
	for nextProvingPeriodEnd < rt.Epoch() {
		nextProvingPeriodEnd += miner.WPoStProvingPeriod
	}
	rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.EnrollCronEvent,
		makeProvingPeriodCronEventParams(h.t, nextProvingPeriodEnd), big.Zero(), nil, exitcode.Ok)
	rt.SetCaller(builtin.InitActorAddr, builtin.InitActorCodeID)
	ret := rt.Call(h.a.Constructor, &params)
	assert.Nil(h.t, ret)
	rt.Verify()
}

//
// State access helpers
//

func (h *actorHarness) deadline(rt *mock.Runtime) *miner.DeadlineInfo {
	st := getState(rt)
	return st.DeadlineInfo(rt.Epoch())
}

func (h *actorHarness) getPreCommit(rt *mock.Runtime, sno abi.SectorNumber) *miner.SectorPreCommitOnChainInfo {
	st := getState(rt)
	pc, found, err := st.GetPrecommittedSector(rt.AdtStore(), sno)
	require.NoError(h.t, err)
	require.True(h.t, found)
	return pc
}

func (h *actorHarness) getSector(rt *mock.Runtime, sno abi.SectorNumber) *miner.SectorOnChainInfo {
	st := getState(rt)
	sector, found, err := st.GetSector(rt.AdtStore(), sno)
	require.NoError(h.t, err)
	require.True(h.t, found)
	return sector
}

func (h *actorHarness) getInfo(rt *mock.Runtime) *miner.MinerInfo {
	var st miner.State
	rt.GetState(&st)
	info, err := st.GetInfo(adt.AsStore(rt))
	require.NoError(h.t, err)
	return info
}

// Collects all sector infos into a map.
func (h *actorHarness) collectSectors(rt *mock.Runtime) map[abi.SectorNumber]*miner.SectorOnChainInfo {
	sectors := map[abi.SectorNumber]*miner.SectorOnChainInfo{}
	st := getState(rt)
	_ = st.ForEachSector(rt.AdtStore(), func(info *miner.SectorOnChainInfo) {
		sector := *info
		sectors[info.SectorNumber] = &sector
	})
	return sectors
}

// Collects the sector numbers of all sectors assigned to a deadline.
func (h *actorHarness) collectProvingSet(rt *mock.Runtime) map[uint64]struct{} {
	pset := map[uint64]struct{}{}
	st := getState(rt)
	deadlines, err := st.LoadDeadlines(rt.AdtStore())
	require.NoError(h.t, err)
	for _, d := range deadlines.Due {
		_ = d.ForEach(func(n uint64) error {
			pset[n] = struct{}{}
			return nil
		})
	}
	return pset
}

// Collects all expirations into a map.
func (h *actorHarness) collectExpirations(rt *mock.Runtime) map[abi.ChainEpoch][]uint64 {
	expirations := map[abi.ChainEpoch][]uint64{}
	st := getState(rt)
	_ = st.ForEachSectorExpiration(rt.AdtStore(), func(expiry abi.ChainEpoch, sectors *abi.BitField) error {
		expanded, err := sectors.All(miner.SectorsMax)
		require.NoError(h.t, err)
		expirations[expiry] = expanded
		return nil
	})
	return expirations
}

//
// Actor method calls
//

func (h *actorHarness) controlAddresses(rt *mock.Runtime) (owner, worker addr.Address) {
	rt.ExpectValidateCallerAny()
	ret := rt.Call(h.a.ControlAddresses, nil).(*miner.GetControlAddressesReturn)
	require.NotNil(h.t, ret)
	rt.Verify()
	return ret.Owner, ret.Worker
}

func (h *actorHarness) preCommitSector(rt *mock.Runtime, params *miner.SectorPreCommitInfo) *miner.SectorPreCommitOnChainInfo {

	rt.SetCaller(h.worker, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(h.worker)

	{
		pwrTotal := &power.CurrentTotalPowerReturn{
			RawBytePower:     h.networkRawPower,
			QualityAdjPower:  h.networkQAPower,
			PledgeCollateral: h.networkPledge,
		}
		expectQueryNetworkInfo(rt, pwrTotal, h.epochReward)
	}
	{
		sectorSize, err := params.SealProof.SectorSize()
		require.NoError(h.t, err)

		vdParams := market.VerifyDealsForActivationParams{
			DealIDs:      params.DealIDs,
			SectorStart:  rt.Epoch(),
			SectorExpiry: params.Expiration,
		}

		vdReturn := market.VerifyDealsForActivationReturn{
			DealWeight:         big.NewInt(int64(sectorSize / 2)),
			VerifiedDealWeight: big.NewInt(int64(sectorSize / 2)),
		}
		rt.ExpectSend(builtin.StorageMarketActorAddr, builtin.MethodsMarket.VerifyDealsForActivation, &vdParams, big.Zero(), &vdReturn, exitcode.Ok)
	}
	{
		eventPayload := miner.CronEventPayload{
			EventType: miner.CronEventPreCommitExpiry,
			Sectors:   bitfield.NewFromSet([]uint64{uint64(params.SectorNumber)}),
		}
		buf := bytes.Buffer{}
		err := eventPayload.MarshalCBOR(&buf)
		require.NoError(h.t, err)
		cronParams := power.EnrollCronEventParams{
			EventEpoch: rt.Epoch() + miner.MaxSealDuration[params.SealProof] + 1,
			Payload:    buf.Bytes(),
		}
		rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.EnrollCronEvent, &cronParams, big.Zero(), nil, exitcode.Ok)
	}

	rt.Call(h.a.PreCommitSector, params)
	rt.Verify()
	return h.getPreCommit(rt, params.SectorNumber)
}

// Options for proveCommitSector behaviour.
// Default zero values should let everything be ok.
type proveCommitConf struct {
	verifyDealsExit map[abi.SectorNumber]exitcode.ExitCode
}

func (h *actorHarness) proveCommitSector(rt *mock.Runtime, precommit *miner.SectorPreCommitInfo, precommitEpoch abi.ChainEpoch,
	params *miner.ProveCommitSectorParams) {
		commd := cbg.CborCid(tutil.MakeCID("commd"))
		sealRand := abi.SealRandomness([]byte{1, 2, 3, 4})
		sealIntRand := abi.InteractiveSealRandomness([]byte{5, 6, 7, 8})
		interactiveEpoch := precommitEpoch + miner.PreCommitChallengeDelay
	
		// Prepare for and receive call to ProveCommitSector
		{
			cdcParams := market.ComputeDataCommitmentParams{
				DealIDs:    precommit.DealIDs,
				SectorType: precommit.SealProof,
			}
			rt.ExpectSend(builtin.StorageMarketActorAddr, builtin.MethodsMarket.ComputeDataCommitment, &cdcParams, big.Zero(), &commd, exitcode.Ok)
		}
		{
			var buf bytes.Buffer
			err := rt.Receiver().MarshalCBOR(&buf)
			require.NoError(h.t, err)
			rt.ExpectGetRandomness(crypto.DomainSeparationTag_SealRandomness, precommit.SealRandEpoch, buf.Bytes(), abi.Randomness(sealRand))
			rt.ExpectGetRandomness(crypto.DomainSeparationTag_InteractiveSealChallengeSeed, interactiveEpoch, buf.Bytes(), abi.Randomness(sealIntRand))
		}
		{
			actorId, err := addr.IDFromAddress(h.receiver)
			require.NoError(h.t, err)
			seal := abi.SealVerifyInfo{
				SectorID: abi.SectorID{
					Miner:  abi.ActorID(actorId),
					Number: precommit.SectorNumber,
				},
				SealedCID:             precommit.SealedCID,
				SealProof:             precommit.SealProof,
				Proof:                 params.Proof,
				DealIDs:               precommit.DealIDs,
				Randomness:            sealRand,
				InteractiveRandomness: sealIntRand,
				UnsealedCID:           cid.Cid(commd),
			}
			rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.SubmitPoRepForBulkVerify, &seal, abi.NewTokenAmount(0), nil, exitcode.Ok)
		}
		rt.SetCaller(h.worker, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAny()
		rt.Call(h.a.ProveCommitSector, params)
		rt.Verify()
}

func (h *actorHarness) confirmSectorProofsValid(rt *mock.Runtime, conf proveCommitConf, precommitEpoch abi.ChainEpoch, precommits ...*miner.SectorPreCommitInfo) {
	// Prepare for and receive call to ConfirmSectorProofsValid.
	var validPrecommits []*miner.SectorPreCommitInfo
	var allSectorNumbers []abi.SectorNumber
	for _, precommit := range precommits {
		allSectorNumbers = append(allSectorNumbers, precommit.SectorNumber)

		vdParams := market.ActivateDealsParams{
			DealIDs:      precommit.DealIDs,
			SectorExpiry: precommit.Expiration,
		}
		exit, found := conf.verifyDealsExit[precommit.SectorNumber]
		if !found {
			exit = exitcode.Ok
			validPrecommits = append(validPrecommits, precommit)
		}
		rt.ExpectSend(builtin.StorageMarketActorAddr, builtin.MethodsMarket.ActivateDeals, &vdParams, big.Zero(), nil, exit)
	}

	// expected pledge is the sum of precommit deposits
	if len(validPrecommits) > 0 {
		expectPledge := big.Zero()
		
		expectQAPower := big.Zero()
		expectRawPower := big.Zero()
		for _, precommit := range validPrecommits {
			precommitOnChain := h.getPreCommit(rt, precommit.SectorNumber)

			qaPowerDelta := miner.QAPowerForWeight(h.sectorSize, precommit.Expiration-precommitEpoch, precommitOnChain.DealWeight, precommitOnChain.VerifiedDealWeight)
			expectQAPower = big.Add(expectQAPower, qaPowerDelta)
			expectRawPower = big.Add(expectRawPower, big.NewIntUnsigned(uint64(h.sectorSize)))

			expectPledge = big.Add(expectPledge, precommitOnChain.PreCommitDeposit)
		}

		pcParams := power.UpdateClaimedPowerParams{
			RawByteDelta:         expectRawPower,
			QualityAdjustedDelta: expectQAPower,
		}
		rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.UpdateClaimedPower, &pcParams, big.Zero(), nil, exitcode.Ok)
		rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.UpdatePledgeTotal, &expectPledge, big.Zero(), nil, exitcode.Ok)
	}

	rt.SetCaller(builtin.StoragePowerActorAddr, builtin.StoragePowerActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.StoragePowerActorAddr)
	rt.Call(h.a.ConfirmSectorProofsValid, &builtin.ConfirmSectorProofsParams{Sectors: allSectorNumbers})
	rt.Verify()
}

func (h *actorHarness) proveCommitSectorAndConfirm(rt *mock.Runtime, precommit *miner.SectorPreCommitInfo, precommitEpoch abi.ChainEpoch,
	params *miner.ProveCommitSectorParams, conf proveCommitConf) *miner.SectorOnChainInfo {
	h.proveCommitSector(rt, precommit, precommitEpoch, params)
	h.confirmSectorProofsValid(rt, conf, precommitEpoch, precommit)

	newSector := h.getSector(rt, params.SectorNumber)
	return newSector
}

// Pre-commits and then proves a number of sectors.
// The sectors will expire at the end of  lifetimePeriods proving periods after now.
// The runtime epoch will be moved forward to the epoch of commitment proofs.
func (h *actorHarness) commitAndProveSectors(rt *mock.Runtime, n int, lifetimePeriods uint64, dealIDs [][]abi.DealID) []*miner.SectorOnChainInfo {
	precommitEpoch := rt.Epoch()
	deadline := h.deadline(rt)
	expiration := deadline.PeriodEnd() + abi.ChainEpoch(lifetimePeriods)*miner.WPoStProvingPeriod

	// Precommit
	precommits := make([]*miner.SectorPreCommitInfo, n)
	for i := 0; i < n; i++ {
		sectorNo := h.nextSectorNo
		var sectorDealIDs []abi.DealID
		if dealIDs != nil {
			sectorDealIDs = dealIDs[i]
		}
		precommit := makePreCommit(sectorNo, precommitEpoch-1, expiration, sectorDealIDs)
		h.preCommitSector(rt, precommit)
		precommits[i] = precommit
		h.nextSectorNo++
	}

	rt.SetEpoch(precommitEpoch + miner.PreCommitChallengeDelay + 1)

	// Ensure this this doesn't cross a proving period boundary, else the expected cron call won't be
	// invoked, which might mess things up later.
	deadline = h.deadline(rt)
	require.True(h.t, !deadline.PeriodElapsed())

	info := []*miner.SectorOnChainInfo{}
	for _, pc := range precommits {
		sector := h.proveCommitSectorAndConfirm(rt, pc, precommitEpoch, makeProveCommit(pc.SectorNumber), proveCommitConf{})
		info = append(info, sector)
	}
	rt.Reset()
	return info
}

func (h *actorHarness) advancePastProvingPeriodWithCron(rt *mock.Runtime) {
	st := getState(rt)
	deadline := st.DeadlineInfo(rt.Epoch())
	rt.SetEpoch(deadline.PeriodEnd())
	nextCron := deadline.NextPeriodStart() + miner.WPoStProvingPeriod - 1
	h.onProvingPeriodCron(rt, nextCron, true, nil, nil)
	rt.SetEpoch(deadline.NextPeriodStart())
}

type poStConfig struct {
	skipped               *bitfield.BitField
	expectedRawPowerDelta abi.StoragePower
	expectedQAPowerDelta  abi.StoragePower
	expectedPenalty       abi.TokenAmount
}

func (h *actorHarness) submitWindowPoSt(rt *mock.Runtime, deadline *miner.DeadlineInfo, partitions []uint64, infos []*miner.SectorOnChainInfo, poStCfg *poStConfig) {
	rt.SetCaller(h.worker, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(h.worker)

	rt.ExpectSend(builtin.RewardActorAddr, builtin.MethodsReward.ThisEpochReward, nil, big.Zero(), &h.epochReward, exitcode.Ok)

	pwrTotal := power.CurrentTotalPowerReturn{
		QualityAdjPower: h.networkQAPower,
	}
	rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.CurrentTotalPower, nil, big.Zero(), &pwrTotal, exitcode.Ok)

	var registeredPoStProof, err = abi.RegisteredSealProof_StackedDrg2KiBV1.RegisteredWindowPoStProof()
	require.NoError(h.t, err)

	proofs := make([]abi.PoStProof, 1) // Number of proofs doesn't depend on partition count
	for i := range proofs {
		proofs[i].PoStProof = registeredPoStProof
		proofs[i].ProofBytes = []byte(fmt.Sprintf("proof%d", i))
	}
	challengeRand := abi.SealRandomness([]byte{10, 11, 12, 13})

	{
		var buf bytes.Buffer
		err := rt.Receiver().MarshalCBOR(&buf)
		require.NoError(h.t, err)

		rt.ExpectGetRandomness(crypto.DomainSeparationTag_WindowedPoStChallengeSeed, deadline.Challenge, buf.Bytes(), abi.Randomness(challengeRand))
	}
	{
		// find the first non-faulty sector in poSt to replace all faulty sectors.
		var goodInfo *miner.SectorOnChainInfo
		if poStCfg != nil {
			for _, ci := range infos {
				contains, err := poStCfg.skipped.IsSet(uint64(ci.SectorNumber))
				require.NoError(h.t, err)
				if !contains {
					goodInfo = ci
					break
				}
			}
		}
		actorId, err := addr.IDFromAddress(h.receiver)
		require.NoError(h.t, err)

		proofInfos := make([]abi.SectorInfo, len(infos))
		for i, ci := range infos {
			si := ci
			if poStCfg != nil {
				contains, err := poStCfg.skipped.IsSet(uint64(ci.SectorNumber))
				require.NoError(h.t, err)
				if contains {
					si = goodInfo
				}
			}
			proofInfos[i] = abi.SectorInfo{
				SealProof:    si.SealProof,
				SectorNumber: si.SectorNumber,
				SealedCID:    si.SealedCID,
			}
		}

		vi := abi.WindowPoStVerifyInfo{
			Randomness:        abi.PoStRandomness(challengeRand),
			Proofs:            proofs,
			ChallengedSectors: proofInfos,
			Prover:            abi.ActorID(actorId),
		}
		rt.ExpectVerifyPoSt(vi, nil)
	}
	skipped := bitfield.New()
	if poStCfg != nil {
		// expect power update
		if !poStCfg.expectedRawPowerDelta.IsZero() || !poStCfg.expectedQAPowerDelta.IsZero() {
			claim := &power.UpdateClaimedPowerParams{
				RawByteDelta:         poStCfg.expectedRawPowerDelta,
				QualityAdjustedDelta: poStCfg.expectedQAPowerDelta,
			}
			rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.UpdateClaimedPower, claim, abi.NewTokenAmount(0),
				nil, exitcode.Ok)
		}
		if !poStCfg.expectedPenalty.IsZero() {
			rt.ExpectSend(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, poStCfg.expectedPenalty, nil, exitcode.Ok)
		}
		pledgeDelta := poStCfg.expectedPenalty.Neg()
		if !pledgeDelta.IsZero() {
			rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.UpdatePledgeTotal, &pledgeDelta,
				abi.NewTokenAmount(0), nil, exitcode.Ok)
		}
		skipped = *poStCfg.skipped
	}

	params := miner.SubmitWindowedPoStParams{
		Deadline:   deadline.Index,
		Partitions: partitions,
		Proofs:     proofs,
		Skipped:    skipped,
	}

	rt.Call(h.a.SubmitWindowedPoSt, &params)
	rt.Verify()
}

func (h *actorHarness) computePartitions(rt *mock.Runtime, deadlines *miner.Deadlines, deadlineIdx uint64) ([]*miner.SectorOnChainInfo, []uint64) {
	st := getState(rt)
	firstPartIdx, sectorCount, err := miner.PartitionsForDeadline(deadlines, h.partitionSize, deadlineIdx)
	require.NoError(h.t, err)
	if sectorCount == 0 {
		return nil, nil
	}
	partitionCount, _, err := miner.DeadlineCount(deadlines, h.partitionSize, deadlineIdx)
	require.NoError(h.t, err)

	partitions := make([]uint64, partitionCount)
	for i := uint64(0); i < partitionCount; i++ {
		partitions[i] = firstPartIdx + i
	}

	partitionsSectors, err := miner.ComputePartitionsSectors(deadlines, h.partitionSize, deadlineIdx, partitions)
	require.NoError(h.t, err)
	provenSectors, err := bitfield.MultiMerge(partitionsSectors...)
	require.NoError(h.t, err)
	infos, _, err := st.LoadSectorInfosForProof(rt.AdtStore(), provenSectors)
	require.NoError(h.t, err)

	return infos, partitions
}

func (h *actorHarness) declareFaults(rt *mock.Runtime, totalQAPower abi.StoragePower, fee abi.TokenAmount, faultSectorInfos ...*miner.SectorOnChainInfo) {
	rt.SetCaller(h.worker, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(h.worker)

	ss, err := faultSectorInfos[0].SealProof.SectorSize()
	require.NoError(h.t, err)
	expectedRawDelta, expectedQADelta := powerForSectors(ss, faultSectorInfos)
	expectedRawDelta = expectedRawDelta.Neg()
	expectedQADelta = expectedQADelta.Neg()

	expectedTotalPower := &power.CurrentTotalPowerReturn{
		QualityAdjPower: totalQAPower,
	}

	expectQueryNetworkInfo(rt, expectedTotalPower, h.epochReward)

	// expect power update
	claim := &power.UpdateClaimedPowerParams{
		RawByteDelta:         expectedRawDelta,
		QualityAdjustedDelta: expectedQADelta,
	}
	rt.ExpectSend(
		builtin.StoragePowerActorAddr,
		builtin.MethodsPower.UpdateClaimedPower,
		claim,
		abi.NewTokenAmount(0),
		nil,
		exitcode.Ok,
	)

	// expect fee
	rt.ExpectSend(
		builtin.BurntFundsActorAddr,
		builtin.MethodSend,
		nil,
		fee,
		nil,
		exitcode.Ok,
	)

	// expect pledge update
	pledgeDelta := fee.Neg()
	rt.ExpectSend(
		builtin.StoragePowerActorAddr,
		builtin.MethodsPower.UpdatePledgeTotal,
		&pledgeDelta,
		abi.NewTokenAmount(0),
		nil,
		exitcode.Ok,
	)

	// Calculate params from faulted sector infos
	st := getState(rt)
	params := makeFaultParamsFromFaultingSectors(h.t, st, rt.AdtStore(), faultSectorInfos)
	rt.Call(h.a.DeclareFaults, params)
	rt.Verify()
}

func (h *actorHarness) advanceProvingPeriodWithoutFaults(rt *mock.Runtime) {

	// Iterate deadlines in the proving period, setting epoch to the first in each deadline.
	// Submit a window post for all partitions due at each deadline when necessary.
	deadline := h.deadline(rt)
	for !deadline.PeriodElapsed() {
		st := getState(rt)
		store := rt.AdtStore()
		deadlines, err := st.LoadDeadlines(store)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "could not load deadlines")

		firstPartIdx, sectorCount, err := miner.PartitionsForDeadline(deadlines, h.partitionSize, deadline.Index)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "could not get partitions for deadline")
		if sectorCount != 0 {
			partitionCount, _, err := miner.DeadlineCount(deadlines, h.partitionSize, deadline.Index)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "could not get partition count")

			partitions := make([]uint64, partitionCount)
			for i := uint64(0); i < partitionCount; i++ {
				partitions[i] = firstPartIdx + i
			}

			partitionsSectors, err := miner.ComputePartitionsSectors(deadlines, h.partitionSize, deadline.Index, partitions)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "could not compute partitions")
			provenSectors, err := bitfield.MultiMerge(partitionsSectors...)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "could not get proven sectors")
			infos, _, err := st.LoadSectorInfosForProof(store, provenSectors)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "could not load sector info for proof")

			h.submitWindowPoSt(rt, deadline, partitions, infos, nil)
		}

		rt.SetEpoch(deadline.Close + 1)
		deadline = h.deadline(rt)
	}
	// Rewind one epoch to leave the current epoch as the penultimate one in the proving period,
	// ready for proving-period cron.
	rt.SetEpoch(rt.Epoch() - 1)
}

func (h *actorHarness) extendSector(rt *mock.Runtime, sector *miner.SectorOnChainInfo, extension abi.ChainEpoch, params *miner.ExtendSectorExpirationParams) {
	rt.SetCaller(h.worker, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(h.worker)

	newSector := *sector
	newSector.Expiration += extension
	qaDelta := big.Sub(miner.QAPowerForSector(h.sectorSize, &newSector), miner.QAPowerForSector(h.sectorSize, sector))

	rt.ExpectSend(builtin.StoragePowerActorAddr,
		builtin.MethodsPower.UpdateClaimedPower,
		&power.UpdateClaimedPowerParams{
			RawByteDelta:         big.Zero(),
			QualityAdjustedDelta: qaDelta,
		},
		abi.NewTokenAmount(0),
		nil,
		exitcode.Ok,
	)
	rt.Call(h.a.ExtendSectorExpiration, params)
	rt.Verify()
}

func (h *actorHarness) terminateSectors(rt *mock.Runtime, sectors *abi.BitField, expectedFee abi.TokenAmount) {
	rt.SetCaller(h.worker, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(h.worker)

	dealIDs := []abi.DealID{}
	sectorInfos := []*miner.SectorOnChainInfo{}
	err := sectors.ForEach(func(secNum uint64) error {
		sector := h.getSector(rt, abi.SectorNumber(secNum))
		dealIDs = append(dealIDs, sector.DealIDs...)

		sectorInfos = append(sectorInfos, sector)
		return nil
	})
	require.NoError(h.t, err)

	{
		expectQueryNetworkInfo(rt, &power.CurrentTotalPowerReturn{
			RawBytePower:     h.networkRawPower,
			QualityAdjPower:  h.networkQAPower,
			PledgeCollateral: h.networkPledge,
		}, h.epochReward)
	}

	{
		rawPower, qaPower := miner.PowerForSectors(h.sectorSize, sectorInfos)
		rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.UpdateClaimedPower, &power.UpdateClaimedPowerParams{
			RawByteDelta:         rawPower.Neg(),
			QualityAdjustedDelta: qaPower.Neg(),
		}, abi.NewTokenAmount(0), nil, exitcode.Ok)
	}
	if big.Zero().LessThan(expectedFee) {
		rt.ExpectSend(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, expectedFee, nil, exitcode.Ok)
		pledgeDelta := expectedFee.Neg()
		rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.UpdatePledgeTotal, &pledgeDelta, big.Zero(), nil, exitcode.Ok)
	}

	params := &miner.TerminateSectorsParams{Sectors: sectors}
	rt.Call(h.a.TerminateSectors, params)
	rt.Verify()
}

func (h *actorHarness) reportConsensusFault(rt *mock.Runtime, from addr.Address, params *miner.ReportConsensusFaultParams, dealIDs []abi.DealID) {
	rt.SetCaller(from, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)

	rt.ExpectVerifyConsensusFault(params.BlockHeader1, params.BlockHeader2, params.BlockHeaderExtra, &runtime.ConsensusFault{
		Target: h.receiver,
		Epoch:  rt.Epoch() - 1,
		Type:   runtime.ConsensusFaultDoubleForkMining,
	}, nil)

	// slash reward
	reward := miner.RewardForConsensusSlashReport(1, rt.Balance())
	rt.ExpectSend(from, builtin.MethodSend, nil, reward, nil, exitcode.Ok)

	// power termination
	lockedFunds := getState(rt).LockedFunds
	rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.OnConsensusFault, &lockedFunds, abi.NewTokenAmount(0), nil, exitcode.Ok)

	// expect every deal to be closed out
	rt.ExpectSend(builtin.StorageMarketActorAddr, builtin.MethodsMarket.OnMinerSectorsTerminate, &market.OnMinerSectorsTerminateParams{
		DealIDs: dealIDs,
	}, abi.NewTokenAmount(0), nil, exitcode.Ok)

	// expect actor to be deleted
	rt.ExpectDeleteActor(builtin.BurntFundsActorAddr)

	rt.Call(h.a.ReportConsensusFault, params)
	rt.Verify()
}

func (h *actorHarness) addLockedFund(rt *mock.Runtime, amt abi.TokenAmount) {
	rt.SetCaller(h.worker, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(h.worker, h.owner, builtin.RewardActorAddr)
	// expect pledge update
	rt.ExpectSend(
		builtin.StoragePowerActorAddr,
		builtin.MethodsPower.UpdatePledgeTotal,
		&amt,
		abi.NewTokenAmount(0),
		nil,
		exitcode.Ok,
	)

	rt.Call(h.a.AddLockedFund, &amt)
	rt.Verify()
}

func (h *actorHarness) onProvingPeriodCron(rt *mock.Runtime, expectedEnrollment abi.ChainEpoch, newSectors bool,
	faultySectors []*miner.SectorOnChainInfo, expireSectors []*miner.SectorOnChainInfo) {
	rt.ExpectValidateCallerAddr(builtin.StoragePowerActorAddr)

	// Preamble
	rt.ExpectSend(builtin.RewardActorAddr, builtin.MethodsReward.ThisEpochReward, nil, big.Zero(), &h.epochReward, exitcode.Ok)
	networkPower := big.NewIntUnsigned(1 << 50)
	rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.CurrentTotalPower, nil, big.Zero(),
		&power.CurrentTotalPowerReturn{
			RawBytePower:     networkPower,
			QualityAdjPower:  networkPower,
			PledgeCollateral: h.networkPledge,
		},
		exitcode.Ok)

	{
		// Detect and penalise missing faults (not yet implemented)
	}

	if len(expireSectors) > 0 {
		// Expire sectors
		rawPower, qaPower := powerForSectors(h.sectorSize, expireSectors)
		rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.UpdateClaimedPower, &power.UpdateClaimedPowerParams{
			RawByteDelta:         rawPower.Neg(),
			QualityAdjustedDelta: qaPower.Neg(),
		}, abi.NewTokenAmount(0), nil, exitcode.Ok)
	}

	if len(faultySectors) > 0 {
		// Process ongoing faulty sectors (not yet implemented)
		_, qaFault := powerForSectors(h.sectorSize, faultySectors)
		fee := miner.PledgePenaltyForDeclaredFault(h.epochReward, networkPower, qaFault)
		rt.ExpectSend(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, fee, nil, exitcode.Ok)

		pledgeDelta := fee.Neg()
		rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.UpdatePledgeTotal, &pledgeDelta, big.Zero(), nil, exitcode.Ok)
	}

	if newSectors {
		// Establish new proving sets
		randEpoch := rt.Epoch() - miner.ElectionLookback
		rt.ExpectGetRandomness(crypto.DomainSeparationTag_WindowedPoStDeadlineAssignment, randEpoch, nil, bytes.Repeat([]byte{0}, 32))
	}

	// Re-enrollment for next period.
	rt.ExpectSend(builtin.StoragePowerActorAddr, builtin.MethodsPower.EnrollCronEvent,
		makeProvingPeriodCronEventParams(h.t, expectedEnrollment), big.Zero(), nil, exitcode.Ok)

	rt.SetCaller(builtin.StoragePowerActorAddr, builtin.StoragePowerActorCodeID)
	rt.Call(h.a.OnDeferredCronEvent, &miner.CronEventPayload{
		EventType: miner.CronEventProvingPeriod,
	})
	rt.Verify()
}

func (h *actorHarness) withdrawFunds(rt *mock.Runtime, amount abi.TokenAmount) {
	rt.SetCaller(h.owner, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(h.owner)

	rt.ExpectSend(h.owner, builtin.MethodSend, nil, amount, nil, exitcode.Ok)

	rt.Call(h.a.WithdrawBalance, &miner.WithdrawBalanceParams{
		AmountRequested: amount,
	})
	rt.Verify()
}

//
// Higher-level orchestration
//

// Completes a proving period by moving the epoch forward to the penultimate one, calling the proving period cron handler,
// and then advancing to the first epoch in the new period.
func completeProvingPeriod(rt *mock.Runtime, h *actorHarness, newSectors bool, faultySectors, expireSectors []*miner.SectorOnChainInfo) {
	deadline := h.deadline(rt)
	rt.SetEpoch(deadline.PeriodEnd())
	nextCron := deadline.NextPeriodStart() + miner.WPoStProvingPeriod - 1
	h.onProvingPeriodCron(rt, nextCron, newSectors, faultySectors, expireSectors)
	rt.SetEpoch(deadline.NextPeriodStart())
}

//
// Construction helpers, etc
//

func builderForHarness(actor *actorHarness) *mock.RuntimeBuilder {
	return mock.NewBuilder(context.Background(), actor.receiver).
		WithActorType(actor.owner, builtin.AccountActorCodeID).
		WithActorType(actor.worker, builtin.AccountActorCodeID).
		WithHasher(fixedHasher(uint64(actor.periodOffset)))
}

func getState(rt *mock.Runtime) *miner.State {
	var st miner.State
	rt.GetState(&st)
	return &st
}

func makeProvingPeriodCronEventParams(t testing.TB, epoch abi.ChainEpoch) *power.EnrollCronEventParams {
	eventPayload := miner.CronEventPayload{EventType: miner.CronEventProvingPeriod}
	buf := bytes.Buffer{}
	err := eventPayload.MarshalCBOR(&buf)
	require.NoError(t, err)
	return &power.EnrollCronEventParams{
		EventEpoch: epoch,
		Payload:    buf.Bytes(),
	}
}

func makePreCommit(sectorNo abi.SectorNumber, challenge, expiration abi.ChainEpoch, dealIDs []abi.DealID) *miner.SectorPreCommitInfo {
	return &miner.SectorPreCommitInfo{
		SealProof:     abi.RegisteredSealProof_StackedDrg2KiBV1,
		SectorNumber:  sectorNo,
		SealedCID:     tutil.MakeCID("commr"),
		SealRandEpoch: challenge,
		DealIDs:       dealIDs,
		Expiration:    expiration,
	}
}

func makeProveCommit(sectorNo abi.SectorNumber) *miner.ProveCommitSectorParams {
	return &miner.ProveCommitSectorParams{
		SectorNumber: sectorNo,
		Proof:        []byte("proof"),
	}
}

func makeFaultParamsFromFaultingSectors(t testing.TB, st *miner.State, store adt.Store, faultSectorInfos []*miner.SectorOnChainInfo) *miner.DeclareFaultsParams {
	deadlines, err := st.LoadDeadlines(store)
	require.NoError(t, err)
	faultAtDeadline := make(map[uint64][]uint64)
	// Find the deadline for each faulty sector which must be provided with the fault declaration
	for _, sectorInfo := range faultSectorInfos {
		dl, err := miner.FindDeadline(deadlines, sectorInfo.SectorNumber)
		require.NoError(t, err)
		faultAtDeadline[dl] = append(faultAtDeadline[dl], uint64(sectorInfo.SectorNumber))
	}
	params := &miner.DeclareFaultsParams{Faults: []miner.FaultDeclaration{}}
	// Group together faults at the same deadline into a bitfield
	for dl, sectorNumbers := range faultAtDeadline {
		fault := miner.FaultDeclaration{
			Deadline: dl,
			Sectors:  bitfield.NewFromSet(sectorNumbers),
		}
		params.Faults = append(params.Faults, fault)
	}
	return params
}

func powerForSectors(sectorSize abi.SectorSize, sectors []*miner.SectorOnChainInfo) (rawBytePower, qaPower big.Int) {
	rawBytePower = big.Mul(big.NewIntUnsigned(uint64(sectorSize)), big.NewIntUnsigned(uint64(len(sectors))))
	qaPower = big.Zero()
	for _, s := range sectors {
		qaPower = big.Add(qaPower, miner.QAPowerForSector(sectorSize, s))
	}
	return rawBytePower, qaPower
}

func assertEmptyBitfield(t *testing.T, b *abi.BitField) {
	empty, err := b.IsEmpty()
	require.NoError(t, err)
	assert.True(t, empty)
}

// Returns a fake hashing function that always arranges the first 8 bytes of the digest to be the binary
// encoding of a target uint64.
func fixedHasher(target uint64) func([]byte) [32]byte {
	return func(_ []byte) [32]byte {
		var buf bytes.Buffer
		err := binary.Write(&buf, binary.BigEndian, target)
		if err != nil {
			panic(err)
		}
		var digest [32]byte
		copy(digest[:], buf.Bytes())
		return digest
	}
}

func expectQueryNetworkInfo(rt *mock.Runtime, expectedTotalPower *power.CurrentTotalPowerReturn, expectedReward big.Int) {
	rt.ExpectSend(
		builtin.RewardActorAddr,
		builtin.MethodsReward.ThisEpochReward,
		nil,
		big.Zero(),
		&expectedReward,
		exitcode.Ok,
	)

	rt.ExpectSend(
		builtin.StoragePowerActorAddr,
		builtin.MethodsPower.CurrentTotalPower,
		nil,
		big.Zero(),
		expectedTotalPower,
		exitcode.Ok,
	)
}
