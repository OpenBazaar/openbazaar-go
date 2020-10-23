package miner

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/dline"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/go-state-types/network"
	rtt "github.com/filecoin-project/go-state-types/rt"
	cid "github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/actors/builtin/reward"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/proof"
	. "github.com/filecoin-project/specs-actors/actors/util"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/filecoin-project/specs-actors/actors/util/smoothing"
)

type Runtime = runtime.Runtime

type CronEventType int64

const (
	CronEventWorkerKeyChange CronEventType = iota
	CronEventProvingDeadline
	CronEventProcessEarlyTerminations
)

type CronEventPayload struct {
	EventType CronEventType
}

// Identifier for a single partition within a miner.
type PartitionKey struct {
	Deadline  uint64
	Partition uint64
}

type Actor struct{}

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.ControlAddresses,
		3:                         a.ChangeWorkerAddress,
		4:                         a.ChangePeerID,
		5:                         a.SubmitWindowedPoSt,
		6:                         a.PreCommitSector,
		7:                         a.ProveCommitSector,
		8:                         a.ExtendSectorExpiration,
		9:                         a.TerminateSectors,
		10:                        a.DeclareFaults,
		11:                        a.DeclareFaultsRecovered,
		12:                        a.OnDeferredCronEvent,
		13:                        a.CheckSectorProven,
		14:                        a.AddLockedFund,
		15:                        a.ReportConsensusFault,
		16:                        a.WithdrawBalance,
		17:                        a.ConfirmSectorProofsValid,
		18:                        a.ChangeMultiaddrs,
		19:                        a.CompactPartitions,
		20:                        a.CompactSectorNumbers,
	}
}

func (a Actor) Code() cid.Cid {
	return builtin.StorageMinerActorCodeID
}

func (a Actor) State() cbor.Er {
	return new(State)
}

var _ runtime.VMActor = Actor{}

/////////////////
// Constructor //
/////////////////

// Storage miner actors are created exclusively by the storage power actor. In order to break a circular dependency
// between the two, the construction parameters are defined in the power actor.
type ConstructorParams = power.MinerConstructorParams

func (a Actor) Constructor(rt Runtime, params *ConstructorParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.InitActorAddr)

	_, ok := SupportedProofTypes[params.SealProofType]
	if !ok {
		rt.Abortf(exitcode.ErrIllegalArgument, "proof type %d not allowed for new miner actors", params.SealProofType)
	}

	owner := resolveControlAddress(rt, params.OwnerAddr)
	worker := resolveWorkerAddress(rt, params.WorkerAddr)
	controlAddrs := make([]addr.Address, 0, len(params.ControlAddrs))
	for _, ca := range params.ControlAddrs {
		resolved := resolveControlAddress(rt, ca)
		controlAddrs = append(controlAddrs, resolved)
	}

	emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to construct initial state: %v", err)
	}

	emptyArray, err := adt.MakeEmptyArray(adt.AsStore(rt)).Root()
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to construct initial state: %v", err)
	}

	emptyBitfield := bitfield.NewFromSet(nil)
	emptyBitfieldCid := rt.StorePut(emptyBitfield)

	emptyDeadline := ConstructDeadline(emptyArray)
	emptyDeadlineCid := rt.StorePut(emptyDeadline)

	emptyDeadlines := ConstructDeadlines(emptyDeadlineCid)
	emptyVestingFunds := ConstructVestingFunds()
	emptyDeadlinesCid := rt.StorePut(emptyDeadlines)
	emptyVestingFundsCid := rt.StorePut(emptyVestingFunds)

	currEpoch := rt.CurrEpoch()
	offset, err := assignProvingPeriodOffset(rt.Receiver(), currEpoch, rt.HashBlake2b)
	builtin.RequireNoErr(rt, err, exitcode.ErrSerialization, "failed to assign proving period offset")
	periodStart := nextProvingPeriodStart(currEpoch, offset)
	Assert(periodStart > currEpoch)

	info, err := ConstructMinerInfo(owner, worker, controlAddrs, params.PeerId, params.Multiaddrs, params.SealProofType)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "failed to construct initial miner info")
	infoCid := rt.StorePut(info)

	state, err := ConstructState(infoCid, periodStart, emptyBitfieldCid, emptyArray, emptyMap, emptyDeadlinesCid, emptyVestingFundsCid)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "failed to construct state")
	rt.StateCreate(state)

	// Register first cron callback for epoch before the first proving period starts.
	enrollCronEvent(rt, periodStart-1, &CronEventPayload{
		EventType: CronEventProvingDeadline,
	})
	return nil
}

/////////////
// Control //
/////////////

type GetControlAddressesReturn struct {
	Owner        addr.Address
	Worker       addr.Address
	ControlAddrs []addr.Address
}

func (a Actor) ControlAddresses(rt Runtime, _ *abi.EmptyValue) *GetControlAddressesReturn {
	rt.ValidateImmediateCallerAcceptAny()
	var st State
	rt.StateReadonly(&st)
	info := getMinerInfo(rt, &st)
	return &GetControlAddressesReturn{
		Owner:        info.Owner,
		Worker:       info.Worker,
		ControlAddrs: info.ControlAddresses,
	}
}

type ChangeWorkerAddressParams struct {
	NewWorker       addr.Address
	NewControlAddrs []addr.Address
}

// ChangeWorkerAddress will ALWAYS overwrite the existing control addresses with the control addresses passed in the params.
// If a nil addresses slice is passed, the control addresses will be cleared.
// A worker change will be scheduled if the worker passed in the params is different from the existing worker.
func (a Actor) ChangeWorkerAddress(rt Runtime, params *ChangeWorkerAddressParams) *abi.EmptyValue {
	var effectiveEpoch abi.ChainEpoch

	newWorker := resolveWorkerAddress(rt, params.NewWorker)

	var controlAddrs []addr.Address
	for _, ca := range params.NewControlAddrs {
		resolved := resolveControlAddress(rt, ca)
		controlAddrs = append(controlAddrs, resolved)
	}

	var st State
	isWorkerChange := false
	rt.StateTransaction(&st, func() {
		info := getMinerInfo(rt, &st)

		// Only the Owner is allowed to change the newWorker and control addresses.
		rt.ValidateImmediateCallerIs(info.Owner)

		{
			// save the new control addresses
			info.ControlAddresses = controlAddrs
		}

		{
			// save newWorker addr key change request
			// This may replace another pending key change.
			if newWorker != info.Worker {
				isWorkerChange = true
				effectiveEpoch = rt.CurrEpoch() + WorkerKeyChangeDelay

				info.PendingWorkerKey = &WorkerKeyChange{
					NewWorker:   newWorker,
					EffectiveAt: effectiveEpoch,
				}
			}
		}

		err := st.SaveInfo(adt.AsStore(rt), info)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "could not save miner info")
	})

	// we only need to enroll the cron event for newWorker key change as we change the control
	// addresses immediately
	if isWorkerChange {
		cronPayload := CronEventPayload{
			EventType: CronEventWorkerKeyChange,
		}
		enrollCronEvent(rt, effectiveEpoch, &cronPayload)
	}

	return nil
}

type ChangePeerIDParams struct {
	NewID abi.PeerID
}

func (a Actor) ChangePeerID(rt Runtime, params *ChangePeerIDParams) *abi.EmptyValue {
	// TODO: Consider limiting the maximum number of bytes used by the peer ID on-chain.
	// https://github.com/filecoin-project/specs-actors/issues/712
	var st State
	rt.StateTransaction(&st, func() {
		info := getMinerInfo(rt, &st)

		rt.ValidateImmediateCallerIs(append(info.ControlAddresses, info.Owner, info.Worker)...)

		info.PeerId = params.NewID
		err := st.SaveInfo(adt.AsStore(rt), info)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "could not save miner info")
	})
	return nil
}

type ChangeMultiaddrsParams struct {
	NewMultiaddrs []abi.Multiaddrs
}

func (a Actor) ChangeMultiaddrs(rt Runtime, params *ChangeMultiaddrsParams) *abi.EmptyValue {
	// TODO: Consider limiting the maximum number of bytes used by multiaddrs on-chain.
	// https://github.com/filecoin-project/specs-actors/issues/712
	var st State
	rt.StateTransaction(&st, func() {
		info := getMinerInfo(rt, &st)

		rt.ValidateImmediateCallerIs(append(info.ControlAddresses, info.Owner, info.Worker)...)

		info.Multiaddrs = params.NewMultiaddrs
		err := st.SaveInfo(adt.AsStore(rt), info)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "could not save miner info")
	})
	return nil
}

//////////////////
// WindowedPoSt //
//////////////////

type PoStPartition struct {
	// Partitions are numbered per-deadline, from zero.
	Index uint64
	// Sectors skipped while proving that weren't already declared faulty
	Skipped bitfield.BitField
}

// Information submitted by a miner to provide a Window PoSt.
type SubmitWindowedPoStParams struct {
	// The deadline index which the submission targets.
	Deadline uint64
	// The partitions being proven.
	Partitions []PoStPartition
	// Array of proofs, one per distinct registered proof type present in the sectors being proven.
	// In the usual case of a single proof type, this array will always have a single element (independent of number of partitions).
	Proofs []proof.PoStProof
	// The epoch at which these proofs is being committed to a particular chain.
	ChainCommitEpoch abi.ChainEpoch
	// The ticket randomness on the chain at the ChainCommitEpoch on the chain this post is committed to
	ChainCommitRand abi.Randomness
}

