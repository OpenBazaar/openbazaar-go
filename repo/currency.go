package repo

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
)

const (
	CurrencyCodeValidMinimumLength = 3
	CurrencyCodeValidMaximumLength = 4
)

var (
	ErrCurrencyValueInsufficientPrecision = errors.New("unable to accurately represent value as int64")
	ErrCurrencyValueNegativeRate          = errors.New("conversion rate must be greater than zero")
	ErrCurrencyValueAmountInvalid         = errors.New("invalid amount")
	ErrCurrencyValueDefinitionInvalid     = errors.New("invalid currency definition")
)

// CurrencyValue represents the amount and variety of currency
type CurrencyValue struct {
	Amount   *big.Int
	Currency *CurrencyDefinition
}

// NewCurrencyValueFromInt is a convenience function which converts an int64
// into a string and passes the arguments to NewCurrencyValue
func NewCurrencyValueFromInt(amount int64, currency *CurrencyDefinition) (*CurrencyValue, error) {
	return NewCurrencyValue(strconv.FormatInt(amount, 10), currency)
}

// NewCurrencyValueFromUint is a convenience function which converts an int64
// into a string and passes the arguments to NewCurrencyValue
func NewCurrencyValueFromUint(amount uint64, currency *CurrencyDefinition) (*CurrencyValue, error) {
	return NewCurrencyValue(strconv.FormatUint(amount, 10), currency)
}

// NewCurrencyValue accepts string amounts and currency codes, and creates
// a valid CurrencyValue
func NewCurrencyValue(amount string, currency *CurrencyDefinition) (*CurrencyValue, error) {
	var (
		i  = new(big.Int)
		ok bool
	)
	if _, ok = i.SetString(amount, 0); !ok {
		return nil, ErrCurrencyValueAmountInvalid
	}
	return &CurrencyValue{Amount: i, Currency: currency}, nil
}

// AmountInt64 returns a valid int64 or an error
func (v *CurrencyValue) AmountInt64() (int64, error) {
	if !v.Amount.IsInt64() {
		return 0, ErrCurrencyValueInsufficientPrecision
	}
	return v.Amount.Int64(), nil
}

// AmountUint64 returns a valid int64 or an error
func (v *CurrencyValue) AmountUint64() (uint64, error) {
	if !v.Amount.IsUint64() {
		return 0, ErrCurrencyValueInsufficientPrecision
	}
	return v.Amount.Uint64(), nil
}

// String returns a string representation of a CurrencyValue
func (v *CurrencyValue) String() string {
	return fmt.Sprintf("%s %s", v.Amount.String(), v.Currency.String())
}

// Valid returns an error if the CurrencyValue is invalid
func (v *CurrencyValue) Valid() error {
	if v.Amount == nil {
		return ErrCurrencyValueAmountInvalid
	}
	if err := v.Currency.Valid(); err != nil {
		return err
	}
	return nil
}

// Equal indicates if the amount and variety of currency is equivalent
func (v *CurrencyValue) Equal(other *CurrencyValue) bool {
	if v == nil || other == nil {
		return false
	}
	if !v.Currency.Equal(other.Currency) {
		return false
	}
	return v.Amount.Cmp(other.Amount) == 0
}

// ConvertTo will perform the following math given its arguments are valid:
// v.Amount * exchangeRate * (final.Currency.Divisibility/v.Currency.Divisibility)
// where v is the receiver, exchangeRate is the ratio of (1 final.Currency/v.Currency)
// v and final must both be Valid() and exchangeRate must not be zero.
func (v *CurrencyValue) ConvertTo(final *CurrencyDefinition, exchangeRate float64) (*CurrencyValue, error) {
	if err := v.Valid(); err != nil {
		return nil, fmt.Errorf("cannot convert invalid value: %s", err.Error())
	}
	if err := final.Valid(); err != nil {
		return nil, fmt.Errorf("cannot convert to invalid currency: %s", err.Error())
	}
	if exchangeRate <= 0 {
		return nil, ErrCurrencyValueNegativeRate
	}

	var (
		j                = new(big.Float)
		currencyRate     = new(big.Float)
		divisibilityRate = new(big.Float)

		divRateFloat = math.Pow10(int(final.Divisibility)) / math.Pow10(int(v.Currency.Divisibility))
	)

	currencyRate.SetFloat64(exchangeRate)
	divisibilityRate.SetFloat64(divRateFloat)

	j.SetInt(v.Amount)
	j.Mul(j, currencyRate)
	j.Mul(j, divisibilityRate)
	result, _ := j.Int(nil)
	return &CurrencyValue{Amount: result, Currency: final}, nil
}
