package repo

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
)

var (
	// ErrModeratorInfoMissing indicates when the moderator information is
	// missing while also indicating they are a moderator
	ErrModeratorInfoMissing = errors.New("moderator is enabled but information is missing")
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
	// ErrModeratorFixedFeeAmountIsEmpty indicates the fee is defined with an empty amount
	ErrModeratorFixedFeeAmountIsEmpty = errors.New("fixed moderator fee amount is missing or not a parseable number")
	// ErrModeratorFixedFeeIsNegative indicates that the fixed fee is non-positive
	ErrModeratorFixedFeeIsNegative = errors.New("fixed moderator fee is negative or not a parsable number")
)

// Profile presents the user's metadata. The profile state is maintained within
// a *pb.Profile internally which captures all state changes suitable to be persisted
// via marshaling to JSON. This struct should ensure the integrity of *pb.Profile to
// its data as indicated by the set schema version.
type Profile struct {
	profileProto *pb.Profile
}

// UnmarshalJSONProfile consumes a JSON byte slice and returns a Profile-wrapped
// unmarshaled protobuf
func UnmarshalJSONProfile(data []byte) (*Profile, error) {
	var (
		p   = new(pb.Profile)
		err = jsonpb.UnmarshalString(string(data), p)
	)
	if err != nil {
		return nil, err
	}
	return NewProfileFromProtobuf(p)
}

// NewProfileFromProtobuf returns a Profile wrapped around a profile protobuf
func NewProfileFromProtobuf(p *pb.Profile) (*Profile, error) {
	clonedProfile := proto.Clone(p).(*pb.Profile)
	return &Profile{profileProto: clonedProfile}, nil
}

// NormalizeDataForAllSchemas converts existing data from its current schema
// into legacy schema. This does not guarantee success as legacy schema that
// was abandoned due to unacceptable constraints will not be able to fulfill
// the full capability of the newer schema. (Ex: FixedFee.BigAmount can support
// full precision, whereas FixedFee.Amount is limited to math.MaxInt64
func (p *Profile) NormalizeDataForAllSchemas() *Profile {
	p.normalizeFees()
	return p
}

// GetProtobuf returns the underlying protobuf which represents the persistable
// state of the profile. (Note: This method is a shim to access data which isn't
// represented in this package's Profile methods. Consider adding missing getters
// and setters which repsect the schema version instead of using the protobuf
// directly for manipulation.)
func (p *Profile) GetProtobuf() *pb.Profile {
	return p.profileProto
}

// GetVersion returns the schema version for the profile protobuf
func (p *Profile) GetVersion() uint32 {
	return p.profileProto.GetVersion()
}

// GetModeratedFixedFee returns the fixed CurrencyValue for moderator services
// currently set on the Profile
func (p *Profile) GetModeratedFixedFee() (*CurrencyValue, error) {
	if p.IsModerationEnabled() &&
		p.profileProto.ModeratorInfo.Fee != nil &&
		p.profileProto.ModeratorInfo.Fee.FixedFee != nil {
		switch p.GetVersion() {
		case 5:
			var (
				amt  = p.profileProto.ModeratorInfo.Fee.FixedFee.BigAmount
				code = p.profileProto.ModeratorInfo.Fee.FixedFee.AmountCurrency
			)
			return NewCurrencyValueFromProtobuf(amt, code)
		default: // v4 and earlier
			var (
				amt  = strconv.Itoa(int(p.profileProto.ModeratorInfo.Fee.FixedFee.Amount))
				code = p.profileProto.ModeratorInfo.Fee.FixedFee.CurrencyCode
			)
			return NewCurrencyValueWithLookup(amt, code)

		}
	}
	return nil, fmt.Errorf("fixed fee not found")
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
	return p != nil &&
		p.profileProto != nil &&
		p.profileProto.Moderator &&
		p.profileProto.ModeratorInfo != nil
}