// Invoked by miner's worker address to submit their fallback post
func (a Actor) SubmitWindowedPoSt(rt Runtime, params *SubmitWindowedPoStParams) *abi.EmptyValue {
	currEpoch := rt.CurrEpoch()
	store := adt.AsStore(rt)
	networkVersion := rt.NetworkVersion()
	var st State

	if params.Deadline >= WPoStPeriodDeadlines {
		rt.Abortf(exitcode.ErrIllegalArgument, "invalid deadline %d of %d", params.Deadline, WPoStPeriodDeadlines)
	}
	if params.ChainCommitEpoch >= currEpoch {
		rt.Abortf(exitcode.ErrIllegalArgument, "PoSt chain commitment %d must be in the past", params.ChainCommitEpoch)
	}
	if params.ChainCommitEpoch < currEpoch-WPoStMaxChainCommitAge {
		rt.Abortf(exitcode.ErrIllegalArgument, "PoSt chain commitment %d too far in the past, must be after %d", params.ChainCommitEpoch, currEpoch-WPoStMaxChainCommitAge)
	}
	commRand := rt.GetRandomnessFromTickets(crypto.DomainSeparationTag_PoStChainCommit, params.ChainCommitEpoch, nil)
	if !bytes.Equal(commRand, params.ChainCommitRand) {
		rt.Abortf(exitcode.ErrIllegalArgument, "post commit randomness mismatched")
	}
	// TODO: limit the length of proofs array https://github.com/filecoin-project/specs-actors/issues/416

	// Get the total power/reward. We need these to compute penalties.
	rewardStats := requestCurrentEpochBlockReward(rt)
	pwrTotal := requestCurrentTotalPower(rt)

	penaltyTotal := abi.NewTokenAmount(0)
	pledgeDelta := abi.NewTokenAmount(0)
	var postResult *PoStResult

	var info *MinerInfo
	rt.StateTransaction(&st, func() {
		info = getMinerInfo(rt, &st)

		rt.ValidateImmediateCallerIs(append(info.ControlAddresses, info.Owner, info.Worker)...)

		// Validate that the miner didn't try to prove too many partitions at once.
		submissionPartitionLimit := loadPartitionsSectorsMax(info.WindowPoStPartitionSectors)
		if uint64(len(params.Partitions)) > submissionPartitionLimit {
			rt.Abortf(exitcode.ErrIllegalArgument, "too many partitions %d, limit %d", len(params.Partitions), submissionPartitionLimit)
		}

		// Load and check deadline.
		currDeadline := st.DeadlineInfo(currEpoch)
		deadlines, err := st.LoadDeadlines(adt.AsStore(rt))
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load deadlines")

		// Check that the miner state indicates that the current proving deadline has started.
		// This should only fail if the cron actor wasn't invoked, and matters only in case that it hasn't been
		// invoked for a whole proving period, and hence the missed PoSt submissions from the prior occurrence
		// of this deadline haven't been processed yet.
		if !currDeadline.IsOpen() {
			rt.Abortf(exitcode.ErrIllegalState, "proving period %d not yet open at %d", currDeadline.PeriodStart, currEpoch)
		}

		// The miner may only submit a proof for the current deadline.
		if params.Deadline != currDeadline.Index {
			rt.Abortf(exitcode.ErrIllegalArgument, "invalid deadline %d at epoch %d, expected %d",
				params.Deadline, currEpoch, currDeadline.Index)
		}

		sectors, err := LoadSectors(store, st.Sectors)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load sectors")

		deadline, err := deadlines.LoadDeadline(store, params.Deadline)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load deadline %d", params.Deadline)

		// Record proven sectors/partitions, returning updates to power and the final set of sectors
		// proven/skipped.
		//
		// NOTE: This function does not actually check the proofs but does assume that they'll be
		// successfully validated. The actual proof verification is done below in verifyWindowedPost.
		//
		// If proof verification fails, the this deadline MUST NOT be saved and this function should
		// be aborted.
		faultExpiration := currDeadline.Last() + FaultMaxAge
		postResult, err = deadline.RecordProvenSectors(store, sectors, info.SectorSize, QuantSpecForDeadline(currDeadline), faultExpiration, params.Partitions)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to process post submission for deadline %d", params.Deadline)

		// Validate proofs

		// Load sector infos for proof, substituting a known-good sector for known-faulty sectors.
		// Note: this is slightly sub-optimal, loading info for the recovering sectors again after they were already
		// loaded above.
		sectorInfos, err := st.LoadSectorInfosForProof(store, postResult.Sectors, postResult.IgnoredSectors)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load proven sector info")

		// Skip verification if all sectors are faults.
		// We still need to allow this call to succeed so the miner can declare a whole partition as skipped.
		if len(sectorInfos) > 0 {
			// Verify the proof.
			// A failed verification doesn't immediately cause a penalty; the miner can try again.
			//
			// This function aborts on failure.
			verifyWindowedPost(rt, currDeadline.Challenge, sectorInfos, params.Proofs)
		}

		// Penalize new skipped faults and retracted recoveries as undeclared faults.
		// These pay a higher fee than faults declared before the deadline challenge window opened.
		undeclaredPenaltyPower := postResult.PenaltyPower()
		undeclaredPenaltyTarget := big.Zero()
		if networkVersion >= network.Version3 {
			// From version 3, skipped faults and retracted recoveries pay nothing at Window PoSt,
			// but will incur the "ongoing" fault fee at deadline end.
		} else {
			undeclaredPenaltyTarget = PledgePenaltyForUndeclaredFault(
				rewardStats.ThisEpochRewardSmoothed, pwrTotal.QualityAdjPowerSmoothed, undeclaredPenaltyPower.QA,
				networkVersion,
			)
			// Subtract the "ongoing" fault fee from the amount charged now, since it will be charged at
			// the end-of-deadline cron.
			undeclaredPenaltyTarget = big.Sub(undeclaredPenaltyTarget, PledgePenaltyForDeclaredFault(
				rewardStats.ThisEpochRewardSmoothed, pwrTotal.QualityAdjPowerSmoothed, undeclaredPenaltyPower.QA,
				networkVersion,
			))
		}

		// Penalize recoveries as declared faults (a lower fee than the undeclared, above).
		// It sounds odd, but because faults are penalized in arrears, at the _end_ of the faulty period, we must
		// penalize recovered sectors here because they won't be penalized by the end-of-deadline cron for the
		// immediately-prior faulty period.
		declaredPenaltyTarget := big.Zero()
		if networkVersion >= network.Version3 {
			// From version 3, recovered sectors pay no penalty.
			// They won't pay anything at deadline end either, since they'll no longer be faulty.
		} else {
			declaredPenaltyTarget = PledgePenaltyForDeclaredFault(
				rewardStats.ThisEpochRewardSmoothed, pwrTotal.QualityAdjPowerSmoothed, postResult.RecoveredPower.QA,
				networkVersion,
			)
		}

		// Note: We could delay this charge until end of deadline, but that would require more accounting state.
		totalPenaltyTarget := big.Add(undeclaredPenaltyTarget, declaredPenaltyTarget)
		unlockedBalance := st.GetUnlockedBalance(rt.CurrentBalance())
		vestingPenaltyTotal, balancePenaltyTotal, err := st.PenalizeFundsInPriorityOrder(store, currEpoch, totalPenaltyTarget, unlockedBalance)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to unlock penalty for %v", undeclaredPenaltyPower)
		penaltyTotal = big.Add(vestingPenaltyTotal, balancePenaltyTotal)
		pledgeDelta = big.Sub(pledgeDelta, vestingPenaltyTotal)

		err = deadlines.UpdateDeadline(store, params.Deadline, deadline)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to update deadline %d", params.Deadline)

		err = st.SaveDeadlines(store, deadlines)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save deadlines")
	})

	// Restore power for recovered sectors. Remove power for new faults.
	// NOTE: It would be permissible to delay the power loss until the deadline closes, but that would require
	// additional accounting state.
	// https://github.com/filecoin-project/specs-actors/issues/414
	requestUpdatePower(rt, postResult.PowerDelta())
	// Burn penalties.
	burnFunds(rt, penaltyTotal)
	notifyPledgeChanged(rt, pledgeDelta)
	return nil
}

///////////////////////
// Sector Commitment //
///////////////////////

// Proposals must be posted on chain via sma.PublishStorageDeals before PreCommitSector.
// Optimization: PreCommitSector could contain a list of deals that are not published yet.
func (a Actor) PreCommitSector(rt Runtime, params *SectorPreCommitInfo) *abi.EmptyValue {
	if _, ok := SupportedProofTypes[params.SealProof]; !ok {
		rt.Abortf(exitcode.ErrIllegalArgument, "unsupported seal proof type: %s", params.SealProof)
	}
	if params.SectorNumber > abi.MaxSectorNumber {
		rt.Abortf(exitcode.ErrIllegalArgument, "sector number %d out of range 0..(2^63-1)", params.SectorNumber)
	}
	if !params.SealedCID.Defined() {
		rt.Abortf(exitcode.ErrIllegalArgument, "sealed CID undefined")
	}
	if params.SealedCID.Prefix() != SealedCIDPrefix {
		rt.Abortf(exitcode.ErrIllegalArgument, "sealed CID had wrong prefix")
	}
	if params.SealRandEpoch >= rt.CurrEpoch() {
		rt.Abortf(exitcode.ErrIllegalArgument, "seal challenge epoch %v must be before now %v", params.SealRandEpoch, rt.CurrEpoch())
	}

	challengeEarliest := sealChallengeEarliest(rt.CurrEpoch(), params.SealProof)
	if params.SealRandEpoch < challengeEarliest {
		// The subsequent commitment proof can't possibly be accepted because the seal challenge will be deemed
		// too old. Note that passing this check doesn't guarantee the proof will be soon enough, depending on
		// when it arrives.
		rt.Abortf(exitcode.ErrIllegalArgument, "seal challenge epoch %v too old, must be after %v", params.SealRandEpoch, challengeEarliest)
	}

	if params.Expiration <= rt.CurrEpoch() {
		rt.Abortf(exitcode.ErrIllegalArgument, "sector expiration %v must be after now (%v)", params.Expiration, rt.CurrEpoch())
	}
	if params.ReplaceCapacity && len(params.DealIDs) == 0 {
		rt.Abortf(exitcode.ErrIllegalArgument, "cannot replace sector without committing deals")
	}
	if params.ReplaceSectorDeadline >= WPoStPeriodDeadlines {
		rt.Abortf(exitcode.ErrIllegalArgument, "invalid deadline %d", params.ReplaceSectorDeadline)
	}
	if params.ReplaceSectorNumber > abi.MaxSectorNumber {
		rt.Abortf(exitcode.ErrIllegalArgument, "invalid sector number %d", params.ReplaceSectorNumber)
	}

	// gather information from other actors

	rewardStats := requestCurrentEpochBlockReward(rt)
	pwrTotal := requestCurrentTotalPower(rt)
	dealWeight := requestDealWeight(rt, params.DealIDs, rt.CurrEpoch(), params.Expiration)

	store := adt.AsStore(rt)
	var st State
	newlyVested := big.Zero()
	rt.StateTransaction(&st, func() {
		info := getMinerInfo(rt, &st)

		rt.ValidateImmediateCallerIs(append(info.ControlAddresses, info.Owner, info.Worker)...)

		if params.SealProof != info.SealProofType {
			rt.Abortf(exitcode.ErrIllegalArgument, "sector seal proof %v must match miner seal proof type %d", params.SealProof, info.SealProofType)
		}

		maxDealLimit := dealPerSectorLimit(info.SectorSize)
		if uint64(len(params.DealIDs)) > maxDealLimit {
			rt.Abortf(exitcode.ErrIllegalArgument, "too many deals for sector %d > %d", len(params.DealIDs), maxDealLimit)
		}

		err := st.AllocateSectorNumber(store, params.SectorNumber)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to allocate sector id %d", params.SectorNumber)

		// The following two checks shouldn't be necessary, but it can't
		// hurt to double-check (unless it's really just too
		// expensive?).
		_, preCommitFound, err := st.GetPrecommittedSector(store, params.SectorNumber)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to check pre-commit %v", params.SectorNumber)
		if preCommitFound {
			rt.Abortf(exitcode.ErrIllegalState, "sector %v already pre-committed", params.SectorNumber)
		}

		sectorFound, err := st.HasSectorNo(store, params.SectorNumber)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to check sector %v", params.SectorNumber)
		if sectorFound {
			rt.Abortf(exitcode.ErrIllegalState, "sector %v already committed", params.SectorNumber)
		}

		// Require sector lifetime meets minimum by assuming activation happens at last epoch permitted for seal proof.
		// This could make sector maximum lifetime validation more lenient if the maximum sector limit isn't hit first.
		maxActivation := rt.CurrEpoch() + MaxSealDuration[params.SealProof]
		validateExpiration(rt, maxActivation, params.Expiration, params.SealProof)

		depositMinimum := big.Zero()
		if params.ReplaceCapacity {
			replaceSector := validateReplaceSector(rt, &st, store, params)
			// Note the replaced sector's initial pledge as a lower bound for the new sector's deposit
			depositMinimum = replaceSector.InitialPledge
		}

		newlyVested, err = st.UnlockVestedFunds(store, rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to vest funds")
		availableBalance := st.GetAvailableBalance(rt.CurrentBalance())
		duration := params.Expiration - rt.CurrEpoch()

		sectorWeight := QAPowerForWeight(info.SectorSize, duration, dealWeight.DealWeight, dealWeight.VerifiedDealWeight)
		depositReq := big.Max(
			PreCommitDepositForPower(rewardStats.ThisEpochRewardSmoothed, pwrTotal.QualityAdjPowerSmoothed, sectorWeight),
			depositMinimum,
		)
		if availableBalance.LessThan(depositReq) {
			rt.Abortf(exitcode.ErrInsufficientFunds, "insufficient funds for pre-commit deposit: %v", depositReq)
		}

		st.AddPreCommitDeposit(depositReq)
		st.AssertBalanceInvariants(rt.CurrentBalance())

		if err := st.PutPrecommittedSector(store, &SectorPreCommitOnChainInfo{
			Info:               *params,
			PreCommitDeposit:   depositReq,
			PreCommitEpoch:     rt.CurrEpoch(),
			DealWeight:         dealWeight.DealWeight,
			VerifiedDealWeight: dealWeight.VerifiedDealWeight,
		}); err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to write pre-committed sector %v: %v", params.SectorNumber, err)
		}
		// add precommit expiry to the queue
		msd, ok := MaxSealDuration[params.SealProof]
		if !ok {
			rt.Abortf(exitcode.ErrIllegalArgument, "no max seal duration set for proof type: %d", params.SealProof)
		}
		// The +1 here is critical for the batch verification of proofs. Without it, if a proof arrived exactly on the
		// due epoch, ProveCommitSector would accept it, then the expiry event would remove it, and then
		// ConfirmSectorProofsValid would fail to find it.
		expiryBound := rt.CurrEpoch() + msd + 1

		err = st.AddPreCommitExpiry(store, expiryBound, params.SectorNumber)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add pre-commit expiry to queue")
	})

	notifyPledgeChanged(rt, newlyVested.Neg())

	return nil
}

