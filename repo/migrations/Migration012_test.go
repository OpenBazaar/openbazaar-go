package migrations_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"

	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/golang/protobuf/proto"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

const (
	testMigration012_PeerID                   = "QmeGAo1o1CoufuoRtRb2FCqi2UhMV7m83KT3xB6MKrTvqx"
	testMigration012_IdentityPrivateKeyBase64 = "CAESYHwrVuRp5s2u0w5ykibsR77aHWBmvpcaDq+vU9pv8lOqae31NJYJbdDsOlxVRqQZS/eDfssdd7N/rJmoVbQvPytp7fU0lglt0Ow6XFVGpBlL94N+yx13s3+smahVtC8/Kw=="
)

var (
	testMigration012_IdentityPubkeyBytes = []byte{8, 1, 18, 32, 105, 237, 245, 52, 150, 9, 109, 208, 236, 58, 92, 85, 70, 164, 25, 75, 247, 131, 126, 203, 29, 119, 179, 127, 172, 153, 168, 85, 180, 47, 63, 43}

	testMigration012_datadir          = path.Join("/", "tmp", "openbazaar-test")
	testMigraion012_listingsIndexPath = testMigration012_filepath("root", "listings.json")
)

func TestMigration012_listingHasNewFeaturesAndOldVersion(t *testing.T) {
	signedListing := &pb.SignedListing{Listing: &pb.Listing{Metadata: &pb.Listing_Metadata{}}}

	for i, test := range []struct {
		version  uint32
		modifier float32
		expected bool
	}{
		// Old version; should be true if and only if modifier is not 0
		{3, 0, false},
		{3, 0.001, true},
		{3, -0.001, true},

		// New version; should all be false
		{4, 0, false},
		{4, 0.001, false},
		{4, -0.001, false},
	} {
		signedListing.Listing.Metadata.Version = test.version
		signedListing.Listing.Metadata.PriceModifier = test.modifier
		hasNewFeatures := migrations.Migration012_listingHasNewFeaturesAndOldVersion(signedListing)
		if hasNewFeatures != test.expected {
			t.Fatal("Test", i, "failed\nExpected:", test.expected, "\nGot:", hasNewFeatures)
		}
	}
}

func TestMigration012_GetIdentityKey(t *testing.T) {
	defer testMigration012_Setup(t)()

	identityKey, err := migrations.Migration012_GetIdentityKey(testMigration012_datadir, "letmein", true)
	if err != nil {
		t.Fatal(err)
	}

	identityKeyBase64 := base64.StdEncoding.EncodeToString(identityKey)
	if identityKeyBase64 != testMigration012_IdentityPrivateKeyBase64 {
		t.Fatal("Incorect key:", identityKeyBase64)
	}
}

func TestMigration012(t *testing.T) {
	defer testMigration012_Setup(t)()

	// Test Up migration
	var m migrations.Migration012
	err := m.Up(testMigration012_datadir, "letmein", true)
	if err != nil {
		t.Fatal(err)
	}

	repoVer, err := ioutil.ReadFile(testMigration012_filepath("repover"))
	if err != nil {
		t.Fatal(err)
	}
	if string(repoVer) != "13" {
		t.Fatal("Failed to write new repo version")
	}

	testMigration012_assertListingIndexMigratedCorrectly(t)
	testMigration012_assertListingsMigratedCorrectly(t)

	// Test Down migrations
	err = m.Down(testMigration012_datadir, "letmein", true)
	if err != nil {
		t.Fatal(err)
	}
	repoVer, err = ioutil.ReadFile(testMigration012_filepath("repover"))
	if err != nil {
		t.Fatal(err)
	}
	if string(repoVer) != "12" {
		t.Fatal("Failed to write new repo version")
	}
}

