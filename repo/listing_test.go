package repo_test

import (
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestListingUnmarshalJSON(t *testing.T) {
	var examples = []string{
		"v3-physical-good",
		"v4-physical-good",
		"v4-digital-good",
		"v4-service",
		"v4-cryptocurrency",
	}

	for _, e := range examples {
		var (
			fixtureBytes = factory.MustLoadListingFixture(e)
			_, err       = repo.UnmarshalJSONListing(fixtureBytes)
		)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestListingVersion(t *testing.T) {
	var examples = []struct {
		fixtureName      string
		expectedResponse uint
	}{
		{
			fixtureName:      "v3-physical-good",
			expectedResponse: 3,
		},
		{
			fixtureName:      "v4-physical-good",
			expectedResponse: 4,
		},
		{
			fixtureName:      "v4-digital-good",
			expectedResponse: 4,
		},
		{
			fixtureName:      "v4-service",
			expectedResponse: 4,
		},
		{
			fixtureName:      "v4-cryptocurrency",
			expectedResponse: 4,
		},
	}

	for _, e := range examples {
		var (
			fixtureBytes = factory.MustLoadListingFixture(e.fixtureName)
			l, err       = repo.UnmarshalJSONListing(fixtureBytes)
		)
		if err != nil {
			t.Errorf("unable to unmarshal example (%s)", e.fixtureName)
			continue
		}
		if l.Metadata.Version != e.expectedResponse {
			t.Errorf("expected example (%s) to have version response (%d), but instead was (%d)", e.fixtureName, e.expectedResponse, l.Metadata.Version)
		}
	}
}