type ProveCommitSectorParams struct {
	SectorNumber abi.SectorNumber
	Proof        []byte
}

// Checks state of the corresponding sector pre-commitment, then schedules the proof to be verified in bulk
// by the power actor.
// If valid, the power actor will call ConfirmSectorProofsValid at the end of the same epoch as this message.
func (a Actor) ProveCommitSector(rt Runtime, params *ProveCommitSectorParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerAcceptAny()

	store := adt.AsStore(rt)
	var st State
	rt.StateReadonly(&st)

	// Verify locked funds are are at least the sum of sector initial pledges.
	// Note that this call does not actually compute recent vesting, so the reported locked funds may be
	// slightly higher than the true amount (i.e. slightly in the miner's favour).
	// Computing vesting here would be almost always redundant since vesting is quantized to ~daily units.
	// Vesting will be at most one proving period old if computed in the cron callback.
	verifyPledgeMeetsInitialRequirements(rt, &st)

	sectorNo := params.SectorNumber
	precommit, found, err := st.GetPrecommittedSector(store, sectorNo)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load pre-committed sector %v", sectorNo)
	if !found {
		rt.Abortf(exitcode.ErrNotFound, "no pre-committed sector %v", sectorNo)
	}

	msd, ok := MaxSealDuration[precommit.Info.SealProof]
	if !ok {
		rt.Abortf(exitcode.ErrIllegalState, "no max seal duration for proof type: %d", precommit.Info.SealProof)
	}
	proveCommitDue := precommit.PreCommitEpoch + msd
	if rt.CurrEpoch() > proveCommitDue {
		rt.Abortf(exitcode.ErrIllegalArgument, "commitment proof for %d too late at %d, due %d", sectorNo, rt.CurrEpoch(), proveCommitDue)
	}

	svi := getVerifyInfo(rt, &SealVerifyStuff{
		SealedCID:           precommit.Info.SealedCID,
		InteractiveEpoch:    precommit.PreCommitEpoch + PreCommitChallengeDelay,
		SealRandEpoch:       precommit.Info.SealRandEpoch,
		Proof:               params.Proof,
		DealIDs:             precommit.Info.DealIDs,
		SectorNumber:        precommit.Info.SectorNumber,
		RegisteredSealProof: precommit.Info.SealProof,
	})

	code := rt.Send(
		builtin.StoragePowerActorAddr,
		builtin.MethodsPower.SubmitPoRepForBulkVerify,
		svi,
		abi.NewTokenAmount(0),
		&builtin.Discard{},
	)
	builtin.RequireSuccess(rt, code, "failed to submit proof for bulk verification")
	return nil
}

func (a Actor) ConfirmSectorProofsValid(rt Runtime, params *builtin.ConfirmSectorProofsParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.StoragePowerActorAddr)

	// get network stats from other actors
	rewardStats := requestCurrentEpochBlockReward(rt)
	pwrTotal := requestCurrentTotalPower(rt)
	circulatingSupply := rt.TotalFilCircSupply()

	// 1. Activate deals, skipping pre-commits with invalid deals.
	//    - calls the market actor.
	// 2. Reschedule replacement sector expiration.
	//    - loads and saves sectors
	//    - loads and saves deadlines/partitions
	// 3. Add new sectors.
	//    - loads and saves sectors.
	//    - loads and saves deadlines/partitions
	//
	// Ideally, we'd combine some of these operations, but at least we have
	// a constant number of them.

	var st State
	rt.StateReadonly(&st)
	store := adt.AsStore(rt)
	info := getMinerInfo(rt, &st)

	//
	// Activate storage deals.
	//

	// This skips missing pre-commits.
	precommittedSectors, err := st.FindPrecommittedSectors(store, params.Sectors...)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load pre-committed sectors")

	// Committed-capacity sectors licensed for early removal by new sectors being proven.
	replaceSectors := make(DeadlineSectorMap)
	// Pre-commits for new sectors.
	var preCommits []*SectorPreCommitOnChainInfo
	for _, precommit := range precommittedSectors {
		// Check (and activate) storage deals associated to sector. Abort if checks failed.
		// TODO: we should batch these calls...
		// https://github.com/filecoin-project/specs-actors/issues/474
		code := rt.Send(
			builtin.StorageMarketActorAddr,
			builtin.MethodsMarket.ActivateDeals,
			&market.ActivateDealsParams{
				DealIDs:      precommit.Info.DealIDs,
				SectorExpiry: precommit.Info.Expiration,
			},
			abi.NewTokenAmount(0),
			&builtin.Discard{},
		)

		if code != exitcode.Ok {
			rt.Log(rtt.INFO, "failed to activate deals on sector %d, dropping from prove commit set", precommit.Info.SectorNumber)
			continue
		}

		preCommits = append(preCommits, precommit)

		if precommit.Info.ReplaceCapacity {
			err := replaceSectors.AddValues(
				precommit.Info.ReplaceSectorDeadline,
				precommit.Info.ReplaceSectorPartition,
				uint64(precommit.Info.ReplaceSectorNumber),
			)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "failed to record sectors for replacement")

		}
	}

	// When all prove commits have failed abort early
	if len(preCommits) == 0 {
		rt.Abortf(exitcode.ErrIllegalArgument, "all prove commits failed to validate")
	}

	var newPower PowerPair
	totalPledge := big.Zero()
	totalPrecommitDeposit := big.Zero()
	newSectors := make([]*SectorOnChainInfo, 0)
	newlyVested := big.Zero()
	rt.StateTransaction(&st, func() {
		// Schedule expiration for replaced sectors to the end of their next deadline window.
		// They can't be removed right now because we want to challenge them immediately before termination.
		err = st.RescheduleSectorExpirations(store, rt.CurrEpoch(), info.SectorSize, replaceSectors)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to replace sector expirations")

		newSectorNos := make([]abi.SectorNumber, 0, len(preCommits))
		for _, precommit := range preCommits {
			// compute initial pledge
			activation := rt.CurrEpoch()
			duration := precommit.Info.Expiration - activation

			// This should have been caught in precommit, but don't let other sectors fail because of it.
			if duration < MinSectorExpiration {
				rt.Log(rtt.WARN, "precommit %d has lifetime %d less than minimum. ignoring", precommit.Info.SectorNumber, duration, MinSectorExpiration)
				continue
			}

			power := QAPowerForWeight(info.SectorSize, duration, precommit.DealWeight, precommit.VerifiedDealWeight)
			dayReward := ExpectedRewardForPower(rewardStats.ThisEpochRewardSmoothed, pwrTotal.QualityAdjPowerSmoothed, power, builtin.EpochsInDay)
			// The storage pledge is recorded for use in computing the penalty if this sector is terminated
			// before its declared expiration.
			// It's not capped to 1 FIL for Space Race, so likely exceeds the actual initial pledge requirement.
			storagePledge := ExpectedRewardForPower(rewardStats.ThisEpochRewardSmoothed, pwrTotal.QualityAdjPowerSmoothed, power, InitialPledgeProjectionPeriod)

			initialPledge := InitialPledgeForPower(power, rewardStats.ThisEpochBaselinePower, pwrTotal.PledgeCollateral,
				rewardStats.ThisEpochRewardSmoothed, pwrTotal.QualityAdjPowerSmoothed, circulatingSupply)

			totalPrecommitDeposit = big.Add(totalPrecommitDeposit, precommit.PreCommitDeposit)
			totalPledge = big.Add(totalPledge, initialPledge)
			newSectorInfo := SectorOnChainInfo{
				SectorNumber:          precommit.Info.SectorNumber,
				SealProof:             precommit.Info.SealProof,
				SealedCID:             precommit.Info.SealedCID,
				DealIDs:               precommit.Info.DealIDs,
				Expiration:            precommit.Info.Expiration,
				Activation:            activation,
				DealWeight:            precommit.DealWeight,
				VerifiedDealWeight:    precommit.VerifiedDealWeight,
				InitialPledge:         initialPledge,
				ExpectedDayReward:     dayReward,
				ExpectedStoragePledge: storagePledge,
			}
			newSectors = append(newSectors, &newSectorInfo)
			newSectorNos = append(newSectorNos, newSectorInfo.SectorNumber)
		}

		err = st.PutSectors(store, newSectors...)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to put new sectors")

		err = st.DeletePrecommittedSectors(store, newSectorNos...)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to delete precommited sectors")

		newPower, err = st.AssignSectorsToDeadlines(store, rt.CurrEpoch(), newSectors, info.WindowPoStPartitionSectors, info.SectorSize)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to assign new sectors to deadlines")

		// Add sector and pledge lock-up to miner state
		newlyVested, err = st.UnlockVestedFunds(store, rt.CurrEpoch())
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to vest new funds: %s", err)
		}

		// Unlock deposit for successful proofs, make it available for lock-up as initial pledge.
		st.AddPreCommitDeposit(totalPrecommitDeposit.Neg())

		availableBalance := st.GetAvailableBalance(rt.CurrentBalance())
		if availableBalance.LessThan(totalPledge) {
			rt.Abortf(exitcode.ErrInsufficientFunds, "insufficient funds for aggregate initial pledge requirement %s, available: %s", totalPledge, availableBalance)
		}

		st.AddInitialPledgeRequirement(totalPledge)
		st.AssertBalanceInvariants(rt.CurrentBalance())
	})

	// Request power and pledge update for activated sector.
	requestUpdatePower(rt, newPower)
	notifyPledgeChanged(rt, big.Sub(totalPledge, newlyVested))

	return nil
}

type CheckSectorProvenParams struct {
	SectorNumber abi.SectorNumber
}

func (a Actor) CheckSectorProven(rt Runtime, params *CheckSectorProvenParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)
	store := adt.AsStore(rt)
	sectorNo := params.SectorNumber

	if _, found, err := st.GetSector(store, sectorNo); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to load proven sector %v", sectorNo)
	} else if !found {
		rt.Abortf(exitcode.ErrNotFound, "sector %v not proven", sectorNo)
	}
	return nil
}

/////////////////////////
// Sector Modification //
/////////////////////////

type ExtendSectorExpirationParams struct {
	Extensions []ExpirationExtension
}

type ExpirationExtension struct {
	Deadline      uint64
	Partition     uint64
	Sectors       bitfield.BitField
	NewExpiration abi.ChainEpoch
}

