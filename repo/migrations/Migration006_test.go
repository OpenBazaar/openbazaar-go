package migrations

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/util"
)

func TestMigration006(t *testing.T) {

	// Setup
	basePath := util.GenerateTempPath()
	testRepoPath, err := util.OpenbazaarPathTransform(basePath, true)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := util.NewCustomSchemaManager(util.SchemaContext{RootPath: testRepoPath, TestModeEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(paths.RootPathJoin("datastore"), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer paths.DestroySchemaDirectories()
	var (
		databasePath = paths.DatastorePath()
		listingPath  = paths.RootPathJoin("listings.json")
		schemaPath   = paths.RootPathJoin("repover")

		schemaSql               = "PRAGMA key = 'foobarbaz';"
		configCreateSql         = "CREATE TABLE `config` (`key` text NOT NULL, `value` blob, PRIMARY KEY(`key`));"
		expectedStoreModerators = []string{
			"storemoderator_peerid_1",
			"storemoderator_peerid_2",
		}
		listingFixtures = []migration006_listingDataBeforeMigration{
			migration006_listingDataBeforeMigration{Hash: "Listing1"},
			migration006_listingDataBeforeMigration{Hash: "Listing2"},
		}
		configRecord         = migration006_configRecord{StoreModerators: expectedStoreModerators}
		insertConfigTemplate = "INSERT INTO config (key,value) VALUES ('settings','%s');"
	)

	// Create/persist settings record
	configJson, err := json.Marshal(configRecord)
	if err != nil {
		t.Fatal(err)
	}
	dbSetupSql := strings.Join([]string{
		schemaSql,
		configCreateSql,
		fmt.Sprintf(insertConfigTemplate, configJson),
	}, " ")
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(dbSetupSql)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Create listings.json file
	listingJSON, err := json.Marshal(listingFixtures)
	if err != nil {
		t.Fatal(err)
	}
	if err = ioutil.WriteFile(listingPath, listingJSON, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// Create schema version file
	if err = ioutil.WriteFile(schemaPath, []byte("6"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// Execute Migration Up
	migration := migration006{}
	if err := migration.Up(testRepoPath, "foobarbaz", true); err != nil {
		t.Fatal(err)
	}

	// Migration Up Assertions
	if err = paths.MustVerifySchemaVersion("7"); err != nil {
		t.Fatal(err)
	}

	listingJSON, err = ioutil.ReadFile(listingPath)
	if err != nil {
		t.Fatal(err)
	}

	var migratedListings = make([]migration006_listingDataAfterMigration, 2)
	err = json.Unmarshal(listingJSON, &migratedListings)
	if err != nil {
		t.Fatal(err)
	}

	if actualCount := len(migratedListings); actualCount != len(listingFixtures) {
		t.Fatalf("Unexpected number of listings returned. Actual count: %d", actualCount)
	}

	for _, listing := range migratedListings {
		actualModeratorIDs := make([]string, 0)
		for _, peerID := range listing.ModeratorIDs {
			actualModeratorIDs = append(actualModeratorIDs, peerID)
		}

		if !reflect.DeepEqual(expectedStoreModerators, actualModeratorIDs) {
			t.Fatalf("Expected moderator IDs were not equal\n\tExpected: %+v\n\tActual: %+v",
				expectedStoreModerators,
				actualModeratorIDs,
			)
		}
	}

	// Execute Migration Down
	if err = migration.Down(testRepoPath, "foobarbaz", true); err != nil {
		t.Fatal(err)
	}

	// Migration Down Assertions
	if err = paths.MustVerifySchemaVersion("6"); err != nil {
		t.Error(err)
	}

	listingJSON, err = ioutil.ReadFile(listingPath)
	if err != nil {
		t.Fatal(err)
	}

	migratedListings = make([]migration006_listingDataAfterMigration, 2)
	err = json.Unmarshal(listingJSON, &migratedListings)
	if err != nil {
		t.Fatal(err)
	}

	if actualCount := len(migratedListings); actualCount != len(listingFixtures) {
		t.Fatalf("Unexpected number of listings returned. Actual count: %d", actualCount)
	}

	for _, listing := range migratedListings {
		if len(listing.ModeratorIDs) != 0 {
			t.Fatalf("Unexpected ModeratorIDs values: %+v", listing.ModeratorIDs)
		}
	}
}
