package miner_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-bitfield"
	cid "github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/filecoin-project/specs-actors/support/ipld"
	tutils "github.com/filecoin-project/specs-actors/support/testing"
)

func TestPrecommittedSectorsStore(t *testing.T) {
	t.Run("Put, get and delete", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))
		sectorNo := abi.SectorNumber(1)

		pc1 := newSectorPreCommitOnChainInfo(sectorNo, tutils.MakeCID("1"), abi.NewTokenAmount(1), abi.ChainEpoch(1))
		harness.putPreCommit(pc1)
		assert.Equal(t, pc1, harness.getPreCommit(sectorNo))

		pc2 := newSectorPreCommitOnChainInfo(sectorNo, tutils.MakeCID("2"), abi.NewTokenAmount(1), abi.ChainEpoch(1))
		harness.putPreCommit(pc2)
		assert.Equal(t, pc2, harness.getPreCommit(sectorNo))

		harness.deletePreCommit(sectorNo)
		assert.False(t, harness.hasPreCommit(sectorNo))
	})

	t.Run("Delete nonexistent value returns an error", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))
		sectorNo := abi.SectorNumber(1)
		err := harness.s.DeletePrecommittedSectors(harness.store, sectorNo)
		assert.Error(t, err)
	})

	t.Run("Get nonexistent value returns false", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))
		sectorNo := abi.SectorNumber(1)
		assert.False(t, harness.hasPreCommit(sectorNo))
	})
}

func TestSectorsStore(t *testing.T) {
	t.Run("Put get and delete", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		sectorNo := abi.SectorNumber(1)
		sectorInfo1 := newSectorOnChainInfo(sectorNo, tutils.MakeCID("1"), big.NewInt(1), abi.ChainEpoch(1))
		sectorInfo2 := newSectorOnChainInfo(sectorNo, tutils.MakeCID("2"), big.NewInt(2), abi.ChainEpoch(2))

		harness.putSector(sectorInfo1)
		assert.True(t, harness.hasSectorNo(sectorNo))
		out := harness.getSector(sectorNo)
		assert.Equal(t, sectorInfo1, out)

		harness.putSector(sectorInfo2)
		out = harness.getSector(sectorNo)
		assert.Equal(t, sectorInfo2, out)

		harness.deleteSectors(uint64(sectorNo))
		assert.False(t, harness.hasSectorNo(sectorNo))
	})

	t.Run("Delete nonexistent value returns an error", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		sectorNo := abi.SectorNumber(1)
		bf := abi.NewBitField()
		bf.Set(uint64(sectorNo))

		assert.Error(t, harness.s.DeleteSectors(harness.store, bf))
	})

	t.Run("Get nonexistent value returns false", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		sectorNo := abi.SectorNumber(1)
		assert.False(t, harness.hasSectorNo(sectorNo))
	})

	t.Run("Iterate and Delete multiple sector", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		// set of sectors, the larger numbers here are not significant
		sectorNos := []uint64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}

		// put all the sectors in the store
		for _, s := range sectorNos {
			i := int64(0)
			harness.putSector(newSectorOnChainInfo(abi.SectorNumber(s), tutils.MakeCID(fmt.Sprintf("%d", i)), big.NewInt(i), abi.ChainEpoch(i)))
			i++
		}

		sectorNoIdx := 0
		err := harness.s.ForEachSector(harness.store, func(si *miner.SectorOnChainInfo) {
			require.Equal(t, abi.SectorNumber(sectorNos[sectorNoIdx]), si.SectorNumber)
			sectorNoIdx++
		})
		assert.NoError(t, err)

		// ensure we iterated over the expected number of sectors
		assert.Equal(t, len(sectorNos), sectorNoIdx)

		harness.deleteSectors(sectorNos...)
		for _, s := range sectorNos {
			assert.False(t, harness.hasSectorNo(abi.SectorNumber(s)))
		}
	})
}

func TestNewSectorsBitField(t *testing.T) {
	t.Run("Add new sectors happy path", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		// set of sectors, the larger numbers here are not significant
		sectorNos := []abi.SectorNumber{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}
		harness.addNewSectors(sectorNos...)
		assert.Equal(t, uint64(len(sectorNos)), harness.getNewSectorCount())
	})

	t.Run("Add new sectors excludes duplicates", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		sectorNos := []abi.SectorNumber{1, 1, 2, 2, 3, 4, 5}
		harness.addNewSectors(sectorNos...)
		assert.Equal(t, uint64(5), harness.getNewSectorCount())
	})

	t.Run("Remove sectors happy path", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		sectorNos := []abi.SectorNumber{1, 2, 3, 4, 5}
		harness.addNewSectors(sectorNos...)
		assert.Equal(t, uint64(len(sectorNos)), harness.getNewSectorCount())

		harness.removeNewSectors(1, 3, 5)
		assert.Equal(t, uint64(2), harness.getNewSectorCount())

		sm, err := harness.s.NewSectors.All(uint64(len(sectorNos)))
		assert.NoError(t, err)
		assert.Equal(t, []uint64{2, 4}, sm)
	})

	t.Run("Add New sectors errors when adding too many new sectors", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		tooManySectors := make([]abi.SectorNumber, miner.NewSectorsPerPeriodMax+1)
		for i := abi.SectorNumber(0); i < miner.NewSectorsPerPeriodMax+1; i++ {
			tooManySectors[i] = i
		}

		err := harness.s.AddNewSectors(tooManySectors...)
		assert.Error(t, err)

		// sanity check nothing was added
		// For omission reason see: https://github.com/filecoin-project/specs-actors/issues/300
		//assert.Equal(t, uint64(0), actorHarness.getNewSectorCount())
	})
}

