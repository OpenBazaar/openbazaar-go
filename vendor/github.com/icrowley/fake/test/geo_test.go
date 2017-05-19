package test

import (
	"testing"

	"github.com/icrowley/fake"
)

func TestGeo(t *testing.T) {
	for _, lang := range fake.GetLangs() {
		fake.SetLang(lang)

		f := fake.Latitute()
		if f == 0 {
			t.Errorf("Latitude failed with lang %s", lang)
		}

		i := fake.LatitudeDegress()
		if i < -180 || i > 180 {
			t.Errorf("LatitudeDegress failed with lang %s", lang)
		}

		i = fake.LatitudeMinutes()
		if i < 0 || i >= 60 {
			t.Errorf("LatitudeMinutes failed with lang %s", lang)
		}

		i = fake.LatitudeSeconds()
		if i < 0 || i >= 60 {
			t.Errorf("LatitudeSeconds failed with lang %s", lang)
		}

		s := fake.LatitudeDirection()
		if s != "N" && s != "S" {
			t.Errorf("LatitudeDirection failed with lang %s", lang)
		}

		f = fake.Longitude()
		if f == 0 {
			t.Errorf("Longitude failed with lang %s", lang)
		}

		i = fake.LongitudeDegrees()
		if i < -180 || i > 180 {
			t.Errorf("LongitudeDegrees failed with lang %s", lang)
		}

		i = fake.LongitudeMinutes()
		if i < 0 || i >= 60 {
			t.Errorf("LongitudeMinutes failed with lang %s", lang)
		}

		i = fake.LongitudeSeconds()
		if i < 0 || i >= 60 {
			t.Errorf("LongitudeSeconds failed with lang %s", lang)
		}

		s = fake.LongitudeDirection()
		if s != "W" && s != "E" {
			t.Errorf("LongitudeDirection failed with lang %s", lang)
		}
	}
}
