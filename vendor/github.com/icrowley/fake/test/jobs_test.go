package test

import (
	"testing"

	"github.com/icrowley/fake"
)

func TestJobs(t *testing.T) {
	for _, lang := range fake.GetLangs() {
		fake.SetLang(lang)

		v := fake.Company()
		if v == "" {
			t.Errorf("Company failed with lang %s", lang)
		}

		v = fake.JobTitle()
		if v == "" {
			t.Errorf("JobTitle failed with lang %s", lang)
		}

		v = fake.Industry()
		if v == "" {
			t.Errorf("Industry failed with lang %s", lang)
		}
	}
}
