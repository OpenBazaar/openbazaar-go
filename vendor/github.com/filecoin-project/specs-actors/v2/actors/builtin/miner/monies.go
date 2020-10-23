package miner

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/math"
	"github.com/filecoin-project/specs-actors/v2/actors/util/smoothing"
)

// Projection period of expected sector block reward for deposit required to pre-commit a sector.
// This deposit is lost if the pre-commitment is not timely followed up by a commitment proof.
var PreCommitDepositFactor = 20 // PARAM_SPEC
var PreCommitDepositProjectionPeriod = abi.ChainEpoch(PreCommitDepositFactor) * builtin.EpochsInDay

// Projection period of expected sector block rewards for storage pledge required to commit a sector.
// This pledge is lost if a sector is terminated before its full committed lifetime.
var InitialPledgeFactor = 20 // PARAM_SPEC
var InitialPledgeProjectionPeriod = abi.ChainEpoch(InitialPledgeFactor) * builtin.EpochsInDay

// Cap on initial pledge requirement for sectors.
// The target is 1 FIL (10**18 attoFIL) per 32GiB.
// This does not divide evenly, so the result is fractionally smaller.
var InitialPledgeMaxPerByte = big.Div(big.NewInt(1e18), big.NewInt(32<<30))

// Multiplier of share of circulating money supply for consensus pledge required to commit a sector.
// This pledge is lost if a sector is terminated before its full committed lifetime.
var InitialPledgeLockTarget = builtin.BigFrac{
	Numerator:   big.NewInt(3), // PARAM_SPEC
	Denominator: big.NewInt(10),
}

// Projection period of expected daily sector block reward penalised when a fault is continued after initial detection.
// This guarantees that a miner pays back at least the expected block reward earned since the last successful PoSt.
// The network conservatively assumes the sector was faulty since the last time it was proven.
// This penalty is currently overly punitive for continued faults.
// FF = BR(t, ContinuedFaultProjectionPeriod)
var ContinuedFaultFactorNum = 351 // PARAM_SPEC
var ContinuedFaultFactorDenom = 100
var ContinuedFaultProjectionPeriod = abi.ChainEpoch((builtin.EpochsInDay * ContinuedFaultFactorNum) / ContinuedFaultFactorDenom)

var TerminationPenaltyLowerBoundProjectionPeriod = abi.ChainEpoch((builtin.EpochsInDay * 35) / 10) // PARAM_SPEC

// Fraction of assumed block reward penalized when a sector is terminated.
var TerminationRewardFactor = builtin.BigFrac{ // PARAM_SPEC
	Numerator:   big.NewInt(1),
	Denominator: big.NewInt(2),
}

// Maximum number of lifetime days penalized when a sector is terminated.
const TerminationLifetimeCap = 140 // PARAM_SPEC

// Multiplier of whole per-winner rewards for a consensus fault penalty.
const ConsensusFaultFactor = 5

// Fraction of total reward (block reward + gas reward) to be locked up as of V6
var LockedRewardFactorNumV6 = big.NewInt(75)
var LockedRewardFactorDenomV6 = big.NewInt(100)

// The projected block reward a sector would earn over some period.
// Also known as "BR(t)".
// BR(t) = ProjectedRewardFraction(t) * SectorQualityAdjustedPower
// ProjectedRewardFraction(t) is the sum of estimated reward over estimated total power
// over all epochs in the projection period [t t+projectionDuration]
func ExpectedRewardForPower(rewardEstimate, networkQAPowerEstimate smoothing.FilterEstimate, qaSectorPower abi.StoragePower, projectionDuration abi.ChainEpoch) abi.TokenAmount {
	networkQAPowerSmoothed := networkQAPowerEstimate.Estimate()
	if networkQAPowerSmoothed.IsZero() {
		return rewardEstimate.Estimate()
	}
	expectedRewardForProvingPeriod := smoothing.ExtrapolatedCumSumOfRatio(projectionDuration, 0, rewardEstimate, networkQAPowerEstimate)
	br128 := big.Mul(qaSectorPower, expectedRewardForProvingPeriod) // Q.0 * Q.128 => Q.128
	br := big.Rsh(br128, math.Precision128)
	return big.Max(br, big.Zero()) // negative BR is clamped at 0
}

// The penalty for a sector continuing faulty for another proving period.
// It is a projection of the expected reward earned by the sector.
// Also known as "FF(t)"
func PledgePenaltyForContinuedFault(rewardEstimate, networkQAPowerEstimate smoothing.FilterEstimate, qaSectorPower abi.StoragePower) abi.TokenAmount {
	return ExpectedRewardForPower(rewardEstimate, networkQAPowerEstimate, qaSectorPower, ContinuedFaultProjectionPeriod)
}

// Lower bound on the penalty for a terminating sector.
// It is a projection of the expected reward earned by the sector.
// Also known as "SP(t)"
func PledgePenaltyForTerminationLowerBound(rewardEstimate, networkQAPowerEstimate smoothing.FilterEstimate, qaSectorPower abi.StoragePower) abi.TokenAmount {
	return ExpectedRewardForPower(rewardEstimate, networkQAPowerEstimate, qaSectorPower, TerminationPenaltyLowerBoundProjectionPeriod)
}

