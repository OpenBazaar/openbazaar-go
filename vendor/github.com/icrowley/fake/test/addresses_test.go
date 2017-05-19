package test

import (
	"testing"

	"github.com/icrowley/fake"
)

func TestAddresses(t *testing.T) {
	for _, lang := range fake.GetLangs() {
		fake.SetLang(lang)

		v := fake.Continent()
		if v == "" {
			t.Errorf("Continent failed with lang %s", lang)
		}

		v = fake.Country()
		if v == "" {
			t.Errorf("Country failed with lang %s", lang)
		}

		v = fake.City()
		if v == "" {
			t.Errorf("City failed with lang %s", lang)
		}

		v = fake.State()
		if v == "" {
			t.Errorf("State failed with lang %s", lang)
		}

		v = fake.StateAbbrev()
		if v == "" && lang == "en" {
			t.Errorf("StateAbbrev failed with lang %s", lang)
		}

		v = fake.Street()
		if v == "" {
			t.Errorf("Street failed with lang %s", lang)
		}

		v = fake.StreetAddress()
		if v == "" {
			t.Errorf("StreetAddress failed with lang %s", lang)
		}

		v = fake.Zip()
		if v == "" {
			t.Errorf("Zip failed with lang %s", lang)
		}

		v = fake.Phone()
		if v == "" {
			t.Errorf("Phone failed with lang %s", lang)
		}
	}
}
