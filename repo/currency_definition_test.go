package repo_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestCurrencyDefinitionsAreEqual(t *testing.T) {
	validDef := factory.NewCurrencyDefinition("BTC")
	matchingDef := factory.NewCurrencyDefinition("BTC")
	differentCodeDef := factory.NewCurrencyDefinition("ETH")
	differentTypeDef := factory.NewCurrencyDefinition("BTC")
	differentTypeDef.CurrencyType = "invalid"
	differentDivisibilityDef := factory.NewCurrencyDefinition("BTC")
	differentDivisibilityDef.Divisibility = 10
	differentNameDef := factory.NewCurrencyDefinition("BTC")
	differentNameDef.Name = "Something else"
	examples := []struct {
		value    repo.CurrencyDefinition
		other    repo.CurrencyDefinition
		expected bool
	}{
		{ // currency and divisibility matching should be equal
			value:    validDef,
			other:    matchingDef,
			expected: true,
		},
		{ // different names should be true
			value:    validDef,
			other:    differentNameDef,
			expected: true,
		},
		{ // nils should be true
			value:    repo.NilCurrencyDefinition,
			other:    repo.NilCurrencyDefinition,
			expected: true,
		},
		{ // different code should be false
			value:    validDef,
			other:    differentCodeDef,
			expected: false,
		},
		{ // different divisibility should be false
			value:    validDef,
			other:    differentDivisibilityDef,
			expected: false,
		},
		{ // different type should be false
			value:    validDef,
			other:    differentTypeDef,
			expected: false,
		},
	}

	for _, c := range examples {
		if c.value.Equal(c.other) != c.expected {
			if c.expected {
				t.Errorf("expected values to be equal but was not")
			} else {
				t.Errorf("expected values to NOT be equal but was")
			}
			t.Logf("\tvalue name: %s code: %s divisibility: %d type: %s", c.value.Name, c.value.Code, c.value.Divisibility, c.value.CurrencyType)
			t.Logf("\tother name: %s code: %s divisibility: %d type: %s", c.other.Name, c.other.Code, c.other.Divisibility, c.other.CurrencyType)
		}
	}
}

func TestCurrencyDefinitionValidation(t *testing.T) {
	var examples = []struct {
		expectErr error
		input     repo.CurrencyDefinition
	}{
		{ // valid mainnet currency
			expectErr: nil,
			input: repo.CurrencyDefinition{
				Code:         repo.CurrencyCode("BTC"),
				Divisibility: 8,
				CurrencyType: repo.Crypto,
			},
		},
		{ // valid testnet currency
			expectErr: nil,
			input: repo.CurrencyDefinition{
				Code:         repo.CurrencyCode("TBTC"),
				Divisibility: 8,
				CurrencyType: repo.Crypto,
			},
		},
		{ // error empty currency code
			expectErr: repo.ErrCurrencyCodeLengthInvalid,
			input: repo.CurrencyDefinition{
				Code:         repo.CurrencyCode(""),
				Divisibility: 8,
				CurrencyType: repo.Crypto,
			},
		},
		{ // error invalid currency type
			expectErr: repo.ErrCurrencyTypeInvalid,
			input: repo.CurrencyDefinition{
				Code:         repo.CurrencyCode("123"),
				Divisibility: 1,
				CurrencyType: "invalid",
			},
		},
		{ // error non-positive divisibility
			expectErr: repo.ErrCurrencyDivisibilityNonPositive,
			input: repo.CurrencyDefinition{
				Code:         repo.CurrencyCode("234"),
				Divisibility: 0,
				CurrencyType: repo.Crypto,
			},
		},
		{ // error nil definition
			expectErr: repo.ErrCurrencyDefinitionUndefined,
			input:     repo.NilCurrencyDefinition,
		},
	}

	for _, e := range examples {
		var err = e.input.Valid()
		if e.expectErr != nil {
			if err != nil {
				if e.expectErr.Error() == err.Error() {
					continue
				}
				t.Errorf("unexpected error for input (%s): have (%s), want (%s)", e.input.String(), err.Error(), e.expectErr.Error())
				continue
			}
			t.Errorf("unexpected error for input (%s): have nil, want (%s)", e.input.String(), e.expectErr.Error())
			continue
		}
		if err != nil {
			t.Errorf("unexpected error for input (%s): have (%s), want nil", e.input.String(), err.Error())
			continue
		}
	}
}

