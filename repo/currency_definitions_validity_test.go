package repo

import "testing"

func TestMainnetCurrencyDictionaryIsValid(t *testing.T) {
	if _, err := NewCurrencyDictionary(allCurrencyDefinitions); err != nil {
		var mappedErrs = map[string]error(err.(CurrencyDictionaryProcessingError))
		t.Logf("invalid currencies: %s", mappedErrs)
		t.Fatal(err)
	}
}
