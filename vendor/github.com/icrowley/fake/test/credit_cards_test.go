package test

import (
	"testing"

	"github.com/icrowley/fake"
)

func TestCreditCards(t *testing.T) {
	for _, lang := range fake.GetLangs() {
		fake.SetLang(lang)

		v := fake.CreditCardType()
		if v == "" {
			t.Errorf("CreditCardType failed with lang %s", lang)
		}

		v = fake.CreditCardNum("")
		if v == "" {
			t.Errorf("CreditCardNum failed with lang %s", lang)
		}

		v = fake.CreditCardNum("visa")
		if v == "" {
			t.Errorf("CreditCardNum failed with lang %s", lang)
		}
	}
}
