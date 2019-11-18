package factory

import (
	"fmt"
	"sync"
)

type mockExchangeRater struct {
	referenceCurrency string
	rates             map[string]float64
	rateMutex         sync.Mutex
}

func NewMockExchangeRater(ref string) *mockExchangeRater {
	return &mockExchangeRater{
		referenceCurrency: ref,
		rates:             make(map[string]float64),
	}
}

// AddRate adds the rate for m.referenceCurrency to become the currency
// of the provided code to the index
func (m *mockExchangeRater) AddRate(code string, rate float64) *mockExchangeRater {
	m.rateMutex.Lock()
	defer m.rateMutex.Unlock()
	m.rates[code] = rate
	return m
}

// GetExchangeRate returns the rate for m.referenceCurrency to become the
// currency of the provided code from the index
func (m *mockExchangeRater) GetExchangeRate(code string) (float64, error) {
	if code == m.referenceCurrency {
		return 1, nil
	}
	m.rateMutex.Lock()
	defer m.rateMutex.Unlock()
	rate, ok := m.rates[code]
	if !ok {
		return 0, fmt.Errorf("rate not found")
	}
	return rate, nil
}