func TestSectorExpirationStore(t *testing.T) {
	exp1 := abi.ChainEpoch(10)
	exp2 := abi.ChainEpoch(20)
	sectorExpirations := map[abi.ChainEpoch][]uint64{
		exp1: {1, 2, 3, 4, 5},
		exp2: {6, 7, 8, 9, 10},
	}

	t.Run("Round trip add get sector expirations", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))
		harness.addSectorExpiration(exp1, sectorExpirations[exp1]...)
		harness.addSectorExpiration(exp2, sectorExpirations[exp2]...)

		assert.Equal(t, sectorExpirations[exp1], harness.getSectorExpirations(exp1))
		assert.Equal(t, sectorExpirations[exp2], harness.getSectorExpirations(exp2))

		// return nothing if there are no sectors at the epoch
		assert.Empty(t, harness.getSectorExpirations(abi.ChainEpoch(0)))

		// remove the first sector from expiration set 1
		harness.removeSectorExpiration(exp1, sectorExpirations[exp1][0])
		assert.Equal(t, sectorExpirations[exp1][1:], harness.getSectorExpirations(exp1))
		assert.Equal(t, sectorExpirations[exp2], harness.getSectorExpirations(exp2)) // No change

		// remove all sectors from expiration set 2
		harness.removeSectorExpiration(exp2, sectorExpirations[exp2]...)
		assert.Empty(t, harness.getSectorExpirations(exp2))

		// Remove remainder
		harness.removeSectorExpiration(exp1, sectorExpirations[exp1][1:]...)
		err := harness.s.ForEachSectorExpiration(harness.store, func(epoch abi.ChainEpoch, expirations *abi.BitField) error {
			assert.Fail(t, "unexpected expiration epoch: %v", epoch)
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("Iteration by expiration", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))
		harness.addSectorExpiration(exp1, sectorExpirations[exp1]...)
		harness.addSectorExpiration(exp2, sectorExpirations[exp2]...)

		var prevCallbackArg *abi.BitField
		found := map[abi.ChainEpoch][]uint64{}
		err := harness.s.ForEachSectorExpiration(harness.store, func(epoch abi.ChainEpoch, expirations *abi.BitField) error {
			sectors, err := expirations.All(100)
			require.NoError(t, err)
			found[epoch] = sectors

			// Check that bitfield pointer argument is newly allocated, not re-used
			assert.True(t, prevCallbackArg != expirations)
			prevCallbackArg = expirations
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, sectorExpirations, found)
	})

	t.Run("Adding sectors at expiry merges with existing", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		mergedSectors := []uint64{21, 22, 23, 24, 25}
		harness.addSectorExpiration(exp1, sectorExpirations[exp1]...)
		harness.addSectorExpiration(exp1, mergedSectors...)

		merged := harness.getSectorExpirations(exp1)
		assert.Equal(t, append(sectorExpirations[exp1], mergedSectors...), merged)
	})

	t.Run("clear sectors by expirations", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		harness.addSectorExpiration(exp1, sectorExpirations[exp1]...)
		harness.addSectorExpiration(exp2, sectorExpirations[exp2]...)

		// ensure clearing works
		harness.clearSectorExpiration(exp1, exp2)
		empty1 := harness.getSectorExpirations(exp1)
		assert.Empty(t, empty1)

		empty2 := harness.getSectorExpirations(exp2)
		assert.Empty(t, empty2)
	})
}

