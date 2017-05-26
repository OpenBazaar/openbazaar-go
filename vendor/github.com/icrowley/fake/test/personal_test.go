package test

import (
	"testing"

	"github.com/icrowley/fake"
)

func TestPersonal(t *testing.T) {
	for _, lang := range fake.GetLangs() {
		fake.SetLang(lang)

		v := fake.Gender()
		if v == "" {
			t.Errorf("Gender failed with lang %s", lang)
		}

		v = fake.GenderAbbrev()
		if v == "" {
			t.Errorf("GenderAbbrev failed with lang %s", lang)
		}

		v = fake.Language()
		if v == "" {
			t.Errorf("Language failed with lang %s", lang)
		}
	}
}
