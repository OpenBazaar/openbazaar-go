package factory

import (
	"fmt"
	"math/big"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

func NewCurrencyDefinition(code string) repo.CurrencyDefinition {
	if code == "" {
		code = "BTC"
	}
	bt := repo.DefaultBlockTime
	if code == "LTC" || code == "TLTC" {
		bt = 150 * time.Second
	}
	return repo.CurrencyDefinition{
		Name:         fmt.Sprintf("%scoin", code),
		Code:         repo.CurrencyCode(code),
		Divisibility: 8,
		CurrencyType: repo.Crypto,
		BlockTime:    bt,
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

func MustNewCurrencyValueUsingDiv(amount, code string, customDiv uint) *repo.CurrencyValue {
	if customDiv == 0 {
		panic("custom divisibility must be greater than 0")
	}
	v := MustNewCurrencyValue(amount, code)
	v.Currency.Divisibility = customDiv
	return v
}
