package repo

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

var (
	// ErrMissingModeratorFee indicates the fee schedule is missing
	ErrMissingModeratorFee = errors.New("moderator info is missing fee schedule")
	// ErrUnknownModeratorFeeType indicates the feeType is unknown
	ErrUnknownModeratorFeeType = errors.New("moderator fee type is unknown")
	// ErrModeratorFeeHasNonPositivePercent indicates when the percentage is non-positive but should be
	ErrModeratorFeeHasNonPositivePercent = errors.New("percentage moderator fee should be greater than zero")
	// ErrFixedFeeHasNonZeroPercentage indicates when the percentage is not zero but should be
	ErrFixedFeeHasNonZeroPercentage = errors.New("fixed moderator fee should have a zero percentage amount")
	// ErrPercentageFeeHasFixedFee indicates that a fixed fee is included when there should not be
	ErrPercentageFeeHasFixedFee = fmt.Errorf("percentage moderator fee should not include a fixed fee or should use (%s) feeType", pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String())
	// ErrModeratorFixedFeeIsMissing indicates when the fixed fee is missing
	ErrModeratorFixedFeeIsMissing = fmt.Errorf("fixed moderator fee is missing or should use (%s) feeType", pb.Moderator_Fee_PERCENTAGE.String())
	// ErrModeratorFixedFeeIsNonPositive indicates that the fixed fee is non-positive
	ErrModeratorFixedFeeIsNonPositive = errors.New("fixed moderator fee is not positive")
)

// ModeratorFixedFee represents the value of a fixed moderation fee
type ModeratorFixedFee struct {
	Amount         string             `json:"bigAmount,omitempty"`
	AmountCurrency CurrencyDefinition `json:"amountCurrency,omitempty"`
}

// ModeratorFee represents the moderator's fee schedule
type ModeratorFee struct {
	FixedFee   *ModeratorFixedFee `json:"fixedFee,omitempty"`
	Percentage float32            `json:"percentage,omitempty"`
	FeeType    string             `json:"feeType,omitempty"`
}

// ModeratorInfo represents the terms for the moderator's services
type ModeratorInfo struct {
	Fee *ModeratorFee `json:"fee,omitempty"`
}

// Profile presents the user's metadata
type Profile struct {
	ModeratorInfo *ModeratorInfo `json:"moderatorInfo,omitempty"`
}

func ProfileFromProtobuf(p *pb.Profile) (*Profile, error) {
	var modInfo *ModeratorInfo
	if p.ModeratorInfo != nil {
		var (
			fees             = p.ModeratorInfo.Fee
			amtCurrency, err = AllCurrencies().Lookup(fees.FixedFee.AmountCurrency.Code)
		)
		if err != nil {
			return nil, fmt.Errorf("lookup currency (%s): %s", fees.FixedFee.AmountCurrency, err.Error())
		}
		amtCurrency.Divisibility = uint(fees.FixedFee.AmountCurrency.Divisibility)

		modInfo = &ModeratorInfo{
			Fee: &ModeratorFee{
				FeeType: fees.FeeType.String(),
				FixedFee: &ModeratorFixedFee{
					Amount:         fees.FixedFee.BigAmount,
					AmountCurrency: amtCurrency,
				},
				Percentage: fees.Percentage,
			},
		}
	}

	return &Profile{
		ModeratorInfo: modInfo,
	}, nil
}

// Valid indicates whether the Profile is valid by returning an error when
// any part of the data is not as expected
func (p *Profile) Valid() error {
	if err := p.validateModeratorFees(); err != nil {
		return err
	}
	return nil
}

func (p *Profile) validateModeratorFees() error {
	if p.ModeratorInfo == nil {
		return nil
	}

	if p.ModeratorInfo.Fee == nil {
		return ErrMissingModeratorFee
	}

	var validateFixedFee = func() error {
		if p.ModeratorInfo.Fee.FixedFee == nil {
			return ErrModeratorFixedFeeIsMissing
		}
		if err := p.ModeratorInfo.Fee.FixedFee.AmountCurrency.Valid(); err != nil {
			return fmt.Errorf("invalid fixed fee currency: %s", err.Error())
		}
		if amt, ok := new(big.Int).SetString(p.ModeratorInfo.Fee.FixedFee.Amount, 10); !ok || amt.Cmp(big.NewInt(0)) <= 0 {
			return ErrModeratorFixedFeeIsNonPositive
		}
		return nil
	}

	switch p.ModeratorInfo.Fee.FeeType {
	case pb.Moderator_Fee_FIXED.String():
		if p.ModeratorInfo.Fee.Percentage != 0 {
			return ErrFixedFeeHasNonZeroPercentage
		}
		if err := validateFixedFee(); err != nil {
			return err
		}
	case pb.Moderator_Fee_PERCENTAGE.String():
		if p.ModeratorInfo.Fee.Percentage <= 0 {
			return ErrModeratorFeeHasNonPositivePercent
		}
		if p.ModeratorInfo.Fee.FixedFee != nil {
			return ErrPercentageFeeHasFixedFee
		}
	case pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String():
		if p.ModeratorInfo.Fee.Percentage <= 0 {
			return ErrModeratorFeeHasNonPositivePercent
		}
		if err := validateFixedFee(); err != nil {
			return err
		}
	default:
		return ErrUnknownModeratorFeeType
	}

	return nil
}
