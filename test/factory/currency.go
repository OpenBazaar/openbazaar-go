package factory

import (
	"fmt"
	"math/big"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

func NewCurrencyDefinition(code string) repo.CurrencyDefinition {
	if code == "" {
		code = "BTC"
	}
	return repo.CurrencyDefinition{
		Name:         fmt.Sprintf("%scoin", code),
		Code:         repo.CurrencyCode(code),
		Divisibility: 8,
		CurrencyType: repo.Crypto,
	}
}

func MustNewCurrencyValue(amount, code string) *repo.CurrencyValue {
	amt, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		panic(fmt.Sprintf("invalid CurrencyValue amount: %s", amount))
	}
	return &repo.CurrencyValue{
		Amount:   amt,
		Currency: NewCurrencyDefinition(code),
	}
}
