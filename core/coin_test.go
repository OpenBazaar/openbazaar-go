package core_test

import (
	"testing"

	"github.com/OpenBazaar/openbazaar-go/core"
)

func TestCurrencyFromString(t *testing.T) {
	examples := map[string]error{
		"invalid": core.ErrUnsupportedCurrencyCode,
	}

	for code, expectedErr := range examples {
		_, err := core.CurrencyFromString(code)
		if err != expectedErr {
			t.Errorf("incorrect error returned for %s: %s", code, err.Error())
		}
	}
}

func TestDivisibility(t *testing.T) {
	examples := map[core.Currency]uint32{
		core.Currency("btc"):  core.DefaultCurrencyDivisibility,
		core.Currency("bch"):  core.DefaultCurrencyDivisibility,
		core.Currency("eth"):  core.DefaultCurrencyDivisibility,
		core.Currency("ltc"):  core.DefaultCurrencyDivisibility,
		core.Currency("zec"):  core.DefaultCurrencyDivisibility,
		core.Currency("tbtc"): core.DefaultCurrencyDivisibility,
		core.Currency("tbch"): core.DefaultCurrencyDivisibility,
		core.Currency("teth"): core.DefaultCurrencyDivisibility,
		core.Currency("tltc"): core.DefaultCurrencyDivisibility,
		core.Currency("tzec"): core.DefaultCurrencyDivisibility,
	}

	for actualCurrency, expectedDivisibility := range examples {
		if actualCurrency.Divisibility() != expectedDivisibility {
			t.Errorf("incorrect divisibility expected for %s: expected %d, got %d", actualCurrency.String(), expectedDivisibility, actualCurrency.Divisibility())
		}
	}
}
