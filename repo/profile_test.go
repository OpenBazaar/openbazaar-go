package repo_test

import (
	"strings"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestProfileFromProtobuf(t *testing.T) {
	var (
		fixedFeeAmount          = "1234"
		feeType                 = pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE
		feePercentage           = float32(1.1)
		feeCurrencyDivisibility = uint32(10)
		feeCurrencyCode         = "BTC"
		pbProfile               = &pb.Profile{
			ModeratorInfo: &pb.Moderator{
				Fee: &pb.Moderator_Fee{
					FixedFee: &pb.Moderator_Price{
						BigAmount: fixedFeeAmount,
						AmountCurrency: &pb.CurrencyDefinition{
							Code:         feeCurrencyCode,
							Divisibility: feeCurrencyDivisibility,
						},
					},
					Percentage: feePercentage,
					FeeType:    feeType,
				},
			},
		}
	)

	actualProfile, err := repo.ProfileFromProtobuf(pbProfile)
	if err != nil {
		t.Fatal(err)
	}

	repoProfileFees := actualProfile.ModeratorInfo.Fee
	if repoProfileFees.FeeType != feeType.String() {
		t.Errorf("expected FeeType to be (%s), but was (%s)", feeType.String(), repoProfileFees.FeeType)
	}
	if repoProfileFees.Percentage != feePercentage {
		t.Errorf("expected Percentage to be (%f), but was (%f)", feePercentage, repoProfileFees.Percentage)
	}
	if repoProfileFees.FixedFee.Amount != fixedFeeAmount {
		t.Errorf("expected FixedFee.Amount to be (%s), but was (%s)", fixedFeeAmount, repoProfileFees.FixedFee.Amount)
	}
	if repoProfileFees.FixedFee.AmountCurrency.Code.String() != feeCurrencyCode {
		t.Errorf("expected FixedFee.AmountCurrency to be (%s), but was (%s)", feeCurrencyCode, repoProfileFees.FixedFee.AmountCurrency)
	}
}

func TestProfileFromProtobufMissingModInfo(t *testing.T) {
	p := factory.NewProfileProtobuf()
	p.ModeratorInfo = nil

	if _, err := repo.ProfileFromProtobuf(p); err != nil {
		t.Errorf("expected missing ModeratorInfo to be valid, but errored (%s)", err.Error())
	}
}

func TestProfileFactoryIsValid(t *testing.T) {
	p := factory.NewProfile()
	if err := p.Valid(); err != nil {
		t.Log(err)
		t.Error("expected factory profile to be valid")
	}
}

func TestProfileInvalidWithUnknownFeeType(t *testing.T) {
	p := factory.NewProfile()
	p.ModeratorInfo.Fee.FeeType = "UNKNOWN"
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile to be invalid with unknown fee type")
	} else if err != repo.ErrUnknownModeratorFeeType {
		t.Errorf("expected ErrUnknownModeratorFeeType error, but was (%s)", err.Error())
	}
}

func TestProfileValidWithoutModeratorInfo(t *testing.T) {
	p := factory.NewProfile()
	p.ModeratorInfo = nil
	if err := p.Valid(); err != nil {
		t.Error("expected profile to be valid without moderator info, but wasn't")
	}
}

func TestProfileInvalidWithoutModeratorFee(t *testing.T) {
	p := factory.NewProfile()
	p.ModeratorInfo.Fee = nil
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile without fee schedule to be invalid")
	} else if err != repo.ErrMissingModeratorFee {
		t.Errorf("expected ErrMissingModeratorFee error, but was (%s)", err.Error())
	}
}

func TestProfileInvalidWithMissingFixedFee(t *testing.T) {
	p := factory.NewProfile()
	p.ModeratorInfo.Fee = &repo.ModeratorFee{
		FeeType: pb.Moderator_Fee_FIXED.String(),
	}
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with fixed feeType but missing fee to be invalid")
	} else if err != repo.ErrModeratorFixedFeeIsMissing {
		t.Errorf("expected ErrModeratorFixedFeeIsMissing error, but was (%s)", err.Error())
	}
}

