package miner

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/util/math"
	"github.com/filecoin-project/specs-actors/actors/util/smoothing"
)

// IP = IPBase(precommit time) + AdditionalIP(precommit time)
// IPBase(t) = BR(t, InitialPledgeProjectionPeriod)
// AdditionalIP(t) = LockTarget(t)*PledgeShare(t)
// LockTarget = (LockTargetFactorNum / LockTargetFactorDenom) * FILCirculatingSupply(t)
// PledgeShare(t) = sectorQAPower / max(BaselinePower(t), NetworkQAPower(t))
// PARAM_FINISH
var PreCommitDepositFactor = 20
var InitialPledgeFactor = 20
var PreCommitDepositProjectionPeriod = abi.ChainEpoch(PreCommitDepositFactor) * builtin.EpochsInDay
var InitialPledgeProjectionPeriod = abi.ChainEpoch(InitialPledgeFactor) * builtin.EpochsInDay
var LockTargetFactorNum = big.NewInt(3)
var LockTargetFactorDenom = big.NewInt(10)
// Cap on initial pledge requirement for sectors during the Space Race network.
// The target is 1 FIL (10**18 attoFIL) per 32GiB.
// This does not divide evenly, so the result is fractionally smaller.
var SpaceRaceInitialPledgeMaxPerByte = big.Div(big.NewInt(1e18), big.NewInt(32 << 30))

// FF = BR(t, DeclaredFaultProjectionPeriod)
// projection period of 2.14 days:  2880 * 2.14 = 6163.2.  Rounded to nearest epoch 6163
var DeclaredFaultFactorNumV0 = 214
var DeclaredFaultFactorNumV3 = 351
var DeclaredFaultFactorDenom = 100
var DeclaredFaultProjectionPeriodV0 = abi.ChainEpoch((builtin.EpochsInDay * DeclaredFaultFactorNumV0) / DeclaredFaultFactorDenom)
var DeclaredFaultProjectionPeriodV3 = abi.ChainEpoch((builtin.EpochsInDay * DeclaredFaultFactorNumV3) / DeclaredFaultFactorDenom)

// SP = BR(t, UndeclaredFaultProjectionPeriod)
var UndeclaredFaultFactorNumV0 = 50
var UndeclaredFaultFactorNumV1 = 35
var UndeclaredFaultFactorDenom = 10

var UndeclaredFaultProjectionPeriodV0 = abi.ChainEpoch((builtin.EpochsInDay * UndeclaredFaultFactorNumV0) / UndeclaredFaultFactorDenom)
var UndeclaredFaultProjectionPeriodV1 = abi.ChainEpoch((builtin.EpochsInDay * UndeclaredFaultFactorNumV1) / UndeclaredFaultFactorDenom)

// Maximum number of days of BR a terminated sector can be penalized
const TerminationLifetimeCap = abi.ChainEpoch(70)

// This is the BR(t) value of the given sector for the current epoch.
// It is the expected reward this sector would pay out over a one day period.
// BR(t) = CurrEpochReward(t) * SectorQualityAdjustedPower * EpochsInDay / TotalNetworkQualityAdjustedPower(t)
func ExpectedRewardForPower(rewardEstimate, networkQAPowerEstimate *smoothing.FilterEstimate, qaSectorPower abi.StoragePower, projectionDuration abi.ChainEpoch) abi.TokenAmount {
	networkQAPowerSmoothed := networkQAPowerEstimate.Estimate()
	if networkQAPowerSmoothed.IsZero() {
		return rewardEstimate.Estimate()
	}
	expectedRewardForProvingPeriod := smoothing.ExtrapolatedCumSumOfRatio(projectionDuration, 0, rewardEstimate, networkQAPowerEstimate)
	br := big.Mul(qaSectorPower, expectedRewardForProvingPeriod) // Q.0 * Q.128 => Q.128
	return big.Rsh(br, math.Precision)
}

// This is the FF(t) penalty for a sector expected to be in the fault state either because the fault was declared or because
// it has been previously detected by the network.
// FF(t) = DeclaredFaultFactor * BR(t)
func PledgePenaltyForDeclaredFault(rewardEstimate, networkQAPowerEstimate *smoothing.FilterEstimate, qaSectorPower abi.StoragePower,
	networkVersion network.Version) abi.TokenAmount {
	projectionPeriod := DeclaredFaultProjectionPeriodV0
	if networkVersion >= network.Version3 {
		projectionPeriod = DeclaredFaultProjectionPeriodV3
	}
	return ExpectedRewardForPower(rewardEstimate, networkQAPowerEstimate, qaSectorPower, projectionPeriod)
}

