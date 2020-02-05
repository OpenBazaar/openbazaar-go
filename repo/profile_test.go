package repo_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestProfileFromProtobufMissingModInfo(t *testing.T) {
	p := factory.MustNewProfileProtobuf()
	p.ModeratorInfo = nil

	if _, err := repo.NewProfileFromProtobuf(p); err != nil {
		t.Errorf("expected missing ModeratorInfo to be valid, but errored (%s)", err.Error())
	}
}

func TestProfileNormalizeSchema(t *testing.T) {
	var (
		exampleFee = &pb.Moderator_Price{
			BigAmount: "10",
			AmountCurrency: &pb.CurrencyDefinition{
				Code:         "BTC",
				Divisibility: 8,
			},
		}
	)
	var examples = []struct {
		example  func() *pb.Profile
		validate func(*pb.Profile)
	}{
		{ // profile with percent fee should remove non-percent fee values
			example: func() *pb.Profile {
				p := factory.MustNewProfileProtobuf()
				p.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_PERCENTAGE
				p.ModeratorInfo.Fee.FixedFee = exampleFee
				p.ModeratorInfo.Fee.Percentage = 1.1
				return p
			},
			validate: func(p *pb.Profile) {
				if p.ModeratorInfo.Fee.FixedFee != nil {
					t.Errorf("expected fixed fee to be removed, but was not")
				}
			},
		},
		{ // profile with fixed fee should zero percentage
			example: func() *pb.Profile {
				p := factory.MustNewProfileProtobuf()
				p.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED
				p.ModeratorInfo.Fee.FixedFee = exampleFee
				p.ModeratorInfo.Fee.Percentage = 1.1
				return p
			},
			validate: func(p *pb.Profile) {
				if p.ModeratorInfo.Fee.Percentage != 0 {
					t.Errorf("expected percentage to be zero, but was not")
				}
			},
		},
		{ // profile with fixed fee should populate legacy schema
			example: func() *pb.Profile {
				p := factory.MustNewProfileProtobuf()
				p.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED
				p.ModeratorInfo.Fee.FixedFee = exampleFee
				return p
			},
			validate: func(p *pb.Profile) {
				if actual := p.ModeratorInfo.Fee.FixedFee.CurrencyCode; actual != exampleFee.AmountCurrency.Code {
					t.Errorf("expected legacy code to be (%s), but was (%s)", exampleFee.AmountCurrency.Code, actual)
				}
				expectedAmt, err := strconv.Atoi(exampleFee.BigAmount)
				if err != nil {
					t.Error(err)
					return
				}
				if actualAmt := p.ModeratorInfo.Fee.FixedFee.Amount; actualAmt != uint64(expectedAmt) {
					t.Errorf("expected legacy amount to be (%d), but was (%d)", expectedAmt, actualAmt)
				}
			},
		},
		{ // legacy v4 profile with fixed fee should populate current schema
			example: func() *pb.Profile {
				pb := factory.MustLoadProfileFixture("v4-profile-moderator-fixed-fee")
				rp, err := repo.UnmarshalJSONProfile(pb)
				if err != nil {
					t.Fatal(err)
				}
				p := rp.GetProtobuf()
				p.ModeratorInfo.Fee.FixedFee.CurrencyCode = "BTC"
				p.ModeratorInfo.Fee.FixedFee.Amount = 10
				return p
			},
			validate: func(p *pb.Profile) {
				if actual := p.ModeratorInfo.Fee.FixedFee.AmountCurrency.Code; actual != "BTC" {
					t.Errorf("expected fee amount currency code to be (%s), but was (%s)", "BTC", actual)
				}
				if actualAmt := p.ModeratorInfo.Fee.FixedFee.BigAmount; actualAmt != "10" {
					t.Errorf("expected fee amount to be (%s), but was (%s)", "10", actualAmt)
				}
			},
		},
	}

	for i, e := range examples {
		var (
			subject = e.example()
			p, err  = repo.NewProfileFromProtobuf(subject)
		)
		if err != nil {
			t.Errorf("failed normalization on example (%d): %s", i, err)
			continue
		}
		p.NormalizeDataForAllSchemas()
		e.validate(p.GetProtobuf())
	}
}

func TestProfileFactoryIsValid(t *testing.T) {
	p := factory.MustNewProfile()
	if err := p.Valid(); err != nil {
		t.Log(err)
		t.Error("expected factory profile to be valid")
	}
}