func TestProfileInvalidWithInvalidFixedFeeCurrency(t *testing.T) {
	invalidCurrency := factory.NewCurrencyDefinition("BTC")
	invalidCurrency.Divisibility = 0
	currencyErr := invalidCurrency.Valid()
	if currencyErr == nil {
		t.Fatal("expected invalid currency to be invalid")
	}
	p := factory.NewProfile()
	p.ModeratorInfo.Fee = &repo.ModeratorFee{
		FixedFee: &repo.ModeratorFixedFee{
			Amount:         "1",
			AmountCurrency: invalidCurrency,
		},
	}

	// test fixed fee
	p.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED.String()
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with invalid currency to be invalid")
	} else if !strings.Contains(err.Error(), "invalid fixed fee currency") {
		t.Errorf("expected (invalid fixed fee currency) in error, but got (%s)", err.Error())
	}

	// test fixed fee plus percentage
	p.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String()
	p.ModeratorInfo.Fee.Percentage = 1.1
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with invalid currency to be invalid")
	} else if !strings.Contains(err.Error(), "invalid fixed fee currency") {
		t.Errorf("expected (invalid fixed fee currency) in error, but got (%s)", err.Error())
	}
}

func TestProfileInvalidWithZeroFixedFee(t *testing.T) {
	p := factory.NewProfile()
	p.ModeratorInfo.Fee = &repo.ModeratorFee{
		FixedFee: &repo.ModeratorFixedFee{
			AmountCurrency: factory.NewCurrencyDefinition("BTC"),
		},
	}

	// test fixed fee
	p.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED.String()
	p.ModeratorInfo.Fee.FixedFee.Amount = "0"
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with zero fee to be invalid")
	} else if err != repo.ErrModeratorFixedFeeIsNonPositive {
		t.Errorf("expected ErrModeratorFixedFeeIsNonPositive error, but was (%s)", err.Error())
	}

	p.ModeratorInfo.Fee.FixedFee.Amount = "-1"
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with negative fee to be invalid")
	} else if err != repo.ErrModeratorFixedFeeIsNonPositive {
		t.Errorf("expected ErrModeratorFixedFeeIsNonPositive error, but was (%s)", err.Error())
	}

	p.ModeratorInfo.Fee.FixedFee.Amount = ""
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with empty fee to be invalid")
	} else if err != repo.ErrModeratorFixedFeeIsNonPositive {
		t.Errorf("expected ErrModeratorFixedFeeIsNonPositive error, but was (%s)", err.Error())
	}

	// test fixed fee plus percentage
	p.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String()
	p.ModeratorInfo.Fee.Percentage = 1.1
	p.ModeratorInfo.Fee.FixedFee.Amount = "0"
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with zero fee to be invalid")
	} else if err != repo.ErrModeratorFixedFeeIsNonPositive {
		t.Errorf("expected ErrModeratorFixedFeeIsNonPositive error, but was (%s)", err.Error())
	}

	p.ModeratorInfo.Fee.FixedFee.Amount = "-1"
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with negative fee to be invalid")
	} else if err != repo.ErrModeratorFixedFeeIsNonPositive {
		t.Errorf("expected ErrModeratorFixedFeeIsNonPositive error, but was (%s)", err.Error())
	}

	p.ModeratorInfo.Fee.FixedFee.Amount = ""
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with empty fee to be invalid")
	} else if err != repo.ErrModeratorFixedFeeIsNonPositive {
		t.Errorf("expected ErrModeratorFixedFeeIsNonPositive error, but was (%s)", err.Error())
	}
}

func TestProfileValidFixedFee(t *testing.T) {
	p := factory.NewProfile()
	p.ModeratorInfo = &repo.ModeratorInfo{
		Fee: &repo.ModeratorFee{
			FeeType: pb.Moderator_Fee_FIXED.String(),
			FixedFee: &repo.ModeratorFixedFee{
				Amount:         "1234",
				AmountCurrency: factory.NewCurrencyDefinition("BTC"),
			},
			Percentage: 0,
		},
	}

	if err := p.Valid(); err != nil {
		t.Error("expected profile example to be valid")
	}

	p.ModeratorInfo.Fee.Percentage = 1.1
	if err := p.Valid(); err == nil {
		t.Errorf("expected non-zero percentage to be invalid")
	} else if err != repo.ErrFixedFeeHasNonZeroPercentage {
		t.Errorf("expected ErrFixedFeeHasNonZeroPercentage error, but was (%s)", err.Error())
	}
}

