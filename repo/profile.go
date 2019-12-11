package repo

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

var (
	// ErrModeratorInfoMissing indicates when the moderator information is
	// missing while also indicating they are a moderator
	ErrModeratorInfoMissing = errors.New("moderator is enabled but information is missing")
	// ErrNonModeratorShouldNotHaveInfo indicates when the moderator information
	// is present, but not indicating moderator is enabled
	ErrNonModeratorShouldNotHaveInfo = errors.New("moderator information is provided but moderator is not enabled")
	// ErrMissingModeratorFee indicates the fee schedule is missing
	ErrMissingModeratorFee = errors.New("moderator info is missing fee schedule")
	// ErrUnknownModeratorFeeType indicates the feeType is unknown
	ErrUnknownModeratorFeeType = errors.New("moderator fee type is unknown")
	// ErrModeratorFeeHasNegativePercentage indicates when the percentage is non-positive but should be
	ErrModeratorFeeHasNegativePercentage = errors.New("percentage moderator fee should not be negative")
	// ErrFixedFeeHasNonZeroPercentage indicates when the percentage is not zero but should be
	ErrFixedFeeHasNonZeroPercentage = errors.New("fixed moderator fee should have a zero percentage amount")
	// ErrPercentageFeeHasFixedFee indicates that a fixed fee is included when there should not be
	ErrPercentageFeeHasFixedFee = fmt.Errorf("percentage moderator fee should not include a fixed fee or should use (%s) feeType", pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String())
	// ErrModeratorFixedFeeIsMissing indicates when the fixed fee is missing
	ErrModeratorFixedFeeIsMissing = fmt.Errorf("fixed moderator fee is missing or should use (%s) feeType", pb.Moderator_Fee_PERCENTAGE.String())
	// ErrModeratorFixedFeeIsNegativeOrNotSet indicates that the fixed fee is non-positive
	ErrModeratorFixedFeeIsNegativeOrNotSet = errors.New("fixed moderator fee is negative or not a parsable number")
)

// ModeratorFixedFee represents the value of a fixed moderation fee
type ModeratorFixedFee struct {
	Amount         string              `json:"bigAmount,omitempty"`
	AmountCurrency *CurrencyDefinition `json:"amountCurrency,omitempty"`
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
	Moderator     bool           `json:"moderator"`
	ModeratorInfo *ModeratorInfo `json:"moderatorInfo,omitempty"`
}

func ProfileFromProtobuf(p *pb.Profile) (*Profile, error) {
	var (
		modInfo     *ModeratorInfo
		modFixedFee *ModeratorFixedFee
	)

	// build ModeratorInfo
	if p.ModeratorInfo != nil && p.ModeratorInfo.Fee != nil {

		var fees = p.ModeratorInfo.Fee

		// build FixedFee
		if fees != nil ||
			fees.FixedFee != nil {
			var (
				amtStr      string
				amtCurrency *CurrencyDefinition
			)

			// Check both amount currency definitions
			if fees.FixedFee.AmountCurrency != nil {
				ac, err := AllCurrencies().Lookup(fees.FixedFee.AmountCurrency.Code)
				if err != nil {
					ac, err = AllCurrencies().Lookup(fees.FixedFee.CurrencyCode)
					if err != nil {
						log.Warningf("unable to find currency defined for fixed fee")
					}
				}
				if err == nil {
					amtCurrency = &ac
					amtCurrency.Divisibility = uint(fees.FixedFee.AmountCurrency.Divisibility)
				}
			}

			// Check both amount values
			amt, ok := new(big.Int).SetString(fees.FixedFee.BigAmount, 10)
			if !ok || amt.Cmp(big.NewInt(0)) == 0 {
				amtStr = fmt.Sprintf("%d", fees.FixedFee.Amount)
			} else {
				amtStr = fees.FixedFee.BigAmount
			}
			modFixedFee = &ModeratorFixedFee{
				Amount:         amtStr,
				AmountCurrency: amtCurrency,
			}
		}

		modInfo = &ModeratorInfo{
			Fee: &ModeratorFee{
				FeeType:    fees.FeeType.String(),
				FixedFee:   modFixedFee,
				Percentage: fees.Percentage,
			},
		}
	}

	return &Profile{
		Moderator:     p.Moderator,
		ModeratorInfo: modInfo,
	}, nil
}

// GetModeratedFixedFee returns the fixed CurrencyValue for moderator services
// currently set on the Profile
func (p *Profile) GetModeratedFixedFee() (*CurrencyValue, error) {
	if p.IsModerationEnabled() &&
		p.ModeratorInfo.Fee != nil &&
		p.ModeratorInfo.Fee.FixedFee != nil {
		amt, ok := new(big.Int).SetString(p.ModeratorInfo.Fee.FixedFee.Amount, 10)
		if ok && amt.Cmp(big.NewInt(0)) != 0 {
			return &CurrencyValue{
				Amount:   amt,
				Currency: *p.ModeratorInfo.Fee.FixedFee.AmountCurrency,
			}, nil
		}
	}
	return nil, fmt.Errorf("fixed fee not found")
}

