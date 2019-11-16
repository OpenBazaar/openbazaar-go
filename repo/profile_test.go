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
			Moderator: true,
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

	if !actualProfile.Moderator {
		t.Errorf("expected Moderator to be true")
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
	p.Moderator = false
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
	if err := p.Valid(); err != nil {
		t.Errorf("expected profile with zero fee to be valid, but errored: %s", err.Error())
	}

	p.ModeratorInfo.Fee.FixedFee.Amount = "-1"
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with negative fee to be invalid")
	} else if err != repo.ErrModeratorFixedFeeIsNegativeOrNotSet {
		t.Errorf("expected ErrModeratorFixedFeeIsNegativeOrNotSet error, but was (%s)", err.Error())
	}

	p.ModeratorInfo.Fee.FixedFee.Amount = ""
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with empty fee to be invalid")
	} else if err != repo.ErrModeratorFixedFeeIsNegativeOrNotSet {
		t.Errorf("expected ErrModeratorFixedFeeIsNegativeOrNotSet error, but was (%s)", err.Error())
	}

	// test fixed fee plus percentage
	p.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String()
	p.ModeratorInfo.Fee.Percentage = 1.1
	p.ModeratorInfo.Fee.FixedFee.Amount = "0"
	if err := p.Valid(); err != nil {
		t.Errorf("expected profile with zero fee to be valid, but errored: %s", err.Error())
	}

	p.ModeratorInfo.Fee.FixedFee.Amount = "-1"
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with negative fee to be invalid")
	} else if err != repo.ErrModeratorFixedFeeIsNegativeOrNotSet {
		t.Errorf("expected ErrModeratorFixedFeeIsNegativeOrNotSet error, but was (%s)", err.Error())
	}

	p.ModeratorInfo.Fee.FixedFee.Amount = ""
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with empty fee to be invalid")
	} else if err != repo.ErrModeratorFixedFeeIsNegativeOrNotSet {
		t.Errorf("expected ErrModeratorFixedFeeIsNegativeOrNotSet error, but was (%s)", err.Error())
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

func TestProfileValidPercentageFeeZero(t *testing.T) {
	p := factory.NewProfile()
	p.ModeratorInfo = &repo.ModeratorInfo{
		Fee: &repo.ModeratorFee{
			FeeType:    pb.Moderator_Fee_PERCENTAGE.String(),
			Percentage: 0,
		},
	}

	if err := p.Valid(); err != nil {
		t.Errorf("expected profile example to be valid")
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

func TestProfileValidPercentWithFixedHavingZeroPercent(t *testing.T) {
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

	if err := p.Valid(); err != nil {
		t.Errorf("expected profile exmaple to be valid")
	}
}

func TestProfileInvalidPercentWithFixedHavingNegativePercent(t *testing.T) {
	p := factory.NewProfile()
	p.ModeratorInfo = &repo.ModeratorInfo{
		Fee: &repo.ModeratorFee{
			FeeType:    pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String(),
			Percentage: -1,
			FixedFee: &repo.ModeratorFixedFee{
				Amount:         "1234",
				AmountCurrency: factory.NewCurrencyDefinition("BTC"),
			},
		},
	}

	if err := p.Valid(); err == nil {
		t.Errorf("expected percentage with fixed fee but negative percentage to be invalid")
	} else if err != repo.ErrModeratorFeeHasNegativePercentage {
		t.Errorf("expected ErrModeratorFeeHasNegativePercentage error, but got (%s)", err.Error())
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

func TestProfileInvalidAsModeratorWithoutInfo(t *testing.T) {
	p := factory.NewProfile()
	p.Moderator = true
	p.ModeratorInfo = nil

	if err := p.Valid(); err == nil {
		t.Errorf("expected moderator without info to be invalid")
	} else if err != repo.ErrModeratorInfoMissing {
		t.Errorf("expected ErrModeratorInfoMissing error, but got (%s)", err.Error())
	}
}

func TestProfileInvalidAsUserWithModeratorInfo(t *testing.T) {
	p := factory.NewProfile()
	p.Moderator = false
	p.ModeratorInfo = &repo.ModeratorInfo{
		Fee: &repo.ModeratorFee{
			FeeType:    pb.Moderator_Fee_PERCENTAGE.String(),
			Percentage: 1.1,
			FixedFee:   nil,
		},
	}

	if err := p.Valid(); err == nil {
		t.Errorf("expected regular user with moderator info to be invalid")
	} else if err != repo.ErrNonModeratorShouldNotHaveInfo {
		t.Errorf("expected ErrNonModeratorShouldNotHaveInfo error, but got (%s)", err.Error())
	}
}

func TestProfileSetModeratorFixedFee(t *testing.T) {
	p := factory.NewProfile()
	p.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String()
	p.ModeratorInfo.Fee.FixedFee = nil
	p.ModeratorInfo.Fee.Percentage = 1.1

	fee, err := repo.NewCurrencyValueWithLookup("123", "USD")
	if err != nil {
		t.Fatalf("failed creating fee value: %s", err.Error())
	}

	if err := p.SetModeratorFixedFee(fee); err != nil {
		t.Fatalf("failed setting fee: %s", err.Error())
	}

	if !p.Moderator {
		t.Errorf("expected Moderator flag to be true")
	}
	if p.ModeratorInfo.Fee.FeeType != pb.Moderator_Fee_FIXED.String() {
		t.Errorf("expected feeType to be (%s), but was (%s)", pb.Moderator_Fee_FIXED.String(), p.ModeratorInfo.Fee.FeeType)
	}
	if p.ModeratorInfo.Fee.Percentage != 0 {
		t.Fatalf("expected percentage to be zero")
	}
	if p.ModeratorInfo.Fee.FixedFee == nil {
		t.Fatalf("expected fixedFee to not be nil")
	}
	if p.ModeratorInfo.Fee.FixedFee.Amount != fee.Amount.String() {
		t.Errorf("expected fixed fee amount to be (%s), but was (%s)", fee.Amount.String(), p.ModeratorInfo.Fee.FixedFee.Amount)
	}
	if !fee.Currency.Equal(p.ModeratorInfo.Fee.FixedFee.AmountCurrency) {
		t.Errorf("expected fixed fee amount currency to be (%s), but was (%s)", fee.Currency.String(), p.ModeratorInfo.Fee.FixedFee.AmountCurrency)
	}
}

func TestProfileSetModeratorFixedPlusPercentageFee(t *testing.T) {
	percentage := float32(2.5)
	p := factory.NewProfile()
	p.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_PERCENTAGE.String()
	p.ModeratorInfo.Fee.FixedFee = nil
	p.ModeratorInfo.Fee.Percentage = 1.1

	fee, err := repo.NewCurrencyValueWithLookup("123", "USD")
	if err != nil {
		t.Fatalf("failed creating fee value: %s", err.Error())
	}

	if err := p.SetModeratorFixedPlusPercentageFee(fee, percentage); err != nil {
		t.Fatalf("failed setting fee: %s", err.Error())
	}

	if !p.Moderator {
		t.Errorf("expected Moderator flag to be true")
	}
	if p.ModeratorInfo.Fee.FeeType != pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String() {
		t.Errorf("expected feeType to be (%s), but was (%s)", pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String(), p.ModeratorInfo.Fee.FeeType)
	}
	if p.ModeratorInfo.Fee.Percentage != percentage {
		t.Fatalf("expected percentage to be zero")
	}
	if p.ModeratorInfo.Fee.FixedFee == nil {
		t.Fatalf("expected fixedFee to not be nil")
	}
	if p.ModeratorInfo.Fee.FixedFee.Amount != fee.Amount.String() {
		t.Errorf("expected fixed fee amount to be (%s), but was (%s)", fee.Amount.String(), p.ModeratorInfo.Fee.FixedFee.Amount)
	}
	if !fee.Currency.Equal(p.ModeratorInfo.Fee.FixedFee.AmountCurrency) {
		t.Errorf("expected fixed fee amount currency to be (%s), but was (%s)", fee.Currency.String(), p.ModeratorInfo.Fee.FixedFee.AmountCurrency)
	}
}

func TestProfileSetModeratorPercentageFee(t *testing.T) {
	percentage := float32(2.5)
	p := factory.NewProfile()
	p.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED.String()
	p.ModeratorInfo.Fee.FixedFee = &repo.ModeratorFixedFee{
		Amount:         "1230",
		AmountCurrency: factory.NewCurrencyDefinition("BTC"),
	}
	p.ModeratorInfo.Fee.Percentage = 0

	if err := p.SetModeratorPercentageFee(percentage); err != nil {
		t.Fatalf("failed setting fee: %s", err.Error())
	}

	if !p.Moderator {
		t.Errorf("expected Moderator flag to be true")
	}
	if p.ModeratorInfo.Fee.FeeType != pb.Moderator_Fee_PERCENTAGE.String() {
		t.Errorf("expected feeType to be (%s), but was (%s)", pb.Moderator_Fee_PERCENTAGE.String(), p.ModeratorInfo.Fee.FeeType)
	}
	if p.ModeratorInfo.Fee.Percentage != percentage {
		t.Fatalf("expected percentage to be zero")
	}
	if p.ModeratorInfo.Fee.FixedFee != nil {
		t.Fatalf("expected fixedFee to be nil")
	}
}

func TestProfileDisableModeration(t *testing.T) {
	p := factory.NewProfile()
	p.Moderator = true
	p.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED.String()
	p.ModeratorInfo.Fee.FixedFee = &repo.ModeratorFixedFee{
		Amount:         "1230",
		AmountCurrency: factory.NewCurrencyDefinition("BTC"),
	}
	p.ModeratorInfo.Fee.Percentage = 0

	if err := p.DisableModeration(); err != nil {
		t.Fatalf("failed disabling moderation: %s", err.Error())
	}

	if p.Moderator {
		t.Errorf("expected Moderator flag to be false")
	}
	if p.ModeratorInfo != nil {
		t.Errorf("expected ModeratorInfo to be nil")
	}
}