func TestProfileInvalidWithUnknownFeeType(t *testing.T) {
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FeeType(-1)

	if err := p.Valid(); err == nil {
		t.Errorf("expected profile to be invalid with unknown fee type")
	} else if err != repo.ErrUnknownModeratorFeeType {
		t.Errorf("expected ErrUnknownModeratorFeeType error, but was (%s)", err.Error())
	}
}

func TestProfileValidWithoutModeratorInfo(t *testing.T) {
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.Moderator = false
	pp.ModeratorInfo = nil

	if err := p.Valid(); err != nil {
		t.Error("expected profile to be valid without moderator info, but wasn't")
	}
}

func TestProfileValidWithModeratorInfoAndModerationDisabled(t *testing.T) {
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.Moderator = false
	pp.ModeratorInfo = &pb.Moderator{
		Fee: &pb.Moderator_Fee{
			FeeType:    pb.Moderator_Fee_PERCENTAGE,
			Percentage: 0,
		},
	}

	if err := p.Valid(); err != nil {
		t.Errorf("expected profile to be valid with moderator info and moderation disabled, but errored (%s)", err)
	}
}

func TestProfileInvalidWithoutModeratorFee(t *testing.T) {
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.ModeratorInfo.Fee = nil

	if err := p.Valid(); err == nil {
		t.Errorf("expected profile without fee schedule to be invalid")
	} else if err != repo.ErrMissingModeratorFee {
		t.Errorf("expected ErrMissingModeratorFee error, but was (%s)", err.Error())
	}
}

func TestProfileInvalidWithMissingFixedFee(t *testing.T) {
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED
	pp.ModeratorInfo.Fee.FixedFee = nil

	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with fixed feeType but missing fee to be invalid")
	} else if err != repo.ErrModeratorFixedFeeIsMissing {
		t.Errorf("expected ErrModeratorFixedFeeIsMissing error, but was (%s)", err.Error())
	}
}

func TestProfileInvalidWithInvalidFixedFeeCurrency(t *testing.T) {
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.ModeratorInfo.Fee.FixedFee = &pb.Moderator_Price{
		BigAmount: "1",
		AmountCurrency: &pb.CurrencyDefinition{
			Code:         "BTC",
			Divisibility: 0, // 0 divisibility is invalid
		},
	}

	// test fixed fee
	pp.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with invalid currency to be invalid")
	} else if !strings.Contains(err.Error(), "invalid fixed fee currency") {
		t.Errorf("expected (invalid fixed fee currency) in error, but got (%s)", err.Error())
	}

	// test fixed fee plus percentage
	pp.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE
	pp.ModeratorInfo.Fee.Percentage = 1.1
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with invalid currency to be invalid")
	} else if !strings.Contains(err.Error(), "invalid fixed fee currency") {
		t.Errorf("expected (invalid fixed fee currency) in error, but got (%s)", err.Error())
	}
}