func testMigration012_Setup(t *testing.T) func() {
	err := os.RemoveAll(testMigration012_filepath())
	if err != nil {
		t.Fatal(err)
	}

	for _, dir := range []string{
		testMigration012_filepath("blocks"),
		testMigration012_filepath("datastore"),
		testMigration012_filepath("root", "listings"),
	} {
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			os.RemoveAll(testMigration012_filepath())
			t.Fatal(err)
		}
	}

	db, err := migrations.OpenDB(testMigration012_datadir, "letmein", true)
	if err != nil {
		t.Error(err)
	}

	identityKey, err := base64.StdEncoding.DecodeString(testMigration012_IdentityPrivateKeyBase64)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec("CREATE TABLE config (key text primary key not null, value blob);")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO config(key,value) VALUES('identityKey', ?)", identityKey)
	if err != nil {
		t.Fatal(err)
	}

	if err = ioutil.WriteFile(testMigration012_filepath("config"), []byte(testMigration012_configFixture), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err = ioutil.WriteFile(testMigration012_filepath("datastore_spec"), []byte(testMigraion012_datastore_specFixture), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err = ioutil.WriteFile(testMigration012_filepath("version"), []byte(fmt.Sprintf("%d\n", fsrepo.RepoVersion)), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err = ioutil.WriteFile(testMigraion012_listingsIndexPath, []byte(testMigration012_listingIndexFixture), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// Create listing files for each of the crypto listings in index
	for _, listing := range testMigraion012_listingFixtures {
		listingJSON, err := json.Marshal(listing)
		if err != nil {
			t.Fatal(err)
		}
		if err = ioutil.WriteFile(testMigration012_filepath("root", "listings", listing.Listing.Slug+".json"), listingJSON, os.ModePerm); err != nil {
			t.Fatal(err)
		}

	}

	return func() {
		os.RemoveAll(testMigration012_filepath())
	}
}

func testMigration012_filepath(fragments ...string) string {
	fragments = append([]string{testMigration012_datadir}, fragments...)
	return path.Join(fragments...)
}

func testMigration012_assertListingsMigratedCorrectly(t *testing.T) {
	for _, listing := range testMigraion012_listingFixtures {
		testMigration012_assertListingMigratedCorrectly(t, listing)
	}
}

func testMigration012_assertListingMigratedCorrectly(t *testing.T, listingBeforeMigration pb.SignedListing) {
	listingPath := testMigration012_filepath("root", "listings", listingBeforeMigration.Listing.Slug+".json")
	listingJSON, err := ioutil.ReadFile(listingPath)
	if err != nil {
		t.Fatal(err)
	}
	sl := new(pb.SignedListing)
	err = jsonpb.UnmarshalString(string(listingJSON), sl)
	if err != nil {
		t.Fatal(err)
	}

	// Check version
	if sl.Listing.Metadata.Version != 4 {
		t.Fatal("Incorrect version.\nWanted:", 4, "\nGot:", sl.Listing.Metadata.Version)
	}

	// We're done checking if the listing wasn't migrated
	if !migrations.Migration012_listingHasNewFeaturesAndOldVersion(&listingBeforeMigration) {
		return
	}

	// Check signature
	err = testMigration012_verifySignature(
		sl.Listing,
		sl.Listing.VendorID.Pubkeys.Identity,
		sl.Signature,
		sl.Listing.VendorID.PeerID,
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedListing := listingBeforeMigration.Listing
	actualListing := sl.Listing

	// check other data is the same
	if expectedListing.Slug != actualListing.Slug {
		t.Fatal("Expected:", expectedListing.Slug, "\nGot:", actualListing.Slug)
	}
	if expectedListing.RefundPolicy != actualListing.RefundPolicy {
		t.Fatal("Expected:", expectedListing.RefundPolicy, "\nGot:", actualListing.RefundPolicy)
	}
	if expectedListing.TermsAndConditions != actualListing.TermsAndConditions {
		t.Fatal("Expected:", expectedListing.TermsAndConditions, "\nGot:", actualListing.TermsAndConditions)
	}
	if strings.Join(expectedListing.Moderators, ",") != strings.Join(actualListing.Moderators, ",") {
		t.Fatal("Expected:", strings.Join(expectedListing.Moderators, ","), "\nGot:", strings.Join(actualListing.Moderators, ","))
	}

	if expectedListing.Item.Title != actualListing.Item.Title {
		t.Fatal("Expected:", expectedListing.Item.Title, "\nGot:", actualListing.Item.Title)
	}
	if expectedListing.Item.Description != actualListing.Item.Description {
		t.Fatal("Expected:", expectedListing.Item.Description, "\nGot:", actualListing.Item.Description)
	}
	if expectedListing.Item.Price != actualListing.Item.Price {
		t.Fatal("Expected:", expectedListing.Item.Price, "\nGot:", actualListing.Item.Price)
	}
	if expectedListing.Item.Grams != actualListing.Item.Grams {
		t.Fatal("Expected:", expectedListing.Item.Grams, "\nGot:", actualListing.Item.Grams)
	}
	if strings.Join(expectedListing.Item.Tags, ",") != strings.Join(actualListing.Item.Tags, ",") {
		t.Fatal("Expected:", strings.Join(expectedListing.Item.Tags, ","), "\nGot:", strings.Join(actualListing.Item.Tags, ","))
	}
	if strings.Join(expectedListing.Item.Categories, ",") != strings.Join(actualListing.Item.Categories, ",") {
		t.Fatal("Expected:", strings.Join(expectedListing.Item.Categories, ","), "\nGot:", strings.Join(actualListing.Item.Categories, ","))
	}

	if expectedListing.Metadata.PriceModifier != actualListing.Metadata.PriceModifier {
		t.Fatal("Expected:", expectedListing.Metadata.PriceModifier, "\nGot:", actualListing.Metadata.PriceModifier)
	}
	if expectedListing.Metadata.ContractType != actualListing.Metadata.ContractType {
		t.Fatal("Expected:", expectedListing.Metadata.ContractType, "\nGot:", actualListing.Metadata.ContractType)
	}
	if expectedListing.Metadata.Format != actualListing.Metadata.Format {
		t.Fatal("Expected:", expectedListing.Metadata.Format, "\nGot:", actualListing.Metadata.Format)
	}
	if strings.Join(expectedListing.Metadata.AcceptedCurrencies, ",") != strings.Join(actualListing.Metadata.AcceptedCurrencies, ",") {
		t.Fatal("Expected:", strings.Join(expectedListing.Metadata.AcceptedCurrencies, ","), "\nGot:", strings.Join(actualListing.Metadata.AcceptedCurrencies, ","))
	}
}

func testMigration012_verifySignature(msg proto.Message, pk []byte, signature []byte, peerID string) error {
	ser, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	pubkey, err := crypto.UnmarshalPublicKey(pk)
	if err != nil {
		return err
	}
	valid, err := pubkey.Verify(ser, signature)
	if err != nil {
		return err
	}
	if !valid {
		return errors.New("Invalid signature")
	}
	pid, err := peer.IDB58Decode(peerID)
	if err != nil {
		return err
	}

	if !pid.MatchesPublicKey(pubkey) {
		return errors.New("Pubkey does not match peer ID")
	}
	return nil
}

func testMigration012_assertListingIndexMigratedCorrectly(t *testing.T) {
	listingsIndexJSON, err := ioutil.ReadFile(testMigraion012_listingsIndexPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(listingsIndexJSON) == testMigration012_listingIndexFixture {
		t.Fatal("listings.json should have different content")
	}

	var (
		whitespaceRegex   = regexp.MustCompile(`\s`)
		hashPropertyRegex = regexp.MustCompile(`"hash":"[a-zA-Z0-9]*",`)
	)

	listingsIndexJSONStr := whitespaceRegex.ReplaceAllString(string(listingsIndexJSON), "")
	listingsIndexJSONStr = hashPropertyRegex.ReplaceAllString(listingsIndexJSONStr, "")
	expectedListingsIndexJSONStr := whitespaceRegex.ReplaceAllString(testMigration012_listingIndexFixture, "")
	expectedListingsIndexJSONStr = hashPropertyRegex.ReplaceAllString(expectedListingsIndexJSONStr, "")

	if listingsIndexJSONStr != expectedListingsIndexJSONStr {
		t.Fatal("listings.json should have the same content without hashes")
	}

	// Check that correct listing's hashes changed
	listingsIndex := []*migrations.Migration012_ListingData{}
	err = json.Unmarshal(listingsIndexJSON, &listingsIndex)
	if err != nil {
		t.Fatal(err)
	}

	for _, listingAbstract := range listingsIndex {
		expectedHash := testMigraion012_listingFixtureHashes[listingAbstract.Slug]
		if listingAbstract.Hash != expectedHash {
			t.Fatal("Incorrect hash. Wanted: '" + expectedHash + "'\nGot: '" + listingAbstract.Hash + "'")
		}

		if listingAbstract.Price.Modifier != 0 && listingAbstract.Hash == "" {
			t.Fatal("Cryptocurrency listing with price modifier should have a new hash")
		}
	}
}

var testMigration012_listingIndexFixture = `[
    {
        "slug": "slug-1",
        "title": "Listing1",
            "categories": [
            "category-1"
        ],
        "nsfw": false,
        "contractType": "PHYSICAL_GOOD",
        "description": "test",
        "thumbnail": {
            "tiny": "a",
            "small": "b",
            "medium": "c"
        },
        "price": {
            "currencyCode": "BCH",
            "amount": 10,
            "modifier": 0
        },
        "shipsTo": [
            "US"
        ],
        "freeShipping": [
            "US"
        ],
        "language": "en",
        "averageRating": 5,
        "ratingCount": 999,
        "moderators": null,
        "acceptedCurrencies": [
            "TBTC"
        ],
        "coinType": "BTC"
    },
    {
        "slug": "slug-2",
        "title": "Listing2",
            "categories": [
            "category-1",
            "category-2"
        ],
        "nsfw": true,
        "contractType": "PHYSICAL_GOOD",
        "description": "test",
        "thumbnail": {
            "tiny": "a",
            "small": "b",
            "medium": "c"
        },
        "price": {
            "currencyCode": "BCH",
            "amount": 10,
            "modifier": 0
        },
        "shipsTo": [
            "US"
        ],
        "freeShipping": [
            "US"
        ],
        "language": "en",
        "averageRating": 5,
        "ratingCount": 999,
        "moderators": null,
        "acceptedCurrencies": [
            "TBTC"
        ],
        "coinType": "BTC"
    },
    {
        "slug": "slug-3",
        "title": "Listing3",
            "categories": [
            "category-1",
            "category-2"
        ],
        "nsfw": true,
        "contractType": "CRYPTOCURRENCY",
        "description": "test",
        "thumbnail": {
            "tiny": "a",
            "small": "b",
            "medium": "c"
        },
        "price": {
            "currencyCode": "BCH",
            "amount": 10,
            "modifier": 0
        },
        "shipsTo": [
            "US"
        ],
        "freeShipping": [
            "US"
        ],
        "language": "en",
        "averageRating": 5,
        "ratingCount": 999,
        "moderators": null,
        "acceptedCurrencies": [
            "TBTC"
        ],
        "coinType": "BTC"
    },
    {
        "slug": "slug-4",
        "title": "Listing4",
            "categories": [
            "category-1",
            "category-2"
        ],
        "nsfw": true,
        "contractType": "CRYPTOCURRENCY",
        "description": "test",
        "thumbnail": {
            "tiny": "a",
            "small": "b",
            "medium": "c"
        },
        "price": {
            "currencyCode": "BCH",
            "amount": 10,
            "modifier": 1
        },
        "shipsTo": [
            "US"
        ],
        "freeShipping": [
            "US"
        ],
        "language": "en",
        "averageRating": 5,
        "ratingCount": 999,
        "moderators": null,
        "acceptedCurrencies": [
            "TBTC"
        ],
        "coinType": "BTC"
    },
    {
        "slug": "slug-5",
        "title": "Listing5",
            "categories": [
            "category-1",
            "category-2"
        ],
        "nsfw": true,
        "contractType": "CRYPTOCURRENCY",
        "description": "test",
        "thumbnail": {
            "tiny": "a",
            "small": "b",
            "medium": "c"
        },
        "price": {
            "currencyCode": "BCH",
            "amount": 10,
            "modifier": -1
        },
        "shipsTo": [
            "US"
        ],
        "freeShipping": [
            "US"
        ],
        "language": "en",
        "averageRating": 5,
        "ratingCount": 999,
        "moderators": [
            "a"
        ],
        "acceptedCurrencies": [
            "TBTC"
        ],
        "coinType": "BTC"
    }
]`

var testMigraion012_vendorIDFixture = &pb.ID{
	PeerID: testMigration012_PeerID,
	Pubkeys: &pb.ID_Pubkeys{
		Identity: testMigration012_IdentityPubkeyBytes,
	},
}

var testMigraion012_listingFixtures = []pb.SignedListing{
	// Listings that shouldn't change at all
	{Listing: &pb.Listing{
		Slug:     "slug-1",
		VendorID: testMigraion012_vendorIDFixture,
		Metadata: &pb.Listing_Metadata{
			Version:      4,
			ContractType: pb.Listing_Metadata_PHYSICAL_GOOD,
			Format:       pb.Listing_Metadata_FIXED_PRICE,
		},
	}},
	{Listing: &pb.Listing{
		Slug:     "slug-2",
		VendorID: testMigraion012_vendorIDFixture,
		Metadata: &pb.Listing_Metadata{
			Version:      4,
			ContractType: pb.Listing_Metadata_PHYSICAL_GOOD,
			Format:       pb.Listing_Metadata_FIXED_PRICE,
		},
	}},
	{Listing: &pb.Listing{
		Slug:     "slug-3",
		VendorID: testMigraion012_vendorIDFixture,
		Metadata: &pb.Listing_Metadata{
			Version:       4,
			PriceModifier: 0,
			ContractType:  pb.Listing_Metadata_CRYPTOCURRENCY,
			Format:        pb.Listing_Metadata_MARKET_PRICE,
		},
	}},

	// These listings should have their versions changed
	{Listing: &pb.Listing{
		Slug:     "slug-4",
		VendorID: testMigraion012_vendorIDFixture,
		Metadata: &pb.Listing_Metadata{
			Version:            3,
			PriceModifier:      1,
			ContractType:       pb.Listing_Metadata_CRYPTOCURRENCY,
			Format:             pb.Listing_Metadata_MARKET_PRICE,
			AcceptedCurrencies: []string{"BTC", "BCH"},
		},
		Item: &pb.Listing_Item{
			Title:       "Title 4",
			Description: "test",
			Price:       999,
			Tags:        []string{"tag1", "tag2"},
			Categories:  []string{"cat1", "cat2"},
			Grams:       28,
		},
		Moderators:         []string{"a", "b"},
		TermsAndConditions: "T&C",
		RefundPolicy:       "refund policy",
	}},
	{Listing: &pb.Listing{
		Slug:     "slug-5",
		VendorID: testMigraion012_vendorIDFixture,
		Metadata: &pb.Listing_Metadata{
			Version:            3,
			PriceModifier:      -1,
			ContractType:       pb.Listing_Metadata_CRYPTOCURRENCY,
			Format:             pb.Listing_Metadata_MARKET_PRICE,
			AcceptedCurrencies: []string{"BTC", "BCH"},
		},
		Item: &pb.Listing_Item{
			Title:       "Title 5",
			Description: "test",
			Price:       999,
			Tags:        []string{"tag1", "tag2"},
			Categories:  []string{"cat1", "cat2"},
			Grams:       28,
		},
		Moderators:         []string{"c", "d", "e"},
		TermsAndConditions: "T&C",
		RefundPolicy:       "refund policy",
	}},
}

var testMigraion012_listingFixtureHashes = map[string]string{
	"slug-4": "zb2rhYFPk5iVCTJYFoGR5gEpzKodhDWu5jESE2yzvWrCou54n",
	"slug-5": "zb2rhbNjVXhbtKkSXbf6hpGUV2CujPTBx9jWsRJpvzKdgpwj9",
}

var testMigration012_configFixture = `{
    "Addresses": {
        "Swarm": [
        "/ip4/127.0.0.1/tcp/4001"
        ]
    },
    "Datastore": {
        "BloomFilterSize": 0,
        "GCPeriod": "1h",
        "HashOnRead": false,
        "Spec": {
        "mounts": [
            {
            "child": {
                "path": "blocks",
                "shardFunc": "/repo/flatfs/shard/v1/next-to-last/2",
                "sync": true,
                "type": "flatfs"
            },
            "mountpoint": "/blocks",
            "prefix": "flatfs.datastore",
            "type": "measure"
            },
            {
            "child": {
                "compression": "none",
                "path": "datastore",
                "type": "levelds"
            },
            "mountpoint": "/",
            "prefix": "leveldb.datastore",
            "type": "measure"
            }
        ],
        "type": "mount"
        },
        "StorageGCWatermark": 90,
        "StorageMax": "10GB"
    }
}`

var testMigraion012_datastore_specFixture = `{"mounts":[{"mountpoint":"/blocks","path":"blocks","shardFunc":"/repo/flatfs/shard/v1/next-to-last/2","type":"flatfs"},{"mountpoint":"/","path":"datastore","type":"levelds"}],"type":"mount"}`