// Changes the expiration epoch for a sector to a new, later one.
// The sector must not be terminated or faulty.
// The sector's power is recomputed for the new expiration.
func (a Actor) ExtendSectorExpiration(rt Runtime, params *ExtendSectorExpirationParams) *abi.EmptyValue {
	if uint64(len(params.Extensions)) > AddressedPartitionsMax {
		rt.Abortf(exitcode.ErrIllegalArgument, "too many declarations %d, max %d", len(params.Extensions), AddressedPartitionsMax)
	}

	// limit the number of sectors declared at once
	// https://github.com/filecoin-project/specs-actors/issues/416
	var sectorCount uint64
	for _, decl := range params.Extensions {
		if decl.Deadline >= WPoStPeriodDeadlines {
			rt.Abortf(exitcode.ErrIllegalArgument, "deadline %d not in range 0..%d", decl.Deadline, WPoStPeriodDeadlines)
		}
		count, err := decl.Sectors.Count()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument,
			"failed to count sectors for deadline %d, partition %d",
			decl.Deadline, decl.Partition,
		)
		if sectorCount > math.MaxUint64-count {
			rt.Abortf(exitcode.ErrIllegalArgument, "sector bitfield integer overflow")
		}
		sectorCount += count
	}
	if sectorCount > AddressedSectorsMax {
		rt.Abortf(exitcode.ErrIllegalArgument,
			"too many sectors for declaration %d, max %d",
			sectorCount, AddressedSectorsMax,
		)
	}

	powerDelta := NewPowerPairZero()
	pledgeDelta := big.Zero()
	store := adt.AsStore(rt)
	var st State
	rt.StateTransaction(&st, func() {
		info := getMinerInfo(rt, &st)

		rt.ValidateImmediateCallerIs(append(info.ControlAddresses, info.Owner, info.Worker)...)

		deadlines, err := st.LoadDeadlines(adt.AsStore(rt))
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load deadlines")

		// Group declarations by deadline, and remember iteration order.
		declsByDeadline := map[uint64][]*ExpirationExtension{}
		var deadlinesToLoad []uint64
		for i := range params.Extensions {
			// Take a pointer to the value inside the slice, don't
			// take a reference to the temporary loop variable as it
			// will be overwritten every iteration.
			decl := &params.Extensions[i]
			if _, ok := declsByDeadline[decl.Deadline]; !ok {
				deadlinesToLoad = append(deadlinesToLoad, decl.Deadline)
			}
			declsByDeadline[decl.Deadline] = append(declsByDeadline[decl.Deadline], decl)
		}

		sectors, err := LoadSectors(store, st.Sectors)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load sectors array")

		for _, dlIdx := range deadlinesToLoad {
			deadline, err := deadlines.LoadDeadline(store, dlIdx)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load deadline %d", dlIdx)

			partitions, err := deadline.PartitionsArray(store)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load partitions for deadline %d", dlIdx)

			quant := st.QuantSpecForDeadline(dlIdx)

			for _, decl := range declsByDeadline[dlIdx] {
				key := PartitionKey{dlIdx, decl.Partition}
				var partition Partition
				found, err := partitions.Get(decl.Partition, &partition)
				builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load partition %v", key)
				if !found {
					rt.Abortf(exitcode.ErrNotFound, "no such partition %v", key)
				}

				oldSectors, err := sectors.Load(decl.Sectors)
				builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load sectors")
				newSectors := make([]*SectorOnChainInfo, len(oldSectors))
				for i, sector := range oldSectors {
					if decl.NewExpiration < sector.Expiration {
						rt.Abortf(exitcode.ErrIllegalArgument, "cannot reduce sector expiration to %d from %d",
							decl.NewExpiration, sector.Expiration)
					}
					validateExpiration(rt, sector.Activation, decl.NewExpiration, sector.SealProof)

					newSector := *sector
					newSector.Expiration = decl.NewExpiration

					newSectors[i] = &newSector
				}

				// Overwrite sector infos.
				err = sectors.Store(newSectors...)
				builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to update sectors %v", decl.Sectors)

				// Remove old sectors from partition and assign new sectors.
				partitionPowerDelta, partitionPledgeDelta, err := partition.ReplaceSectors(store, oldSectors, newSectors, info.SectorSize, quant)
				builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to replaces sector expirations at %v", key)

				powerDelta = powerDelta.Add(partitionPowerDelta)
				pledgeDelta = big.Add(pledgeDelta, partitionPledgeDelta) // expected to be zero, see note below.

				err = partitions.Set(decl.Partition, &partition)
				builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save partition", key)
			}

			deadline.Partitions, err = partitions.Root()
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save partitions for deadline %d", dlIdx)

			err = deadlines.UpdateDeadline(store, dlIdx, deadline)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save deadline %d", dlIdx)
		}

		st.Sectors, err = sectors.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save sectors")

		err = st.SaveDeadlines(store, deadlines)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save deadlines")
	})

	requestUpdatePower(rt, powerDelta)
	// Note: the pledge delta is expected to be zero, since pledge is not re-calculated for the extension.
	// But in case that ever changes, we can do the right thing here.
	notifyPledgeChanged(rt, pledgeDelta)
	return nil
}

type TerminateSectorsParams struct {
	Terminations []TerminationDeclaration
}

type TerminationDeclaration struct {
	Deadline  uint64
	Partition uint64
	Sectors   bitfield.BitField
}

type TerminateSectorsReturn struct {
	// Set to true if all early termination work has been completed. When
	// false, the miner may choose to repeatedly invoke TerminateSectors
	// with no new sectors to process the remainder of the pending
	// terminations. While pending terminations are outstanding, the miner
	// will not be able to withdraw funds.
	Done bool
}

// Marks some sectors as terminated at the present epoch, earlier than their
// scheduled termination, and adds these sectors to the early termination queue.
// This method then processes up to AddressedSectorsMax sectors and
// AddressedPartitionsMax partitions from the early termination queue,
// terminating deals, paying fines, and returning pledge collateral. While
// sectors remain in this queue:
//
//  1. The miner will be unable to withdraw funds.
//  2. The chain will process up to AddressedSectorsMax sectors and
//     AddressedPartitionsMax per epoch until the queue is empty.
//
// The sectors are immediately ignored for Window PoSt proofs, and should be
// masked in the same way as faulty sectors. A miner terminating sectors in the
// current deadline must be careful to compute an appropriate Window PoSt proof
// for the sectors that will be active at the time the PoSt is submitted.
//
// This function may be invoked with no new sectors to explicitly process the
// next batch of sectors.
func (a Actor) TerminateSectors(rt Runtime, params *TerminateSectorsParams) *TerminateSectorsReturn {
	// Note: this cannot terminate pre-committed but un-proven sectors.
	// They must be allowed to expire (and deposit burnt).

	toProcess := make(DeadlineSectorMap)
	for _, term := range params.Terminations {
		err := toProcess.Add(term.Deadline, term.Partition, term.Sectors)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument,
			"failed to process deadline %d, partition %d", term.Deadline, term.Partition,
		)
	}
	err := toProcess.Check(AddressedPartitionsMax, AddressedSectorsMax)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "cannot process requested parameters")

	var hadEarlyTerminations bool
	var st State
	store := adt.AsStore(rt)
	currEpoch := rt.CurrEpoch()
	powerDelta := NewPowerPairZero()
	rt.StateTransaction(&st, func() {
		hadEarlyTerminations = havePendingEarlyTerminations(rt, &st)

		info := getMinerInfo(rt, &st)
		rt.ValidateImmediateCallerIs(append(info.ControlAddresses, info.Owner, info.Worker)...)

		deadlines, err := st.LoadDeadlines(adt.AsStore(rt))
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load deadlines")

		// We're only reading the sectors, so there's no need to save this back.
		// However, we still want to avoid re-loading this array per-partition.
		sectors, err := LoadSectors(store, st.Sectors)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load sectors")

		err = toProcess.ForEach(func(dlIdx uint64, partitionSectors PartitionSectorMap) error {
			quant := st.QuantSpecForDeadline(dlIdx)

			deadline, err := deadlines.LoadDeadline(store, dlIdx)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load deadline %d", dlIdx)

			removedPower, err := deadline.TerminateSectors(store, sectors, currEpoch, partitionSectors, info.SectorSize, quant)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to terminate sectors in deadline %d", dlIdx)

			st.EarlyTerminations.Set(dlIdx)

			powerDelta = powerDelta.Sub(removedPower)

			err = deadlines.UpdateDeadline(store, dlIdx, deadline)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to update deadline %d", dlIdx)

			return nil
		})
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to walk sectors")

		err = st.SaveDeadlines(store, deadlines)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save deadlines")
	})

	// Now, try to process these sectors.
	more := processEarlyTerminations(rt)
	if more && !hadEarlyTerminations {
		// We have remaining terminations, and we didn't _previously_
		// have early terminations to process, schedule a cron job.
		// NOTE: This isn't quite correct. If we repeatedly fill, empty,
		// fill, and empty, the queue, we'll keep scheduling new cron
		// jobs. However, in practice, that shouldn't be all that bad.
		scheduleEarlyTerminationWork(rt)
	}

	requestUpdatePower(rt, powerDelta)

	return &TerminateSectorsReturn{Done: !more}
}

////////////
// Faults //
////////////

type DeclareFaultsParams struct {
	Faults []FaultDeclaration
}

type FaultDeclaration struct {
	// The deadline to which the faulty sectors are assigned, in range [0..WPoStPeriodDeadlines)
	Deadline uint64
	// Partition index within the deadline containing the faulty sectors.
	Partition uint64
	// Sectors in the partition being declared faulty.
	Sectors bitfield.BitField
}

func (a Actor) DeclareFaults(rt Runtime, params *DeclareFaultsParams) *abi.EmptyValue {
	toProcess := make(DeadlineSectorMap)
	for _, term := range params.Faults {
		err := toProcess.Add(term.Deadline, term.Partition, term.Sectors)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument,
			"failed to process deadline %d, partition %d", term.Deadline, term.Partition,
		)
	}
	err := toProcess.Check(AddressedPartitionsMax, AddressedSectorsMax)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "cannot process requested parameters")

	store := adt.AsStore(rt)
	var st State
	newFaultPowerTotal := NewPowerPairZero()
	rt.StateTransaction(&st, func() {
		info := getMinerInfo(rt, &st)
		rt.ValidateImmediateCallerIs(append(info.ControlAddresses, info.Owner, info.Worker)...)

		deadlines, err := st.LoadDeadlines(store)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load deadlines")

		sectors, err := LoadSectors(store, st.Sectors)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load sectors array")

		err = toProcess.ForEach(func(dlIdx uint64, pm PartitionSectorMap) error {
			targetDeadline, err := declarationDeadlineInfo(st.ProvingPeriodStart, dlIdx, rt.CurrEpoch())
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "invalid fault declaration deadline %d", dlIdx)

			err = validateFRDeclarationDeadline(targetDeadline)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "failed fault declaration at deadline %d", dlIdx)

			deadline, err := deadlines.LoadDeadline(store, dlIdx)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load deadline %d", dlIdx)

			faultExpirationEpoch := targetDeadline.Last() + FaultMaxAge
			newFaultyPower, err := deadline.DeclareFaults(store, sectors, info.SectorSize, QuantSpecForDeadline(targetDeadline), faultExpirationEpoch, pm)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to declare faults for deadline %d", dlIdx)

			err = deadlines.UpdateDeadline(store, dlIdx, deadline)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to store deadline %d partitions", dlIdx)

			newFaultPowerTotal = newFaultPowerTotal.Add(newFaultyPower)
			return nil
		})
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to iterate deadlines")

		err = st.SaveDeadlines(store, deadlines)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save deadlines")
	})

	// Remove power for new faulty sectors.
	// NOTE: It would be permissible to delay the power loss until the deadline closes, but that would require
	// additional accounting state.
	// https://github.com/filecoin-project/specs-actors/issues/414
	requestUpdatePower(rt, newFaultPowerTotal.Neg())

	// Payment of penalty for declared faults is deferred to the deadline cron.
	return nil
}