func TestProfileInvalidWithZeroFixedFee(t *testing.T) {
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)

	// test fixed fee
	pp.ModeratorInfo.Fee = &pb.Moderator_Fee{
		FeeType: pb.Moderator_Fee_FIXED,
		FixedFee: &pb.Moderator_Price{
			AmountCurrency: &pb.CurrencyDefinition{
				Code:         "BTC",
				Divisibility: 8,
			},
			BigAmount: "0",
		},
	}
	if err := p.Valid(); err != nil {
		t.Errorf("expected profile with zero fee to be valid, but errored: %s", err.Error())
	}

	pp.ModeratorInfo.Fee = &pb.Moderator_Fee{
		FeeType: pb.Moderator_Fee_FIXED,
		FixedFee: &pb.Moderator_Price{
			AmountCurrency: &pb.CurrencyDefinition{
				Code:         "BTC",
				Divisibility: 8,
			},
			BigAmount: "-1",
		},
	}
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with negative fee to be invalid")
	} else if err != repo.ErrModeratorFixedFeeIsNegative {
		t.Errorf("expected ErrModeratorFixedFeeIsNegative error, but was (%s)", err.Error())
	}

	pp.ModeratorInfo.Fee = &pb.Moderator_Fee{
		FeeType: pb.Moderator_Fee_FIXED,
		FixedFee: &pb.Moderator_Price{
			AmountCurrency: &pb.CurrencyDefinition{
				Code:         "BTC",
				Divisibility: 8,
			},
			BigAmount: "",
		},
	}
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with empty fee amount to be invalid")
	} else if err != repo.ErrModeratorFixedFeeAmountIsEmpty {
		t.Errorf("expected ErrModeratorFixedFeeAmountIsEmpty error, but was (%s)", err.Error())
	}

	// test fixed fee plus percentage
	pp.ModeratorInfo.Fee = &pb.Moderator_Fee{
		FeeType: pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE,
		FixedFee: &pb.Moderator_Price{
			AmountCurrency: &pb.CurrencyDefinition{
				Code:         "BTC",
				Divisibility: 8,
			},
			BigAmount: "0",
		},
		Percentage: 1.1,
	}
	if err := p.Valid(); err != nil {
		t.Errorf("expected profile with zero fee to be valid, but errored: %s", err.Error())
	}

	pp.ModeratorInfo.Fee = &pb.Moderator_Fee{
		FeeType: pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE,
		FixedFee: &pb.Moderator_Price{
			AmountCurrency: &pb.CurrencyDefinition{
				Code:         "BTC",
				Divisibility: 8,
			},
			BigAmount: "-1",
		},
		Percentage: 1.1,
	}
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with negative fee to be invalid")
	} else if err != repo.ErrModeratorFixedFeeIsNegative {
		t.Errorf("expected ErrModeratorFixedFeeIsNegative error, but was (%s)", err.Error())
	}

	pp.ModeratorInfo.Fee = &pb.Moderator_Fee{
		FeeType: pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE,
		FixedFee: &pb.Moderator_Price{
			AmountCurrency: &pb.CurrencyDefinition{
				Code:         "BTC",
				Divisibility: 8,
			},
			BigAmount: "",
		},
		Percentage: 1.1,
	}
	if err := p.Valid(); err == nil {
		t.Errorf("expected profile with empty fee amount to be invalid")
	} else if err != repo.ErrModeratorFixedFeeAmountIsEmpty {
		t.Errorf("expected ErrModeratorFixedFeeAmountIsEmpty error, but was (%s)", err.Error())
	}
}

func TestProfileValidFixedFee(t *testing.T) {
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)

	pp.ModeratorInfo.Fee = &pb.Moderator_Fee{
		FeeType: pb.Moderator_Fee_FIXED,
		FixedFee: &pb.Moderator_Price{
			BigAmount: "1234",
			AmountCurrency: &pb.CurrencyDefinition{
				Code:         "BTC",
				Divisibility: 8,
			},
		},
		Percentage: 0,
	}

	if err := p.Valid(); err != nil {
		t.Error("expected profile example to be valid")
	}

	pp.ModeratorInfo.Fee.Percentage = 1.1
	if err := p.Valid(); err == nil {
		t.Errorf("expected non-zero percentage to be invalid")
	} else if err != repo.ErrFixedFeeHasNonZeroPercentage {
		t.Errorf("expected ErrFixedFeeHasNonZeroPercentage error, but was (%s)", err.Error())
	}
}

func TestProfileValidPercentageFee(t *testing.T) {
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.ModeratorInfo = &pb.Moderator{
		Fee: &pb.Moderator_Fee{
			FeeType:    pb.Moderator_Fee_PERCENTAGE,
			Percentage: 1.1,
		},
	}

	if err := p.Valid(); err != nil {
		t.Errorf("expected profile example to be valid")
	}
}

func TestProfileValidPercentageFeeZero(t *testing.T) {
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.ModeratorInfo = &pb.Moderator{
		Fee: &pb.Moderator_Fee{
			FeeType:    pb.Moderator_Fee_PERCENTAGE,
			Percentage: 0,
		},
	}

	if err := p.Valid(); err != nil {
		t.Errorf("expected profile example to be valid")
	}
}

