package repo_test

import (
	"bytes"
	"crypto/rand"
	crypto "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	"math/big"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestListingUnmarshalJSONSignedListing(t *testing.T) {
	var examples = []string{
		"v3-physical-good",
		"v4-physical-good",
		"v4-digital-good",
		"v4-service",
		"v4-cryptocurrency",
		"v5-physical-good",
	}

	for _, e := range examples {
		var (
			fixtureBytes = factory.MustLoadListingFixture(e)
			_, err       = repo.UnmarshalJSONSignedListing(fixtureBytes)
		)
		if err != nil {
			t.Errorf("exmaple (%s): %s", e, err)
		}
	}
}

// nolint:dupl
func TestSignedListingAttributes(t *testing.T) {
	var examples = []struct {
		fixtureName                string
		expectedSchemaVersion      uint32
		expectedTitle              string
		expectedSlug               string
		expectedPrice              *repo.CurrencyValue
		expectedAcceptedCurrencies []string
		expectedCryptoDivisibility uint32
		expectedCryptoCurrencyCode string
	}{
		{
			fixtureName:           "v3-physical-good",
			expectedSchemaVersion: 3,
			expectedTitle:         "Physical Listing",
			expectedSlug:          "physical-listing",
			expectedPrice: &repo.CurrencyValue{
				Amount: big.NewInt(1235000000),
				Currency: repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("BCH"),
					Divisibility: 8,
					CurrencyType: "crypto",
				},
			},
			expectedAcceptedCurrencies: []string{"BCH"},
			expectedCryptoDivisibility: 0,
			expectedCryptoCurrencyCode: "",
		},
		{
			fixtureName:           "v4-physical-good",
			expectedSchemaVersion: 4,
			expectedTitle:         "Physical Good Listing",
			expectedSlug:          "physical-good-listing",
			expectedPrice: &repo.CurrencyValue{
				Amount: big.NewInt(12345678000),
				Currency: repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("BCH"),
					Divisibility: 8,
					CurrencyType: "crypto",
				},
			},
			expectedAcceptedCurrencies: []string{"ZEC", "LTC", "BTC", "BCH"},
			expectedCryptoDivisibility: 0,
			expectedCryptoCurrencyCode: "",
		},
		{
			fixtureName:           "v4-digital-good",
			expectedSchemaVersion: 4,
			expectedTitle:         "Digital Good Listing",
			expectedSlug:          "digital-good-listing",
			expectedPrice: &repo.CurrencyValue{
				Amount: big.NewInt(1320),
				Currency: repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("USD"),
					Divisibility: 2,
					CurrencyType: "fiat",
				},
			},
			expectedAcceptedCurrencies: []string{"ZEC"},
			expectedCryptoDivisibility: 0,
			expectedCryptoCurrencyCode: "",
		},
		{
			fixtureName:           "v4-service",
			expectedSchemaVersion: 4,
			expectedTitle:         "Service Listing",
			expectedSlug:          "service-listing",
			expectedPrice: &repo.CurrencyValue{
				Amount: big.NewInt(9877000000),
				Currency: repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("BTC"),
					Divisibility: 8,
					CurrencyType: "crypto",
				},
			},
			expectedAcceptedCurrencies: []string{"ZEC", "LTC", "BCH", "BTC"},
			expectedCryptoDivisibility: 0,
			expectedCryptoCurrencyCode: "",
		},
		{
			fixtureName:           "v4-cryptocurrency",
			expectedSchemaVersion: 4,
			expectedTitle:         "LTC-XMR",
			expectedSlug:          "ltc-xmr",
			expectedPrice: &repo.CurrencyValue{
				Amount:   big.NewInt(0),
				Currency: repo.NewUnknownCryptoDefinition("XMR", 0),
			},
			expectedAcceptedCurrencies: []string{"LTC"},
			expectedCryptoDivisibility: 8,
			expectedCryptoCurrencyCode: "XMR",
		},
		{
			fixtureName:           "v5-physical-good",
			expectedSchemaVersion: 5,
			expectedTitle:         "ETH - $1",
			expectedSlug:          "eth-1",
			expectedPrice: &repo.CurrencyValue{
				Amount: big.NewInt(100),
				Currency: repo.CurrencyDefinition{
					Code:         repo.CurrencyCode("USD"),
					Divisibility: 2,
					CurrencyType: "fiat",
				},
			},
			expectedAcceptedCurrencies: []string{"BTC", "BCH", "ZEC", "LTC", "ETH"},
			expectedCryptoDivisibility: 0,
			expectedCryptoCurrencyCode: "",
		},
	}

	for _, e := range examples {
		t.Logf("example listing (%s)", e.fixtureName)
		var (
			fixtureBytes = factory.MustLoadListingFixture(e.fixtureName)
			l, err       = repo.UnmarshalJSONSignedListing(fixtureBytes)
		)
		if err != nil {
			t.Errorf("unable to unmarshal example (%s)", e.fixtureName)
			continue
		}

		// test version
		if l.GetVersion() != e.expectedSchemaVersion {
			t.Errorf("expected to have version response (%+v), but instead was (%+v)", e.expectedSchemaVersion, l.GetVersion())
		}

		// test title
		if title := l.GetTitle(); title != e.expectedTitle {
			t.Errorf("expected to have title response (%+v), but instead was (%+v)", e.expectedTitle, title)
		}

		// test slug
		if slug := l.GetSlug(); slug != e.expectedSlug {
			t.Errorf("expected to have slug response (%+v), but instead was (%+v)", e.expectedSlug, slug)
		}

		// test price
		if price, err := l.GetPrice(); err == nil {
			if !price.Equal(e.expectedPrice) {
				t.Errorf("expected to have price response (%+v), but instead was (%+v)", e.expectedPrice, price)
			}
		} else {
			t.Errorf("get price: %s", err.Error())
		}

		// test accepted currencies
		if acceptedCurrencies := l.GetAcceptedCurrencies(); len(acceptedCurrencies) != len(e.expectedAcceptedCurrencies) {
			t.Errorf("expected to have acceptedCurrencies response (%+v), but instead was (%+v)", e.expectedAcceptedCurrencies, acceptedCurrencies)
		}

		// test crypto divisibility
		if actual := l.GetCryptoDivisibility(); actual != e.expectedCryptoDivisibility {
			t.Errorf("expected to have divisibility (%d), but was (%d)", e.expectedCryptoDivisibility, actual)
		}

		// test crypto currency code
		if actual := l.GetCryptoCurrencyCode(); actual != e.expectedCryptoCurrencyCode {
			t.Errorf("expected to have currency code (%s), but was (%s)", e.expectedCryptoCurrencyCode, actual)
		}
	}
}

