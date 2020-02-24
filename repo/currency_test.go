package repo_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func mustNewCurrencyValue(t *testing.T, amount, currencyCode string) *repo.CurrencyValue {
	var (
		def    = factory.NewCurrencyDefinition(currencyCode)
		c, err = repo.NewCurrencyValue(amount, def)
	)
	if err != nil {
		t.Fatalf("currency value (%s, %s): %s", amount, currencyCode, err.Error())
		return nil
	}
	return c
}

func TestCurrencyValueMarshalsToJSON(t *testing.T) {
	var (
		examples = []struct {
			value    string
			currency repo.CurrencyDefinition
		}{
			{ // valid currency value
				value:    "123456789012345678",
				currency: factory.NewCurrencyDefinition("ABC"),
			},
			{ // valid currency value large enough to overflow primative ints
				value:    "123456789012345678901234567890",
				currency: factory.NewCurrencyDefinition("BCD"),
			},
		}
	)

	for _, e := range examples {
		var (
			example, err = repo.NewCurrencyValue(e.value, e.currency)
			//actual       *repo.CurrencyValue
		)
		actual := &repo.CurrencyValue{}
		if err != nil {
			t.Errorf("unable to parse valid input '%s': %s", e.value, err.Error())
			continue
		}
		j, err := example.MarshalJSON()
		if err != nil {
			t.Errorf("marshaling %s: %s", example.String(), err)
			continue
		}

		if err := actual.UnmarshalJSON(j); err != nil {
			t.Errorf("unmarhsaling %s, %s", example.String(), err)
			continue
		}

		if !actual.Equal(example) {
			t.Errorf("expected %s and %s to be equal, but was not", example.String(), actual.String())
		}
	}
}

func TestCurrencyValuesAreValid(t *testing.T) {
	var examples = []struct {
		value       func() *repo.CurrencyValue
		expectedErr error
	}{
		{ // valid value
			value:       func() *repo.CurrencyValue { return mustNewCurrencyValue(t, "1", "BTC") },
			expectedErr: nil,
		},
		{ // invalid nil value
			value: func() *repo.CurrencyValue {
				var value = factory.MustNewCurrencyValue("123", "BTC")
				value.Amount = nil
				return value
			},
			expectedErr: repo.ErrCurrencyValueAmountInvalid,
		},
		{ // invalid nil currency
			value: func() *repo.CurrencyValue {
				var value = factory.MustNewCurrencyValue("123", "BTC")
				value.Currency = repo.NilCurrencyDefinition
				return value
			},
			expectedErr: repo.ErrCurrencyDefinitionUndefined,
		},
		{ // invalid currency definition makes value invalid (0 divisibility)
			value: func() *repo.CurrencyValue {
				var value = factory.MustNewCurrencyValue("123", "BTC")
				value.Currency.Divisibility = 0
				return value
			},
			expectedErr: repo.ErrCurrencyDivisibilityNonPositive,
		},
	}

	for _, e := range examples {
		var value = e.value()
		if err := value.Valid(); err != nil {
			if e.expectedErr != nil && err != e.expectedErr {
				t.Errorf("expected error (%s), but was (%s) for value (%s)", e.expectedErr.Error(), err.Error(), value.String())
			}
			if e.expectedErr == nil {
				t.Errorf("expected error to be nil, but was (%s) for value (%s)", err.Error(), value.String())
			}
			continue
		}
		if e.expectedErr != nil {
			t.Errorf("expected an error, but was nil for (%+v)", value)
			continue
		}
	}
}

