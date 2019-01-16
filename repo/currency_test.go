package repo_test

import (
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

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
