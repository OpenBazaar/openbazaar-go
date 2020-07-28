package miner

import (
	"testing"

	"github.com/minio/blake2b-simd"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	tutils "github.com/filecoin-project/specs-actors/support/testing"
)

func TestAssignProvingPeriodBoundary(t *testing.T) {
	addr1 := tutils.NewActorAddr(t, "a")
	addr2 := tutils.NewActorAddr(t, "b")
	startEpoch := abi.ChainEpoch(1)

	// ensure the values are different for different addresses
	b1, err := assignProvingPeriodOffset(addr1, startEpoch, blake2b.Sum256)
	assert.NoError(t, err)
	assert.True(t, b1 >= 0)
	assert.True(t, b1 < WPoStProvingPeriod)

	b2, err := assignProvingPeriodOffset(addr2, startEpoch, blake2b.Sum256)
	assert.NoError(t, err)
	assert.True(t, b2 >= 0)
	assert.True(t, b2 < WPoStProvingPeriod)

	assert.NotEqual(t, b1, b2)

	// Ensure boundaries are always less than a proving period.
	for i := 0; i < 10_000; i++ {
		boundary, err := assignProvingPeriodOffset(addr1, abi.ChainEpoch(i), blake2b.Sum256)
		assert.NoError(t, err)
		assert.True(t, boundary >= 0)
		assert.True(t, boundary < WPoStProvingPeriod)
	}
}

func TestNextProvingPeriodStart(t *testing.T) {
	// At epoch zero...
	curr := e(0)
	// ... with offset zero, the first period start skips one period ahead, ...
	assert.Equal(t, WPoStProvingPeriod, nextProvingPeriodStart(curr, 0))

	// ... and all non-zero offsets are simple.
	assert.Equal(t, e(1), nextProvingPeriodStart(curr, 1))
	assert.Equal(t, e(10), nextProvingPeriodStart(curr, 10))
	assert.Equal(t, WPoStProvingPeriod-1, nextProvingPeriodStart(curr, WPoStProvingPeriod-1))

	// At epoch 1, offsets 0 and 1 start a long way forward, but offsets 2 and later start soon.
	curr = 1
	assert.Equal(t, WPoStProvingPeriod, nextProvingPeriodStart(curr, 0))
	assert.Equal(t, WPoStProvingPeriod+1, nextProvingPeriodStart(curr, 1))
	assert.Equal(t, e(2), nextProvingPeriodStart(curr, 2))
	assert.Equal(t, e(3), nextProvingPeriodStart(curr, 3))
	assert.Equal(t, WPoStProvingPeriod-1, nextProvingPeriodStart(curr, WPoStProvingPeriod-1))

	// An arbitrary mid-period epoch.
	curr = 123
	assert.Equal(t, WPoStProvingPeriod, nextProvingPeriodStart(curr, 0))
	assert.Equal(t, WPoStProvingPeriod+1, nextProvingPeriodStart(curr, 1))
	assert.Equal(t, WPoStProvingPeriod+122, nextProvingPeriodStart(curr, 122))
	assert.Equal(t, WPoStProvingPeriod+123, nextProvingPeriodStart(curr, 123))
	assert.Equal(t, e(124), nextProvingPeriodStart(curr, 124))
	assert.Equal(t, WPoStProvingPeriod-1, nextProvingPeriodStart(curr, WPoStProvingPeriod-1))

	// The final epoch in the chain's first full period
	curr = WPoStProvingPeriod - 1
	assert.Equal(t, WPoStProvingPeriod, nextProvingPeriodStart(curr, 0))
	assert.Equal(t, WPoStProvingPeriod+1, nextProvingPeriodStart(curr, 1))
	assert.Equal(t, WPoStProvingPeriod+2, nextProvingPeriodStart(curr, 2))
	assert.Equal(t, WPoStProvingPeriod+WPoStProvingPeriod-2, nextProvingPeriodStart(curr, WPoStProvingPeriod-2))
	assert.Equal(t, WPoStProvingPeriod+WPoStProvingPeriod-1, nextProvingPeriodStart(curr, WPoStProvingPeriod-1))

	// Into the chain's second period
	curr = WPoStProvingPeriod
	assert.Equal(t, 2*WPoStProvingPeriod, nextProvingPeriodStart(curr, 0))
	assert.Equal(t, WPoStProvingPeriod+1, nextProvingPeriodStart(curr, 1))
	assert.Equal(t, WPoStProvingPeriod+2, nextProvingPeriodStart(curr, 2))
	assert.Equal(t, WPoStProvingPeriod+WPoStProvingPeriod-1, nextProvingPeriodStart(curr, WPoStProvingPeriod-1))

	curr = WPoStProvingPeriod + 234
	assert.Equal(t, 2*WPoStProvingPeriod, nextProvingPeriodStart(curr, 0))
	assert.Equal(t, 2*WPoStProvingPeriod+1, nextProvingPeriodStart(curr, 1))
	assert.Equal(t, 2*WPoStProvingPeriod+233, nextProvingPeriodStart(curr, 233))
	assert.Equal(t, 2*WPoStProvingPeriod+234, nextProvingPeriodStart(curr, 234))
	assert.Equal(t, WPoStProvingPeriod+235, nextProvingPeriodStart(curr, 235))
	assert.Equal(t, WPoStProvingPeriod+WPoStProvingPeriod-1, nextProvingPeriodStart(curr, WPoStProvingPeriod-1))
}

