package market

import (
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
)

// DealUpdatesInterval is the number of blocks between payouts for deals
const DealUpdatesInterval = 100

// Bounds (inclusive) on deal duration
func dealDurationBounds(size abi.PaddedPieceSize) (min abi.ChainEpoch, max abi.ChainEpoch) {
	// Cryptoeconomic modelling to date has used an assumption of a maximum deal duration of up to one year.
	// It very likely can be much longer, but we're not sure yet.
	return abi.ChainEpoch(0), abi.ChainEpoch(1 * builtin.EpochsInYear) // PARAM_FINISH
}

func dealPricePerEpochBounds(size abi.PaddedPieceSize, duration abi.ChainEpoch) (min abi.TokenAmount, max abi.TokenAmount) {
	return abi.NewTokenAmount(0), abi.TotalFilecoin // PARAM_FINISH
}

func dealProviderCollateralBounds(pieceSize abi.PaddedPieceSize, duration abi.ChainEpoch) (min abi.TokenAmount, max abi.TokenAmount) {
	return abi.NewTokenAmount(0), abi.TotalFilecoin // PARAM_FINISH
}

func dealClientCollateralBounds(pieceSize abi.PaddedPieceSize, duration abi.ChainEpoch) (min abi.TokenAmount, max abi.TokenAmount) {
	return abi.NewTokenAmount(0), abi.TotalFilecoin // PARAM_FINISH
}

// Penalty to provider deal collateral if the deadline expires before sector commitment.
func collateralPenaltyForDealActivationMissed(providerCollateral abi.TokenAmount) abi.TokenAmount {
	return providerCollateral // PARAM_FINISH
}

// Computes the weight for a deal proposal, which is a function of its size and duration.
func DealWeight(proposal *DealProposal) abi.DealWeight {
	dealDuration := big.NewInt(int64(proposal.Duration()))
	dealSize := big.NewIntUnsigned(uint64(proposal.PieceSize))
	dealSpaceTime := big.Mul(dealDuration, dealSize)
	return dealSpaceTime
}