func TestFaultStore(t *testing.T) {
	fault1 := abi.ChainEpoch(10)
	fault2 := abi.ChainEpoch(20)
	sectorFaults := map[abi.ChainEpoch][]uint64{
		fault1: {1, 2, 3, 4, 5},
		fault2: {6, 7, 8, 9, 10, 11},
	}

	t.Run("Add/remove all", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))
		harness.addFaults(fault1, sectorFaults[fault1]...)
		harness.addFaults(fault2, sectorFaults[fault2]...)

		found := map[abi.ChainEpoch][]uint64{}
		var prevCallbackArg *abi.BitField
		err := harness.s.ForEachFaultEpoch(harness.store, func(epoch abi.ChainEpoch, faults *abi.BitField) error {
			sectors, err := faults.All(100)
			require.NoError(t, err)
			found[epoch] = sectors

			// Assert the bitfield pointer is pointing to a distinct object on different calls, so that holding on
			// to the reference is safe.
			assert.True(t, faults != prevCallbackArg)
			prevCallbackArg = faults
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, sectorFaults, found)

		// remove all the faults
		harness.removeFaults(sectorFaults[fault1]...)
		harness.removeFaults(sectorFaults[fault2]...)
		err = harness.s.ForEachFaultEpoch(harness.store, func(epoch abi.ChainEpoch, faults *abi.BitField) error {
			assert.Fail(t, "unexpected fault epoch: %v", epoch)
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("Add/remove some", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))
		harness.addFaults(fault1, sectorFaults[fault1]...)
		harness.addFaults(fault2, sectorFaults[fault2]...)

		found := map[abi.ChainEpoch][]uint64{}
		err := harness.s.ForEachFaultEpoch(harness.store, func(epoch abi.ChainEpoch, faults *abi.BitField) error {
			sectors, err := faults.All(100)
			require.NoError(t, err)
			found[epoch] = sectors
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, sectorFaults, found)

		// remove the faults
		harness.removeFaults(sectorFaults[fault1][1:]...)
		harness.removeFaults(sectorFaults[fault2][2:]...)

		found = map[abi.ChainEpoch][]uint64{}
		err = harness.s.ForEachFaultEpoch(harness.store, func(epoch abi.ChainEpoch, faults *abi.BitField) error {
			sectors, err := faults.All(100)
			require.NoError(t, err)
			found[epoch] = sectors
			return nil
		})
		require.NoError(t, err)
		expected := map[abi.ChainEpoch][]uint64{
			fault1: {1},
			fault2: {6, 7},
		}
		assert.Equal(t, expected, found) // FOIXME
	})

	t.Run("Clear all", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))
		harness.addFaults(fault1, []uint64{1, 2, 3, 4, 5}...)
		harness.addFaults(fault2, []uint64{6, 7, 8, 9, 10, 11}...)

		// now clear all the faults
		err := harness.s.ClearFaultEpochs(harness.store, fault1, fault2)
		require.NoError(t, err)

		err = harness.s.ForEachFaultEpoch(harness.store, func(epoch abi.ChainEpoch, faults *abi.BitField) error {
			assert.Fail(t, "unexpected fault epoch: %v", epoch)
			return nil
		})
		require.NoError(t, err)
	})
}

func TestRecoveriesBitfield(t *testing.T) {
	t.Run("Add new recoveries happy path", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		// set of sectors, the larger numbers here are not significant
		sectorNos := []uint64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}
		harness.addRecoveries(sectorNos...)
		assert.Equal(t, uint64(len(sectorNos)), harness.getRecoveriesCount())
	})

	t.Run("Add new recoveries excludes duplicates", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		sectorNos := []uint64{1, 1, 2, 2, 3, 4, 5}
		harness.addRecoveries(sectorNos...)
		assert.Equal(t, uint64(5), harness.getRecoveriesCount())
	})

	t.Run("Remove recoveries happy path", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		sectorNos := []uint64{1, 2, 3, 4, 5}
		harness.addRecoveries(sectorNos...)
		assert.Equal(t, uint64(len(sectorNos)), harness.getRecoveriesCount())

		harness.removeRecoveries(1, 3, 5)
		assert.Equal(t, uint64(2), harness.getRecoveriesCount())

		recoveries, err := harness.s.Recoveries.All(uint64(len(sectorNos)))
		assert.NoError(t, err)
		assert.Equal(t, []uint64{2, 4}, recoveries)
	})
}

func TestPostSubmissionsBitfield(t *testing.T) {
	t.Run("Add new submission happy path", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		// set of sectors, the larger numbers here are not significant
		partitionNos := []uint64{10, 20, 30, 40}
		harness.addPoStSubmissions(partitionNos...)
		assert.Equal(t, uint64(len(partitionNos)), harness.getPoStSubmissionsCount())
	})

	t.Run("Add new submission excludes duplicates", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		sectorNos := []uint64{1, 1, 2, 2, 3, 4, 5}
		harness.addPoStSubmissions(sectorNos...)
		assert.Equal(t, uint64(5), harness.getPoStSubmissionsCount())
	})

	t.Run("Clear submission happy path", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))

		sectorNos := []uint64{1, 2, 3, 4, 5}
		harness.addPoStSubmissions(sectorNos...)
		assert.Equal(t, uint64(len(sectorNos)), harness.getPoStSubmissionsCount())

		harness.clearPoStSubmissions()
		assert.Equal(t, uint64(0), harness.getPoStSubmissionsCount())
	})
}

