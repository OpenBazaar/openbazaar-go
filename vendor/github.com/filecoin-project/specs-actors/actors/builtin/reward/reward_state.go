package reward

import (
	abi "github.com/filecoin-project/specs-actors/actors/abi"
	big "github.com/filecoin-project/specs-actors/actors/abi/big"
	builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	adt "github.com/filecoin-project/specs-actors/actors/util/adt"
)

// Fractional representation of NetworkTime with an implicit denominator of (2^MintingInputFixedPoint).
type NetworkTime = big.Int

// A quantity of space * time (in byte-epochs) representing power committed to the network for some duration.
type Spacetime = big.Int

type State struct {
	BaselinePower        abi.StoragePower
	RealizedPower        abi.StoragePower
	CumsumBaseline       Spacetime
	CumsumRealized       Spacetime
	EffectiveNetworkTime NetworkTime

	SimpleSupply   abi.TokenAmount // current supply
	BaselineSupply abi.TokenAmount // current supply

	// The reward to be paid in total to block producers, if exactly the expected number of them produce a block.
	// The actual reward total paid out depends on the number of winners in any round.
	// This is computed at the end of the previous epoch, and should really be called ThisEpochReward.
	ThisEpochReward abi.TokenAmount

	// The count of epochs for which a reward has been paid.
	// This should equal the number of non-empty tipsets after the genesis, aka "chain height".
	RewardEpochsPaid abi.ChainEpoch
}

type AddrKey = adt.AddrKey

// These numbers are placeholders, but should be in units of attoFIL, 10^-18 FIL
var SimpleTotal = big.Mul(big.NewInt(100e6), big.NewInt(1e18))   // 100M for testnet, PARAM_FINISH
var BaselineTotal = big.Mul(big.NewInt(900e6), big.NewInt(1e18)) // 900M for testnet, PARAM_FINISH

func ConstructState(currRealizedPower *abi.StoragePower) *State {
	st := &State{
		BaselinePower:        big.Zero(),
		RealizedPower:        big.Zero(),
		CumsumBaseline:       big.Zero(),
		CumsumRealized:       big.Zero(),
		EffectiveNetworkTime: big.Zero(),

		SimpleSupply:     big.Zero(),
		BaselineSupply:   big.Zero(),
		ThisEpochReward:  big.Zero(),
		RewardEpochsPaid: 0,
	}
	st.updateToNextEpochReward(currRealizedPower)

	return st
}

// Takes in a current realized power for a reward epoch and computes
// and updates reward state to track reward for the next epoch
func (st *State) updateToNextEpochReward(currRealizedPower *abi.StoragePower) {
	st.RealizedPower = *currRealizedPower

	st.BaselinePower = st.newBaselinePower()
	st.CumsumBaseline = big.Add(st.CumsumBaseline, st.BaselinePower)

	// Cap realized power in computing CumsumRealized so that progress is only relative to the current epoch.
	cappedRealizedPower := big.Min(st.BaselinePower, st.RealizedPower)
	st.CumsumRealized = big.Add(st.CumsumRealized, cappedRealizedPower)

	st.EffectiveNetworkTime = st.getEffectiveNetworkTime()

	st.computePerEpochReward()
}

// Updates the simple/baseline supply state and last epoch reward with computation for for a single epoch.
func (st *State) computePerEpochReward() abi.TokenAmount {
	// TODO: PARAM_FINISH
	clockTime := st.RewardEpochsPaid
	networkTime := st.EffectiveNetworkTime
	newSimpleSupply := mintingFunction(SimpleTotal, big.Lsh(big.NewInt(int64(clockTime)), MintingInputFixedPoint))
	newBaselineSupply := mintingFunction(BaselineTotal, networkTime)

	newSimpleMinted := big.Max(big.Sub(newSimpleSupply, st.SimpleSupply), big.Zero())
	newBaselineMinted := big.Max(big.Sub(newBaselineSupply, st.BaselineSupply), big.Zero())

	// TODO: this isn't actually counting emitted reward, but expected reward (which will generally over-estimate).
	// It's difficult to extract this from the minting function in its current form.
	// https://github.com/filecoin-project/specs-actors/issues/317
	st.SimpleSupply = newSimpleSupply
	st.BaselineSupply = newBaselineSupply

	perEpochReward := big.Add(newSimpleMinted, newBaselineMinted)
	st.ThisEpochReward = perEpochReward

	return perEpochReward
}

const baselinePower = 1 << 50 // 1PiB for testnet, PARAM_FINISH
func (st *State) newBaselinePower() abi.StoragePower {
	// TODO: this is not the final baseline function or value, PARAM_FINISH
	return big.NewInt(baselinePower)
}