// ToValidModeratorFee returns a protobuf which has data coersed into
// their valid fields to be applied in an UpdateProfile call
func (p *Profile) ToValidModeratorFee() (*pb.Moderator_Fee, error) {
	if !p.IsModerationEnabled() ||
		p.ModeratorInfo.Fee == nil {
		return nil, ErrModeratorInfoMissing
	}

	// Setters will normalize the fee schedule
	var feeType pb.Moderator_Fee_FeeType
	switch p.ModeratorInfo.Fee.FeeType {
	case pb.Moderator_Fee_FIXED.String():
		modFee, err := p.GetModeratedFixedFee()
		if err != nil {
			return nil, err
		}
		if err := p.SetModeratorFixedFee(modFee); err != nil {
			return nil, err
		}
		feeType = pb.Moderator_Fee_FIXED
	case pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String():
		percentFee := p.ModeratorInfo.Fee.Percentage
		modFee, err := p.GetModeratedFixedFee()
		if err != nil {
			return nil, err
		}
		err = p.SetModeratorFixedPlusPercentageFee(modFee, percentFee)
		if err != nil {
			return nil, err
		}
		feeType = pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE
	case pb.Moderator_Fee_PERCENTAGE.String():
		err := p.SetModeratorPercentageFee(p.ModeratorInfo.Fee.Percentage)
		if err != nil {
			return nil, err
		}
		feeType = pb.Moderator_Fee_PERCENTAGE
	}

	if err := p.Valid(); err != nil {
		return nil, fmt.Errorf("invalid profile: %s", err.Error())
	}

	var normalizedFixedFee *pb.Moderator_Price
	if ff, err := p.GetModeratedFixedFee(); err == nil {
		var amtInt uint64
		if ai, err := strconv.Atoi(p.ModeratorInfo.Fee.FixedFee.Amount); err == nil {
			amtInt = uint64(ai)
		}
		normalizedFixedFee = &pb.Moderator_Price{
			CurrencyCode: ff.Currency.Code.String(),
			AmountCurrency: &pb.CurrencyDefinition{
				Code:         ff.Currency.Code.String(),
				Divisibility: uint32(ff.Currency.Divisibility),
			},
			BigAmount: ff.Amount.String(),
			Amount:    amtInt,
		}
	}

	return &pb.Moderator_Fee{
		FixedFee:   normalizedFixedFee,
		Percentage: p.ModeratorInfo.Fee.Percentage,
		FeeType:    feeType,
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

// IsModerationEnabled checks if the Moderator flag and info are present
func (p *Profile) IsModerationEnabled() bool {
	return p != nil && p.Moderator && p.ModeratorInfo != nil
}

func (p *Profile) validateModeratorFees() error {
	if !p.Moderator && p.ModeratorInfo != nil {
		return ErrNonModeratorShouldNotHaveInfo
	}
	if p.Moderator && p.ModeratorInfo == nil {
		return ErrModeratorInfoMissing
	}
	if !p.IsModerationEnabled() {
		return nil
	}

	// Moderator is true, Info is present
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
		if amt, ok := new(big.Int).SetString(p.ModeratorInfo.Fee.FixedFee.Amount, 10); !ok || amt.Cmp(big.NewInt(0)) < 0 {
			return ErrModeratorFixedFeeIsNegativeOrNotSet
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
		if p.ModeratorInfo.Fee.Percentage < 0 {
			return ErrModeratorFeeHasNegativePercentage
		}
		if p.ModeratorInfo.Fee.FixedFee != nil {
			amt, ok := new(big.Int).SetString(p.ModeratorInfo.Fee.FixedFee.Amount, 10)
			if ok && amt.Cmp(big.NewInt(0)) != 0 {
				return ErrPercentageFeeHasFixedFee
			}
		}
	case pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String():
		if p.ModeratorInfo.Fee.Percentage < 0 {
			return ErrModeratorFeeHasNegativePercentage
		}
		if err := validateFixedFee(); err != nil {
			return err
		}
	default:
		return ErrUnknownModeratorFeeType
	}

	return nil
}

// DisableModeration sets the profile so moderationr is disabled and
// all fee schedules are removed
func (p *Profile) DisableModeration() error {
	p.Moderator = false
	p.ModeratorInfo = nil
	return nil
}

// SetModeratorFixedFee sets the profile to be a moderator with a
// fixed fee schedule
func (p *Profile) SetModeratorFixedFee(fee *CurrencyValue) error {
	p.Moderator = true
	p.ModeratorInfo = &ModeratorInfo{
		Fee: &ModeratorFee{
			FeeType: pb.Moderator_Fee_FIXED.String(),
			FixedFee: &ModeratorFixedFee{
				Amount:         fee.Amount.String(),
				AmountCurrency: &fee.Currency,
			},
			Percentage: 0,
		},
	}
	return nil
}

// SetModeratorFixedPlusPercentageFee sets the profile to be a moderator
// with a fixed fee plus percentage schedule
func (p *Profile) SetModeratorFixedPlusPercentageFee(fee *CurrencyValue, percentage float32) error {
	p.Moderator = true
	p.ModeratorInfo = &ModeratorInfo{
		Fee: &ModeratorFee{
			FeeType: pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String(),
			FixedFee: &ModeratorFixedFee{
				Amount:         fee.Amount.String(),
				AmountCurrency: &fee.Currency,
			},
			Percentage: percentage,
		},
	}
	return nil
}

// SetModeratorPercentageFee sets the profile to be a moderator with a
// percentage fee schedule
func (p *Profile) SetModeratorPercentageFee(percentage float32) error {
	p.Moderator = true
	p.ModeratorInfo = &ModeratorInfo{
		Fee: &ModeratorFee{
			FeeType:    pb.Moderator_Fee_PERCENTAGE.String(),
			FixedFee:   nil,
			Percentage: percentage,
		},
	}
	return nil
}