func TestVesting_AddLockedFunds_Table(t *testing.T) {
	vestStartDelay := abi.ChainEpoch(10)
	vestSum := int64(100)

	testcase := []struct {
		desc        string
		vspec       *miner.VestSpec
		periodStart abi.ChainEpoch
		vepocs      []int64
	}{
		{
			desc: "vest funds in a single epoch",
			vspec: &miner.VestSpec{
				InitialDelay: 0,
				VestPeriod:   1,
				StepDuration: 1,
				Quantization: 1,
			},
			vepocs: []int64{0, 0, 100, 0},
		},
		{
			desc: "vest funds with period=2",
			vspec: &miner.VestSpec{
				InitialDelay: 0,
				VestPeriod:   2,
				StepDuration: 1,
				Quantization: 1,
			},
			vepocs: []int64{0, 0, 50, 50, 0},
		},
		{
			desc: "vest funds with period=2 quantization=2",
			vspec: &miner.VestSpec{
				InitialDelay: 0,
				VestPeriod:   2,
				StepDuration: 1,
				Quantization: 2,
			},
			vepocs: []int64{0, 0, 0, 100, 0},
		},
		{desc: "vest funds with period=3",
			vspec: &miner.VestSpec{
				InitialDelay: 0,
				VestPeriod:   3,
				StepDuration: 1,
				Quantization: 1,
			},
			vepocs: []int64{0, 0, 33, 33, 34, 0},
		},
		{
			desc: "vest funds with period=3 quantization=2",
			vspec: &miner.VestSpec{
				InitialDelay: 0,
				VestPeriod:   3,
				StepDuration: 1,
				Quantization: 2,
			},
			vepocs: []int64{0, 0, 0, 66, 0, 34, 0},
		},
		{desc: "vest funds with period=2 step=2",
			vspec: &miner.VestSpec{
				InitialDelay: 0,
				VestPeriod:   2,
				StepDuration: 2,
				Quantization: 1,
			},
			vepocs: []int64{0, 0, 0, 100, 0},
		},
		{
			desc: "vest funds with period=5 step=2",
			vspec: &miner.VestSpec{
				InitialDelay: 0,
				VestPeriod:   5,
				StepDuration: 2,
				Quantization: 1,
			},
			vepocs: []int64{0, 0, 0, 40, 0, 40, 0, 20, 0},
		},
		{
			desc: "vest funds with delay=1 period=5 step=2",
			vspec: &miner.VestSpec{
				InitialDelay: 1,
				VestPeriod:   5,
				StepDuration: 2,
				Quantization: 1,
			},
			vepocs: []int64{0, 0, 0, 0, 40, 0, 40, 0, 20, 0},
		},
		{
			desc: "vest funds with period=5 step=2 quantization=2",
			vspec: &miner.VestSpec{
				InitialDelay: 0,
				VestPeriod:   5,
				StepDuration: 2,
				Quantization: 2,
			},
			vepocs: []int64{0, 0, 0, 40, 0, 40, 0, 20, 0},
		},
		{
			desc: "vest funds with period=5 step=3 quantization=1",
			vspec: &miner.VestSpec{
				InitialDelay: 0,
				VestPeriod:   5,
				StepDuration: 3,
				Quantization: 1,
			},
			vepocs: []int64{0, 0, 0, 0, 60, 0, 0, 40, 0},
		},
		{
			desc: "vest funds with period=5 step=3 quantization=2",
			vspec: &miner.VestSpec{
				InitialDelay: 0,
				VestPeriod:   5,
				StepDuration: 3,
				Quantization: 2,
			},
			vepocs: []int64{0, 0, 0, 0, 0, 80, 0, 20, 0},
		},
		{
			desc: "(step greater than period) vest funds with period=5 step=6 quantization=1",
			vspec: &miner.VestSpec{
				InitialDelay: 0,
				VestPeriod:   5,
				StepDuration: 6,
				Quantization: 1,
			},
			vepocs: []int64{0, 0, 0, 0, 0, 0, 0, 100, 0},
		},
		{
			desc: "vest funds with delay=5 period=5 step=1 quantization=1",
			vspec: &miner.VestSpec{
				InitialDelay: 5,
				VestPeriod:   5,
				StepDuration: 1,
				Quantization: 1,
			},
			vepocs: []int64{0, 0, 0, 0, 0, 0, 0, 20, 20, 20, 20, 20, 0},
		},
		{
			desc: "vest funds with offset 0",
			vspec: &miner.VestSpec{
				InitialDelay: 0,
				VestPeriod:   10,
				StepDuration: 2,
				Quantization: 2,
			},
			vepocs: []int64{0, 0, 0, 20, 0, 20, 0, 20, 0, 20, 0, 20},
		},
		{
			desc: "vest funds with offset 1",
			vspec: &miner.VestSpec{
				InitialDelay: 0,
				VestPeriod:   10,
				StepDuration: 2,
				Quantization: 2,
			},
			periodStart: abi.ChainEpoch(1),
			// start epoch is at 11 instead of 10 so vepocs are shifted by one from above case
			vepocs: []int64{0, 0, 0, 20, 0, 20, 0, 20, 0, 20, 0, 20},
		},
		{
			desc: "vest funds with proving period start > quantization unit",
			vspec: &miner.VestSpec{
				InitialDelay: 0,
				VestPeriod:   10,
				StepDuration: 2,
				Quantization: 2,
			},
			// 55 % 2 = 1 so expect same vepocs with offset 1 as in previous case
			periodStart: abi.ChainEpoch(55),
			vepocs:      []int64{0, 0, 0, 20, 0, 20, 0, 20, 0, 20, 0, 20},
		},
	}
	for _, tc := range testcase {
		t.Run(tc.desc, func(t *testing.T) {
			harness := constructStateHarness(t, tc.periodStart)
			vestStart := tc.periodStart + vestStartDelay

			harness.addLockedFunds(vestStart, abi.NewTokenAmount(vestSum), tc.vspec)
			assert.Equal(t, abi.NewTokenAmount(vestSum), harness.s.LockedFunds)

			var totalVested int64
			for e, v := range tc.vepocs {
				assert.Equal(t, abi.NewTokenAmount(v), harness.unlockVestedFunds(vestStart+abi.ChainEpoch(e)))
				totalVested += v
				assert.Equal(t, vestSum-totalVested, harness.s.LockedFunds.Int64())
			}

			assert.Equal(t, abi.NewTokenAmount(vestSum), abi.NewTokenAmount(totalVested))
			assert.True(t, harness.vestingFundsStoreEmpty())
			assert.Zero(t, harness.s.LockedFunds.Int64())
		})
	}
}

