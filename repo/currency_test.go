package repo_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

func mustNewCurrencyValue(t *testing.T, amount, currencyCode string) *repo.CurrencyValue {
	var c, err = repo.NewCurrencyValue(amount, currencyCode)
	if err != nil {
		t.Fatalf("new currency value: %s", err.Error())
		return nil
	}
	return c
}

func TestNewCurrencyCode(t *testing.T) {
	var examples = []struct {
		expectErr    error
		expectString string
		input        string
	}{
		{
			expectErr:    nil,
			expectString: "BTC",
			input:        "btc",
		},
		{
			expectErr:    nil,
			expectString: "TBTC",
			input:        "tbtc", // only "t" permitted as first of 4 characters
		},
		{
			expectErr:    repo.ErrCurrencyCodeTestSymbolInvalid,
			expectString: "",
			input:        "xbtc",
		},
		{
			expectErr:    repo.ErrCurrencyCodeLengthInvalid,
			expectString: "",
			input:        "bt",
		},
		{
			expectErr:    repo.ErrCurrencyCodeLengthInvalid,
			expectString: "",
			input:        "",
		},
	}

	for _, e := range examples {
		var c, err = repo.NewCurrencyCode(e.input)
		if e.expectErr != nil {
			if err != nil {
				if e.expectErr.Error() == err.Error() {
					continue
				}
				t.Errorf("unexpected error for input (%s): have (%s), want (%s)", e.input, err.Error(), e.expectErr.Error())
				continue
			}
			t.Errorf("unexpected error for input (%s): have nil, want (%s)", e.input, e.expectErr.Error())
			continue
		}
		if err != nil {
			t.Errorf("unexpected error for input (%s): have (%s), want nil", e.input, err.Error())
			continue
		}
		if c.String() != e.expectString {
			t.Errorf("unexpected string for input (%s): have (%s), want (%s)", e.input, c.String(), e.expectString)
		}
	}
}

func TestCurrencyValueMarshalsToJSON(t *testing.T) {
	var (
		invalidAmountErr = "invalid amount"
		examples         = []struct {
			value     string
			expectErr *string
		}{
			{
				value:     "123456789012345678",
				expectErr: nil,
			},
			{
				value:     "123456789012345678901234567890",
				expectErr: nil,
			},
			{
				value:     "",
				expectErr: &invalidAmountErr,
			},
		}
	)

	for _, e := range examples {
		var (
			example, err = repo.NewCurrencyValue(e.value, "BTC")
			actual       *repo.CurrencyValue
		)
		if e.expectErr == nil && err != nil {
			t.Errorf("unable to parse valid input '%s': %s", e.value, err.Error())
			continue
		} else {
			if err != nil && !strings.Contains(err.Error(), *e.expectErr) {
				t.Errorf("expected error to contain (%s), was: %s", *e.expectErr, err.Error())
			}
			continue
		}
		j, err := json.Marshal(example)
		if err != nil {
			t.Errorf("marshaling %s: %s", example.String(), err)
			continue
		}

		if err := json.Unmarshal(j, &actual); err != nil {
			t.Errorf("unmarhsaling %s, %s", example.String(), err)
			continue
		}

		if !actual.Equal(example) {
			t.Errorf("expected %s and %s to be equal, but was not", example.String(), actual.String())
		}
	}
}

func TestCurrencyValuesAreEqual(t *testing.T) {
	var examples = []struct {
		value    *repo.CurrencyValue
		other    *repo.CurrencyValue
		expected bool
	}{
		{
			value:    nil,
			other:    nil,
			expected: true,
		},
		{
			value:    mustNewCurrencyValue(t, "1", "BTC"),
			other:    mustNewCurrencyValue(t, "1", "BTC"),
			expected: true,
		},
		{
			value:    nil,
			other:    mustNewCurrencyValue(t, "1", "BTC"),
			expected: false,
		},
		{
			value:    mustNewCurrencyValue(t, "1", "BTC"),
			other:    nil,
			expected: false,
		},
		{
			value:    mustNewCurrencyValue(t, "1", "BTC"),
			other:    mustNewCurrencyValue(t, "2", "BTC"),
			expected: false,
		},
	}

	for _, c := range examples {
		if c.value.Equal(c.other) != c.expected {
			if c.expected {
				t.Errorf("expected %s to equal %s but did not", c.value.String(), c.other.String())
			} else {
				t.Errorf("expected %s to not equal %s but did", c.value.String(), c.other.String())
			}
		}
	}
}

func TestCurrencyValuesConvertCorrectly(t *testing.T) {
	var (
		zeroRateErr = "rate must be greater than zero"

		examples = []struct {
			value        int64
			exchangeRate float64
			expected     int64
			expectedErr  *string
		}{
			{ // errors zero rate
				value:        123,
				exchangeRate: 0,
				expected:     0,
				expectedErr:  &zeroRateErr,
			},
			{ // errors negative rate
				value:        123,
				exchangeRate: -0.1,
				expected:     0,
				expectedErr:  &zeroRateErr,
			},
			{ // rounds down
				value:        1,
				exchangeRate: 0.9,
				expected:     0,
				expectedErr:  nil,
			},
			{ // handles negative values
				value:        -100,
				exchangeRate: 0.123,
				expected:     -12,
				expectedErr:  nil,
			},
			{ // handles zero
				value:        0,
				exchangeRate: 0.99999,
				expected:     0,
				expectedErr:  nil,
			},
		}
	)

	for _, e := range examples {
		before, err := repo.NewCurrencyValueFromInt(e.value, "AAA")
		if err != nil {
			t.Errorf("unable to create CurrencyValue for value (%d): %s", e.value, err.Error())
			continue
		}
		actual, err := before.ConvertTo("BBB", e.exchangeRate)
		if err != nil {
			if e.expectedErr != nil && !strings.Contains(err.Error(), *e.expectedErr) {
				t.Errorf("expected value (%d) to error with (%s) but returned: %s", e.value, *e.expectedErr, err.Error())
			}
			continue
		} else {
			if e.expectedErr != nil {
				t.Errorf("expected error (%s) but produced none", *e.expectedErr)
				t.Logf("\tfor example: %+v", e)
			}
		}

		intAmount, err := actual.AmountInt64()
		if err != nil {
			t.Errorf("error producing int64 amount: %s", err.Error())
			continue
		}
		if intAmount != e.expected {
			t.Errorf("expected converted value to be %d, but was %d", e.expected, intAmount)
			continue
		}
	}
}
