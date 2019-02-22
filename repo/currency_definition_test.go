package repo_test

import (
	"reflect"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

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
		{ // error invalid 4-char currency code
			expectErr: repo.ErrCurrencyCodeTestSymbolInvalid,
			input: repo.CurrencyDefinition{
				Code:         repo.CurrencyCode("XBTC"),
				Divisibility: 8,
				CurrencyType: repo.Crypto,
			},
		},
		{ // error invalid currency code length
			expectErr: repo.ErrCurrencyCodeLengthInvalid,
			input: repo.CurrencyDefinition{
				Code:         repo.CurrencyCode("BT"),
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
	var expected = factory.NewCurrencyDefinition()
	expected.Code = "123"
	var dict = repo.CurrencyDictionary{
		expected.Code.String(): expected,
	}

	if _, err := dict.Lookup("123"); err != nil {
		t.Errorf("expected lookup to succeed, but did not: %s", err.Error())
	}

	if _, err := dict.Lookup("FAIL"); err == nil {
		t.Errorf("expected lookup to fail, but did not")
	}
}

func TestCurrencyDictionaryValid(t *testing.T) {
	var (
		valid      = factory.NewCurrencyDefinition()
		invalidOne = factory.NewCurrencyDefinition()
		invalidTwo = factory.NewCurrencyDefinition()
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
