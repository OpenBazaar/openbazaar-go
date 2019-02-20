package repo

import (
	"errors"
	"fmt"
	"math/big"
	"runtime/debug"
	"strings"
)

const (
	CurrencyCodeValidMinimumLength = 3
	CurrencyCodeValidMaximumLength = 4
)

var (
	ErrCurrencyCodeLengthInvalid     = errors.New("invalid length for currency code, must be three characters or four characters and begin with a 'T'")
	ErrCurrencyCodeTestSymbolInvalid = errors.New("invalid test indicator for currency code, four characters must begin with a 'T'")
	ErrInsufficientPrecision         = errors.New("unable to accurately represent value as int64")
	ErrNegativeRate                  = errors.New("convertion rate must be greater than zero")
)

type (
	CurrencyCode  string
	CurrencyValue struct {
		Amount *big.Int
	}
)

func NewCurrencyCode(c string) (*CurrencyCode, error) {
	if len(c) < CurrencyCodeValidMinimumLength || len(c) > CurrencyCodeValidMaximumLength {
		return nil, ErrCurrencyCodeLengthInvalid
	}
	if len(c) == 4 && strings.Index(strings.ToLower(c), "t") != 0 {
		return nil, ErrCurrencyCodeTestSymbolInvalid
	}
	var cc = CurrencyCode(strings.ToUpper(c))
	return &cc, nil
}

func (c *CurrencyCode) String() string {
	if c == nil {
		log.Errorf("returning nil CurrencyCode, please report this bug")
		debug.PrintStack()
		return "nil"
	}
	return string(*c)
}

func NewCurrencyValueFromInt(amount int64, currencyCode string) (*CurrencyValue, error) {
	var i = new(big.Int)
	i.SetInt64(amount)
	return &CurrencyValue{Amount: i}, nil
}

func NewCurrencyValueFromUint(amount uint64, currencyCode string) (*CurrencyValue, error) {
	var i = new(big.Int)
	i.SetUint64(amount)
	return &CurrencyValue{Amount: i}, nil
}

func NewCurrencyValue(amount, currencyCode string) (*CurrencyValue, error) {
	var (
		i  = new(big.Int)
		ok bool
	)
	if i, ok = i.SetString(amount, 0); !ok {
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