type DeclareFaultsRecoveredParams struct {
	Recoveries []RecoveryDeclaration
}

type RecoveryDeclaration struct {
	// The deadline to which the recovered sectors are assigned, in range [0..WPoStPeriodDeadlines)
	Deadline uint64
	// Partition index within the deadline containing the recovered sectors.
	Partition uint64
	// Sectors in the partition being declared recovered.
	Sectors bitfield.BitField
}

func (a Actor) DeclareFaultsRecovered(rt Runtime, params *DeclareFaultsRecoveredParams) *abi.EmptyValue {
	toProcess := make(DeadlineSectorMap)
	for _, term := range params.Recoveries {
		err := toProcess.Add(term.Deadline, term.Partition, term.Sectors)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument,
			"failed to process deadline %d, partition %d", term.Deadline, term.Partition,
		)
	}
	err := toProcess.Check(AddressedPartitionsMax, AddressedSectorsMax)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "cannot process requested parameters")

	store := adt.AsStore(rt)
	var st State
	rt.StateTransaction(&st, func() {
		info := getMinerInfo(rt, &st)
		rt.ValidateImmediateCallerIs(append(info.ControlAddresses, info.Owner, info.Worker)...)

		deadlines, err := st.LoadDeadlines(adt.AsStore(rt))
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load deadlines")

		sectors, err := LoadSectors(store, st.Sectors)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load sectors array")

		err = toProcess.ForEach(func(dlIdx uint64, pm PartitionSectorMap) error {
			targetDeadline, err := declarationDeadlineInfo(st.ProvingPeriodStart, dlIdx, rt.CurrEpoch())
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "invalid recovery declaration deadline %d", dlIdx)
			err = validateFRDeclarationDeadline(targetDeadline)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "failed recovery declaration at deadline %d", dlIdx)

			deadline, err := deadlines.LoadDeadline(store, dlIdx)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load deadline %d", dlIdx)

			err = deadline.DeclareFaultsRecovered(store, sectors, info.SectorSize, pm)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to declare recoveries for deadline %d", dlIdx)

			err = deadlines.UpdateDeadline(store, dlIdx, deadline)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to store deadline %d", dlIdx)
			return nil
		})
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to walk sectors")

		err = st.SaveDeadlines(store, deadlines)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save deadlines")
	})

	// Power is not restored yet, but when the recovered sectors are successfully PoSted.
	return nil
}

/////////////////
// Maintenance //
/////////////////

type CompactPartitionsParams struct {
	Deadline   uint64
	Partitions bitfield.BitField
}

// Compacts a number of partitions at one deadline by removing terminated sectors, re-ordering the remaining sectors,
// and assigning them to new partitions so as to completely fill all but one partition with live sectors.
// The addressed partitions are removed from the deadline, and new ones appended.
// The final partition in the deadline is always included in the compaction, whether or not explicitly requested.
// Removed sectors are removed from state entirely.
// May not be invoked if the deadline has any un-processed early terminations.
func (a Actor) CompactPartitions(rt Runtime, params *CompactPartitionsParams) *abi.EmptyValue {
	if params.Deadline >= WPoStPeriodDeadlines {
		rt.Abortf(exitcode.ErrIllegalArgument, "invalid deadline %v", params.Deadline)
	}

	partitionCount, err := params.Partitions.Count()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "failed to parse partitions bitfield")

	store := adt.AsStore(rt)
	var st State
	rt.StateTransaction(&st, func() {
		info := getMinerInfo(rt, &st)
		rt.ValidateImmediateCallerIs(append(info.ControlAddresses, info.Owner, info.Worker)...)

		if !deadlineIsMutable(st.ProvingPeriodStart, params.Deadline, rt.CurrEpoch()) {
			rt.Abortf(exitcode.ErrForbidden,
				"cannot compact deadline %d during its challenge window or the prior challenge window", params.Deadline)
		}

		submissionPartitionLimit := loadPartitionsSectorsMax(info.WindowPoStPartitionSectors)
		if partitionCount > submissionPartitionLimit {
			rt.Abortf(exitcode.ErrIllegalArgument, "too many partitions %d, limit %d", partitionCount, submissionPartitionLimit)
		}

		quant := st.QuantSpecForDeadline(params.Deadline)

		deadlines, err := st.LoadDeadlines(store)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load deadlines")

		deadline, err := deadlines.LoadDeadline(store, params.Deadline)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load deadline %d", params.Deadline)

		live, dead, removedPower, err := deadline.RemovePartitions(store, params.Partitions, quant)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to remove partitions from deadline %d", params.Deadline)

		err = st.DeleteSectors(store, dead)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to delete dead sectors")

		sectors, err := st.LoadSectorInfos(store, live)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load moved sectors")

		newPower, err := deadline.AddSectors(store, info.WindowPoStPartitionSectors, sectors, info.SectorSize, quant)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add back moved sectors")

		if !removedPower.Equals(newPower) {
			rt.Abortf(exitcode.ErrIllegalState, "power changed when compacting partitions: was %v, is now %v", removedPower, newPower)
		}
	})
	return nil
}

type CompactSectorNumbersParams struct {
	MaskSectorNumbers bitfield.BitField
}

// Compacts sector number allocations to reduce the size of the allocated sector
// number bitfield.
//
// When allocating sector numbers sequentially, or in sequential groups, this
// bitfield should remain fairly small. However, if the bitfield grows large
// enough such that PreCommitSector fails (or becomes expensive), this method
// can be called to mask out (throw away) entire ranges of unused sector IDs.
// For example, if sectors 1-99 and 101-200 have been allocated, sector number
// 99 can be masked out to collapse these two ranges into one.
func (a Actor) CompactSectorNumbers(rt Runtime, params *CompactSectorNumbersParams) *abi.EmptyValue {
	lastSectorNo, err := params.MaskSectorNumbers.Last()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "invalid mask bitfield")
	if lastSectorNo > abi.MaxSectorNumber {
		rt.Abortf(exitcode.ErrIllegalArgument, "masked sector number %d exceeded max sector number", lastSectorNo)
	}

	store := adt.AsStore(rt)
	var st State
	rt.StateTransaction(&st, func() {
		info := getMinerInfo(rt, &st)
		rt.ValidateImmediateCallerIs(append(info.ControlAddresses, info.Owner, info.Worker)...)

		err := st.MaskSectorNumbers(store, params.MaskSectorNumbers)

		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to mask sector numbers")
	})
	return nil
}

///////////////////////
// Pledge Collateral //
///////////////////////

// Locks up some amount of the miner's unlocked balance (including funds received alongside the invoking message).
func (a Actor) AddLockedFund(rt Runtime, amountToLock *abi.TokenAmount) *abi.EmptyValue {
	if amountToLock.Sign() < 0 {
		rt.Abortf(exitcode.ErrIllegalArgument, "cannot lock up a negative amount of funds")
	}

	vestingSchedule := &RewardVestingSpecV0
	if rt.NetworkVersion() >= network.Version1 {
		vestingSchedule = &RewardVestingSpecV1
	}

	var st State
	newlyVested := big.Zero()
	rt.StateTransaction(&st, func() {
		var err error
		info := getMinerInfo(rt, &st)
		rt.ValidateImmediateCallerIs(append(info.ControlAddresses, info.Owner, info.Worker, builtin.RewardActorAddr)...)

		// This may lock up unlocked balance that was covering InitialPledgeRequirements
		// This ensures that the amountToLock is always locked up if the miner account
		// can cover it.
		unlockedBalance := st.GetUnlockedBalance(rt.CurrentBalance())
		if unlockedBalance.LessThan(*amountToLock) {
			rt.Abortf(exitcode.ErrInsufficientFunds, "insufficient funds to lock, available: %v, requested: %v", unlockedBalance, *amountToLock)
		}

		newlyVested, err = st.AddLockedFunds(adt.AsStore(rt), rt.CurrEpoch(), *amountToLock, vestingSchedule)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to lock funds in vesting table")
	})

	notifyPledgeChanged(rt, big.Sub(*amountToLock, newlyVested))

	return nil
}

type ReportConsensusFaultParams struct {
	BlockHeader1     []byte
	BlockHeader2     []byte
	BlockHeaderExtra []byte
}

func (a Actor) ReportConsensusFault(rt Runtime, params *ReportConsensusFaultParams) *abi.EmptyValue {
	// Note: only the first reporter of any fault is rewarded.
	// Subsequent invocations fail because the target miner has been removed.
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)
	reporter := rt.Caller()

	fault, err := rt.VerifyConsensusFault(params.BlockHeader1, params.BlockHeader2, params.BlockHeaderExtra)
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalArgument, "fault not verified: %s", err)
	}

	// Elapsed since the fault (i.e. since the higher of the two blocks)
	faultAge := rt.CurrEpoch() - fault.Epoch
	if faultAge <= 0 {
		rt.Abortf(exitcode.ErrIllegalArgument, "invalid fault epoch %v ahead of current %v", fault.Epoch, rt.CurrEpoch())
	}

	// Reward reporter with a share of the miner's current balance.
	slasherReward := RewardForConsensusSlashReport(faultAge, rt.CurrentBalance())
	code := rt.Send(reporter, builtin.MethodSend, nil, slasherReward, &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to reward reporter")

	var st State
	rt.StateReadonly(&st)

	// Notify power actor with lock-up total being removed.
	code = rt.Send(
		builtin.StoragePowerActorAddr,
		builtin.MethodsPower.OnConsensusFault,
		&st.LockedFunds,
		abi.NewTokenAmount(0),
		&builtin.Discard{},
	)
	builtin.RequireSuccess(rt, code, "failed to notify power actor on consensus fault")

	// close deals and burn funds
	terminateMiner(rt)

	return nil
}

type WithdrawBalanceParams struct {
	AmountRequested abi.TokenAmount
}

func (a Actor) WithdrawBalance(rt Runtime, params *WithdrawBalanceParams) *abi.EmptyValue {
	var st State
	if params.AmountRequested.LessThan(big.Zero()) {
		rt.Abortf(exitcode.ErrIllegalArgument, "negative fund requested for withdrawal: %s", params.AmountRequested)
	}
	var info *MinerInfo
	newlyVested := big.Zero()
	rt.StateTransaction(&st, func() {
		var err error
		info = getMinerInfo(rt, &st)
		// Only the owner is allowed to withdraw the balance as it belongs to/is controlled by the owner
		// and not the worker.
		rt.ValidateImmediateCallerIs(info.Owner)
		// Ensure we don't have any pending terminations.
		if count, err := st.EarlyTerminations.Count(); err != nil {
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to count early terminations")
		} else if count > 0 {
			rt.Abortf(exitcode.ErrForbidden,
				"cannot withdraw funds while %d deadlines have terminated sectors with outstanding fees",
				count,
			)
		}

		// Unlock vested funds so we can spend them.
		newlyVested, err = st.UnlockVestedFunds(adt.AsStore(rt), rt.CurrEpoch())
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to vest fund: %v", err)
		}

		// Verify InitialPledgeRequirement does not exceed unlocked funds
		verifyPledgeMeetsInitialRequirements(rt, &st)
	})

	currBalance := rt.CurrentBalance()
	amountWithdrawn := big.Min(st.GetAvailableBalance(currBalance), params.AmountRequested)
	Assert(amountWithdrawn.GreaterThanEqual(big.Zero()))
	Assert(amountWithdrawn.LessThanEqual(currBalance))

	code := rt.Send(info.Owner, builtin.MethodSend, nil, amountWithdrawn, &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to withdraw balance")

	pledgeDelta := newlyVested.Neg()
	notifyPledgeChanged(rt, pledgeDelta)

	st.AssertBalanceInvariants(rt.CurrentBalance())
	return nil
}