func (st *State) getEffectiveNetworkTime() NetworkTime {
	// TODO: this function depends on the final baseline
	// EffectiveNetworkTime is a fractional input with an implicit denominator of (2^MintingInputFixedPoint).
	// realizedCumsum is thus left shifted by MintingInputFixedPoint before converted into a FixedPoint fraction
	// through division (which is an inverse function for the integral of the baseline).
	return big.Div(big.Lsh(st.CumsumRealized, MintingInputFixedPoint), big.NewInt(baselinePower))
}

// Minting Function: Taylor series expansion
//
// Intent
//   The intent of the following code is to compute the desired fraction of
//   coins that should have been minted at a given epoch according to the
//   simple exponential decay supply. This function is used both directly,
//   to compute simple minting, and indirectly, to compute baseline minting
//   by providing a synthetic "effective network time" instead of an actual
//   epoch number. The prose specification of the simple exponential decay is
//   that the unminted supply should decay exponentially with a half-life of
//   6 years. The formalization of the minted fraction at epoch t is thus:
//
//                            (            t             )
//                            ( ------------------------ )
//                      ( 1 )^( [# of epochs in 6 years] )
//           f(t) = 1 - ( - )
//                      ( 2 )
// Challenges
//
// 1. Since we cannot rely on floating-point computations in this setting, we
//    have resorted to using an ad-hoc fixed-point standard. Experimental
//    testing with the relevant scales of inputs and with a desired "atto"
//    level of output precision yielded a recommendation of a 97-bit fractional
//    part, which was stored in the constant "MintingOutputFixedPoint".
//    Fractional input is only necessary when considering "effective network
//    time"; there, the desired precision is determined by the minimum
//    plausible ratio between realized network power and network baseline,
//    which is set in "MintingInputFixedPoint".
//
// !IMPORTANT!: the time input to this function should be a factor of
//   2^MintingInputFixedPoint greater than the semantically intended value, and
//   the return value from this function is a factor of 2^MintingOutputFixedPoint
//   greater. The semantics of the output value will always be a fraction
//   between 0 and 1, but will be represented as an integer between 0 and
//   2^FixedPoint. The expectation is that callers will multiply the result by
//   some number, and THEN right-shift the result of the multiplication by
//   FixedPoint bits, thus implementing fixed-point multiplication by the
//   returned fraction.  Analogously, if callers intend to pass in an integer
//   "t", it should be left-shifted by MintingInputFixedPoint before being
//   passed; if it is fractional, its fractional part should be
//   MintingInputFixedPoint bits long.
//
// 2. Since we do not have a math library in this setting, we cannot directly
//    implement the intended closed form using stock implementations of
//    elementary functions like exp and ln. Not even powf is available.
//    Instead, we have manipulated the function into a form with a tractable
//    Taylor expansion, and then implemented the fixed-point Taylor expansion
//    in an efficient way.
//
// Mathematical Derivation
//
//   Note that the function f above is analytically equivalent to:
//
//                    (   ( 1 )              1                 )
//                    ( ln( - ) * ------------------------ * t )
//                    (   ( 2 )   [# of epochs in 6 years]     )
//        f(t) = 1 - e
//
//   We define λ = -ln(1/2)/[# of epochs in 6 years]
//               = -ln(1/2)*([Seconds per epoch] / (6 * [Seconds per year]))
//   such that
//                    -λt
//        f(t) = 1 - e
//
//   Now, we substitute for the exponential its well-known Taylor series at 0:
//
//                 infinity     n
//                   \```` (-λt)
//        f(t) = 1 -  >    ------
//                   /,,,,   n!
//                   n = 0
//
//   Simplifying, and truncating to the empirically necessary precision:
//
//                  24           n
//                \```` (-1)(-λt)
//        f(t) =   >    ----------
//                /,,,,     n!
//                n = 1
//
//   This is the final mathematical form of what is computed below. What remains
//   is to explain how the series calculation is carried out in fixed-point.
//
// Algorithm
//
//   The key observation is that each successive term can be represented as a
//   rational number, and derived from the previous term by simple
//   multiplications on the numerator and denominator. In particular:
//   * the numerator is the previous numerator multiplied by (-λt)
//   * the denominator is the previous denominator multiplied by n
//   We also need to represent λ itself as a rational, so the denominator of
//   the series term is actually multiplied by both n and the denominator of
//   lambda.
//
//   When we have the numerator and denominator for a given term set up, we
//   compute their fixed-point fraction by left-shifting the numerator before
//   performing integer division.
//
//   Finally, at the end of each loop, we remove unnecessary bits of precision
//   from both the numerator and denominator accumulators to control the
//   computational complexity of the bigint multiplications.

