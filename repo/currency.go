package repo

import (
	"errors"
	"runtime/debug"
	"strings"
)

const CurrencyCodeValidLength = 3

var (
	ErrCurrencyCodeLengthInvalid = errors.New("invalid length for currency code, must be three characters")
)

type CurrencyCode string

func NewCurrencyCode(c string) (*CurrencyCode, error) {
	if len(c) != CurrencyCodeValidLength {
		return nil, ErrCurrencyCodeLengthInvalid
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
