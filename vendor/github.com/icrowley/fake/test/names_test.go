package test

import (
	"testing"

	"github.com/icrowley/fake"
)

func TestNames(t *testing.T) {
	for _, lang := range fake.GetLangs() {
		fake.SetLang(lang)

		v := fake.MaleFirstName()
		if v == "" {
			t.Errorf("MaleFirstName failed with lang %s", lang)
		}

		v = fake.FemaleFirstName()
		if v == "" {
			t.Errorf("FemaleFirstName failed with lang %s", lang)
		}

		v = fake.FirstName()
		if v == "" {
			t.Errorf("FirstName failed with lang %s", lang)
		}

		v = fake.MaleLastName()
		if v == "" {
			t.Errorf("MaleLastName failed with lang %s", lang)
		}

		v = fake.FemaleLastName()
		if v == "" {
			t.Errorf("FemaleLastName failed with lang %s", lang)
		}

		v = fake.LastName()
		if v == "" {
			t.Errorf("LastName failed with lang %s", lang)
		}

		v = fake.MalePatronymic()
		if v == "" {
			t.Errorf("MalePatronymic failed with lang %s", lang)
		}

		v = fake.FemalePatronymic()
		if v == "" {
			t.Errorf("FemalePatronymic failed with lang %s", lang)
		}

		v = fake.Patronymic()
		if v == "" {
			t.Errorf("Patronymic failed with lang %s", lang)
		}

		v = fake.MaleFullNameWithPrefix()
		if v == "" {
			t.Errorf("MaleFullNameWithPrefix failed with lang %s", lang)
		}

		v = fake.FemaleFullNameWithPrefix()
		if v == "" {
			t.Errorf("FemaleFullNameWithPrefix failed with lang %s", lang)
		}

		v = fake.FullNameWithPrefix()
		if v == "" {
			t.Errorf("FullNameWithPrefix failed with lang %s", lang)
		}

		v = fake.MaleFullNameWithSuffix()
		if v == "" {
			t.Errorf("MaleFullNameWithSuffix failed with lang %s", lang)
		}

		v = fake.FemaleFullNameWithSuffix()
		if v == "" {
			t.Errorf("FemaleFullNameWithSuffix failed with lang %s", lang)
		}

		v = fake.FullNameWithSuffix()
		if v == "" {
			t.Errorf("FullNameWithSuffix failed with lang %s", lang)
		}

		v = fake.MaleFullName()
		if v == "" {
			t.Errorf("MaleFullName failed with lang %s", lang)
		}

		v = fake.FemaleFullName()
		if v == "" {
			t.Errorf("FemaleFullName failed with lang %s", lang)
		}

		v = fake.FullName()
		if v == "" {
			t.Errorf("FullName failed with lang %s", lang)
		}
	}
}