func TestProfileInvalidPercentageFeeWithFixedFee(t *testing.T) {
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.ModeratorInfo = &pb.Moderator{
		Fee: &pb.Moderator_Fee{
			FeeType:    pb.Moderator_Fee_PERCENTAGE,
			Percentage: 1,
			FixedFee: &pb.Moderator_Price{
				BigAmount: "1234",
				AmountCurrency: &pb.CurrencyDefinition{
					Code:         "BTC",
					Divisibility: 8,
				},
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
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.ModeratorInfo = &pb.Moderator{
		Fee: &pb.Moderator_Fee{
			FeeType:    pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE,
			Percentage: 1,
			FixedFee: &pb.Moderator_Price{
				BigAmount: "1234",
				AmountCurrency: &pb.CurrencyDefinition{
					Code:         "BTC",
					Divisibility: 8,
				},
			},
		},
	}

	if err := p.Valid(); err != nil {
		t.Errorf("expected profile exmaple to be valid")
	}
}

func TestProfileValidPercentWithFixedHavingZeroPercent(t *testing.T) {
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.ModeratorInfo = &pb.Moderator{
		Fee: &pb.Moderator_Fee{
			FeeType:    pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE,
			Percentage: 0,
			FixedFee: &pb.Moderator_Price{
				BigAmount: "1234",
				AmountCurrency: &pb.CurrencyDefinition{
					Code:         "BTC",
					Divisibility: 8,
				},
			},
		},
	}

	if err := p.Valid(); err != nil {
		t.Errorf("expected profile exmaple to be valid")
	}
}

func TestProfileInvalidPercentWithFixedHavingNegativePercent(t *testing.T) {
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.ModeratorInfo = &pb.Moderator{
		Fee: &pb.Moderator_Fee{
			FeeType:    pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE,
			Percentage: -1,
			FixedFee: &pb.Moderator_Price{
				BigAmount: "1234",
				AmountCurrency: &pb.CurrencyDefinition{
					Code:         "BTC",
					Divisibility: 8,
				},
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
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.ModeratorInfo = &pb.Moderator{
		Fee: &pb.Moderator_Fee{
			FeeType:    pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE,
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
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.Moderator = true
	pp.ModeratorInfo = nil

	if err := p.Valid(); err == nil {
		t.Errorf("expected moderator without info to be invalid")
	} else if err != repo.ErrModeratorInfoMissing {
		t.Errorf("expected ErrModeratorInfoMissing error, but got (%s)", err.Error())
	}
}

func TestProfileGetModeratedFixedFee(t *testing.T) {
	var examples = []string{
		"v0-profile-moderator-fixed-fee",
		"v4-profile-moderator-fixed-fee",
		"v5-profile-moderator-fixed-fee",
	}

	for _, e := range examples {
		var (
			fixtureBytes       = factory.MustLoadProfileFixture(e)
			actualProfile, err = repo.UnmarshalJSONProfile(fixtureBytes)
		)
		if err != nil {
			t.Errorf("unmarshal (%s): %s", e, err)
			continue
		}

		fee, err := actualProfile.GetModeratedFixedFee()
		if err != nil {
			t.Errorf("fixed fee (%s): %s", e, err)
			continue
		}

		// all profile fixtures have equivalent data
		// validate they are intepreted from their resepctive schemas
		if fee.Amount.String() != "100" {
			t.Errorf("amount (%s): expected (%s), got (%s)", e, "100", fee.Amount.String())
		}
		if fee.Currency.Code.String() != "USD" {
			t.Errorf("currency code (%s): expected (%s), got (%s)", e, "USD", fee.Currency.Code.String())
		}
		if fee.Currency.Divisibility != 2 {
			t.Errorf("currency code (%s): expected (%d), got (%d)", e, 2, fee.Currency.Divisibility)
		}
	}
}

func TestProfileSetModeratorFixedFee(t *testing.T) {
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE
	pp.ModeratorInfo.Fee.FixedFee = nil
	pp.ModeratorInfo.Fee.Percentage = 1.1

	fee, err := repo.NewCurrencyValueWithLookup("123", "USD")
	if err != nil {
		t.Fatalf("failed creating fee value: %s", err.Error())
	}

	if err := p.SetModeratorFixedFee(fee); err != nil {
		t.Fatalf("failed setting fee: %s", err.Error())
	}

	if !pp.Moderator {
		t.Errorf("expected Moderator flag to be true")
	}
	if pp.ModeratorInfo.Fee.FeeType != pb.Moderator_Fee_FIXED {
		t.Errorf("expected feeType to be (%s), but was (%s)", pb.Moderator_Fee_FIXED.String(), pp.ModeratorInfo.Fee.FeeType)
	}
	if pp.ModeratorInfo.Fee.Percentage != 0 {
		t.Fatalf("expected percentage to be zero")
	}
	if pp.ModeratorInfo.Fee.FixedFee == nil {
		t.Fatalf("expected fixedFee to not be nil")
	}
	actualValue, err := repo.NewCurrencyValueFromProtobuf(
		pp.ModeratorInfo.Fee.FixedFee.BigAmount,
		pp.ModeratorInfo.Fee.FixedFee.AmountCurrency,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !fee.Equal(actualValue) {
		t.Errorf("expected fixed fee amount currency to be (%s), but was (%s)", fee.String(), actualValue.String())
	}
}

func TestProfileSetModeratorFixedPlusPercentageFee(t *testing.T) {
	percentage := float32(2.5)
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_PERCENTAGE
	pp.ModeratorInfo.Fee.FixedFee = nil
	pp.ModeratorInfo.Fee.Percentage = 1.1

	fee, err := repo.NewCurrencyValueWithLookup("123", "USD")
	if err != nil {
		t.Fatalf("failed creating fee value: %s", err.Error())
	}

	if err := p.SetModeratorFixedPlusPercentageFee(fee, percentage); err != nil {
		t.Fatalf("failed setting fee: %s", err.Error())
	}

	if !pp.Moderator {
		t.Errorf("expected Moderator flag to be true")
	}
	if pp.ModeratorInfo.Fee.FeeType != pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE {
		t.Errorf("expected feeType to be (%s), but was (%s)", pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String(), pp.ModeratorInfo.Fee.FeeType)
	}
	if pp.ModeratorInfo.Fee.Percentage != percentage {
		t.Fatalf("expected percentage to be zero")
	}
	if pp.ModeratorInfo.Fee.FixedFee == nil {
		t.Fatalf("expected fixedFee to not be nil")
	}
	actualValue, err := repo.NewCurrencyValueFromProtobuf(
		pp.ModeratorInfo.Fee.FixedFee.BigAmount,
		pp.ModeratorInfo.Fee.FixedFee.AmountCurrency,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !fee.Equal(actualValue) {
		t.Errorf("expected fixed fee amount currency to be (%s), but was (%s)", fee.String(), actualValue.String())
	}
}

func TestProfileSetModeratorPercentageFee(t *testing.T) {
	percentage := float32(2.5)
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED
	pp.ModeratorInfo.Fee.FixedFee = &pb.Moderator_Price{
		BigAmount: "1230",
		AmountCurrency: &pb.CurrencyDefinition{
			Code:         "BTC",
			Divisibility: 8,
		},
	}
	pp.ModeratorInfo.Fee.Percentage = 0

	if err := p.SetModeratorPercentageFee(percentage); err != nil {
		t.Fatalf("failed setting fee: %s", err.Error())
	}

	if !pp.Moderator {
		t.Errorf("expected Moderator flag to be true")
	}
	if pp.ModeratorInfo.Fee.FeeType != pb.Moderator_Fee_PERCENTAGE {
		t.Errorf("expected feeType to be (%s), but was (%s)", pb.Moderator_Fee_PERCENTAGE.String(), pp.ModeratorInfo.Fee.FeeType)
	}
	if pp.ModeratorInfo.Fee.Percentage != percentage {
		t.Fatalf("expected percentage to be zero")
	}
	if pp.ModeratorInfo.Fee.FixedFee != nil {
		t.Fatalf("expected fixedFee to be nil")
	}
}

func TestProfileDisableModeration(t *testing.T) {
	var (
		p  = factory.MustNewProfile()
		pp = p.GetProtobuf()
	)
	pp.Moderator = true
	pp.ModeratorInfo.Fee.FeeType = pb.Moderator_Fee_FIXED
	pp.ModeratorInfo.Fee.FixedFee = &pb.Moderator_Price{
		BigAmount: "1230",
		AmountCurrency: &pb.CurrencyDefinition{
			Code:         "BTC",
			Divisibility: 8,
		},
	}
	pp.ModeratorInfo.Fee.Percentage = 0

	if err := p.DisableModeration(); err != nil {
		t.Fatalf("failed disabling moderation: %s", err.Error())
	}

	if pp.Moderator {
		t.Errorf("expected Moderator flag to be false")
	}
	if pp.ModeratorInfo != nil {
		t.Errorf("expected ModeratorInfo to be nil")
	}
}
