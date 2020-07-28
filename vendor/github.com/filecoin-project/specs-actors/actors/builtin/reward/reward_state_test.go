package reward

import (
	"testing"

	"github.com/filecoin-project/specs-actors/actors/abi/big"
)

// Minting function test vectors
//
// Intent
//   Check that for the first 7 years, the current minting function
//   approximation is exact to the desired precision, namely 1 attoFIL out of 1
//   gigaFIL.
//
// Approach
//   The desired precision is, in particular lg(1e9/1e-18) ≈ 90 bits.
//   An offline calculation with arbitrary-precision arithmetic was used to
//   establish ground truth about the first 90 bits of the minting function,
//   f(t):=1-exp(t*ln(1/2)*SecondsPerEpoch/(6*SecondsPerYear)), for epoch
//   numbers corresponding to the endpoints of the first 7 years with
//   SecondsPerEpoch set to 30. These numbers were written below as strings,
//   because they contain more digits than literals support.

var mintingTestVectorPrecision = uint(90)

var mintingTestVectors = []struct {
	in  int64
	out string
}{
	{1051897, "135060784589637453410950129"},
	{2103794, "255386271058940593613485187"},
	{3155691, "362584098600550296025821387"},
	{4207588, "458086510989070493849325308"},
	{5259485, "543169492437427724953202180"},
	{6311382, "618969815707708523300124489"},
	{7363279, "686500230252085183344830372"},
}

const SecondsInYear = 31556925
const TestEpochDurationSeconds = 30

var TestLambdaNum = big.Mul(big.NewInt(TestEpochDurationSeconds), LnTwoNum)
var TestLambdaDen = big.Mul(big.NewInt(6*SecondsInYear), LnTwoDen)

func TestMintingFunction(t *testing.T) {
	for _, vector := range mintingTestVectors {
		// In order to supply an integer as an input to the minting function, we
		// first left-shift zeroes into the fractional part of its fixed-point
		// representation.
		ts_input := big.Lsh(big.NewInt(vector.in), MintingInputFixedPoint)

		ts_output := taylorSeriesExpansion(TestLambdaNum, TestLambdaDen, ts_input)

		// ts_output will always range between 0 and 2^FixedPoint. If we
		// right-shifted by FixedPoint, without first multiplying by something, we
		// would discard _all_ the bits and get 0. Instead, we want to discard only
		// those bits in the FixedPoint representation that we don't also want to
		// require to exactly match the test vectors.
		ts_truncated_fractional_part := big.Rsh(ts_output, MintingOutputFixedPoint-mintingTestVectorPrecision)

		expected_truncated_fractional_part, _ := big.FromString(vector.out)
		if !(ts_truncated_fractional_part.Equals(expected_truncated_fractional_part)) {
			t.Errorf("minting function: on input %q, computed %q, expected %q", ts_input, ts_truncated_fractional_part, expected_truncated_fractional_part)
		}
	}
}