func TestCurrencyValuesAreEqual(t *testing.T) {
	var examples = []struct {
		value    *repo.CurrencyValue
		other    *repo.CurrencyValue
		expected bool
	}{
		{ // value and currency divisibility different but
			// equal after normalizing
			value: &repo.CurrencyValue{
				Amount: big.NewInt(1234),
				Currency: repo.CurrencyDefinition{
					Code:         "BTC",
					Divisibility: 2,
					CurrencyType: "crypto",
				},
			},
			other: &repo.CurrencyValue{
				Amount: big.NewInt(123400),
				Currency: repo.CurrencyDefinition{
					Code:         "BTC",
					Divisibility: 4,
					CurrencyType: "crypto",
				},
			},
			expected: true,
		},
		{ // value and currency matching should be equal
			value: &repo.CurrencyValue{
				Amount:   big.NewInt(1),
				Currency: factory.NewCurrencyDefinition("BTC"),
			},
			other: &repo.CurrencyValue{
				Amount:   big.NewInt(1),
				Currency: factory.NewCurrencyDefinition("BTC"),
			},
			expected: true,
		},
		{ // nils should not be equal
			value:    nil,
			other:    nil,
			expected: true,
		},
		{ // nil should not match with a value
			value: nil,
			other: &repo.CurrencyValue{
				Amount:   big.NewInt(1),
				Currency: factory.NewCurrencyDefinition("BTC"),
			},
			expected: false,
		},
		{ // value should not match with nil
			value: &repo.CurrencyValue{
				Amount:   big.NewInt(1),
				Currency: factory.NewCurrencyDefinition("BTC"),
			},
			other:    nil,
			expected: false,
		},
		{ // value difference
			value: &repo.CurrencyValue{
				Amount:   big.NewInt(1),
				Currency: factory.NewCurrencyDefinition("BTC"),
			},
			other: &repo.CurrencyValue{
				Amount:   big.NewInt(2),
				Currency: factory.NewCurrencyDefinition("BTC"),
			},
			expected: false,
		},
		{ // currency code difference
			value: &repo.CurrencyValue{
				Amount:   big.NewInt(1),
				Currency: factory.NewCurrencyDefinition("BTC"),
			},
			other: &repo.CurrencyValue{
				Amount:   big.NewInt(1),
				Currency: factory.NewCurrencyDefinition("ETH"),
			},
			expected: false,
		},
		{ // currency code missing
			value: &repo.CurrencyValue{
				Amount:   big.NewInt(1),
				Currency: repo.NilCurrencyDefinition,
			},
			other: &repo.CurrencyValue{
				Amount:   big.NewInt(1),
				Currency: factory.NewCurrencyDefinition("ETH"),
			},
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

		// test that equal is communitive
		if c.other.Equal(c.value) != c.expected {
			if c.expected {
				t.Errorf("expected %s to equal %s but did not", c.other.String(), c.value.String())
			} else {
				t.Errorf("expected %s to not equal %s but did", c.other.String(), c.value.String())
			}
		}
	}
}

func TestCurrencyValuesConvertCorrectly(t *testing.T) {
	var (
		zeroRateErr          = "must be greater than zero"
		undefinedCurrencyErr = "unknown currency"
		invalidErr           = "cannot convert invalid value"
		reserveCurrency      = "BTC"

		examples = []struct {
			value         *repo.CurrencyValue
			convertTo     repo.CurrencyDefinition
			exchangeRates map[string]float64
			expected      *repo.CurrencyValue
			expectedAcc   int8
			expectedErr   *string
		}{
			{ // 0. errors when definition is nil
				value:     factory.MustNewCurrencyValue("0", "BTC"),
				convertTo: repo.NilCurrencyDefinition,
				exchangeRates: map[string]float64{
					"BCH": 0.99999,
				},
				expected:    nil,
				expectedAcc: 0,
				expectedErr: &undefinedCurrencyErr,
			},
			{ // 1. errors zero rate
				value:     factory.MustNewCurrencyValue("123", "BTC"),
				convertTo: factory.NewCurrencyDefinition("BCH"),
				exchangeRates: map[string]float64{
					"BCH": 0,
				},
				expected:    nil,
				expectedAcc: 0,
				expectedErr: &zeroRateErr,
			},
			{ // 2. errors negative rate
				value:     factory.MustNewCurrencyValue("123", "BTC"),
				convertTo: factory.NewCurrencyDefinition("BCH"),
				exchangeRates: map[string]float64{
					"BCH": -0.1,
				},
				expected:    nil,
				expectedAcc: 0,
				expectedErr: &zeroRateErr,
			},
			{ // 3. rounds up
				value:     factory.MustNewCurrencyValue("1", "BTC"),
				convertTo: factory.NewCurrencyDefinition("BCH"),
				exchangeRates: map[string]float64{
					"BCH": 0.9,
				},
				expected:    factory.MustNewCurrencyValue("1", "BCH"),
				expectedAcc: 1,
				expectedErr: nil,
			},
			{ // 4. handles negative values
				value:     factory.MustNewCurrencyValue("-100", "BTC"),
				convertTo: factory.NewCurrencyDefinition("BCH"),
				exchangeRates: map[string]float64{
					"BCH": 0.123,
				},
				expected:    factory.MustNewCurrencyValue("-13", "BCH"),
				expectedAcc: -1,
				expectedErr: nil,
			},
			{ // 5. handles zero
				value:     factory.MustNewCurrencyValue("0", "BTC"),
				convertTo: factory.NewCurrencyDefinition("BCH"),
				exchangeRates: map[string]float64{
					"BCH": 0.99999,
				},
				expected:    factory.MustNewCurrencyValue("0", "BCH"),
				expectedAcc: 0,
				expectedErr: nil,
			},
			{ // 6. handles invalid value
				value: &repo.CurrencyValue{
					Amount:   big.NewInt(1000),
					Currency: repo.NilCurrencyDefinition,
				},
				convertTo: factory.NewCurrencyDefinition("BCH"),
				exchangeRates: map[string]float64{
					"BCH": 0.5,
				},
				expected:    nil,
				expectedAcc: 0,
				expectedErr: &invalidErr,
			},
			{ // 7. handles conversions between different divisibility
				value: &repo.CurrencyValue{
					Amount: big.NewInt(1000),
					Currency: repo.CurrencyDefinition{
						Name:         "United States Dollar",
						Code:         "USD",
						Divisibility: 2,
						CurrencyType: repo.Fiat,
					},
				},
				convertTo: repo.CurrencyDefinition{
					Name:         "SimpleCoin",
					Code:         "SPC",
					Divisibility: 6,
					CurrencyType: repo.Crypto,
				},
				exchangeRates: map[string]float64{
					"USD": 1,
					"SPC": 0.5,
				},
				expected: &repo.CurrencyValue{
					Amount: big.NewInt(5000000),
					Currency: repo.CurrencyDefinition{
						Name:         "SimpleCoin",
						Code:         "SPC",
						Divisibility: 6,
						CurrencyType: repo.Crypto,
					},
				},
				expectedAcc: 0,
				expectedErr: nil,
			},
			{ // 8. handles conversions between different
				// divisibility and rates
				value: &repo.CurrencyValue{ // 99.123456 SPC
					Amount: big.NewInt(99123456),
					Currency: repo.CurrencyDefinition{
						Name:         "SimpleCoin",
						Code:         "SPC",
						Divisibility: 6,
						CurrencyType: repo.Crypto,
					},
				},
				convertTo: repo.CurrencyDefinition{
					Name:         "United States Dollar",
					Code:         "USD",
					Divisibility: 2,
					CurrencyType: repo.Fiat,
				},
				exchangeRates: map[string]float64{
					"SPC": 0.5,
					"USD": 2,
				},
				expected: &repo.CurrencyValue{ // 99.123456 SPC * (2/0.5 USD/SPC * 100/1000000) (rounded up to next largest int)
					Amount: big.NewInt(39650),
					Currency: repo.CurrencyDefinition{
						Name:         "United States Dollar",
						Code:         "USD",
						Divisibility: 2,
						CurrencyType: repo.Fiat,
					},
				},
				expectedAcc: 1,
				expectedErr: nil,
			},
			{ // 9. handles conversions which reduce significant figures
				value: &repo.CurrencyValue{
					Amount: big.NewInt(7654321),
					Currency: repo.CurrencyDefinition{
						Name:         "SimpleCoin",
						Code:         "SPC",
						Divisibility: 4,
						CurrencyType: repo.Crypto,
					},
				},
				convertTo: repo.CurrencyDefinition{
					Name:         "SimpleCoin",
					Code:         "SPC",
					Divisibility: 2,
					CurrencyType: repo.Crypto,
				},
				exchangeRates: map[string]float64{
					"SPC": 1.0,
				},
				expected: &repo.CurrencyValue{
					Amount: big.NewInt(76544),
					Currency: repo.CurrencyDefinition{
						Name:         "SimpleCoin",
						Code:         "SPC",
						Divisibility: 2,
						CurrencyType: repo.Crypto,
					},
				},
				expectedAcc: 1,
				expectedErr: nil,
			},
		}
	)

	for i, e := range examples {
		cc, err := factory.NewCurrencyConverter(reserveCurrency, e.exchangeRates)
		if err != nil {
			t.Errorf("failed to create currency converter: %s", err.Error())
			t.Logf("with rates: %v", e.exchangeRates)
			continue
		}

		actual, actualAcc, err := e.value.ConvertTo(e.convertTo, cc)
		if err != nil {
			if e.expectedErr != nil {
				if !strings.Contains(err.Error(), *e.expectedErr) {
					t.Errorf("expected value (%s) to error with (%s) but returned: %s", e.value, *e.expectedErr, err.Error())
				}
			} else {
				t.Errorf("unexpected error for example (%d): %s", i, err.Error())
			}
			continue
		} else {
			if e.expectedErr != nil {
				t.Errorf("expected error (%s) but produced none", *e.expectedErr)
				t.Logf("\texample: (%d) for value: (%s) convertTo: (%s) rates: (%v)", i, e.value, e.convertTo, e.exchangeRates)
			}
		}

		if !actual.Equal(e.expected) {
			t.Errorf("expected converted value to be %s, but was %s", e.expected, actual)
			t.Logf("\texample: (%d) for value: (%s) convertTo: (%s) rates: (%v)", i, e.value, e.convertTo, e.exchangeRates)
			continue
		}

		if expectedAcc := big.Accuracy(e.expectedAcc); actualAcc != expectedAcc {
			t.Errorf("expected converted accuracy to be %s, but was %s", expectedAcc.String(), actualAcc.String())
			t.Logf("\texample: (%d) for value: (%s) convertTo: (%s) rates: (%v)", i, e.value, e.convertTo, e.exchangeRates)
		}
	}
}

func TestConvertUsingProtobufDef(t *testing.T) {
	subject := factory.MustNewCurrencyValue("10", "USD")
	subject.Currency.Divisibility = 2
	conv, err := factory.NewCurrencyConverter("BTC", map[string]float64{
		"USD": 2,
		"BCH": 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	convertTo := &pb.CurrencyDefinition{
		Code:         "TBCH",
		Divisibility: 8,
	}
	expected := &repo.CurrencyValue{
		Amount: big.NewInt(2500000), // 10 * (1/2 BTC/USD) * (1/2 BCH/BTC) * 1000000 (divisibility)
		Currency: repo.CurrencyDefinition{
			Code:         "TBCH",
			CurrencyType: repo.Crypto,
			Divisibility: 8,
		},
	}

	cv, acc, err := subject.ConvertUsingProtobufDef(convertTo, conv)
	if err != nil {
		t.Fatal(err)
	}

	if acc != big.Exact {
		t.Errorf("expected result to be exact, but was (%s)", acc.String())
	}

	if !cv.Equal(expected) {
		t.Errorf("expected result amount to be (%v), but was (%v)", expected, cv)
	}
}

func TestNewCurrencyValueWithLookup(t *testing.T) {
	_, err := repo.NewCurrencyValueWithLookup("0", "")
	if err == nil {
		t.Errorf("expected empty code to return an error, but did not")
	}

	_, err = repo.NewCurrencyValueWithLookup("0", "invalid")
	if err == nil {
		t.Errorf("expected invalid/undefined code to return an error, but did not")
	}

	subject, err := repo.NewCurrencyValueWithLookup("", "USD")
	if err != nil {
		t.Errorf("expected empty value to be accepted, but returned error: %s", err.Error())
	}
	if subject.String() != "0 United States Dollar (USDdiv2)" {
		t.Errorf("expected empty value to be set as (0 United State Dollar (USDdiv2)), but was (%s)", subject.String())
	}

	_, err = repo.NewCurrencyValueWithLookup("1234567890987654321", "ETH")
	if err != nil {
		t.Errorf("expected large value to be accepted, but returned error: %s", err.Error())
	}
}

func TestCurrencyValueAmount(t *testing.T) {
	subject := &repo.CurrencyValue{}
	actual := subject.AmountString()
	if actual != "0" {
		t.Errorf("expected zero value amount string to be (0), but was (%s)", actual)
	}

	subject = &repo.CurrencyValue{Amount: big.NewInt(100)}
	actual = subject.AmountString()
	if actual != "100" {
		t.Errorf("expected set value to be (%s), but was (%s)", "100", actual)
	}
}

func TestCurrencyValueAdjustDivisibility(t *testing.T) {
	sameDiv := uint(8)
	subject := factory.MustNewCurrencyValue("123000000", "BTC")
	subject.Currency.Divisibility = sameDiv

	if newValue, _, err := subject.AdjustDivisibility(sameDiv); err != nil {
		t.Fatalf("expected same divisibility to not return an error, but did: %s", err.Error())
	} else {
		if !newValue.Currency.Equal(subject.Currency) {
			t.Errorf("expected same divisibility to produce equal currencies, but did not")
		}
	}

	if newValue, _, err := subject.AdjustDivisibility(2); err != nil {
		t.Fatalf("expected new divisibility to not return an error, but did: %s", err.Error())
	} else {
		if newValue.Currency.Equal(subject.Currency) {
			t.Errorf("expected new divisibility to produce different currency, but did not")
		}
	}
}

func TestCurrencyValueCmp(t *testing.T) {
	var examples = []struct {
		subject     *repo.CurrencyValue
		other       *repo.CurrencyValue
		expected    int
		expectedErr error
	}{
		{ // success case
			subject:  factory.MustNewCurrencyValue("1234", "USD"),
			other:    factory.MustNewCurrencyValue("12345", "USD"),
			expected: -1, // subject.Amount.Cmp(other.Amount) == -1
		},
		{ // different divisibilities handled
			subject:  factory.MustNewCurrencyValueUsingDiv("1234", "USD", 2),
			other:    factory.MustNewCurrencyValueUsingDiv("123456", "USD", 4),
			expected: -1, // subject.AdjustDivisibility(4).Amount.Cmp(other.Amount) == -1
		},
		{ // different divisibilities (reverse suject/other) handled
			subject:  factory.MustNewCurrencyValueUsingDiv("123456", "USD", 4),
			other:    factory.MustNewCurrencyValueUsingDiv("1234", "USD", 2),
			expected: 1, // subject.AdjustDivisibility(4).Amount.Cmp(other.Amount) == 1
		},
		{ // different currencies return error
			subject:     factory.MustNewCurrencyValue("1234", "USD"),
			other:       factory.MustNewCurrencyValue("1234", "NOTUSD"),
			expectedErr: repo.ErrCurrencyValueInvalidCmpDifferentCurrencies,
		},
	}

	for _, e := range examples {
		t.Logf("subject (%+v), other (%+v)", e.subject, e.other)
		actual, err := e.subject.Cmp(e.other)
		if e.expectedErr != nil {
			if err != e.expectedErr {
				t.Errorf("expected err (%s), but was (%s)", e.expectedErr.Error(), err.Error())
			}
		} else {
			if err != nil {
				t.Errorf("unexpected err: %s", err)
				continue
			}
		}
		if e.expected != actual {
			t.Errorf("expected comparison result (%d), but was (%d)", e.expected, actual)
		}
	}
}

func TestCurrencyValueIsZero(t *testing.T) {
	subject := factory.MustNewCurrencyValue("0", "BTC")

	subject.Amount = nil
	if subject.IsZero() {
		t.Errorf("expected IsZero to be false for (%s), but was not", subject)
	}
	subject.Amount = big.NewInt(0)
	if !subject.IsZero() {
		t.Errorf("expected IsZero to be true for (%s), but was not", subject)
	}
	subject.Amount = big.NewInt(-1)
	if subject.IsZero() {
		t.Errorf("expected IsZero to be false for (%s), but was not", subject)
	}
	subject.Amount = big.NewInt(1)
	if subject.IsZero() {
		t.Errorf("expected IsZero to be false for (%s), but was not", subject)
	}
}

func TestCurrencyValueIsNegative(t *testing.T) {
	subject := factory.MustNewCurrencyValue("0", "BTC")

	subject.Amount = nil
	if subject.IsNegative() {
		t.Errorf("expected IsNegative to be false for (%s), but was not", subject)
	}
	subject.Amount = big.NewInt(0)
	if subject.IsNegative() {
		t.Errorf("expected IsNegative to be false for (%s), but was not", subject)
	}
	subject.Amount = big.NewInt(-1)
	if !subject.IsNegative() {
		t.Errorf("expected IsNegative to be true for (%s), but was not", subject)
	}
	subject.Amount = big.NewInt(1)
	if subject.IsNegative() {
		t.Errorf("expected IsNegative to be false for (%s), but was not", subject)
	}
}

func TestCurrencyValueAddBigFloatProduct(t *testing.T) {
	var (
		subject  = factory.MustNewCurrencyValue("100", "BTC")
		examples = []struct {
			example  *big.Float
			expected *repo.CurrencyValue
		}{
			{ // add zero
				example:  big.NewFloat(0.0),
				expected: factory.MustNewCurrencyValue("100", "BTC"),
			},
			{ // add one percent
				example:  big.NewFloat(0.01),
				expected: factory.MustNewCurrencyValue("101", "BTC"),
			},
			{ // add negative one percent
				example:  big.NewFloat(-0.01),
				expected: factory.MustNewCurrencyValue("99", "BTC"),
			},
			{ // add double
				example:  big.NewFloat(2.0),
				expected: factory.MustNewCurrencyValue("300", "BTC"),
			},
		}
	)

	for _, e := range examples {
		if actual := subject.AddBigFloatProduct(e.example); !actual.Equal(e.expected) {
			t.Errorf("expected (%v) to be (%v), for (%s + (%s * (%s))", actual.AmountString(), e.expected.AmountString(), subject.AmountString(), subject.AmountString(), e.example.String())
		}
	}

	if !subject.Equal(factory.MustNewCurrencyValue("100", "BTC")) {
		t.Errorf("expected subject to not be mutated, but was")
	}
}

func TestCurrencyValueAddBigInt(t *testing.T) {
	var (
		subject  = factory.MustNewCurrencyValue("100", "BTC")
		examples = []struct {
			example  *big.Int
			expected *repo.CurrencyValue
		}{
			{ // add one
				example:  big.NewInt(1),
				expected: factory.MustNewCurrencyValue("101", "BTC"),
			},
			{ // add negative one
				example:  big.NewInt(-1),
				expected: factory.MustNewCurrencyValue("99", "BTC"),
			},
			{ // add zero
				example:  big.NewInt(0),
				expected: factory.MustNewCurrencyValue("100", "BTC"),
			},
		}
	)

	for _, e := range examples {
		if actual := subject.AddBigInt(e.example); !actual.Equal(e.expected) {
			t.Errorf("expected (%v) to be (%v), for (%s + %s)", actual.AmountString(), e.expected.AmountString(), subject.AmountString(), e.example.String())
		}
	}

	if !subject.Equal(factory.MustNewCurrencyValue("100", "BTC")) {
		t.Errorf("expected subject to not be mutated, but was")
	}
}

func TestCurrencyValueSubBigInt(t *testing.T) {
	var (
		subject  = factory.MustNewCurrencyValue("100", "BTC")
		examples = []struct {
			example  *big.Int
			expected *repo.CurrencyValue
		}{
			{ // sub one
				example:  big.NewInt(1),
				expected: factory.MustNewCurrencyValue("99", "BTC"),
			},
			{ // sub negative one
				example:  big.NewInt(-1),
				expected: factory.MustNewCurrencyValue("101", "BTC"),
			},
			{ // sub zero
				example:  big.NewInt(0),
				expected: factory.MustNewCurrencyValue("100", "BTC"),
			},
		}
	)

	for _, e := range examples {
		if actual := subject.SubBigInt(e.example); !actual.Equal(e.expected) {
			t.Errorf("expected (%v) to be (%v), for (%s + %s)", actual.AmountString(), e.expected.AmountString(), subject.AmountString(), e.example.String())
		}
	}

	if !subject.Equal(factory.MustNewCurrencyValue("100", "BTC")) {
		t.Errorf("expected subject to not be mutated, but was")
	}
}

func TestCurrencyValueMulBigInt(t *testing.T) {
	var (
		subject  = factory.MustNewCurrencyValue("100", "BTC")
		examples = []struct {
			example  *big.Int
			expected *repo.CurrencyValue
		}{
			{ // multiply zero
				example:  big.NewInt(0),
				expected: factory.MustNewCurrencyValue("0", "BTC"),
			},
			{ // multiply 3
				example:  big.NewInt(3),
				expected: factory.MustNewCurrencyValue("300", "BTC"),
			},
		}
	)

	for _, e := range examples {
		if actual := subject.MulBigInt(e.example); !actual.Equal(e.expected) {
			t.Errorf("expected (%v) to be (%v), for (%s * %s)", actual.AmountString(), e.expected.AmountString(), subject.AmountString(), e.example.String())
		}
	}

	if !subject.Equal(factory.MustNewCurrencyValue("100", "BTC")) {
		t.Errorf("expected subject to not be mutated, but was")
	}
}