func TestSignAndVerifyListing(t *testing.T) {
	testnode, err := test.NewNode()
	if err != nil {
		t.Fatalf("create test node: %v", err)
	}

	priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}

	testnode.IpfsNode.Identity = pid
	testnode.IpfsNode.PrivateKey = priv

	lpb := factory.NewListing("test")
	l, err := repo.NewListingFromProtobuf(lpb)
	if err != nil {
		t.Fatalf("create repo listing: %v", err)
	}

	sl, err := l.Sign(testnode)
	if err != nil {
		t.Fatalf("sign listing: %v", err)
	}

	// get identities from node and listing and compare
	nodeID, err := testnode.GetNodeID()
	if err != nil {
		t.Fatalf("get node id: %v", err)
	}
	nodeIdentityBytes := nodeID.GetPubkeys().GetIdentity()
	listingIdentityBytes := sl.GetListing().GetVendorID().IdentityKeyBytes()
	if !bytes.Equal(nodeIdentityBytes, listingIdentityBytes) {
		t.Fatal("expected listing and node identity bytes to match, but did not")
	}

	// get peer IDs from node and listing and compare
	listingPeerHash, err := sl.GetListing().GetVendorID().Hash()
	if err != nil {
		t.Fatalf("get listing peer hash: %v", err)
	}
	if listingPeerHash != nodeID.PeerID {
		t.Errorf("expected listing has to be (%s), but was (%s)", nodeID.PeerID, listingPeerHash)
	}

	// get listing signature and ensure it's as expected
	serializedListing, err := l.MarshalProtobuf()
	if err != nil {
		t.Fatalf("serialize listing: %v", err)
	}
	expectedSig, err := testnode.IpfsNode.PrivateKey.Sign(serializedListing)
	if err != nil {
		t.Fatalf("sign listing: %v", err)
	}
	actualSig := sl.GetSignature()
	if !bytes.Equal(expectedSig, actualSig) {
		t.Fatal("expected signature on listing to match generated signature, but did not")
	}

	if err := sl.VerifySignature(); err != nil {
		t.Errorf("verify signed listing: %v", err)
	}
}

func TestNormalize(t *testing.T) {
	testnode, err := test.NewNode()
	if err != nil {
		t.Fatalf("create test node: %v", err)
	}

	priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}

	testnode.IpfsNode.Identity = pid
	testnode.IpfsNode.PrivateKey = priv

	// v4 listing is guaranteed to mutate when normalized
	lb := factory.MustLoadListingFixture("v4-cryptocurrency")
	l, err := repo.UnmarshalJSONSignedListing(lb)
	if err != nil {
		t.Fatalf("unmarshal fixtured listing: %v", err)
	}

	// sign replaces listing vendor ID with the one from the testnode
	sl, err := l.GetListing().Sign(testnode)
	if err != nil {
		t.Fatalf("sign listing: %v", err)
	}

	origSig := sl.GetSignature()
	origListingJSON, err := sl.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal listing: %v", err)
	}

	if err := sl.Normalize(); err != nil {
		t.Fatalf("normalize listing: %v", err)
	}

	if sig := sl.GetSignature(); !bytes.Equal(origSig, sig) {
		t.Errorf("expected normalized signature to not change, but did")
	}

	if listingJSON, err := sl.MarshalJSON(); err != nil {
		t.Errorf("marshal normalized listing: %v", err)
	} else {
		// when normalizing a signed listing, the listing data mutates in place, but
		// the signature should remain matched to the original (unchanged)
		if bytes.Equal(origListingJSON, listingJSON) {
			t.Errorf("expected listing JSON to change from normalization, but did not")
			t.Logf("orig: %s\nactual: %s\n", string(origListingJSON), string(listingJSON))
		}
	}
}
