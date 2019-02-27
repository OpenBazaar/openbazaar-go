package repo_test

import (
	"reflect"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestCurrencyDefinitionsAreEqual(t *testing.T) {
	var (
		validDef                 = factory.NewCurrencyDefinition("BTC")
		matchingDef              = factory.NewCurrencyDefinition("BTC")
		differentCodeDef         = factory.NewCurrencyDefinition("ETH")
		differentTypeDef         = factory.NewCurrencyDefinition("BTC")
		differentDivisibilityDef = factory.NewCurrencyDefinition("BTC")
		differentNameDef         = factory.NewCurrencyDefinition("BTC")
		examples                 = []struct {
			value    *repo.CurrencyDefinition
			other    *repo.CurrencyDefinition
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
			{ // nils should be false
				value:    nil,
				other:    nil,
				expected: false,
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
	)
	differentDivisibilityDef.Divisibility = 10
	differentNameDef.Name = "Something else"
	differentTypeDef.CurrencyType = "invalid"

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
		input     *repo.CurrencyDefinition
	}{
		{ // valid mainnet currency
			expectErr: nil,
			input: &repo.CurrencyDefinition{
				Code:         repo.CurrencyCode("BTC"),
				Divisibility: 8,
				CurrencyType: repo.Crypto,
			},
		},
		{ // valid testnet currency
			expectErr: nil,
			input: &repo.CurrencyDefinition{
				Code:         repo.CurrencyCode("TBTC"),
				Divisibility: 8,
				CurrencyType: repo.Crypto,
			},
		},
		{ // error invalid 4-char currency code
			expectErr: repo.ErrCurrencyCodeTestSymbolInvalid,
			input: &repo.CurrencyDefinition{
				Code:         repo.CurrencyCode("XBTC"),
				Divisibility: 8,
				CurrencyType: repo.Crypto,
			},
		},
		{ // error invalid currency code length
			expectErr: repo.ErrCurrencyCodeLengthInvalid,
			input: &repo.CurrencyDefinition{
				Code:         repo.CurrencyCode("BT"),
				Divisibility: 8,
				CurrencyType: repo.Crypto,
			},
		},
		{ // error empty currency code
			expectErr: repo.ErrCurrencyCodeLengthInvalid,
			input: &repo.CurrencyDefinition{
				Code:         repo.CurrencyCode(""),
				Divisibility: 8,
				CurrencyType: repo.Crypto,
			},
		},
		{ // error invalid currency type
			expectErr: repo.ErrCurrencyTypeInvalid,
			input: &repo.CurrencyDefinition{
				Code:         repo.CurrencyCode("123"),
				Divisibility: 1,
				CurrencyType: "invalid",
			},
		},
		{ // error non-positive divisibility
			expectErr: repo.ErrCurrencyDivisibilityNonPositive,
			input: &repo.CurrencyDefinition{
				Code:         repo.CurrencyCode("234"),
				Divisibility: 0,
				CurrencyType: repo.Crypto,
			},
		},
		{ // error nil definition
			expectErr: repo.ErrCurrencyDefinitionUndefined,
			input:     nil,
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
		expected = factory.NewCurrencyDefinition("ABC")
		dict     = repo.CurrencyDictionary{
			expected.Code.String(): expected,
		}

		examples = []struct {
			lookup      string
			expected    *repo.CurrencyDefinition
			expectedErr error
		}{
			{ // upcase lookup
				lookup:      "ABC",
				expected:    expected,
				expectedErr: nil,
			},
			{ // lowercase lookup
				lookup:      "abc",
				expected:    expected,
				expectedErr: nil,
			},
			{ // testnet lookup
				lookup:      "TABC",
				expected:    factory.NewCurrencyDefinition("TABC"),
				expectedErr: nil,
			},
			{ // undefined key
				lookup:      "FAIL",
				expected:    nil,
				expectedErr: repo.ErrCurrencyDefinitionUndefined,
			},
		}
	)

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
	var (
		valid      = factory.NewCurrencyDefinition("BTC")
		invalidOne = factory.NewCurrencyDefinition("LTC")
		invalidTwo = factory.NewCurrencyDefinition("BCH")
	)
	invalidOne.Divisibility = 0
	invalidTwo.Code = "X"

	errOne := invalidOne.Valid()
	if errOne == nil {
		t.Fatalf("expected invalidOne to be invalid, but was not")
	}
	errTwo := invalidTwo.Valid()
	if errOne == nil {
		t.Fatalf("expected invalidTwo to be invalid, but was not")
	}

	expectedErrs := map[string]error{
		invalidOne.Code.String(): errOne,
		invalidTwo.Code.String(): errTwo,
		"DIF": repo.ErrDictionaryIndexMismatchedCode,
	}
	_, err := repo.NewCurrencyDictionary(map[string]*repo.CurrencyDefinition{
		valid.Code.String():      valid,
		invalidOne.Code.String(): invalidOne,
		invalidTwo.Code.String(): invalidTwo,
		"DIF": valid,
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