func TestVestingFunds_AddLockedFunds(t *testing.T) {
	t.Run("LockedFunds increases with sequential calls", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))
		vspec := &miner.VestSpec{
			InitialDelay: 0,
			VestPeriod:   1,
			StepDuration: 1,
			Quantization: 1,
		}

		vestStart := abi.ChainEpoch(10)
		vestSum := abi.NewTokenAmount(100)

		harness.addLockedFunds(vestStart, vestSum, vspec)
		assert.Equal(t, vestSum, harness.s.LockedFunds)

		harness.addLockedFunds(vestStart, vestSum, vspec)
		assert.Equal(t, big.Mul(vestSum, big.NewInt(2)), harness.s.LockedFunds)
	})

	t.Run("Vests when quantize, step duration, and vesting period are coprime", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))
		vspec := &miner.VestSpec{
			InitialDelay: 0,
			VestPeriod:   27,
			StepDuration: 5,
			Quantization: 7,
		}
		vestStart := abi.ChainEpoch(10)
		vestSum := abi.NewTokenAmount(100)
		harness.addLockedFunds(vestStart, vestSum, vspec)
		assert.Equal(t, vestSum, harness.s.LockedFunds)

		totalVested := abi.NewTokenAmount(0)
		for e := vestStart; e <= 43; e++ {
			amountVested := harness.unlockVestedFunds(e)
			switch e {
			case 22:
				assert.Equal(t, abi.NewTokenAmount(40), amountVested)
				totalVested = big.Add(totalVested, amountVested)
			case 29:
				assert.Equal(t, abi.NewTokenAmount(26), amountVested)
				totalVested = big.Add(totalVested, amountVested)
			case 36:
				assert.Equal(t, abi.NewTokenAmount(26), amountVested)
				totalVested = big.Add(totalVested, amountVested)
			case 43:
				assert.Equal(t, abi.NewTokenAmount(8), amountVested)
				totalVested = big.Add(totalVested, amountVested)
			default:
				assert.Equal(t, abi.NewTokenAmount(0), amountVested)
			}
		}
		assert.Equal(t, vestSum, totalVested)
		assert.Zero(t, harness.s.LockedFunds.Int64())
		assert.True(t, harness.vestingFundsStoreEmpty())
	})
}