//////////
// Cron //
//////////

func (a Actor) OnDeferredCronEvent(rt Runtime, payload *CronEventPayload) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.StoragePowerActorAddr)

	switch payload.EventType {
	case CronEventProvingDeadline:
		handleProvingDeadline(rt)
	case CronEventWorkerKeyChange:
		commitWorkerKeyChange(rt)
	case CronEventProcessEarlyTerminations:
		if processEarlyTerminations(rt) {
			scheduleEarlyTerminationWork(rt)
		}
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////
// Utility functions & helpers
////////////////////////////////////////////////////////////////////////////////

func processEarlyTerminations(rt Runtime) (more bool) {
	store := adt.AsStore(rt)
	networkVersion := rt.NetworkVersion()

	// TODO: We're using the current power+epoch reward. Technically, we
	// should use the power/reward at the time of termination.
	// https://github.com/filecoin-project/specs-actors/pull/648
	rewardStats := requestCurrentEpochBlockReward(rt)
	pwrTotal := requestCurrentTotalPower(rt)

	var (
		result           TerminationResult
		dealsToTerminate []market.OnMinerSectorsTerminateParams
		penalty          = big.Zero()
		pledgeDelta      = big.Zero()
	)

	var st State
	rt.StateTransaction(&st, func() {
		var err error
		result, more, err = st.PopEarlyTerminations(store, AddressedPartitionsMax, AddressedSectorsMax)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to pop early terminations")

		// Nothing to do, don't waste any time.
		// This can happen if we end up processing early terminations
		// before the cron callback fires.
		if result.IsEmpty() {
			return
		}

		info := getMinerInfo(rt, &st)

		sectors, err := LoadSectors(store, st.Sectors)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load sectors array")

		totalInitialPledge := big.Zero()
		dealsToTerminate = make([]market.OnMinerSectorsTerminateParams, 0, len(result.Sectors))
		err = result.ForEach(func(epoch abi.ChainEpoch, sectorNos bitfield.BitField) error {
			sectors, err := sectors.Load(sectorNos)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load sector infos")
			params := market.OnMinerSectorsTerminateParams{
				Epoch:   epoch,
				DealIDs: make([]abi.DealID, 0, len(sectors)), // estimate ~one deal per sector.
			}
			for _, sector := range sectors {
				params.DealIDs = append(params.DealIDs, sector.DealIDs...)
				totalInitialPledge = big.Add(totalInitialPledge, sector.InitialPledge)
			}
			penalty = big.Add(penalty, terminationPenalty(info.SectorSize, epoch, networkVersion,
				rewardStats.ThisEpochRewardSmoothed, pwrTotal.QualityAdjPowerSmoothed, sectors))
			dealsToTerminate = append(dealsToTerminate, params)

			return nil
		})
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to process terminations")

		// Unlock funds for penalties.
		// TODO: handle bankrupt miner: https://github.com/filecoin-project/specs-actors/issues/627
		// We're intentionally reducing the penalty paid to what we have.
		unlockedBalance := st.GetUnlockedBalance(rt.CurrentBalance())
		penaltyFromVesting, penaltyFromBalance, err := st.PenalizeFundsInPriorityOrder(store, rt.CurrEpoch(), penalty, unlockedBalance)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to unlock unvested funds")
		penalty = big.Add(penaltyFromVesting, penaltyFromBalance)

		// Remove pledge requirement.
		st.AddInitialPledgeRequirement(totalInitialPledge.Neg())
		pledgeDelta = big.Add(totalInitialPledge, penaltyFromVesting).Neg()
	})

	// We didn't do anything, abort.
	if result.IsEmpty() {
		return more
	}

	// Burn penalty.
	burnFunds(rt, penalty)

	// Return pledge.
	notifyPledgeChanged(rt, pledgeDelta)

	// Terminate deals.
	for _, params := range dealsToTerminate {
		requestTerminateDeals(rt, params.Epoch, params.DealIDs)
	}

	// reschedule cron worker, if necessary.
	return more
}

// Invoked at the end of the last epoch for each proving deadline.
func handleProvingDeadline(rt Runtime) {
	currEpoch := rt.CurrEpoch()
	store := adt.AsStore(rt)
	networkVersion := rt.NetworkVersion()

	epochReward := requestCurrentEpochBlockReward(rt)
	pwrTotal := requestCurrentTotalPower(rt)

	hadEarlyTerminations := false

	powerDelta := PowerPair{big.Zero(), big.Zero()}
	penaltyTotal := abi.NewTokenAmount(0)
	pledgeDelta := abi.NewTokenAmount(0)

	var st State
	rt.StateTransaction(&st, func() {
		var err error
		{
			// Vest locked funds.
			// This happens first so that any subsequent penalties are taken
			// from locked vesting funds before funds free this epoch.
			newlyVested, err := st.UnlockVestedFunds(store, rt.CurrEpoch())
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to vest funds")
			pledgeDelta = big.Add(pledgeDelta, newlyVested.Neg())
		}

		{
			// expire pre-committed sectors
			expiryQ, err := LoadBitfieldQueue(store, st.PreCommittedSectorsExpiry, st.QuantSpecEveryDeadline())
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load sector expiry queue")

			bf, modified, err := expiryQ.PopUntil(currEpoch)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to pop expired sectors")

			if modified {
				st.PreCommittedSectorsExpiry, err = expiryQ.Root()
				builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save expiry queue")
			}

			depositToBurn, err := st.checkPrecommitExpiry(store, bf)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to expire pre-committed sectors")
			penaltyTotal = big.Add(penaltyTotal, depositToBurn)
		}

		// Record whether or not we _had_ early terminations in the queue before this method.
		// That way, don't re-schedule a cron callback if one is already scheduled.
		hadEarlyTerminations = havePendingEarlyTerminations(rt, &st)

		// Note: because the cron actor is not invoked on epochs with empty tipsets, the current epoch is not necessarily
		// exactly the final epoch of the deadline; it may be slightly later (i.e. in the subsequent deadline/period).
		// Further, this method is invoked once *before* the first proving period starts, after the actor is first
		// constructed; this is detected by !dlInfo.PeriodStarted().
		// Use dlInfo.PeriodEnd() rather than rt.CurrEpoch unless certain of the desired semantics.
		dlInfo := st.DeadlineInfo(currEpoch)
		if !dlInfo.PeriodStarted() {
			return // Skip checking faults on the first, incomplete period.
		}
		deadlines, err := st.LoadDeadlines(store)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load deadlines")
		deadline, err := deadlines.LoadDeadline(store, dlInfo.Index)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load deadline %d", dlInfo.Index)
		quant := QuantSpecForDeadline(dlInfo)
		unlockedBalance := st.GetUnlockedBalance(rt.CurrentBalance())

		// Remember power that was faulty before processing any missed PoSts.
		previouslyFaultyPower := deadline.FaultyPower.QA

		{
			// Detect and penalize missing proofs.
			faultExpiration := dlInfo.Last() + FaultMaxAge

			newFaultyPower, failedRecoveryPower, err := deadline.ProcessDeadlineEnd(store, quant, faultExpiration)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to process end of deadline %d", dlInfo.Index)

			powerDelta = powerDelta.Sub(newFaultyPower)

			if networkVersion >= network.Version3 {
				// From network version 3, faults detected from a missed PoSt pay nothing.
				// Failed recoveries pay nothing here, but will pay the ongoing fault fee in the subsequent block.
			} else {
				penalizePowerTotal := big.Add(newFaultyPower.QA, failedRecoveryPower.QA)

				// Unlock sector penalty for all undeclared faults.
				penaltyTarget := PledgePenaltyForUndeclaredFault(epochReward.ThisEpochRewardSmoothed, pwrTotal.QualityAdjPowerSmoothed,
					penalizePowerTotal, rt.NetworkVersion())
				// Subtract the "ongoing" fault fee from the amount charged now, since it will be added on just below.
				penaltyTarget = big.Sub(penaltyTarget, PledgePenaltyForDeclaredFault(epochReward.ThisEpochRewardSmoothed,
					pwrTotal.QualityAdjPowerSmoothed, penalizePowerTotal, networkVersion))
				penaltyFromVesting, penaltyFromBalance, err := st.PenalizeFundsInPriorityOrder(store, currEpoch, penaltyTarget, unlockedBalance)
				builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to unlock penalty")
				unlockedBalance = big.Sub(unlockedBalance, penaltyFromBalance)
				penaltyTotal = big.Sum(penaltyTotal, penaltyFromVesting, penaltyFromBalance)
				pledgeDelta = big.Sub(pledgeDelta, penaltyFromVesting)
			}
		}
		{
			// Record faulty power for penalisation of ongoing faults, before popping expirations.
			// This includes any power that was just faulted from missing a PoSt.
			ongoingFaultyPower := deadline.FaultyPower.QA
			if networkVersion >= network.Version3 {
				// From network version 3, this *excludes* any power that was just faulted from missing a PoSt.
				// It includes power that was previously declared, skipped, or detected faulty, whether or
				// not it is also marked for recovery.
				ongoingFaultyPower = previouslyFaultyPower
			}
			penaltyTarget := PledgePenaltyForDeclaredFault(epochReward.ThisEpochRewardSmoothed,
				pwrTotal.QualityAdjPowerSmoothed, ongoingFaultyPower, networkVersion)
			penaltyFromVesting, penaltyFromBalance, err := st.PenalizeFundsInPriorityOrder(store, currEpoch, penaltyTarget, unlockedBalance)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to unlock penalty")
			unlockedBalance = big.Sub(unlockedBalance, penaltyFromBalance) //nolint:ineffassign
			penaltyTotal = big.Sum(penaltyTotal, penaltyFromVesting, penaltyFromBalance)
			pledgeDelta = big.Sub(pledgeDelta, penaltyFromVesting)
		}
		{
			// Expire sectors that are due, either for on-time expiration or "early" faulty-for-too-long.
			expired, err := deadline.PopExpiredSectors(store, dlInfo.Last(), quant)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load expired sectors")

			// Release pledge requirements for the sectors expiring on-time.
			// Pledge for the sectors expiring early is retained to support the termination fee that will be assessed
			// when the early termination is processed.
			pledgeDelta = big.Sub(pledgeDelta, expired.OnTimePledge)
			st.AddInitialPledgeRequirement(expired.OnTimePledge.Neg())

			// Record reduction in power of the amount of expiring active power.
			// Faulty power has already been lost, so the amount expiring can be excluded from the delta.
			powerDelta = powerDelta.Sub(expired.ActivePower)

			// Record deadlines with early terminations. While this
			// bitfield is non-empty, the miner is locked until they
			// pay the fee.
			noEarlyTerminations, err := expired.EarlySectors.IsEmpty()
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to count early terminations")
			if !noEarlyTerminations {
				st.EarlyTerminations.Set(dlInfo.Index)
			}

			// The termination fee is paid later, in early-termination queue processing.
			// We could charge at least the undeclared fault fee here, which is a lower bound on the penalty.
			// https://github.com/filecoin-project/specs-actors/issues/674

			// The deals are not terminated yet, that is left for processing of the early termination queue.
		}

		// Save new deadline state.
		err = deadlines.UpdateDeadline(store, dlInfo.Index, deadline)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to update deadline %d", dlInfo.Index)

		err = st.SaveDeadlines(store, deadlines)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save deadlines")

		// Increment current deadline, and proving period if necessary.
		if dlInfo.PeriodStarted() {
			st.CurrentDeadline = (st.CurrentDeadline + 1) % WPoStPeriodDeadlines
			if st.CurrentDeadline == 0 {
				st.ProvingPeriodStart = st.ProvingPeriodStart + WPoStProvingPeriod
			}
		}
	})

	// Remove power for new faults, and burn penalties.
	requestUpdatePower(rt, powerDelta)
	burnFunds(rt, penaltyTotal)
	notifyPledgeChanged(rt, pledgeDelta)

	// Schedule cron callback for next deadline's last epoch.
	newDlInfo := st.DeadlineInfo(currEpoch)
	enrollCronEvent(rt, newDlInfo.Last(), &CronEventPayload{
		EventType: CronEventProvingDeadline,
	})

	// Record whether or not we _have_ early terminations now.
	hasEarlyTerminations := havePendingEarlyTerminations(rt, &st)

	// If we didn't have pending early terminations before, but we do now,
	// handle them at the next epoch.
	if !hadEarlyTerminations && hasEarlyTerminations {
		// First, try to process some of these terminations.
		if processEarlyTerminations(rt) {
			// If that doesn't work, just defer till the next epoch.
			scheduleEarlyTerminationWork(rt)
		}
		// Note: _don't_ process early terminations if we had a cron
		// callback already scheduled. In that case, we'll already have
		// processed AddressedSectorsMax terminations this epoch.
	}
}

// Check expiry is exactly *the epoch before* the start of a proving period.
func validateExpiration(rt Runtime, activation, expiration abi.ChainEpoch, sealProof abi.RegisteredSealProof) {
	// expiration cannot be less than minimum after activation
	if expiration-activation < MinSectorExpiration {
		rt.Abortf(exitcode.ErrIllegalArgument, "invalid expiration %d, total sector lifetime (%d) must exceed %d after activation %d",
			expiration, expiration-activation, MinSectorExpiration, activation)
	}

	// expiration cannot exceed MaxSectorExpirationExtension from now
	if expiration > rt.CurrEpoch()+MaxSectorExpirationExtension {
		rt.Abortf(exitcode.ErrIllegalArgument, "invalid expiration %d, cannot be more than %d past current epoch %d",
			expiration, MaxSectorExpirationExtension, rt.CurrEpoch())
	}

	// total sector lifetime cannot exceed SectorMaximumLifetime for the sector's seal proof
	maxLifetime, err := builtin.SealProofSectorMaximumLifetime(sealProof)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "unknown seal proof %d", sealProof)
	if expiration-activation > maxLifetime {
		rt.Abortf(exitcode.ErrIllegalArgument, "invalid expiration %d, total sector lifetime (%d) cannot exceed %d after activation %d",
			expiration, expiration-activation, maxLifetime, activation)
	}
}