func (p *Profile) validateModeratorFees() error {
	if p.profileProto.Moderator && p.profileProto.ModeratorInfo == nil {
		return ErrModeratorInfoMissing
	}
	if !p.IsModerationEnabled() {
		return nil
	}

	// Moderator is true, Info is present
	if p.profileProto.ModeratorInfo.Fee == nil {
		return ErrMissingModeratorFee
	}

	var validateFixedFee = func() error {
		if p.profileProto.ModeratorInfo.Fee.FixedFee == nil {
			return ErrModeratorFixedFeeIsMissing
		}
		if p.profileProto.ModeratorInfo.Fee.FixedFee.BigAmount == "" {
			return ErrModeratorFixedFeeAmountIsEmpty
		}

		feeCurrency, err := NewCurrencyValueFromProtobuf(
			p.profileProto.ModeratorInfo.Fee.FixedFee.BigAmount,
			p.profileProto.ModeratorInfo.Fee.FixedFee.AmountCurrency,
		)
		if err != nil {
			return err
		}
		if err := feeCurrency.Valid(); err != nil {
			return fmt.Errorf("invalid fixed fee currency: %s", err.Error())
		}
		if feeCurrency.IsNegative() {
			return ErrModeratorFixedFeeIsNegative
		}
		return nil
	}

	switch p.profileProto.ModeratorInfo.Fee.FeeType {
	case pb.Moderator_Fee_FIXED:
		if p.profileProto.ModeratorInfo.Fee.Percentage != 0 {
			return ErrFixedFeeHasNonZeroPercentage
		}
		if err := validateFixedFee(); err != nil {
			return err
		}
	case pb.Moderator_Fee_PERCENTAGE:
		if p.profileProto.ModeratorInfo.Fee.Percentage < 0 {
			return ErrModeratorFeeHasNegativePercentage
		}
		if p.profileProto.ModeratorInfo.Fee.FixedFee != nil {
			amt, ok := new(big.Int).SetString(p.profileProto.ModeratorInfo.Fee.FixedFee.BigAmount, 10)
			if ok && amt.Cmp(big.NewInt(0)) != 0 {
				return ErrPercentageFeeHasFixedFee
			}
		}
	case pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE:
		if p.profileProto.ModeratorInfo.Fee.Percentage < 0 {
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
	p.profileProto.Moderator = false
	p.profileProto.ModeratorInfo = nil
	return nil
}

// normalizeFees ensures the fee schedule is readable for as
// many other nodes as possible by populating prior version schema
func (p *Profile) normalizeFees() {
	if p.profileProto.ModeratorInfo != nil && p.profileProto.ModeratorInfo.Fee != nil {
		var fees = p.profileProto.ModeratorInfo.Fee
		switch fees.FeeType {
		case pb.Moderator_Fee_FIXED:
			fees.Percentage = 0
		case pb.Moderator_Fee_PERCENTAGE:
			fees.FixedFee = nil
		}

		if ff, err := p.GetModeratedFixedFee(); err == nil {
			switch p.GetVersion() {
			case 5:
				var amtInt uint64
				if ai, err := strconv.Atoi(p.profileProto.ModeratorInfo.Fee.FixedFee.BigAmount); err == nil {
					amtInt = uint64(ai)
				}
				p.profileProto.ModeratorInfo.Fee.FixedFee.CurrencyCode = ff.Currency.Code.String()
				p.profileProto.ModeratorInfo.Fee.FixedFee.Amount = amtInt
			default: // v4 and earlier
				p.profileProto.ModeratorInfo.Fee.FixedFee.AmountCurrency = &pb.CurrencyDefinition{
					Code:         ff.Currency.Code.String(),
					Divisibility: uint32(ff.Currency.Divisibility),
				}
				p.profileProto.ModeratorInfo.Fee.FixedFee.BigAmount = ff.Amount.String()
			}
		}
	}
}

// SetModeratorFixedFee sets the profile to be a moderator with a
// fixed fee schedule
func (p *Profile) SetModeratorFixedFee(fee *CurrencyValue) error {
	p.profileProto.Moderator = true
	p.profileProto.ModeratorInfo = &pb.Moderator{
		Fee: &pb.Moderator_Fee{
			FeeType: pb.Moderator_Fee_FIXED,
			FixedFee: &pb.Moderator_Price{
				BigAmount: fee.Amount.String(),
				AmountCurrency: &pb.CurrencyDefinition{
					Code:         fee.Currency.Code.String(),
					Divisibility: uint32(fee.Currency.Divisibility),
				},
			},
			Percentage: 0,
		},
	}
	p.normalizeFees()
	return nil
}

// SetModeratorFixedPlusPercentageFee sets the profile to be a moderator
// with a fixed fee plus percentage schedule
func (p *Profile) SetModeratorFixedPlusPercentageFee(fee *CurrencyValue, percentage float32) error {
	p.profileProto.Moderator = true
	p.profileProto.ModeratorInfo = &pb.Moderator{
		Fee: &pb.Moderator_Fee{
			FeeType: pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE,
			FixedFee: &pb.Moderator_Price{
				BigAmount: fee.Amount.String(),
				AmountCurrency: &pb.CurrencyDefinition{
					Code:         fee.Currency.Code.String(),
					Divisibility: uint32(fee.Currency.Divisibility),
				},
			},
			Percentage: percentage,
		},
	}
	p.normalizeFees()
	return nil
}

// SetModeratorPercentageFee sets the profile to be a moderator with a
// percentage fee schedule
func (p *Profile) SetModeratorPercentageFee(percentage float32) error {
	p.profileProto.Moderator = true
	p.profileProto.ModeratorInfo = &pb.Moderator{
		Fee: &pb.Moderator_Fee{
			FeeType:    pb.Moderator_Fee_PERCENTAGE,
			FixedFee:   nil,
			Percentage: percentage,
		},
	}
	return nil
}
