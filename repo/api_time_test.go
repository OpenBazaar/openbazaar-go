package repo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestAPITimeMarshalJSON(t *testing.T) {
	var (
		when     = time.Now()
		subject  = factory.NewAPITime(when)
		expected = []byte(when.Format(fmt.Sprintf(`"%s"`, time.RFC3339Nano)))
	)
	actual, err := json.Marshal(&subject)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, expected) {
		t.Errorf("expected (%s) to equal (%s), but did not", actual, expected)
	}
}

func TestAPITimeUnmarshalJSONSupportsRFC3339(t *testing.T) {
	var (
		when             = time.Now().Truncate(time.Second)
		format           = time.RFC3339
		marshaledExample = []byte(fmt.Sprintf(`"%s"`, when.Format(format)))

		actual repo.APITime
	)
	if err := json.Unmarshal(marshaledExample, &actual); err != nil {
		t.Logf("output: %s", string(marshaledExample))
		t.Fatal(err)
	}
	if !when.Equal(actual.Time) {
		t.Errorf("expected (%s) to equal (%s), but did not when using format (%s)", actual.Time, when, format)
	}
}

func TestAPITimeUnmarshalJSONSupportsRFC3339Nano(t *testing.T) {
	var (
		when             = time.Now().Add(1 * time.Nanosecond)
		format           = time.RFC3339Nano
		marshaledExample = []byte(fmt.Sprintf(`"%s"`, when.Format(format)))

		actual repo.APITime
	)
	if err := json.Unmarshal(marshaledExample, &actual); err != nil {
		t.Logf("output: %s", string(marshaledExample))
		t.Fatal(err)
	}
	if !when.Equal(actual.Time) {
		t.Errorf("expected (%s) to equal (%s), but did not when using format (%s)", actual.Time, when, format)
	}
}

func TestAPITimeMarshalIsReciprocal(t *testing.T) {
	var when = repo.NewAPITime(time.Now())
	subjectBytes, err := json.Marshal(&when)
	if err != nil {
		t.Fatal(err)
	}

	var actual repo.APITime
	if err := json.Unmarshal(subjectBytes, &actual); err != nil {
		t.Fatal(err)
	}

	if !when.Equal(actual.Time) {
		t.Errorf("expected (%s) to equal (%s), but did not", actual, when)
	}
}
