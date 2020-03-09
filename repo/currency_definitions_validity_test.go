package repo

import "testing"

func TestValidDictionaries(t *testing.T) {
	if err := bootstrapCurrencyDictionaries(); err != nil {
		t.Fatal(err)
	}
}
