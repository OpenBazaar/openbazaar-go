package reward

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/specs-actors/v2/actors/util/smoothing"
)

// A quantity of space * time (in byte-epochs) representing power committed to the network for some duration.
type Spacetime = big.Int

// 36.266260308195979333 FIL
// https://www.wolframalpha.com/input/?i=IntegerPart%5B330%2C000%2C000+*+%281+-+Exp%5B-Log%5B2%5D+%2F+%286+*+%281+year+%2F+30+seconds%29%29%5D%29+*+10%5E18%5D
const InitialRewardPositionEstimateStr = "36266260308195979333"

var InitialRewardPositionEstimate = big.MustFromString(InitialRewardPositionEstimateStr)

// -1.0982489*10^-7 FIL per epoch.  Change of simple minted tokens between epochs 0 and 1
// https://www.wolframalpha.com/input/?i=IntegerPart%5B%28Exp%5B-Log%5B2%5D+%2F+%286+*+%281+year+%2F+30+seconds%29%29%5D+-+1%29+*+10%5E18%5D
var InitialRewardVelocityEstimate = abi.NewTokenAmount(-109897758509)

// Changed since v0:
// - ThisEpochRewardSmoothed is not a pointer
type State struct {
	// CumsumBaseline is a target CumsumRealized needs to reach for EffectiveNetworkTime to increase
	// CumsumBaseline and CumsumRealized are expressed in byte-epochs.
	CumsumBaseline Spacetime

	// CumsumRealized is cumulative sum of network power capped by BaselinePower(epoch)
	CumsumRealized Spacetime

	// EffectiveNetworkTime is ceiling of real effective network time `theta` based on
	// CumsumBaselinePower(theta) == CumsumRealizedPower
	// Theta captures the notion of how much the network has progressed in its baseline
	// and in advancing network time.
	EffectiveNetworkTime abi.ChainEpoch

	// EffectiveBaselinePower is the baseline power at the EffectiveNetworkTime epoch
	EffectiveBaselinePower abi.StoragePower

	// The reward to be paid in per WinCount to block producers.
	// The actual reward total paid out depends on the number of winners in any round.
	// This value is recomputed every non-null epoch and used in the next non-null epoch.
	ThisEpochReward abi.TokenAmount
	// Smoothed ThisEpochReward
	ThisEpochRewardSmoothed smoothing.FilterEstimate

	// The baseline power the network is targeting at st.Epoch
	ThisEpochBaselinePower abi.StoragePower

	// Epoch tracks for which epoch the Reward was computed
	Epoch abi.ChainEpoch

	// TotalStoragePowerReward tracks the total FIL awarded to block miners
	TotalStoragePowerReward abi.TokenAmount

	// Simple and Baseline totals are constants used for computing rewards.
	// They are on chain because of a historical fix resetting baseline value
	// in a way that depended on the history leading immediately up to the
	// migration fixing the value.  These values can be moved from state back
	// into a code constant in a subsequent upgrade.
	SimpleTotal   abi.TokenAmount
	BaselineTotal abi.TokenAmount
}

func ConstructState(currRealizedPower abi.StoragePower) *State {
	st := &State{
		CumsumBaseline:         big.Zero(),
		CumsumRealized:         big.Zero(),
		EffectiveNetworkTime:   0,
		EffectiveBaselinePower: BaselineInitialValue,

		ThisEpochReward:        big.Zero(),
		ThisEpochBaselinePower: InitBaselinePower(),
		Epoch:                  -1,

		ThisEpochRewardSmoothed: smoothing.NewEstimate(InitialRewardPositionEstimate, InitialRewardVelocityEstimate),
		TotalStoragePowerReward: big.Zero(),

		SimpleTotal:   DefaultSimpleTotal,
		BaselineTotal: DefaultBaselineTotal,
	}

	st.updateToNextEpochWithReward(currRealizedPower)

	return st
}

// Takes in current realized power and updates internal state
// Used for update of internal state during null rounds
func (st *State) updateToNextEpoch(currRealizedPower abi.StoragePower) {
	st.Epoch++
	st.ThisEpochBaselinePower = BaselinePowerFromPrev(st.ThisEpochBaselinePower)
	cappedRealizedPower := big.Min(st.ThisEpochBaselinePower, currRealizedPower)
	st.CumsumRealized = big.Add(st.CumsumRealized, cappedRealizedPower)

	for st.CumsumRealized.GreaterThan(st.CumsumBaseline) {
		st.EffectiveNetworkTime++
		st.EffectiveBaselinePower = BaselinePowerFromPrev(st.EffectiveBaselinePower)
		st.CumsumBaseline = big.Add(st.CumsumBaseline, st.EffectiveBaselinePower)
	}
}

// Takes in a current realized power for a reward epoch and computes
// and updates reward state to track reward for the next epoch
func (st *State) updateToNextEpochWithReward(currRealizedPower abi.StoragePower) {
	prevRewardTheta := ComputeRTheta(st.EffectiveNetworkTime, st.EffectiveBaselinePower, st.CumsumRealized, st.CumsumBaseline)
	st.updateToNextEpoch(currRealizedPower)
	currRewardTheta := ComputeRTheta(st.EffectiveNetworkTime, st.EffectiveBaselinePower, st.CumsumRealized, st.CumsumBaseline)

	st.ThisEpochReward = computeReward(st.Epoch, prevRewardTheta, currRewardTheta, st.SimpleTotal, st.BaselineTotal)
}

func (st *State) updateSmoothedEstimates(delta abi.ChainEpoch) {
	filterReward := smoothing.LoadFilter(st.ThisEpochRewardSmoothed, smoothing.DefaultAlpha, smoothing.DefaultBeta)
	st.ThisEpochRewardSmoothed = filterReward.NextEstimate(st.ThisEpochReward, delta)
}