// This is the SP(t) penalty for a newly faulty sector that has not been declared.
// SP(t) = UndeclaredFaultFactor * BR(t)
func PledgePenaltyForUndeclaredFault(rewardEstimate, networkQAPowerEstimate *smoothing.FilterEstimate, qaSectorPower abi.StoragePower,
	networkVersion network.Version) abi.TokenAmount {
	projectionPeriod := UndeclaredFaultProjectionPeriodV0
	if networkVersion >= network.Version1 {
		projectionPeriod = UndeclaredFaultProjectionPeriodV1
	}
	return ExpectedRewardForPower(rewardEstimate, networkQAPowerEstimate, qaSectorPower, projectionPeriod)
}

// Penalty to locked pledge collateral for the termination of a sector before scheduled expiry.
// SectorAge is the time between the sector's activation and termination.
func PledgePenaltyForTermination(dayRewardAtActivation, twentyDayRewardAtActivation abi.TokenAmount, sectorAge abi.ChainEpoch,
	rewardEstimate, networkQAPowerEstimate *smoothing.FilterEstimate, qaSectorPower abi.StoragePower, networkVersion network.Version) abi.TokenAmount {
	// max(SP(t), BR(StartEpoch, 20d) + BR(StartEpoch, 1d)*min(SectorAgeInDays, 70))
	// and sectorAgeInDays = sectorAge / EpochsInDay

	cappedSectorAge := big.NewInt(int64(minEpoch(sectorAge, TerminationLifetimeCap*builtin.EpochsInDay)))
	if networkVersion >= network.Version1 {
		cappedSectorAge = big.NewInt(int64(minEpoch(sectorAge / 2, TerminationLifetimeCap*builtin.EpochsInDay)))
	}

	return big.Max(
		PledgePenaltyForUndeclaredFault(rewardEstimate, networkQAPowerEstimate, qaSectorPower, networkVersion),
		big.Add(
			twentyDayRewardAtActivation,
			big.Div(
				big.Mul(dayRewardAtActivation, cappedSectorAge),
				big.NewInt(builtin.EpochsInDay))))
}

// Computes the PreCommit Deposit given sector qa weight and current network conditions.
// PreCommit Deposit = 20 * BR(t)
func PreCommitDepositForPower(rewardEstimate, networkQAPowerEstimate *smoothing.FilterEstimate, qaSectorPower abi.StoragePower) abi.TokenAmount {
	return ExpectedRewardForPower(rewardEstimate, networkQAPowerEstimate, qaSectorPower, PreCommitDepositProjectionPeriod)
}

// Computes the pledge requirement for committing new quality-adjusted power to the network, given the current
// total power, total pledge commitment, epoch block reward, and circulating token supply.
// In plain language, the pledge requirement is a multiple of the block reward expected to be earned by the
// newly-committed power, holding the per-epoch block reward constant (though in reality it will change over time).
func InitialPledgeForPower(qaPower abi.StoragePower, baselinePower abi.StoragePower, networkTotalPledge abi.TokenAmount, rewardEstimate, networkQAPowerEstimate *smoothing.FilterEstimate, networkCirculatingSupplySmoothed abi.TokenAmount) abi.TokenAmount {
	networkQAPower := networkQAPowerEstimate.Estimate()
	ipBase := ExpectedRewardForPower(rewardEstimate, networkQAPowerEstimate, qaPower, InitialPledgeProjectionPeriod)

	lockTargetNum := big.Mul(LockTargetFactorNum, networkCirculatingSupplySmoothed)
	lockTargetDenom := LockTargetFactorDenom
	pledgeShareNum := qaPower
	pledgeShareDenom := big.Max(big.Max(networkQAPower, baselinePower), qaPower) // use qaPower in case others are 0
	additionalIPNum := big.Mul(lockTargetNum, pledgeShareNum)
	additionalIPDenom := big.Mul(lockTargetDenom, pledgeShareDenom)
	additionalIP := big.Div(additionalIPNum, additionalIPDenom)

	nominalPledge := big.Add(ipBase, additionalIP)
	spaceRacePledgeCap := big.Mul(SpaceRaceInitialPledgeMaxPerByte, qaPower)
	return big.Min(nominalPledge, spaceRacePledgeCap)
}