func TestVestingFunds_UnvestedFunds(t *testing.T) {
	t.Run("Unlock unvested funds leaving bucket with non-zero tokens", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))
		vspec := &miner.VestSpec{
			InitialDelay: 0,
			VestPeriod:   5,
			StepDuration: 1,
			Quantization: 1,
		}
		vestStart := abi.ChainEpoch(100)
		vestSum := abi.NewTokenAmount(100)

		harness.addLockedFunds(vestStart, vestSum, vspec)

		amountUnlocked := harness.unlockUnvestedFunds(vestStart, big.NewInt(39))
		assert.Equal(t, big.NewInt(39), amountUnlocked)

		assert.Equal(t, abi.NewTokenAmount(0), harness.unlockVestedFunds(vestStart))
		assert.Equal(t, abi.NewTokenAmount(0), harness.unlockVestedFunds(vestStart+1))

		// expected to be zero due to unlocking of UNvested funds
		assert.Equal(t, abi.NewTokenAmount(0), harness.unlockVestedFunds(vestStart+2))
		// expected to be non-zero due to unlocking of UNvested funds
		assert.Equal(t, abi.NewTokenAmount(1), harness.unlockVestedFunds(vestStart+3))

		assert.Equal(t, abi.NewTokenAmount(20), harness.unlockVestedFunds(vestStart+4))
		assert.Equal(t, abi.NewTokenAmount(20), harness.unlockVestedFunds(vestStart+5))
		assert.Equal(t, abi.NewTokenAmount(20), harness.unlockVestedFunds(vestStart+6))

		assert.Equal(t, abi.NewTokenAmount(0), harness.unlockVestedFunds(vestStart+7))

		assert.Zero(t, harness.s.LockedFunds.Int64())
		assert.True(t, harness.vestingFundsStoreEmpty())
	})

	t.Run("Unlock unvested funds leaving bucket with zero tokens", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))
		vspec := &miner.VestSpec{
			InitialDelay: 0,
			VestPeriod:   5,
			StepDuration: 1,
			Quantization: 1,
		}
		vestStart := abi.ChainEpoch(100)
		vestSum := abi.NewTokenAmount(100)

		harness.addLockedFunds(vestStart, vestSum, vspec)

		amountUnlocked := harness.unlockUnvestedFunds(vestStart, big.NewInt(40))
		assert.Equal(t, big.NewInt(40), amountUnlocked)

		assert.Equal(t, abi.NewTokenAmount(0), harness.unlockVestedFunds(vestStart))
		assert.Equal(t, abi.NewTokenAmount(0), harness.unlockVestedFunds(vestStart+1))

		// expected to be zero due to unlocking of UNvested funds
		assert.Equal(t, abi.NewTokenAmount(0), harness.unlockVestedFunds(vestStart+2))
		assert.Equal(t, abi.NewTokenAmount(0), harness.unlockVestedFunds(vestStart+3))

		assert.Equal(t, abi.NewTokenAmount(20), harness.unlockVestedFunds(vestStart+4))
		assert.Equal(t, abi.NewTokenAmount(20), harness.unlockVestedFunds(vestStart+5))
		assert.Equal(t, abi.NewTokenAmount(20), harness.unlockVestedFunds(vestStart+6))

		assert.Equal(t, abi.NewTokenAmount(0), harness.unlockVestedFunds(vestStart+7))

		assert.Zero(t, harness.s.LockedFunds.Int64())
		assert.True(t, harness.vestingFundsStoreEmpty())
	})

	t.Run("Unlock all unvested funds", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))
		vspec := &miner.VestSpec{
			InitialDelay: 0,
			VestPeriod:   5,
			StepDuration: 1,
			Quantization: 1,
		}
		vestStart := abi.ChainEpoch(10)
		vestSum := abi.NewTokenAmount(100)
		harness.addLockedFunds(vestStart, vestSum, vspec)
		unvestedFunds := harness.unlockUnvestedFunds(vestStart, vestSum)
		assert.Equal(t, vestSum, unvestedFunds)

		assert.Zero(t, harness.s.LockedFunds.Int64())
		assert.True(t, harness.vestingFundsStoreEmpty())
	})

	t.Run("Unlock unvested funds value greater than LockedFunds", func(t *testing.T) {
		harness := constructStateHarness(t, abi.ChainEpoch(0))
		vspec := &miner.VestSpec{
			InitialDelay: 0,
			VestPeriod:   1,
			StepDuration: 1,
			Quantization: 1,
		}
		vestStart := abi.ChainEpoch(10)
		vestSum := abi.NewTokenAmount(100)
		harness.addLockedFunds(vestStart, vestSum, vspec)
		unvestedFunds := harness.unlockUnvestedFunds(vestStart, abi.NewTokenAmount(200))
		assert.Equal(t, vestSum, unvestedFunds)

		assert.Zero(t, harness.s.LockedFunds.Int64())
		assert.True(t, harness.vestingFundsStoreEmpty())

	})

}

type stateHarness struct {
	t testing.TB

	s     *miner.State
	store adt.Store
}

//
// Vesting Store
//

func (h *stateHarness) addLockedFunds(epoch abi.ChainEpoch, sum abi.TokenAmount, spec *miner.VestSpec) {
	err := h.s.AddLockedFunds(h.store, epoch, sum, spec)
	require.NoError(h.t, err)
}

func (h *stateHarness) unlockUnvestedFunds(epoch abi.ChainEpoch, target abi.TokenAmount) abi.TokenAmount {
	amount, err := h.s.UnlockUnvestedFunds(h.store, epoch, target)
	require.NoError(h.t, err)
	return amount
}

func (h *stateHarness) unlockVestedFunds(epoch abi.ChainEpoch) abi.TokenAmount {
	amount, err := h.s.UnlockVestedFunds(h.store, epoch)
	require.NoError(h.t, err)
	return amount
}

func (h *stateHarness) vestingFundsStoreEmpty() bool {
	vestingFunds, err := adt.AsArray(h.store, h.s.VestingFunds)
	require.NoError(h.t, err)
	empty := true
	lockedEntry := abi.NewTokenAmount(0)
	err = vestingFunds.ForEach(&lockedEntry, func(k int64) error {
		empty = false
		return nil
	})
	require.NoError(h.t, err)
	return empty
}

//
// PostSubmissions Bitfield
//

