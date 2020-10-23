package miner

import (
	"sort"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
)

// VestingFunds represents the vesting table state for the miner.
// It is a slice of (VestingEpoch, VestingAmount).
// The slice will always be sorted by the VestingEpoch.
type VestingFunds struct {
	Funds []VestingFund
}

func (v *VestingFunds) unlockVestedFunds(currEpoch abi.ChainEpoch) abi.TokenAmount {
	amountUnlocked := abi.NewTokenAmount(0)

	lastIndexToRemove := -1
	for i, vf := range v.Funds {
		if vf.Epoch >= currEpoch {
			break
		}

		amountUnlocked = big.Add(amountUnlocked, vf.Amount)
		lastIndexToRemove = i
	}

	// remove all entries upto and including lastIndexToRemove
	if lastIndexToRemove != -1 {
		v.Funds = v.Funds[lastIndexToRemove+1:]
	}

	return amountUnlocked
}

func (v *VestingFunds) addLockedFunds(currEpoch abi.ChainEpoch, vestingSum abi.TokenAmount,
	provingPeriodStart abi.ChainEpoch, spec *VestSpec) {
	// maps the epochs in VestingFunds to their indices in the slice
	epochToIndex := make(map[abi.ChainEpoch]int, len(v.Funds))
	for i, vf := range v.Funds {
		epochToIndex[vf.Epoch] = i
	}

	// Quantization is aligned with when regular cron will be invoked, in the last epoch of deadlines.
	vestBegin := currEpoch + spec.InitialDelay // Nothing unlocks here, this is just the start of the clock.
	vestPeriod := big.NewInt(int64(spec.VestPeriod))
	vestedSoFar := big.Zero()
	for e := vestBegin + spec.StepDuration; vestedSoFar.LessThan(vestingSum); e += spec.StepDuration {
		vestEpoch := quantizeUp(e, spec.Quantization, provingPeriodStart)
		elapsed := vestEpoch - vestBegin

		targetVest := big.Zero() //nolint:ineffassign
		if elapsed < spec.VestPeriod {
			// Linear vesting
			targetVest = big.Div(big.Mul(vestingSum, big.NewInt(int64(elapsed))), vestPeriod)
		} else {
			targetVest = vestingSum
		}

		vestThisTime := big.Sub(targetVest, vestedSoFar)
		vestedSoFar = targetVest

		// epoch already exists. Load existing entry
		// and update amount.
		if index, ok := epochToIndex[vestEpoch]; ok {
			currentAmt := v.Funds[index].Amount
			v.Funds[index].Amount = big.Add(currentAmt, vestThisTime)
		} else {
			// append a new entry -> slice will be sorted by epoch later.
			entry := VestingFund{Epoch: vestEpoch, Amount: vestThisTime}
			v.Funds = append(v.Funds, entry)
			epochToIndex[vestEpoch] = len(v.Funds) - 1
		}
	}

	// sort slice by epoch
	sort.Slice(v.Funds, func(first, second int) bool {
		return v.Funds[first].Epoch < v.Funds[second].Epoch
	})
}

func (v *VestingFunds) unlockUnvestedFunds(currEpoch abi.ChainEpoch, target abi.TokenAmount) abi.TokenAmount {
	amountUnlocked := abi.NewTokenAmount(0)
	lastIndexToRemove := -1
	startIndexForRemove := 0

	// retain funds that should have vested and unlock unvested funds
	for i, vf := range v.Funds {
		if amountUnlocked.GreaterThanEqual(target) {
			break
		}

		if vf.Epoch >= currEpoch {
			unlockAmount := big.Min(big.Sub(target, amountUnlocked), vf.Amount)
			amountUnlocked = big.Add(amountUnlocked, unlockAmount)
			newAmount := big.Sub(vf.Amount, unlockAmount)

			if newAmount.IsZero() {
				lastIndexToRemove = i
			} else {
				v.Funds[i].Amount = newAmount
			}
		} else {
			startIndexForRemove = i + 1
		}
	}

	// remove all entries in [startIndexForRemove, lastIndexToRemove]
	if lastIndexToRemove != -1 {
		v.Funds = append(v.Funds[0:startIndexForRemove], v.Funds[lastIndexToRemove+1:]...)
	}

	return amountUnlocked
}

// VestingFund represents miner funds that will vest at the given epoch.
type VestingFund struct {
	Epoch  abi.ChainEpoch
	Amount abi.TokenAmount
}

// ConstructVestingFunds constructs empty VestingFunds state.
func ConstructVestingFunds() *VestingFunds {
	v := new(VestingFunds)
	v.Funds = nil
	return v
}