func TestProfileValidPercentageFee(t *testing.T) {
	p := factory.NewProfile()
	p.ModeratorInfo = &repo.ModeratorInfo{
		Fee: &repo.ModeratorFee{
			FeeType:    pb.Moderator_Fee_PERCENTAGE.String(),
			Percentage: 1.1,
		},
	}

	if err := p.Valid(); err != nil {
		t.Errorf("expected profile example to be valid")
	}
}

func TestProfileInvalidPercentageFeeZero(t *testing.T) {
	p := factory.NewProfile()
	p.ModeratorInfo = &repo.ModeratorInfo{
		Fee: &repo.ModeratorFee{
			FeeType:    pb.Moderator_Fee_PERCENTAGE.String(),
			Percentage: 0,
		},
	}

	if err := p.Valid(); err == nil {
		t.Errorf("expected zero percentage fee to be invalid")
	} else if err != repo.ErrModeratorFeeHasNonPositivePercent {
		t.Errorf("expected ErrModeratorFeeHasNonPositivePercent error, but was (%s)", err.Error())
	}
}

func TestProfileInvalidPercentageFeeWithFixedFee(t *testing.T) {
	p := factory.NewProfile()
	p.ModeratorInfo = &repo.ModeratorInfo{
		Fee: &repo.ModeratorFee{
			FeeType:    pb.Moderator_Fee_PERCENTAGE.String(),
			Percentage: 1,
			FixedFee: &repo.ModeratorFixedFee{
				Amount:         "1234",
				AmountCurrency: factory.NewCurrencyDefinition("BTC"),
			},
		},
	}

	if err := p.Valid(); err == nil {
		t.Errorf("expected percentage fee with fixed fee to be invalid")
	} else if err != repo.ErrPercentageFeeHasFixedFee {
		t.Errorf("expected ErrPercentageFeeHasFixedFee error, but was (%s)", err.Error())
	}
}

func TestProfileValidPercentageWithFixedFee(t *testing.T) {
	p := factory.NewProfile()
	p.ModeratorInfo = &repo.ModeratorInfo{
		Fee: &repo.ModeratorFee{
			FeeType:    pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String(),
			Percentage: 1,
			FixedFee: &repo.ModeratorFixedFee{
				Amount:         "1234",
				AmountCurrency: factory.NewCurrencyDefinition("BTC"),
			},
		},
	}

	if err := p.Valid(); err != nil {
		t.Errorf("expected profile exmaple to be valid")
	}
}

func TestProfileInvalidPercentWithFixedHavingZeroPercent(t *testing.T) {
	p := factory.NewProfile()
	p.ModeratorInfo = &repo.ModeratorInfo{
		Fee: &repo.ModeratorFee{
			FeeType:    pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String(),
			Percentage: 0,
			FixedFee: &repo.ModeratorFixedFee{
				Amount:         "1234",
				AmountCurrency: factory.NewCurrencyDefinition("BTC"),
			},
		},
	}

	if err := p.Valid(); err == nil {
		t.Errorf("expected percentage with fixed fee but percent is zero to be invalid")
	} else if err != repo.ErrModeratorFeeHasNonPositivePercent {
		t.Errorf("expected ErrModeratorFeeHasNonPositivePercent error, but got (%s)", err.Error())
	}
}

func TestProfileInvalidPercentWithFixedHavingNoFixedFee(t *testing.T) {
	p := factory.NewProfile()
	p.ModeratorInfo = &repo.ModeratorInfo{
		Fee: &repo.ModeratorFee{
			FeeType:    pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String(),
			Percentage: 1.1,
			FixedFee:   nil,
		},
	}

	if err := p.Valid(); err == nil {
		t.Errorf("expected percentage with fixed fee but fixed fee is missing to be invalid")
	} else if err != repo.ErrModeratorFixedFeeIsMissing {
		t.Errorf("expected ErrModeratorFixedFeeIsMissing error, but got (%s)", err.Error())
	}
}
