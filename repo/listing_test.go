package repo_test

import (
	"bytes"
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
			t.Errorf("expected example (%s) to have version response (%+v), but instead was (%+v)", e.fixtureName, e.expectedResponse, l.Metadata.Version)
		}
	}
}

func TestListingFromProtobuf(t *testing.T) {
	var (
		subject     = factory.NewListing("slug")
		actual, err = repo.NewListingFromProtobuf(subject)
	)
	if err != nil {
		t.Fatal(err)
	}

	if subject.Slug != actual.Slug {
		t.Errorf("expected slug to be (%s), but was (%s)", subject.Slug, actual.Slug)
	}
	if subject.TermsAndConditions != actual.TermsAndConditions {
		t.Errorf("expected terms/conditions to be (%s), but was (%s)", subject.TermsAndConditions, actual.TermsAndConditions)
	}
	if subject.RefundPolicy != actual.RefundPolicy {
		t.Errorf("expected refund policy to be (%s), but was (%s)", subject.RefundPolicy, actual.RefundPolicy)
	}
	if hash, err := actual.Vendor.Hash(); err != nil && subject.VendorID.PeerID != hash {
		t.Errorf("expected hash to be (%s), but was (%s)", subject.VendorID.PeerID, hash)
		if err != nil {
			t.Logf("hash had an error: %s", err)
		}
	}
	if !bytes.Equal(subject.VendorID.BitcoinSig, actual.Vendor.BitcoinSignature()) {
		t.Errorf("expected refund policy to be (%s), but was (%s)", subject.VendorID.BitcoinSig, actual.Vendor.BitcoinSignature())
	}
}
