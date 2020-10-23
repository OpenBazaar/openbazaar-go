package cache_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/OpenBazaar/multiwallet/cache"
)

type testStructSubject struct {
	StringType string
	IntType    int
	TimeType   time.Time
}

func TestSettingGettingStructs(t *testing.T) {
	var (
		subject = testStructSubject{
			StringType: "teststring",
			IntType:    123456,
			TimeType:   time.Now(),
		}
		cacher = cache.NewMockCacher()
	)
	marshalledSubject, err := json.Marshal(subject)
	if err != nil {
		t.Fatal(err)
	}
	err = cacher.Set("thing1", marshalledSubject)
	if err != nil {
		t.Fatal(err)
	}

	marshalledThing, err := cacher.Get("thing1")
	if err != nil {
		t.Fatal(err)
	}

	var actual testStructSubject
	err = json.Unmarshal(marshalledThing, &actual)
	if err != nil {
		t.Fatal(err)
	}

	if subject.StringType != actual.StringType {
		t.Error("expected StringType to match but did not")
	}
	if subject.IntType != actual.IntType {
		t.Error("expected IntType to match but did not")
	}
	if !subject.TimeType.Equal(actual.TimeType) {
		t.Error("expected TimeType to match but did not")
	}
}
