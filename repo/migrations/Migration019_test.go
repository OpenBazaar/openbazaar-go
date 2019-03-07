package migrations_test

import (
	"io/ioutil"
	"os"
	"regexp"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

const preMigration019ListingsJson = `[
{
        "hash": "zb2rhfStkSCXf79aDs78MnfLycnEr3Zd4UN3AtN45FuSbhBAW",
        "slug": "adopt-a-queen",
        "title": "Adopt a queen",
        "categories": [
            "Adoption"
        ],
        "nsfw": false,
        "contractType": "SERVICE",
        "description": "\u003cp\u003e\u003cstrong\u003eAnd what if you help a queen from one of our hives to develop her colony for the next season ?\u003c/strong\u003e\u003c/p\u003e\u003cp\u003eBy adopting a queen, you participate in",
        "thumbnail": {
            "tiny": "zb2rhdrULdmNMVZx6xNxLnr7cF4MbYUx14kPBPpWrAW9guc6J",
            "small": "zb2rhhmQKH8iNPYvBA3RV9s1YLdn16AJTo7k12Gj1wfSLFZzB",
            "medium": "zb2rhc1oSQVmNiQCFZup9B88UaxXW1hadZF7pmwCJk9UNak2e"
        },
        "price": {
            "currencyCode": "EUR",
            "amount": 2500,
            "modifier": 0
        },
        "shipsTo": [],
        "freeShipping": [],
        "language": "",
        "averageRating": 0,
        "ratingCount": 0,
        "moderators": [
            "QmXDPYhcehPoYKVPwK3b5qSmafDzrv8VLdL64W4KByWuWL",
            "QmVFNEj1rv2d3ZqSwhQZW2KT4zsext4cAMsTZRt5dAQqFJ",
            "QmeSyTRaNZMD8ajcfbhC8eYibWgnSZtSGUp3Vn59bCnPWC"
        ],
        "acceptedCurrencies": [
            "BTC"
        ],
        "coinType": ""
    }
]`

const postMigration019ListingsJson = `[
{
        "hash": "QmbJWAESqCsf4RFCqEY7jecCashj8usXiyDNfKtZCwwzGb",
        "slug": "adopt-a-queen",
        "title": "Adopt a queen",
        "categories": [
            "Adoption"
        ],
        "nsfw": false,
        "contractType": "SERVICE",
        "description": "\u003cp\u003e\u003cstrong\u003eAnd what if you help a queen from one of our hives to develop her colony for the next season ?\u003c/strong\u003e\u003c/p\u003e\u003cp\u003eBy adopting a queen, you participate in",
        "thumbnail": {
            "tiny": "zb2rhdrULdmNMVZx6xNxLnr7cF4MbYUx14kPBPpWrAW9guc6J",
            "small": "zb2rhhmQKH8iNPYvBA3RV9s1YLdn16AJTo7k12Gj1wfSLFZzB",
            "medium": "zb2rhc1oSQVmNiQCFZup9B88UaxXW1hadZF7pmwCJk9UNak2e"
        },
        "price": {
            "currencyCode": "EUR",
            "amount": 2500,
            "modifier": 0
        },
        "shipsTo": [],
        "freeShipping": [],
        "language": "",
        "averageRating": 0,
        "ratingCount": 0,
        "moderators": [
            "QmXDPYhcehPoYKVPwK3b5qSmafDzrv8VLdL64W4KByWuWL",
            "QmVFNEj1rv2d3ZqSwhQZW2KT4zsext4cAMsTZRt5dAQqFJ",
            "QmeSyTRaNZMD8ajcfbhC8eYibWgnSZtSGUp3Vn59bCnPWC"
        ],
        "acceptedCurrencies": [
            "BTC"
        ],
        "coinType": ""
    }
]`

const migration019Listing = `{}`

func TestMigration019(t *testing.T) {
	var testRepo, err = schema.NewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err = testRepo.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer testRepo.DestroySchemaDirectories()

	var (
		listingIndexPath = testRepo.DataPathJoin("root", "listings.json")
		listingPath      = testRepo.DataPathJoin("root", "listings", "adopt-a-queen.json")
		repoverPath      = testRepo.DataPathJoin("repover")
	)
	if err = ioutil.WriteFile(listingIndexPath, []byte(preMigration019ListingsJson), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	if err = ioutil.WriteFile(listingPath, []byte(migration019Listing), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	if err = ioutil.WriteFile(repoverPath, []byte("19"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	var m migrations.Migration019
	err = m.Up(testRepo.DataPath(), "", true)
	if err != nil {
		t.Fatal(err)
	}

	listingIndexBytes, err := ioutil.ReadFile(listingIndexPath)
	if err != nil {
		t.Fatal(err)
	}

	var re = regexp.MustCompile(`\s`)
	if re.ReplaceAllString(string(listingIndexBytes), "") != re.ReplaceAllString(string(postMigration019ListingsJson), "") {
		t.Logf("actual: %s, expected %s", re.ReplaceAllString(string(listingIndexBytes), ""), re.ReplaceAllString(string(postMigration019ListingsJson), ""))
		t.Fatal("incorrect post-migration config")
	}

	assertCorrectRepoVer(t, repoverPath, "20")

	err = m.Down(testRepo.DataPath(), "", true)
	if err != nil {
		t.Fatal(err)
	}

	assertCorrectRepoVer(t, repoverPath, "19")
}
