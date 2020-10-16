package util

import (
	"bytes"
	"testing"

	"github.com/OpenBazaar/wallet-interface"
)

type ClosingBuffer struct {
	*bytes.Buffer
}

func (cb *ClosingBuffer) Close() (err error) {
	return
}

type mockExchangeRate struct {
	rate float64
}

func (m *mockExchangeRate) GetExchangeRate(currencyCode string) (float64, error) {
	return 0, nil
}

func (m *mockExchangeRate) GetLatestRate(currencyCode string) (float64, error) {
	return m.rate, nil
}

func (m *mockExchangeRate) GetAllRates(usecache bool) (map[string]float64, error) {
	return make(map[string]float64), nil
}

func (m *mockExchangeRate) UnitsPerCoin() int {
	return 0
}

func TestFeeProvider_GetFeePerByte(t *testing.T) {
	er := &mockExchangeRate{438}
	fp := NewFeeProvider(2000, 360, 320, 280, er)

	// Test using exchange rates
	if fp.GetFeePerByte(wallet.PRIOIRTY) != 50 {
		t.Error("Returned incorrect fee per byte")
	}
	if fp.GetFeePerByte(wallet.NORMAL) != 10 {
		t.Error("Returned incorrect fee per byte")
	}
	if fp.GetFeePerByte(wallet.ECONOMIC) != 1 {
		t.Error("Returned incorrect fee per byte")
	}
	if fp.GetFeePerByte(wallet.FEE_BUMP) != 101 {
		t.Error("Returned incorrect fee per byte")
	}

	// Test exchange rate is limited at max if bad exchange rate is returned
	er.rate = 0.1
	if fp.GetFeePerByte(wallet.PRIOIRTY) != 2000 {
		t.Error("Returned incorrect fee per byte")
	}
	if fp.GetFeePerByte(wallet.NORMAL) != 2000 {
		t.Error("Returned incorrect fee per byte")
	}
	if fp.GetFeePerByte(wallet.ECONOMIC) != 2000 {
		t.Error("Returned incorrect fee per byte")
	}
	if fp.GetFeePerByte(wallet.FEE_BUMP) != 2000 {
		t.Error("Returned incorrect fee per byte")
	}

	// Test no Exchange rate provided
	fp.exchangeRates = nil
	if fp.GetFeePerByte(wallet.PRIOIRTY) != 360 {
		t.Error("Returned incorrect fee per byte")
	}
	if fp.GetFeePerByte(wallet.NORMAL) != 320 {
		t.Error("Returned incorrect fee per byte")
	}
	if fp.GetFeePerByte(wallet.ECONOMIC) != 280 {
		t.Error("Returned incorrect fee per byte")
	}
	if fp.GetFeePerByte(wallet.FEE_BUMP) != 720 {
		t.Error("Returned incorrect fee per byte")
	}
}
