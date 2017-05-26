package test

import (
	"testing"

	"github.com/icrowley/fake"
)

func TestInternet(t *testing.T) {
	for _, lang := range fake.GetLangs() {
		fake.SetLang(lang)

		v := fake.UserName()
		if v == "" {
			t.Errorf("UserName failed with lang %s", lang)
		}

		v = fake.TopLevelDomain()
		if v == "" {
			t.Errorf("TopLevelDomain failed with lang %s", lang)
		}

		v = fake.DomainName()
		if v == "" {
			t.Errorf("DomainName failed with lang %s", lang)
		}

		v = fake.EmailAddress()
		if v == "" {
			t.Errorf("EmailAddress failed with lang %s", lang)
		}

		v = fake.EmailSubject()
		if v == "" {
			t.Errorf("EmailSubject failed with lang %s", lang)
		}

		v = fake.EmailBody()
		if v == "" {
			t.Errorf("EmailBody failed with lang %s", lang)
		}

		v = fake.DomainZone()
		if v == "" {
			t.Errorf("DomainZone failed with lang %s", lang)
		}

		v = fake.IPv4()
		if v == "" {
			t.Errorf("IPv4 failed with lang %s", lang)
		}
	}
}
