package reward

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/math"
)

const (
	// This number is not exported because it's not suitable for
	// calculations outside reward calculations. Importantly, there are more
	// than 365 days in a year so this number cannot be used to calculate
	// sector lifetimes, etc.
	daysInYear   = 365
	epochsInYear = daysInYear * builtin.EpochsInDay
)

// Baseline function = BaselineInitialValue * (BaselineExponent) ^(t), t in epochs
// Note: we compute exponential iteratively using recurrence e(n) = e * e(n-1).
// Caller of baseline power function is responsible for keeping track of intermediate,
// state e(n-1), the baseline power function just does the next multiplication

// Floor(e^(ln[1 + 200%] / epochsInYear) * 2^128
// Q.128 formatted number such that f(epoch) = baseExponent^epoch grows 200% in one year of epochs
// Calculation here: https://www.wolframalpha.com/input/?i=IntegerPart%5BExp%5BLog%5B1%2B100%25%5D%2F%28%28365+days%29%2F%2830+seconds%29%29%5D*2%5E128%5D
var BaselineExponent = big.MustFromString("340282591298641078465964189926313473653") // Q.128

// 2.5057116798121726 EiB
var BaselineInitialValue = big.NewInt(2_888_888_880_000_000_000) // Q.0

// Initialize baseline power for epoch -1 so that baseline power at epoch 0 is
// BaselineInitialValue.
func InitBaselinePower() abi.StoragePower {
	baselineInitialValue256 := big.Lsh(BaselineInitialValue, 2*math.Precision128) // Q.0 => Q.256
	baselineAtMinusOne := big.Div(baselineInitialValue256, BaselineExponent)   // Q.256 / Q.128 => Q.128
	return big.Rsh(baselineAtMinusOne, math.Precision128)                         // Q.128 => Q.0
}

// Compute BaselinePower(t) from BaselinePower(t-1) with an additional multiplication
// of the base exponent.
func BaselinePowerFromPrev(prevEpochBaselinePower abi.StoragePower) abi.StoragePower {
	thisEpochBaselinePower := big.Mul(prevEpochBaselinePower, BaselineExponent) // Q.0 * Q.128 => Q.128
	return big.Rsh(thisEpochBaselinePower, math.Precision128)                      // Q.128 => Q.0
}

// These numbers are estimates of the onchain constants.  They are good for initializing state in
// devnets and testing but will not match the on chain values exactly which depend on storage onboarding
// and upgrade epoch history. They are in units of attoFIL, 10^-18 FIL
var DefaultSimpleTotal = big.Mul(big.NewInt(330e6), big.NewInt(1e18))   // 330M
var DefaultBaselineTotal = big.Mul(big.NewInt(770e6), big.NewInt(1e18)) // 770M

// Computes RewardTheta which is is precise fractional value of effectiveNetworkTime.
// The effectiveNetworkTime is defined by CumsumBaselinePower(theta) == CumsumRealizedPower
// As baseline power is defined over integers and the RewardTheta is required to be fractional,
// we perform linear interpolation between CumsumBaseline(⌊theta⌋) and CumsumBaseline(⌈theta⌉).
// The effectiveNetworkTime argument is ceiling of theta.
// The result is a fractional effectiveNetworkTime (theta) in Q.128 format.
func ComputeRTheta(effectiveNetworkTime abi.ChainEpoch, baselinePowerAtEffectiveNetworkTime, cumsumRealized, cumsumBaseline big.Int) big.Int {
	var rewardTheta big.Int
	if effectiveNetworkTime != 0 {
		rewardTheta = big.NewInt(int64(effectiveNetworkTime)) // Q.0
		rewardTheta = big.Lsh(rewardTheta, math.Precision128)    // Q.0 => Q.128
		diff := big.Sub(cumsumBaseline, cumsumRealized)
		diff = big.Lsh(diff, math.Precision128)                      // Q.0 => Q.128
		diff = big.Div(diff, baselinePowerAtEffectiveNetworkTime) // Q.128 / Q.0 => Q.128
		rewardTheta = big.Sub(rewardTheta, diff)                  // Q.128
	} else {
		// special case for initialization
		rewardTheta = big.Zero()
	}
	return rewardTheta
}

var (
	// lambda = ln(2) / (6 * epochsInYear)
	// for Q.128: int(lambda * 2^128)
	// Calculation here: https://www.wolframalpha.com/input/?i=IntegerPart%5BLog%5B2%5D+%2F+%286+*+%281+year+%2F+30+seconds%29%29+*+2%5E128%5D
	Lambda = big.MustFromString("37396271439864487274534522888786")
	// expLamSubOne = e^lambda - 1
	// for Q.128: int(expLamSubOne * 2^128)
	// Calculation here: https://www.wolframalpha.com/input/?i=IntegerPart%5B%5BExp%5BLog%5B2%5D+%2F+%286+*+%281+year+%2F+30+seconds%29%29%5D+-+1%5D+*+2%5E128%5D
	ExpLamSubOne = big.MustFromString("37396273494747879394193016954629")
)

// Computes a reward for all expected leaders when effective network time changes from prevTheta to currTheta
// Inputs are in Q.128 format
func computeReward(epoch abi.ChainEpoch, prevTheta, currTheta, simpleTotal, baselineTotal big.Int) abi.TokenAmount {
	simpleReward := big.Mul(simpleTotal, ExpLamSubOne)    //Q.0 * Q.128 =>  Q.128
	epochLam := big.Mul(big.NewInt(int64(epoch)), Lambda) // Q.0 * Q.128 => Q.128

	simpleReward = big.Mul(simpleReward, big.NewFromGo(math.ExpNeg(epochLam.Int))) // Q.128 * Q.128 => Q.256
	simpleReward = big.Rsh(simpleReward, math.Precision128)                      // Q.256 >> 128 => Q.128

	baselineReward := big.Sub(computeBaselineSupply(currTheta, baselineTotal), computeBaselineSupply(prevTheta, baselineTotal)) // Q.128

	reward := big.Add(simpleReward, baselineReward) // Q.128

	return big.Rsh(reward, math.Precision128) // Q.128 => Q.0
}

// Computes baseline supply based on theta in Q.128 format.
// Return is in Q.128 format
func computeBaselineSupply(theta, baselineTotal big.Int) big.Int {
	thetaLam := big.Mul(theta, Lambda)           // Q.128 * Q.128 => Q.256
	thetaLam = big.Rsh(thetaLam, math.Precision128) // Q.256 >> 128 => Q.128

	eTL := big.NewFromGo(math.ExpNeg(thetaLam.Int)) // Q.128

	one := big.NewInt(1)
	one = big.Lsh(one, math.Precision128) // Q.0 => Q.128
	oneSub := big.Sub(one, eTL)        // Q.128

	return big.Mul(baselineTotal, oneSub) // Q.0 * Q.128 => Q.128
}

// SlowConvenientBaselineForEpoch computes baseline power for use in epoch t
// by calculating the value of ThisEpochBaselinePower that shows up in block at t - 1
// It multiplies ~t times so it should not be used in actor code directly.  It is exported as
// convenience for consuming node.
func SlowConvenientBaselineForEpoch(targetEpoch abi.ChainEpoch) abi.StoragePower {
	baseline := InitBaselinePower()
	baseline = BaselinePowerFromPrev(baseline) // value in genesis block (for epoch 1)
	for i := abi.ChainEpoch(1); i < targetEpoch; i++ {
		baseline = BaselinePowerFromPrev(baseline) // value in block i (for epoch i+1)
	}
	return baseline
}
