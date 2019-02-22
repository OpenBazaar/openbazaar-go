package factory

import "github.com/OpenBazaar/openbazaar-go/repo"

func NewCurrencyDefinition() *repo.CurrencyDefinition {
	return &repo.CurrencyDefinition{
		Name:         "Bitcoin",
		Code:         repo.CurrencyCode("BTC"),
		Divisibility: 8,
		CurrencyType: repo.Crypto,
	}
}