// Penalty to locked pledge collateral for the termination of a sector before scheduled expiry.
// SectorAge is the time between the sector's activation and termination.
// replacedDayReward and replacedSectorAge are the day reward and age of the replaced sector in a capacity upgrade.
// They must be zero if no upgrade occurred.
func PledgePenaltyForTermination(dayReward abi.TokenAmount, sectorAge abi.ChainEpoch,
	twentyDayRewardAtActivation abi.TokenAmount, networkQAPowerEstimate smoothing.FilterEstimate,
	qaSectorPower abi.StoragePower, rewardEstimate smoothing.FilterEstimate, replacedDayReward abi.TokenAmount,
	replacedSectorAge abi.ChainEpoch) abi.TokenAmount {
	// max(SP(t), BR(StartEpoch, 20d) + BR(StartEpoch, 1d) * terminationRewardFactor * min(SectorAgeInDays, 140))
	// and sectorAgeInDays = sectorAge / EpochsInDay
	lifetimeCap := abi.ChainEpoch(TerminationLifetimeCap) * builtin.EpochsInDay
	cappedSectorAge := minEpoch(sectorAge, lifetimeCap)
	// expected reward for lifetime of new sector (epochs*AttoFIL/day)
	expectedReward := big.Mul(dayReward, big.NewInt(int64(cappedSectorAge)))
	// if lifetime under cap and this sector replaced capacity, add expected reward for old sector's lifetime up to cap
	relevantReplacedAge := minEpoch(replacedSectorAge, lifetimeCap-cappedSectorAge)
	expectedReward = big.Add(expectedReward, big.Mul(replacedDayReward, big.NewInt(int64(relevantReplacedAge))))

	penalizedReward := big.Mul(expectedReward, TerminationRewardFactor.Numerator)

	return big.Max(
		PledgePenaltyForTerminationLowerBound(rewardEstimate, networkQAPowerEstimate, qaSectorPower),
		big.Add(
			twentyDayRewardAtActivation,
			big.Div(
				penalizedReward,
				big.Mul(big.NewInt(builtin.EpochsInDay), TerminationRewardFactor.Denominator)))) // (epochs*AttoFIL/day -> AttoFIL)
}

// Computes the PreCommit deposit given sector qa weight and current network conditions.
// PreCommit Deposit = BR(PreCommitDepositProjectionPeriod)
func PreCommitDepositForPower(rewardEstimate, networkQAPowerEstimate smoothing.FilterEstimate, qaSectorPower abi.StoragePower) abi.TokenAmount {
	return ExpectedRewardForPower(rewardEstimate, networkQAPowerEstimate, qaSectorPower, PreCommitDepositProjectionPeriod)
}

// Computes the pledge requirement for committing new quality-adjusted power to the network, given the current
// network total and baseline power, per-epoch  reward, and circulating token supply.
// The pledge comprises two parts:
// - storage pledge, aka IP base: a multiple of the reward expected to be earned by newly-committed power
// - consensus pledge, aka additional IP: a pro-rata fraction of the circulating money supply
//
// IP = IPBase(t) + AdditionalIP(t)
// IPBase(t) = BR(t, InitialPledgeProjectionPeriod)
// AdditionalIP(t) = LockTarget(t)*PledgeShare(t)
// LockTarget = (LockTargetFactorNum / LockTargetFactorDenom) * FILCirculatingSupply(t)
// PledgeShare(t) = sectorQAPower / max(BaselinePower(t), NetworkQAPower(t))
func InitialPledgeForPower(qaPower, baselinePower abi.StoragePower, rewardEstimate, networkQAPowerEstimate smoothing.FilterEstimate, circulatingSupply abi.TokenAmount) abi.TokenAmount {
	ipBase := ExpectedRewardForPower(rewardEstimate, networkQAPowerEstimate, qaPower, InitialPledgeProjectionPeriod)

	lockTargetNum := big.Mul(InitialPledgeLockTarget.Numerator, circulatingSupply)
	lockTargetDenom := InitialPledgeLockTarget.Denominator
	pledgeShareNum := qaPower
	networkQAPower := networkQAPowerEstimate.Estimate()
	pledgeShareDenom := big.Max(big.Max(networkQAPower, baselinePower), qaPower) // use qaPower in case others are 0
	additionalIPNum := big.Mul(lockTargetNum, pledgeShareNum)
	additionalIPDenom := big.Mul(lockTargetDenom, pledgeShareDenom)
	additionalIP := big.Div(additionalIPNum, additionalIPDenom)

	nominalPledge := big.Add(ipBase, additionalIP)
	spaceRacePledgeCap := big.Mul(InitialPledgeMaxPerByte, qaPower)
	return big.Min(nominalPledge, spaceRacePledgeCap)
}

// Repays all fee debt and then verifies that the miner has amount needed to cover
// the pledge requirement after burning all fee debt.  If not aborts.
// Returns an amount that must be burnt by the actor.
// Note that this call does not compute recent vesting so reported unlocked balance
// may be slightly lower than the true amount. Computing vesting here would be
// almost always redundant since vesting is quantized to ~daily units.  Vesting
// will be at most one proving period old if computed in the cron callback.
func RepayDebtsOrAbort(rt Runtime, st *State) abi.TokenAmount {
	currBalance := rt.CurrentBalance()
	toBurn, err := st.repayDebts(currBalance)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "unlocked balance can not repay fee debt")

	return toBurn
}

func ConsensusFaultPenalty(thisEpochReward abi.TokenAmount) abi.TokenAmount {
	return big.Div(
		big.Mul(thisEpochReward, big.NewInt(ConsensusFaultFactor)),
		big.NewInt(builtin.ExpectedLeadersPerEpoch),
	)
}

// Returns the amount of a reward to vest, and the vesting schedule, for a reward amount.
func LockedRewardFromReward(reward abi.TokenAmount, nv network.Version) (abi.TokenAmount, *VestSpec) {
	lockAmount := reward
	spec := &RewardVestingSpec
	if nv >= network.Version6 {
		// Locked amount is 75% of award.
		lockAmount = big.Div(big.Mul(reward, LockedRewardFactorNumV6), LockedRewardFactorDenomV6)
	}
	return lockAmount, spec
}