func validateReplaceSector(rt Runtime, st *State, store adt.Store, params *SectorPreCommitInfo) *SectorOnChainInfo {
	replaceSector, found, err := st.GetSector(store, params.ReplaceSectorNumber)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load sector %v", params.SectorNumber)
	if !found {
		rt.Abortf(exitcode.ErrNotFound, "no such sector %v to replace", params.ReplaceSectorNumber)
	}

	if len(replaceSector.DealIDs) > 0 {
		rt.Abortf(exitcode.ErrIllegalArgument, "cannot replace sector %v which has deals", params.ReplaceSectorNumber)
	}
	if params.SealProof != replaceSector.SealProof {
		rt.Abortf(exitcode.ErrIllegalArgument, "cannot replace sector %v seal proof %v with seal proof %v",
			params.ReplaceSectorNumber, replaceSector.SealProof, params.SealProof)
	}
	if params.Expiration < replaceSector.Expiration {
		rt.Abortf(exitcode.ErrIllegalArgument, "cannot replace sector %v expiration %v with sooner expiration %v",
			params.ReplaceSectorNumber, replaceSector.Expiration, params.Expiration)
	}

	err = st.CheckSectorHealth(store, params.ReplaceSectorDeadline, params.ReplaceSectorPartition, params.ReplaceSectorNumber)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to replace sector %v", params.ReplaceSectorNumber)

	return replaceSector
}

func enrollCronEvent(rt Runtime, eventEpoch abi.ChainEpoch, callbackPayload *CronEventPayload) {
	payload := new(bytes.Buffer)
	err := callbackPayload.MarshalCBOR(payload)
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalArgument, "failed to serialize payload: %v", err)
	}
	code := rt.Send(
		builtin.StoragePowerActorAddr,
		builtin.MethodsPower.EnrollCronEvent,
		&power.EnrollCronEventParams{
			EventEpoch: eventEpoch,
			Payload:    payload.Bytes(),
		},
		abi.NewTokenAmount(0),
		&builtin.Discard{},
	)
	builtin.RequireSuccess(rt, code, "failed to enroll cron event")
}

func requestUpdatePower(rt Runtime, delta PowerPair) {
	if delta.IsZero() {
		return
	}
	code := rt.Send(
		builtin.StoragePowerActorAddr,
		builtin.MethodsPower.UpdateClaimedPower,
		&power.UpdateClaimedPowerParams{
			RawByteDelta:         delta.Raw,
			QualityAdjustedDelta: delta.QA,
		},
		abi.NewTokenAmount(0),
		&builtin.Discard{},
	)
	builtin.RequireSuccess(rt, code, "failed to update power with %v", delta)
}

func requestTerminateDeals(rt Runtime, epoch abi.ChainEpoch, dealIDs []abi.DealID) {
	for len(dealIDs) > 0 {
		size := min64(cbg.MaxLength, uint64(len(dealIDs)))
		code := rt.Send(
			builtin.StorageMarketActorAddr,
			builtin.MethodsMarket.OnMinerSectorsTerminate,
			&market.OnMinerSectorsTerminateParams{
				Epoch:   epoch,
				DealIDs: dealIDs[:size],
			},
			abi.NewTokenAmount(0),
			&builtin.Discard{},
		)
		builtin.RequireSuccess(rt, code, "failed to terminate deals, exit code %v", code)
		dealIDs = dealIDs[size:]
	}
}

func requestTerminateAllDeals(rt Runtime, st *State) { //nolint:deadcode,unused
	// TODO: red flag this is an ~unbounded computation.
	// Transform into an idempotent partial computation that can be progressed on each invocation.
	// https://github.com/filecoin-project/specs-actors/issues/675
	dealIds := []abi.DealID{}
	if err := st.ForEachSector(adt.AsStore(rt), func(sector *SectorOnChainInfo) {
		dealIds = append(dealIds, sector.DealIDs...)
	}); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to traverse sectors for termination: %v", err)
	}

	requestTerminateDeals(rt, rt.CurrEpoch(), dealIds)
}

func scheduleEarlyTerminationWork(rt Runtime) {
	enrollCronEvent(rt, rt.CurrEpoch()+1, &CronEventPayload{
		EventType: CronEventProcessEarlyTerminations,
	})
}

func havePendingEarlyTerminations(rt Runtime, st *State) bool {
	// Record this up-front
	noEarlyTerminations, err := st.EarlyTerminations.IsEmpty()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to count early terminations")
	return !noEarlyTerminations
}

func verifyWindowedPost(rt Runtime, challengeEpoch abi.ChainEpoch, sectors []*SectorOnChainInfo, proofs []proof.PoStProof) {
	minerActorID, err := addr.IDFromAddress(rt.Receiver())
	AssertNoError(err) // Runtime always provides ID-addresses

	// Regenerate challenge randomness, which must match that generated for the proof.
	var addrBuf bytes.Buffer
	receiver := rt.Receiver()
	err = receiver.MarshalCBOR(&addrBuf)
	AssertNoError(err)
	postRandomness := rt.GetRandomnessFromBeacon(crypto.DomainSeparationTag_WindowedPoStChallengeSeed, challengeEpoch, addrBuf.Bytes())

	sectorProofInfo := make([]proof.SectorInfo, len(sectors))
	for i, s := range sectors {
		sectorProofInfo[i] = proof.SectorInfo{
			SealProof:    s.SealProof,
			SectorNumber: s.SectorNumber,
			SealedCID:    s.SealedCID,
		}
	}

	// Get public inputs
	pvInfo := proof.WindowPoStVerifyInfo{
		Randomness:        abi.PoStRandomness(postRandomness),
		Proofs:            proofs,
		ChallengedSectors: sectorProofInfo,
		Prover:            abi.ActorID(minerActorID),
	}

	// Verify the PoSt Proof
	if err = rt.VerifyPoSt(pvInfo); err != nil {
		rt.Abortf(exitcode.ErrIllegalArgument, "invalid PoSt %+v: %s", pvInfo, err)
	}
}

// SealVerifyParams is the structure of information that must be sent with a
// message to commit a sector. Most of this information is not needed in the
// state tree but will be verified in sm.CommitSector. See SealCommitment for
// data stored on the state tree for each sector.
type SealVerifyStuff struct {
	SealedCID        cid.Cid        // CommR
	InteractiveEpoch abi.ChainEpoch // Used to derive the interactive PoRep challenge.
	abi.RegisteredSealProof
	Proof   []byte
	DealIDs []abi.DealID
	abi.SectorNumber
	SealRandEpoch abi.ChainEpoch // Used to tie the seal to a chain.
}

func getVerifyInfo(rt Runtime, params *SealVerifyStuff) *proof.SealVerifyInfo {
	if rt.CurrEpoch() <= params.InteractiveEpoch {
		rt.Abortf(exitcode.ErrForbidden, "too early to prove sector")
	}

	// Check randomness.
	challengeEarliest := sealChallengeEarliest(rt.CurrEpoch(), params.RegisteredSealProof)
	if params.SealRandEpoch < challengeEarliest {
		rt.Abortf(exitcode.ErrIllegalArgument, "seal epoch %v too old, expected >= %v", params.SealRandEpoch, challengeEarliest)
	}

	commD := requestUnsealedSectorCID(rt, params.RegisteredSealProof, params.DealIDs)

	minerActorID, err := addr.IDFromAddress(rt.Receiver())
	AssertNoError(err) // Runtime always provides ID-addresses

	buf := new(bytes.Buffer)
	receiver := rt.Receiver()
	err = receiver.MarshalCBOR(buf)
	AssertNoError(err)

	svInfoRandomness := rt.GetRandomnessFromTickets(crypto.DomainSeparationTag_SealRandomness, params.SealRandEpoch, buf.Bytes())
	svInfoInteractiveRandomness := rt.GetRandomnessFromBeacon(crypto.DomainSeparationTag_InteractiveSealChallengeSeed, params.InteractiveEpoch, buf.Bytes())

	return &proof.SealVerifyInfo{
		SealProof: params.RegisteredSealProof,
		SectorID: abi.SectorID{
			Miner:  abi.ActorID(minerActorID),
			Number: params.SectorNumber,
		},
		DealIDs:               params.DealIDs,
		InteractiveRandomness: abi.InteractiveSealRandomness(svInfoInteractiveRandomness),
		Proof:                 params.Proof,
		Randomness:            abi.SealRandomness(svInfoRandomness),
		SealedCID:             params.SealedCID,
		UnsealedCID:           commD,
	}
}

// Closes down this miner by erasing its power, terminating all its deals and burning its funds
func terminateMiner(rt Runtime) {
	var st State
	rt.StateReadonly(&st)

	requestTerminateAllDeals(rt, &st)

	// Delete the actor and burn all remaining funds
	rt.DeleteActor(builtin.BurntFundsActorAddr)
}

