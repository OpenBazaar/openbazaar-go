package repo

import (
	"errors"
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
)

type CurrencyCode string

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