func TestCurrencyDictionaryLookup(t *testing.T) {
	var (
		code     = "ABC"
		expected = factory.NewCurrencyDefinition(code)
		defs     = map[string]repo.CurrencyDefinition{
			expected.Code.String(): expected,
		}
	)
	dict, err := repo.NewCurrencyDictionary(defs)
	if err != nil {
		t.Fatal(err)
	}

	var examples = []struct {
		lookup      string
		expected    repo.CurrencyDefinition
		expectedErr error
	}{
		{ // upcase lookup
			lookup:      code,
			expected:    expected,
			expectedErr: nil,
		},
		{ // lowercase lookup
			lookup:      strings.ToLower(code),
			expected:    expected,
			expectedErr: nil,
		},
		{ // undefined key
			lookup:      "FAIL",
			expected:    repo.NilCurrencyDefinition,
			expectedErr: repo.ErrCurrencyDefinitionUndefined,
		},
	}

	for _, e := range examples {
		var def, err = dict.Lookup(e.lookup)
		if err != nil {
			if e.expectedErr != nil {
				if err != e.expectedErr {
					t.Errorf("expected err to be (%s), but was (%s)", e.expectedErr.Error(), err.Error())
					t.Logf("\tlookup: %s", e.lookup)
				}
				continue
			}
			t.Errorf("unexpected error: %s", err.Error())
			t.Logf("\tlookup: %s", e.lookup)
			continue
		}

		if !e.expected.Equal(def) {
			t.Errorf("expected (%s) but got (%s)", e.expected, def)
			t.Logf("\tlookup: %s", e.lookup)
		}
	}
}

func TestCurrencyDictionaryValid(t *testing.T) {
	valid := factory.NewCurrencyDefinition("BTC")
	// invalidOne is invalid because the divisibility is 0
	invalidOne := factory.NewCurrencyDefinition("LTC")
	invalidOne.Divisibility = 0
	// colliding is invalid because the code collides with BTC above
	colliding := factory.NewCurrencyDefinition("BTC")

	errOne := invalidOne.Valid()
	if errOne == nil {
		t.Fatalf("expected invalidOne to be invalid, but was not")
	}

	expectedErrs := map[string]error{
		invalidOne.CurrencyCode().String(): errOne,
		"DIF":                              repo.ErrDictionaryIndexMismatchedCode,
	}
	_, err := repo.NewCurrencyDictionary(map[string]repo.CurrencyDefinition{
		valid.CurrencyCode().String():      valid,
		colliding.CurrencyCode().String():  colliding,
		invalidOne.CurrencyCode().String(): invalidOne,
		"DIF":                              valid,
	})

	var mappedErrs map[string]error
	if err != nil {
		mappedErrs = map[string]error(err.(repo.CurrencyDictionaryProcessingError))
	}
	if !reflect.DeepEqual(expectedErrs, mappedErrs) {
		t.Logf("\texpected: %+v", expectedErrs)
		t.Logf("\tactual: %+v", mappedErrs)
		t.Fatalf("expected error map to match, but did not")
	}
}

func TestNilCodeCollision(t *testing.T) {
	subject := repo.NilCurrencyCode
	if _, err := repo.AllCurrencies().Lookup(subject.String()); err == nil {
		t.Fatal("expected nil currency lookup to error, but did not")
	}
}

func TestCurrencyDefinitionBlockTime(t *testing.T) {
	dict := repo.AllCurrencies()
	var examples = []struct {
		input                        string
		expectedBlockTime            time.Duration
		expectedConfirmationsPerHour uint32
	}{
		{
			input:                        "BTC",
			expectedBlockTime:            10 * time.Minute,
			expectedConfirmationsPerHour: 6,
		},
		{
			input:                        "TBTC",
			expectedBlockTime:            10 * time.Minute,
			expectedConfirmationsPerHour: 6,
		},
		{
			input:                        "BCH",
			expectedBlockTime:            10 * time.Minute,
			expectedConfirmationsPerHour: 6,
		},
		{
			input:                        "TBCH",
			expectedBlockTime:            10 * time.Minute,
			expectedConfirmationsPerHour: 6,
		},
		{
			input:                        "LTC",
			expectedBlockTime:            150 * time.Second,
			expectedConfirmationsPerHour: 24,
		},
		{
			input:                        "TLTC",
			expectedBlockTime:            150 * time.Second,
			expectedConfirmationsPerHour: 24,
		},
		{
			input:                        "ZEC",
			expectedBlockTime:            10 * time.Minute,
			expectedConfirmationsPerHour: 6,
		},
		{
			input:                        "TZEC",
			expectedBlockTime:            10 * time.Minute,
			expectedConfirmationsPerHour: 6,
		},
		{
			input:                        "ETH",
			expectedBlockTime:            10 * time.Second,
			expectedConfirmationsPerHour: 360,
		},
		{
			input:                        "TETH",
			expectedBlockTime:            10 * time.Second,
			expectedConfirmationsPerHour: 360,
		},
		{
			input:                        "USD",
			expectedBlockTime:            0 * time.Second,
			expectedConfirmationsPerHour: 1,
		},
	}

	for _, e := range examples {
		defn, err := dict.Lookup(e.input)
		if err != nil {
			t.Errorf("lookup should not fail for code : %s with error : %s", e.input, err.Error())
		}
		if defn.BlockTime != e.expectedBlockTime {
			t.Errorf("blocktime validation fail for code : %s, expected : %v , actual : %v",
				e.input, e.expectedBlockTime, defn.BlockTime)
		}
		if defn.ConfirmationsPerHour() != e.expectedConfirmationsPerHour {
			t.Errorf("confirmations per hour validation fail for code : %s, expected : %v , actual : %v",
				e.input, e.expectedConfirmationsPerHour, defn.BlockTime)
		}
	}
}