type e = abi.ChainEpoch

func TestFaultFeeInvariants(t *testing.T) {
	t.Run("Undeclared faults are more expensive than declared faults", func(t *testing.T) {
		epochReward := abi.NewTokenAmount(1_000)
		networkPower := abi.NewStoragePower(100 << 50)
		faultySectorPower := abi.NewStoragePower(1 << 50)

		ff := PledgePenaltyForDeclaredFault(epochReward, networkPower, faultySectorPower)
		sp := PledgePenaltyForUndeclaredFault(epochReward, networkPower, faultySectorPower)
		assert.True(t, sp.GreaterThan(ff))
	}) 

	 t.Run("Declared and Undeclared fault penalties are linear over sectorQAPower term", func(t *testing.T) {
		epochReward := abi.NewTokenAmount(1_000)
		networkPower := abi.NewStoragePower(100 << 50)
		faultySectorAPower := abi.NewStoragePower(1 << 50)
		faultySectorBPower := abi.NewStoragePower(19 << 50)		
		faultySectorCPower := abi.NewStoragePower(63 << 50)				
		totalFaultPower := big.Add(big.Add(faultySectorAPower, faultySectorBPower), faultySectorCPower)		

		// Declared faults
		ffA := PledgePenaltyForDeclaredFault(epochReward, networkPower, faultySectorAPower)
		ffB := PledgePenaltyForDeclaredFault(epochReward, networkPower, faultySectorBPower)		
		ffC := PledgePenaltyForDeclaredFault(epochReward, networkPower, faultySectorCPower)				

		ffAll := PledgePenaltyForDeclaredFault(epochReward, networkPower, totalFaultPower)

		// Because we can introduce rounding error between 1 and zero for every penalty calculation
		// we can at best expect n calculations of 1 power to be within n of 1 calculation of n powers.
		diff := big.Sub(ffAll, big.Add(ffC, big.Add(ffA, ffB)))
		assert.True(t, diff.GreaterThanEqual(big.Zero()))
		assert.True(t, diff.LessThan(big.NewInt(3)))

		// Undeclared faults		
		spA := PledgePenaltyForUndeclaredFault(epochReward, networkPower, faultySectorAPower)
		spB := PledgePenaltyForUndeclaredFault(epochReward, networkPower, faultySectorBPower)		
		spC := PledgePenaltyForUndeclaredFault(epochReward, networkPower, faultySectorCPower)				

		spAll := PledgePenaltyForUndeclaredFault(epochReward, networkPower, totalFaultPower)

		// Because we can introduce rounding error between 1 and zero for every penalty calculation
		// we can at best expect n calculations of 1 power to be within n of 1 calculation of n powers.
		diff = big.Sub(spAll, big.Add(spC, big.Add(spA, spB)))
		assert.True(t, diff.GreaterThanEqual(big.Zero()))
		assert.True(t, diff.LessThan(big.NewInt(3)))

	 })
}

func TestQuantizeUp(t *testing.T) {
	t.Run("zero offset", func(t *testing.T) {
		assert.Equal(t, abi.ChainEpoch(50), quantizeUp(42, 10, 0))
		assert.Equal(t, abi.ChainEpoch(16000), quantizeUp(16000, 100, 0))
		assert.Equal(t, abi.ChainEpoch(0), quantizeUp(-5, 10, 0))
		assert.Equal(t, abi.ChainEpoch(-50), quantizeUp(-50, 10, 0))
		assert.Equal(t, abi.ChainEpoch(-50), quantizeUp(-53, 10, 0))
	})

	t.Run("non zero offset", func(t *testing.T) {
		assert.Equal(t, abi.ChainEpoch(6), quantizeUp(4, 5, 1))
		assert.Equal(t, abi.ChainEpoch(1), quantizeUp(0, 5, 1))
		assert.Equal(t, abi.ChainEpoch(-4), quantizeUp(-6, 5, 1))
		assert.Equal(t, abi.ChainEpoch(4), quantizeUp(2, 10, 4))
	})

	t.Run("offset seed bigger than unit is normalized", func(t *testing.T) {
		assert.Equal(t, abi.ChainEpoch(13), quantizeUp(9, 5, 28)) // offset should be 3
		assert.Equal(t, abi.ChainEpoch(10000), quantizeUp(10000, 100, 2000000))
	})

}