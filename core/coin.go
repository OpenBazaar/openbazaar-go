package core

import (
	"errors"
	"strings"
)

// DefaultCurrencyDivisibility - decimals for price
const DefaultCurrencyDivisibility uint32 = 1e8

var (
	ErrUnsupportedCurrencyCode = errors.New("unsupported currency code")
	SupportedCurrencies        = []Currency{
		Currency("btc"),
		Currency("bch"),
		Currency("eth"),
		Currency("ltc"),
		Currency("zec"),
		Currency("tbtc"),
		Currency("tbch"),
		Currency("teth"),
		Currency("tltc"),
		Currency("tzec"),
	}
)

// Currency describes
type Currency string

func CurrencyFromString(kind string) (Currency, error) {
	lowerKind := strings.ToLower(kind)
	for _, c := range SupportedCurrencies {
		if lowerKind == string(c) {
			return c, nil
		}
	}
	return Currency(""), ErrUnsupportedCurrencyCode
}

func (c Currency) String() string { return string(c) }

func (c Currency) Divisibility() uint32 {
	return DefaultCurrencyDivisibility
}
