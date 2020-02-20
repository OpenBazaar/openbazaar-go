package factory

import (
	"fmt"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

type mockRater struct {
	rates map[string]float64
}

func (m mockRater) GetExchangeRate(code string) (float64, error) {
	if r, ok := m.rates[code]; ok {
		return r, nil
	}
	return 0.0, fmt.Errorf("rate for code (%s) not found", code)
}

func NewCurrencyConverter(reserveCode string, mockRates map[string]float64) (*repo.CurrencyConverter, error) {
	r, ok := mockRates[reserveCode]
	if !ok {
		mockRates[reserveCode] = 1.0
	} else if r != 1.0 {
		return nil, fmt.Errorf("reserve currency rate is not 1.0")
	}
	return repo.NewCurrencyConverter(reserveCode, mockRater{mockRates})
}
