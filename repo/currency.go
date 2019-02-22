package repo

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
)

const (
	CurrencyCodeValidMinimumLength = 3
	CurrencyCodeValidMaximumLength = 4
)

var (
	ErrInsufficientPrecision = errors.New("unable to accurately represent value as int64")
	ErrNegativeRate          = errors.New("convertion rate must be greater than zero")
)

type CurrencyValue struct {
	Amount   *big.Int
	Currency *CurrencyDefinition
}

// NewCurrencyValueFromInt is a convenience function which converts an int64
// into a string and passes the arguments to NewCurrencyValue
func NewCurrencyValueFromInt(amount int64, currencyCode string) (*CurrencyValue, error) {
	return NewCurrencyValue(strconv.FormatInt(amount, 10), currencyCode)
}

// NewCurrencyValueFromUint is a convenience function which converts an int64
// into a string and passes the arguments to NewCurrencyValue
func NewCurrencyValueFromUint(amount uint64, currencyCode string) (*CurrencyValue, error) {
	return NewCurrencyValue(strconv.FormatUint(amount, 10), currencyCode)
}

// NewCurrencyValue accepts string amounts and currency codes, and creates
// a valid CurrencyValue
func NewCurrencyValue(amount, currencyCode string) (*CurrencyValue, error) {
	var (
		i  = new(big.Int)
		ok bool
	)
	if _, ok = i.SetString(amount, 0); !ok {
		return nil, fmt.Errorf("invalid amount")
	}
	return &CurrencyValue{Amount: i}, nil
}

func (v *CurrencyValue) AmountInt64() (int64, error) {
	if !v.Amount.IsInt64() {
		return 0, ErrInsufficientPrecision
	}
	return v.Amount.Int64(), nil
}

func (v *CurrencyValue) AmountUint64() (uint64, error) {
	if !v.Amount.IsUint64() {
		return 0, ErrInsufficientPrecision
	}
	return v.Amount.Uint64(), nil
}

func (v *CurrencyValue) String() string {
	return v.Amount.String()
}

func (v *CurrencyValue) Equal(other *CurrencyValue) bool {
	if v == nil && other == nil {
		return true
	}
	if v == nil || other == nil {
		return false
	}
	return v.Amount.Cmp(other.Amount) == 0
}

func (v *CurrencyValue) ConvertTo(currencyCode string, exchangeRate float64) (*CurrencyValue, error) {
	if exchangeRate <= 0 {
		return nil, ErrNegativeRate
	}
	var (
		j    = new(big.Float)
		rate = new(big.Float)
	)
	j.SetInt(v.Amount)
	rate.SetFloat64(exchangeRate)
	j.Mul(j, rate)
	result, _ := j.Int(nil)
	return &CurrencyValue{Amount: result}, nil
}