// Fixed-point precision (in bits) used for minting function's input "t"
const MintingInputFixedPoint = 30

// Fixed-point precision (in bits) used internally and for output
const MintingOutputFixedPoint = 97

// The following are the numerator and denominator of -ln(1/2)=ln(2),
// represented as a rational with sufficient precision. They are parsed from
// strings because literals cannot be this long; they are stored as separate
// variables only because the string parsing function has multiple returns.
var LnTwoNum, _ = big.FromString("6931471805599453094172321215")
var LnTwoDen, _ = big.FromString("10000000000000000000000000000")

// We multiply the fraction ([Seconds per epoch] / (6 * [Seconds per year]))
// into the rational representation of -ln(1/2) which was just loaded, to
// produce the final, constant, rational representation of λ.
var LambdaNum = big.Mul(big.NewInt(builtin.EpochDurationSeconds), LnTwoNum)
var LambdaDen = big.Mul(big.NewInt(6*builtin.SecondsInYear), LnTwoDen)

// This function implements f(t) as described in the large comment block above,
// with the important caveat that its return value must not be interpreted
// semantically as an integer, but rather as a fixed-point number with
// FixedPoint bits of fractional part.
func taylorSeriesExpansion(lambdaNum big.Int, lambdaDen big.Int, t big.Int) big.Int {
	// `numeratorBase` is the numerator of the rational representation of (-λt).
	numeratorBase := big.Mul(lambdaNum.Neg(), t)
	// The denominator of (-λt) is the denominator of λ times the denominator of t,
	// which is a fixed 2^MintingInputFixedPoint. Multiplying by this is a left shift.
	denominatorBase := big.Lsh(lambdaDen, MintingInputFixedPoint)

	// `numerator` is the accumulator for numerators of the series terms. The
	// first term is simply (-1)(-λt). To include that factor of (-1), which
	// appears in every term, we introduce this negation into the numerator of
	// the first term. (All terms will be negated by this, because each term is
	// derived from the last by multiplying into it.)
	numerator := numeratorBase.Neg()
	// `denominator` is the accumulator for denominators of the series terms.
	denominator := denominatorBase

	// `ret` is an _additive_ accumulator for partial sums of the series, and
	// carries a _fixed-point_ representation rather than a rational
	// representation. This just means it has an implicit denominator of
	// 2^(FixedPoint).
	ret := big.Zero()

	// The first term computed has order 1; the final term has order 24.
	for n := int64(1); n < int64(25); n++ {

		// Multiplying the denominator by `n` on every loop accounts for the
		// `n!` (factorial) in the denominator of the series.
		denominator = big.Mul(denominator, big.NewInt(n))

		// Left-shift and divide to convert rational into fixed-point.
		term := big.Div(big.Lsh(numerator, MintingOutputFixedPoint), denominator)

		// Accumulate the fixed-point result into the return accumulator.
		ret = big.Add(ret, term)

		// Multiply the rational representation of (-λt) into the term accumulators
		// for the next iteration.  Doing this here in the loop allows us to save a
		// couple bigint operations by initializing numerator and denominator
		// directly instead of multiplying by 1.
		numerator = big.Mul(numerator, numeratorBase)
		denominator = big.Mul(denominator, denominatorBase)

		// If the denominator has grown beyond the necessary precision, then we can
		// truncate it by right-shifting. As long as we right-shift the numerator
		// by the same number of bits, all we have done is lose unnecessary
		// precision that would slow down the next iteration's multiplies.
		denominatorLen := big.BitLen(denominator)
		unnecessaryBits := denominatorLen - MintingOutputFixedPoint

		numerator = big.Rsh(numerator, unnecessaryBits)
		denominator = big.Rsh(denominator, unnecessaryBits)

	}

	return ret
}

// Minting Function Wrapper
//
// Intent
//   The necessary calling conventions for the function above are unwieldy:
//   the common case is to supply the canonical Lambda, multiply by some other
//   number, and right-shift down by MintingOutputFixedPoint. This convenience
//   wrapper implements those conventions. However, it does NOT implement
//   left-shifting the input by the MintingInputFixedPoint, because baseline
//   minting will actually supply a fractional input.
func mintingFunction(factor big.Int, t big.Int) big.Int {
	return big.Rsh(big.Mul(factor, taylorSeriesExpansion(LambdaNum, LambdaDen, t)), MintingOutputFixedPoint)
}