func (h *stateHarness) addPoStSubmissions(partitionNos ...uint64) {
	err := h.s.AddPoStSubmissions(bitfield.NewFromSet(partitionNos))
	require.NoError(h.t, err)
}

func (h *stateHarness) clearPoStSubmissions() {
	err := h.s.ClearPoStSubmissions()
	require.NoError(h.t, err)
}

func (h *stateHarness) getPoStSubmissionsCount() uint64 {
	count, err := h.s.PostSubmissions.Count()
	require.NoError(h.t, err)
	return count
}

//
// Recoveries Bitfield
//

func (h *stateHarness) addRecoveries(sectorNos ...uint64) {
	bf := bitfield.NewFromSet(sectorNos)
	err := h.s.AddRecoveries(bf)
	require.NoError(h.t, err)
}

func (h *stateHarness) removeRecoveries(sectorNos ...uint64) {
	bf := bitfield.NewFromSet(sectorNos)
	err := h.s.RemoveRecoveries(bf)
	require.NoError(h.t, err)
}

func (h *stateHarness) getRecoveriesCount() uint64 {
	count, err := h.s.Recoveries.Count()
	require.NoError(h.t, err)
	return count
}

//
// Faults Store
//

func (h *stateHarness) addFaults(epoch abi.ChainEpoch, sectorNos ...uint64) {
	bf := bitfield.NewFromSet(sectorNos)
	err := h.s.AddFaults(h.store, bf, epoch)
	require.NoError(h.t, err)
}

func (h *stateHarness) removeFaults(sectorNos ...uint64) {
	bf := bitfield.NewFromSet(sectorNos)
	err := h.s.RemoveFaults(h.store, bf)
	require.NoError(h.t, err)
}

//
// Sector Expiration Store
//

func (h *stateHarness) getSectorExpirations(expiry abi.ChainEpoch) []uint64 {
	bf, err := h.s.GetSectorExpirations(h.store, expiry)
	require.NoError(h.t, err)
	sectors, err := bf.All(miner.SectorsMax)
	require.NoError(h.t, err)
	return sectors
}

func (h *stateHarness) addSectorExpiration(expiry abi.ChainEpoch, sectorNos ...uint64) {
	infos := make([]*miner.SectorOnChainInfo, len(sectorNos))
	for i, sectorNo := range sectorNos {
		infos[i] = &miner.SectorOnChainInfo{
			SectorNumber: abi.SectorNumber(sectorNo),
			Expiration:   expiry,
		}
	}
	err := h.s.AddSectorExpirations(h.store, infos...)
	require.NoError(h.t, err)
}

func (h *stateHarness) removeSectorExpiration(expiry abi.ChainEpoch, sectorNos ...uint64) {
	infos := make([]*miner.SectorOnChainInfo, len(sectorNos))
	for i, sectorNo := range sectorNos {
		infos[i] = &miner.SectorOnChainInfo{
			SectorNumber: abi.SectorNumber(sectorNo),
			Expiration:   expiry,
		}
	}
	err := h.s.RemoveSectorExpirations(h.store, infos...)
	require.NoError(h.t, err)
}

func (h *stateHarness) clearSectorExpiration(excitations ...abi.ChainEpoch) {
	err := h.s.ClearSectorExpirations(h.store, excitations...)
	require.NoError(h.t, err)
}

//
// NewSectors BitField Assertions
//

func (h *stateHarness) addNewSectors(sectorNos ...abi.SectorNumber) {
	err := h.s.AddNewSectors(sectorNos...)
	require.NoError(h.t, err)
}

// makes a bit field from the passed sector numbers
func (h *stateHarness) removeNewSectors(sectorNos ...uint64) {
	bf := bitfield.NewFromSet(sectorNos)
	err := h.s.RemoveNewSectors(bf)
	require.NoError(h.t, err)
}

func (h *stateHarness) getNewSectorCount() uint64 {
	out, err := h.s.NewSectors.Count()
	require.NoError(h.t, err)
	return out
}

//
// Sector Store Assertion Operations
//

func (h *stateHarness) hasSectorNo(sectorNo abi.SectorNumber) bool {
	found, err := h.s.HasSectorNo(h.store, sectorNo)
	require.NoError(h.t, err)
	return found
}

func (h *stateHarness) putSector(sector *miner.SectorOnChainInfo) {
	err := h.s.PutSectors(h.store, sector)
	require.NoError(h.t, err)
}

func (h *stateHarness) getSector(sectorNo abi.SectorNumber) *miner.SectorOnChainInfo {
	sectors, found, err := h.s.GetSector(h.store, sectorNo)
	require.NoError(h.t, err)
	assert.True(h.t, found)
	assert.NotNil(h.t, sectors)
	return sectors
}

// makes a bit field from the passed sector numbers
func (h *stateHarness) deleteSectors(sectorNos ...uint64) {
	bf := bitfield.NewFromSet(sectorNos)
	err := h.s.DeleteSectors(h.store, bf)
	require.NoError(h.t, err)
}