// Requests the storage market actor compute the unsealed sector CID from a sector's deals.
func requestUnsealedSectorCID(rt Runtime, proofType abi.RegisteredSealProof, dealIDs []abi.DealID) cid.Cid {
	var unsealedCID cbg.CborCid
	code := rt.Send(
		builtin.StorageMarketActorAddr,
		builtin.MethodsMarket.ComputeDataCommitment,
		&market.ComputeDataCommitmentParams{
			SectorType: proofType,
			DealIDs:    dealIDs,
		},
		abi.NewTokenAmount(0),
		&unsealedCID,
	)
	builtin.RequireSuccess(rt, code, "failed request for unsealed sector CID for deals %v", dealIDs)
	return cid.Cid(unsealedCID)
}

func requestDealWeight(rt Runtime, dealIDs []abi.DealID, sectorStart, sectorExpiry abi.ChainEpoch) market.VerifyDealsForActivationReturn {
	var dealWeights market.VerifyDealsForActivationReturn

	code := rt.Send(
		builtin.StorageMarketActorAddr,
		builtin.MethodsMarket.VerifyDealsForActivation,
		&market.VerifyDealsForActivationParams{
			DealIDs:      dealIDs,
			SectorStart:  sectorStart,
			SectorExpiry: sectorExpiry,
		},
		abi.NewTokenAmount(0),
		&dealWeights,
	)
	builtin.RequireSuccess(rt, code, "failed to verify deals and get deal weight")
	return dealWeights

}

func commitWorkerKeyChange(rt Runtime) *abi.EmptyValue {
	var st State
	rt.StateTransaction(&st, func() {
		info := getMinerInfo(rt, &st)
		// A previously scheduled key change could have been replaced with a new key change request
		// scheduled in the future. This case should be treated as a no-op.
		if info.PendingWorkerKey == nil || info.PendingWorkerKey.EffectiveAt > rt.CurrEpoch() {
			return
		}

		info.Worker = info.PendingWorkerKey.NewWorker
		info.PendingWorkerKey = nil
		err := st.SaveInfo(adt.AsStore(rt), info)
		builtin.RequireNoErr(rt, err, exitcode.ErrSerialization, "failed to save miner info")
	})
	return nil
}

// Requests the current epoch target block reward from the reward actor.
// return value includes reward, smoothed estimate of reward, and baseline power
func requestCurrentEpochBlockReward(rt Runtime) reward.ThisEpochRewardReturn {
	var ret reward.ThisEpochRewardReturn
	code := rt.Send(builtin.RewardActorAddr, builtin.MethodsReward.ThisEpochReward, nil, big.Zero(), &ret)
	builtin.RequireSuccess(rt, code, "failed to check epoch baseline power")
	return ret
}

// Requests the current network total power and pledge from the power actor.
func requestCurrentTotalPower(rt Runtime) *power.CurrentTotalPowerReturn {
	var pwr power.CurrentTotalPowerReturn
	code := rt.Send(builtin.StoragePowerActorAddr, builtin.MethodsPower.CurrentTotalPower, nil, big.Zero(), &pwr)
	builtin.RequireSuccess(rt, code, "failed to check current power")
	return &pwr
}

// Verifies that the total locked balance exceeds the sum of sector initial pledges.
func verifyPledgeMeetsInitialRequirements(rt Runtime, st *State) {
	if !st.MeetsInitialPledgeCondition(rt.CurrentBalance()) {
		rt.Abortf(exitcode.ErrInsufficientFunds,
			"unlocked balance does not cover pledge requirements (%v < %v)",
			st.GetUnlockedBalance(rt.CurrentBalance()), st.InitialPledgeRequirement)
	}
}

// Resolves an address to an ID address and verifies that it is address of an account or multisig actor.
func resolveControlAddress(rt Runtime, raw addr.Address) addr.Address {
	resolved, ok := rt.ResolveAddress(raw)
	if !ok {
		rt.Abortf(exitcode.ErrIllegalArgument, "unable to resolve address %v", raw)
	}
	Assert(resolved.Protocol() == addr.ID)

	ownerCode, ok := rt.GetActorCodeCID(resolved)
	if !ok {
		rt.Abortf(exitcode.ErrIllegalArgument, "no code for address %v", resolved)
	}
	if !builtin.IsPrincipal(ownerCode) {
		rt.Abortf(exitcode.ErrIllegalArgument, "owner actor type must be a principal, was %v", ownerCode)
	}
	return resolved
}

// Resolves an address to an ID address and verifies that it is address of an account actor with an associated BLS key.
// The worker must be BLS since the worker key will be used alongside a BLS-VRF.
func resolveWorkerAddress(rt Runtime, raw addr.Address) addr.Address {
	resolved, ok := rt.ResolveAddress(raw)
	if !ok {
		rt.Abortf(exitcode.ErrIllegalArgument, "unable to resolve address %v", raw)
	}
	Assert(resolved.Protocol() == addr.ID)

	ownerCode, ok := rt.GetActorCodeCID(resolved)
	if !ok {
		rt.Abortf(exitcode.ErrIllegalArgument, "no code for address %v", resolved)
	}
	if ownerCode != builtin.AccountActorCodeID {
		rt.Abortf(exitcode.ErrIllegalArgument, "worker actor type must be an account, was %v", ownerCode)
	}

	if raw.Protocol() != addr.BLS {
		var pubkey addr.Address
		code := rt.Send(resolved, builtin.MethodsAccount.PubkeyAddress, nil, big.Zero(), &pubkey)
		builtin.RequireSuccess(rt, code, "failed to fetch account pubkey from %v", resolved)
		if pubkey.Protocol() != addr.BLS {
			rt.Abortf(exitcode.ErrIllegalArgument, "worker account %v must have BLS pubkey, was %v", resolved, pubkey.Protocol())
		}
	}
	return resolved
}

func burnFunds(rt Runtime, amt abi.TokenAmount) {
	if amt.GreaterThan(big.Zero()) {
		code := rt.Send(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, amt, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to burn funds")
	}
}

func notifyPledgeChanged(rt Runtime, pledgeDelta abi.TokenAmount) {
	if !pledgeDelta.IsZero() {
		code := rt.Send(builtin.StoragePowerActorAddr, builtin.MethodsPower.UpdatePledgeTotal, &pledgeDelta, big.Zero(), &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to update total pledge")
	}
}

// Assigns proving period offset randomly in the range [0, WPoStProvingPeriod) by hashing
// the actor's address and current epoch.
func assignProvingPeriodOffset(myAddr addr.Address, currEpoch abi.ChainEpoch, hash func(data []byte) [32]byte) (abi.ChainEpoch, error) {
	offsetSeed := bytes.Buffer{}
	err := myAddr.MarshalCBOR(&offsetSeed)
	if err != nil {
		return 0, fmt.Errorf("failed to serialize address: %w", err)
	}

	err = binary.Write(&offsetSeed, binary.BigEndian, currEpoch)
	if err != nil {
		return 0, fmt.Errorf("failed to serialize epoch: %w", err)
	}

	digest := hash(offsetSeed.Bytes())
	var offset uint64
	err = binary.Read(bytes.NewBuffer(digest[:]), binary.BigEndian, &offset)
	if err != nil {
		return 0, fmt.Errorf("failed to interpret digest: %w", err)
	}

	offset = offset % uint64(WPoStProvingPeriod)
	return abi.ChainEpoch(offset), nil
}

// Computes the epoch at which a proving period should start such that it is greater than the current epoch, and
// has a defined offset from being an exact multiple of WPoStProvingPeriod.
// A miner is exempt from Winow PoSt until the first full proving period starts.
func nextProvingPeriodStart(currEpoch abi.ChainEpoch, offset abi.ChainEpoch) abi.ChainEpoch {
	currModulus := currEpoch % WPoStProvingPeriod
	var periodProgress abi.ChainEpoch // How far ahead is currEpoch from previous offset boundary.
	if currModulus >= offset {
		periodProgress = currModulus - offset
	} else {
		periodProgress = WPoStProvingPeriod - (offset - currModulus)
	}

	periodStart := currEpoch - periodProgress + WPoStProvingPeriod
	Assert(periodStart > currEpoch)
	return periodStart
}

// Computes deadline information for a fault or recovery declaration.
// If the deadline has not yet elapsed, the declaration is taken as being for the current proving period.
// If the deadline has elapsed, it's instead taken as being for the next proving period after the current epoch.
func declarationDeadlineInfo(periodStart abi.ChainEpoch, deadlineIdx uint64, currEpoch abi.ChainEpoch) (*dline.Info, error) {
	if deadlineIdx >= WPoStPeriodDeadlines {
		return nil, fmt.Errorf("invalid deadline %d, must be < %d", deadlineIdx, WPoStPeriodDeadlines)
	}

	deadline := NewDeadlineInfo(periodStart, deadlineIdx, currEpoch).NextNotElapsed()
	return deadline, nil
}

// Checks that a fault or recovery declaration at a specific deadline is outside the exclusion window for the deadline.
func validateFRDeclarationDeadline(deadline *dline.Info) error {
	if deadline.FaultCutoffPassed() {
		return fmt.Errorf("late fault or recovery declaration at %v", deadline)
	}
	return nil
}

// Validates that a partition contains the given sectors.
func validatePartitionContainsSectors(partition *Partition, sectors bitfield.BitField) error {
	// Check that the declared sectors are actually assigned to the partition.
	contains, err := BitFieldContainsAll(partition.Sectors, sectors)
	if err != nil {
		return xerrors.Errorf("failed to check sectors: %w", err)
	}
	if !contains {
		return xerrors.Errorf("not all sectors are assigned to the partition")
	}
	return nil
}

func terminationPenalty(sectorSize abi.SectorSize, currEpoch abi.ChainEpoch, networkVersion network.Version,
	rewardEstimate, networkQAPowerEstimate *smoothing.FilterEstimate, sectors []*SectorOnChainInfo) abi.TokenAmount {
	totalFee := big.Zero()
	for _, s := range sectors {
		sectorPower := QAPowerForSector(sectorSize, s)
		fee := PledgePenaltyForTermination(s.ExpectedDayReward, s.ExpectedStoragePledge, currEpoch-s.Activation, rewardEstimate,
			networkQAPowerEstimate, sectorPower, networkVersion)
		totalFee = big.Add(fee, totalFee)
	}
	return totalFee
}

func PowerForSector(sectorSize abi.SectorSize, sector *SectorOnChainInfo) PowerPair {
	return PowerPair{
		Raw: big.NewIntUnsigned(uint64(sectorSize)),
		QA:  QAPowerForSector(sectorSize, sector),
	}
}

// Returns the sum of the raw byte and quality-adjusted power for sectors.
func PowerForSectors(ssize abi.SectorSize, sectors []*SectorOnChainInfo) PowerPair {
	qa := big.Zero()
	for _, s := range sectors {
		qa = big.Add(qa, QAPowerForSector(ssize, s))
	}

	return PowerPair{
		Raw: big.Mul(big.NewIntUnsigned(uint64(ssize)), big.NewIntUnsigned(uint64(len(sectors)))),
		QA:  qa,
	}
}

// The oldest seal challenge epoch that will be accepted in the current epoch.
func sealChallengeEarliest(currEpoch abi.ChainEpoch, proof abi.RegisteredSealProof) abi.ChainEpoch {
	return currEpoch - ChainFinality - MaxSealDuration[proof]
}

func getMinerInfo(rt Runtime, st *State) *MinerInfo {
	info, err := st.GetInfo(adt.AsStore(rt))
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "could not read miner info")
	return info
}

func min64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func minEpoch(a, b abi.ChainEpoch) abi.ChainEpoch {
	if a < b {
		return a
	}
	return b
}
