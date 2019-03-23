package factory

import (
	"fmt"
	"math/big"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

func NewBigInt(amount string) *big.Int {
	var i = new(big.Int)
	if _, ok := i.SetString(amount, 0); !ok {
		i.SetString("0", 0)
	}
	return i
}

func NewCurrencyDefinition(code string) *repo.CurrencyDefinition {
	if code == "" {
		code = "BTC"
	}
	return &repo.CurrencyDefinition{
		Name:         fmt.Sprintf("%scoin", code),
		Code:         repo.CurrencyCode(code),
		Divisibility: 8,
		CurrencyType: repo.Crypto,
	}
}

func NewCurrencyValue(amount, code string) *repo.CurrencyValue {
	return &repo.CurrencyValue{
		Amount:   NewBigInt(amount),
		Currency: NewCurrencyDefinition(code),
	}
}