//
// Precommit Store Operations
//

func (h *stateHarness) putPreCommit(info *miner.SectorPreCommitOnChainInfo) {
	err := h.s.PutPrecommittedSector(h.store, info)
	require.NoError(h.t, err)
}

func (h *stateHarness) getPreCommit(sectorNo abi.SectorNumber) *miner.SectorPreCommitOnChainInfo {
	out, found, err := h.s.GetPrecommittedSector(h.store, sectorNo)
	require.NoError(h.t, err)
	assert.True(h.t, found)
	return out
}

func (h *stateHarness) hasPreCommit(sectorNo abi.SectorNumber) bool {
	_, found, err := h.s.GetPrecommittedSector(h.store, sectorNo)
	require.NoError(h.t, err)
	return found
}

func (h *stateHarness) deletePreCommit(sectorNo abi.SectorNumber) {
	err := h.s.DeletePrecommittedSectors(h.store, sectorNo)
	require.NoError(h.t, err)
}

func constructStateHarness(t *testing.T, periodBoundary abi.ChainEpoch) *stateHarness {
	// store init
	store := ipld.NewADTStore(context.Background())
	emptyMap, err := adt.MakeEmptyMap(store).Root()
	require.NoError(t, err)

	emptyArray, err := adt.MakeEmptyArray(store).Root()
	require.NoError(t, err)

	emptyDeadlines := miner.ConstructDeadlines()
	emptyDeadlinesCid, err := store.Put(context.Background(), emptyDeadlines)
	require.NoError(t, err)

	// state field init
	owner := tutils.NewBLSAddr(t, 1)
	worker := tutils.NewBLSAddr(t, 2)

	testSealProofType := abi.RegisteredSealProof_StackedDrg2KiBV1

	sectorSize, err := testSealProofType.SectorSize()
	require.NoError(t, err)

	partitionSectors, err := testSealProofType.WindowPoStPartitionSectors()
	require.NoError(t, err)

	info := miner.MinerInfo{
		Owner:                      owner,
		Worker:                     worker,
		PendingWorkerKey:           nil,
		PeerId:                     abi.PeerID("peer"),
		Multiaddrs:                 testMultiaddrs,
		SealProofType:              testSealProofType,
		SectorSize:                 sectorSize,
		WindowPoStPartitionSectors: partitionSectors,
	}
	infoCid, err := store.Put(context.Background(), &info)
	require.NoError(t, err)

	state, err := miner.ConstructState(infoCid, periodBoundary, emptyArray, emptyMap, emptyDeadlinesCid)
	require.NoError(t, err)

	// assert NewSectors bitfield was constructed correctly (empty)
	newSectorsCount, err := state.NewSectors.Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), newSectorsCount)

	return &stateHarness{
		t: t,

		s:     state,
		store: store,
	}
}

//
// Type Construction Methods
//

// returns a unique SectorPreCommitOnChainInfo with each invocation with SectorNumber set to `sectorNo`.
func newSectorPreCommitOnChainInfo(sectorNo abi.SectorNumber, sealed cid.Cid, deposit abi.TokenAmount, epoch abi.ChainEpoch) *miner.SectorPreCommitOnChainInfo {
	info := newSectorPreCommitInfo(sectorNo, sealed)
	return &miner.SectorPreCommitOnChainInfo{
		Info:               *info,
		PreCommitDeposit:   deposit,
		PreCommitEpoch:     epoch,
		DealWeight:         big.Zero(),
		VerifiedDealWeight: big.Zero(),
	}
}

const (
	sectorSealRandEpochValue = abi.ChainEpoch(1)
	sectorExpiration         = abi.ChainEpoch(1)
)

// returns a unique SectorOnChainInfo with each invocation with SectorNumber set to `sectorNo`.
func newSectorOnChainInfo(sectorNo abi.SectorNumber, sealed cid.Cid, weight big.Int, activation abi.ChainEpoch) *miner.SectorOnChainInfo {
	return &miner.SectorOnChainInfo{
		SectorNumber:       sectorNo,
		SealProof:          abi.RegisteredSealProof_StackedDrg32GiBV1,
		SealedCID:          sealed,
		DealIDs:            nil,
		Activation:         activation,
		Expiration:         sectorExpiration,
		DealWeight:         weight,
		VerifiedDealWeight: weight,
		InitialPledge:      abi.NewTokenAmount(0),
	}
}

// returns a unique SectorPreCommitInfo with each invocation with SectorNumber set to `sectorNo`.
func newSectorPreCommitInfo(sectorNo abi.SectorNumber, sealed cid.Cid) *miner.SectorPreCommitInfo {
	return &miner.SectorPreCommitInfo{
		SealProof:     abi.RegisteredSealProof_StackedDrg32GiBV1,
		SectorNumber:  sectorNo,
		SealedCID:     sealed,
		SealRandEpoch: sectorSealRandEpochValue,
		DealIDs:       nil,
		Expiration:    sectorExpiration,
	}
}
